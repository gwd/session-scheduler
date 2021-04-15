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

	m := &mirrorData{}

	//
	// SETUP: Make some users, some discussions, a timetable, and some locations
	//
	t.Logf("Setting up users, discussion, timetable")
	if testNewUsers(t, m, 10) {
		return
	}

	testDiscussionCount := 12
	m.discussions = make([]Discussion, testDiscussionCount)

	for i := range m.discussions {
		subexit := false
		uidx := rand.Int31n(int32(len(m.users)))
		m.discussions[i], subexit = testNewDiscussion(t, m.users[uidx].UserID)
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
	for i := range m.discussions {
		did := m.discussions[i].DiscussionID
		interestMap[did] = make(map[UserID]int)
		interestMap[did][m.discussions[i].Owner] = 100
	}

	for i := 0; i < (len(m.users)*len(m.discussions))/2; i++ {
		uidx := rand.Int31n(int32(len(m.users)))
		uid := m.users[uidx].UserID
		didx := rand.Int31n(int32(len(m.discussions)))
		did := m.discussions[didx].DiscussionID
		interest := int(rand.Int31n(101))
		err = m.users[uidx].SetInterest(&m.discussions[didx], interest)
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
		t.Logf("Restricting Discussion[0] (did %v) to Tuesday", m.discussions[0].DiscussionID)
		gotdisc, err := DiscussionFindByIdFull(m.discussions[0].DiscussionID)
		if err != nil {
			t.Errorf("Finding discussion 0 by id: %v", err)
			return
		}
		err = DiscussionSetPossibleSlots(m.discussions[0].DiscussionID, CheckedToSlotList(gotdisc.PossibleSlots)[3:])
	}

	//
	// TESTING
	//

	//
	// Set at least one discussion non-public and make sure it fails
	//
	err = DiscussionSetPublic(m.discussions[0].DiscussionID, false)
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
	for i := range m.discussions {
		err = DiscussionSetPublic(m.discussions[i].DiscussionID, true)
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
	if len(store.Users) != len(m.users)+1 {
		t.Errorf("Snapshot: Wanted %d users, got %d", len(m.users)+1, len(store.Users))
		return
	}

	if len(store.Discussions) != len(m.discussions) {
		t.Errorf("Snapshot: Wanted %d discussions, got %d", len(m.discussions), len(store.Discussions))
		return
	}

	// Check to make sure discussion possible slots make sense
	for i := range store.Discussions {
		sd := &store.Discussions[i]
		expectedSlots := 6
		if sd.DiscussionID == m.discussions[0].DiscussionID {
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
	gotdisc, err := DiscussionFindByIdFull(m.discussions[1].DiscussionID)
	if err != nil {
		t.Errorf("Getting discussion 1: %v", err)
		return
	}
	for i := range gotdisc.PossibleSlots {
		sched.Slots = append(sched.Slots, scheduleSlot{SlotID: gotdisc.PossibleSlots[i].SlotID})
	}

	// NB this ignores possibleslots.
	slotidx := 0
	for didx := range m.discussions {
		ss := &sched.Slots[slotidx]
		ss.Discussions = append(ss.Discussions, &searchDiscussion{DiscussionID: m.discussions[didx].DiscussionID})
		slotidx = (slotidx + 1) % len(sched.Slots)
	}

	store.CurrentSchedule = &sched
	placeDiscussions(store)

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
	if len(store.Users) != len(m.users)+1 {
		t.Errorf("Snapshot: Wanted %d users, got %d", len(m.users)+1, len(store.Users))
		return
	}

	if len(store.Discussions) != len(m.discussions)-6 {
		t.Errorf("Snapshot: Wanted %d discussions, got %d", len(m.discussions), len(store.Discussions))
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

	//
	// Re-test other operations: Delete discussions and users
	//
	t.Logf("Testing DeleteDiscussion / DeleteUsers with discussions")
	if testDeleteDiscussionsAndUsers(t, m) {
		return
	}

	tc.cleanup()

	return false
}
