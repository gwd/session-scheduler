package event

import (
	"math/rand"
	"testing"
	"time"
)

func testUnitSchedule(t *testing.T) (exit bool) {
	// Any "early" exit is a failure
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	//
	// SETUP: Make some users, some discussions, a timetable, and some locations
	//
	t.Logf("Setting up users, discussion, timetable")
	testUserCount := 10
	users := make([]User, testUserCount)
	for i := range users {
		subexit := false
		users[i], subexit = testNewUser(t)
		if subexit {
			return
		}
	}

	testDiscussionCount := 12
	discussions := make([]Discussion, testDiscussionCount)

	for i := range discussions {
		subexit := false
		uidx := rand.Int31n(int32(len(users)))
		discussions[i], subexit = testNewDiscussion(t, users[uidx].UserID)
		if subexit {
			return
		}
	}

	testLocationCount := 3
	locations := make([]Location, testLocationCount)
	for i := range locations {
		subexit := false
		locations[i], subexit = testNewLocation(t)
		if subexit {
			return
		}
	}

	tt := Timetable{
		Days: []TimetableDay{
			{DayName: "Monday", Slots: []TimetableSlot{
				{Time: Date(2020, 7, 6, 14, 30, 0, 0, time.UTC)},
				{Time: Date(2020, 7, 6, 15, 15, 0, 0, time.UTC)},
				{Time: Date(2020, 7, 6, 16, 00, 0, 0, time.UTC), IsBreak: true},
				{Time: Date(2020, 7, 6, 16, 30, 0, 0, time.UTC)},
			}},
			{DayName: "Tuesday", Slots: []TimetableSlot{
				{Time: Date(2020, 7, 7, 14, 30, 0, 0, time.UTC)},
				{Time: Date(2020, 7, 7, 15, 15, 0, 0, time.UTC)},
				{Time: Date(2020, 7, 7, 16, 00, 0, 0, time.UTC), IsBreak: true},
				{Time: Date(2020, 7, 7, 16, 30, 0, 0, time.UTC)},
			}},
		},
	}
	totalSlots := 6

	err := TimetableSet(&tt)
	if err != nil {
		t.Errorf("ERROR Basic TimetableSet: %v", err)
		return
	}

	// Make 50% interest.
	interestMap := make(map[DiscussionID]map[UserID]int)
	for i := range discussions {
		did := discussions[i].DiscussionID
		interestMap[did] = make(map[UserID]int)
		interestMap[did][discussions[i].Owner] = 100
	}

	for i := 0; i < (len(users)*len(discussions))/2; i++ {
		uidx := rand.Int31n(int32(len(users)))
		uid := users[uidx].UserID
		didx := rand.Int31n(int32(len(discussions)))
		did := discussions[didx].DiscussionID
		interest := int(rand.Int31n(101))
		err = users[uidx].SetInterest(&discussions[didx], interest)
		if err != nil {
			t.Errorf("Setting interest: %v", err)
			return
		}
		if interest > 0 {
			interestMap[did][uid] = interest
		} else {
			delete(interestMap[did], uid)
		}
	}

	// Restrict discussion 0 to Monday
	{
		t.Logf("Restricting Discussion[0] (did %v) to Tuesday", discussions[0].DiscussionID)
		gotdisc, err := DiscussionFindByIdFull(discussions[0].DiscussionID)
		if err != nil {
			t.Errorf("Finding discussion 0 by id: %v", err)
			return
		}
		err = DiscussionSetPossibleSlots(discussions[0].DiscussionID, CheckedToSlotList(gotdisc.PossibleSlots)[3:])
	}

	//
	// TESTING
	//

	//
	// Set at least one discussion non-public and make sure it fails
	//
	err = DiscussionSetPublic(discussions[0].DiscussionID, false)
	if err != nil {
		t.Errorf("Setting discussion 0 non-public: %v", err)
		return
	}
	_, err = makeSnapshot()
	if err == nil {
		t.Errorf("Snapshot unexpectedly succeeded with non-public discussion!")
		return
	}

	// Make all discussions public
	for i := range discussions {
		err = DiscussionSetPublic(discussions[i].DiscussionID, true)
		if err != nil {
			t.Errorf("Setting discussion public: %v", err)
			return
		}
	}

	//
	// Take a snapshot, make sure it has what we expect
	//
	store, err := makeSnapshot()
	if err != nil {
		t.Errorf("Getting snapshot: %v", err)
		return
	}

	if len(store.Slots) != totalSlots {
		t.Errorf("Snapshot: Wanted %d slots, got %d", totalSlots, len(store.Slots))
		return
	}

	// Account for admin user
	if len(store.Users) != len(users)+1 {
		t.Errorf("Snapshot: Wanted %d users, got %d", len(users)+1, len(store.Users))
		return
	}

	if len(store.Discussions) != len(discussions) {
		t.Errorf("Snapshot: Wanted %d discussions, got %d", len(discussions), len(store.Discussions))
		return
	}

	// Check to make sure discussion possible slots make sense
	for i := range store.Discussions {
		sd := &store.Discussions[i]
		expectedSlots := 6
		if sd.DiscussionID == discussions[0].DiscussionID {
			expectedSlots = 3
		}
		if len(sd.PossibleSlots) != expectedSlots {
			t.Errorf("Snapshot.Discussions[%d] (did %v): Wanted %d possible slots, got %d",
				i, sd.DiscussionID, expectedSlots, len(sd.PossibleSlots))
			return
		}

		if len(interestMap[sd.DiscussionID]) != len(sd.UserInterest) {
			t.Errorf("Snapshot.Discussions[%d] (did %v): Wanted %d interests, got %d",
				i, sd.DiscussionID, len(interestMap[sd.DiscussionID]), len(sd.UserInterest))
		}

		for _, ui := range sd.UserInterest {
			if interestMap[sd.DiscussionID][ui.UserID] != ui.Interest {
				t.Errorf("Unexpected interest")
				return
			}
		}
	}

	//
	// Try setting and locking some of the schedule
	//
	var sched schedule
	// No restrictions on discussion 1, so this should get us all slots
	gotdisc, err := DiscussionFindByIdFull(discussions[1].DiscussionID)
	if err != nil {
		t.Errorf("Getting discussion 1: %v", err)
		return
	}
	for i := range gotdisc.PossibleSlots {
		sched.Slots = append(sched.Slots, scheduleSlot{SlotID: gotdisc.PossibleSlots[i].SlotID})
	}

	// NB this ignores possibleslots.
	slotidx := 0
	for didx := range discussions {
		ss := &sched.Slots[slotidx]
		ss.Discussions = append(ss.Discussions, &searchDiscussion{DiscussionID: discussions[didx].DiscussionID})
		slotidx = (slotidx + 1) % len(sched.Slots)
	}
	err = scheduleSet(&sched)
	if err != nil {
		t.Errorf("Setting schedule: %v", err)
		return
	}

	err = TimetableSetLockedSlots(CheckedToSlotList(gotdisc.PossibleSlots[:3]))
	if err != nil {
		t.Errorf("Locking slots: %v", err)
		return
	}
	unlockedSlots := totalSlots - len(gotdisc.PossibleSlots[:3])

	store, err = makeSnapshot()
	if err != nil {
		t.Errorf("Getting snapshot: %v", err)
		return
	}

	if len(store.Slots) != unlockedSlots {
		t.Errorf("Snapshot: Wanted %d slots, got %d", totalSlots, len(store.Slots))
		return
	}

	// Account for admin user
	if len(store.Users) != len(users)+1 {
		t.Errorf("Snapshot: Wanted %d users, got %d", len(users)+1, len(store.Users))
		return
	}

	if len(store.Discussions) != len(discussions)-6 {
		t.Errorf("Snapshot: Wanted %d discussions, got %d", len(discussions), len(store.Discussions))
		return
	}

	// Check to make sure discussion possible slots make sense
	for i := range store.Discussions {
		sd := &store.Discussions[i]
		expectedSlots := 3
		if len(sd.PossibleSlots) != expectedSlots {
			t.Errorf("Snapshot.Discussions[%d] (did %v): Wanted %d possible slots, got %d",
				i, sd.DiscussionID, expectedSlots, len(sd.PossibleSlots))
			return
		}

		if len(interestMap[sd.DiscussionID]) != len(sd.UserInterest) {
			t.Errorf("Snapshot.Discussions[%d] (did %v): Wanted %d interests, got %d",
				i, sd.DiscussionID, len(interestMap[sd.DiscussionID]), len(sd.UserInterest))
		}

		for _, ui := range sd.UserInterest {
			if interestMap[sd.DiscussionID][ui.UserID] != ui.Interest {
				t.Errorf("Unexpected interest")
				return
			}
		}
	}

	tc.cleanup()

	return false
}
