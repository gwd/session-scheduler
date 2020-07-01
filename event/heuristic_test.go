package event

import (
	"math/rand"
	"testing"
	"time"
)

func testScheduleHeuristic(t *testing.T) (exit bool) {
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
	//totalSlots := 6

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

	// Make all discussions public
	for i := range discussions {
		err = DiscussionSetPublic(discussions[i].DiscussionID, true)
		if err != nil {
			t.Errorf("Setting discussion public: %v", err)
			return
		}
	}

	//
	// TESTING
	//

	err = MakeSchedule(SearchOptions{})
	if err != nil {
		t.Errorf("Making Schedule: %v", err)
		return
	}

	tc.cleanup()

	return false
}
