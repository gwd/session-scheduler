package event

import (
	"fmt"
	"testing"
	"time"
)

func comparePossibleSlots(a []DisplaySlot, b []bool, t *testing.T) bool {
	ret := true

	if len(a) != len(b) {
		t.Errorf("Slot length mismatch: %d != %d", len(a), len(b))
		ret = false
	}

	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].Checked != b[i] {
			t.Errorf("Slot checked mismatch[%d]: %v != %v",
				i, a[i].Checked, b[i])
			ret = false
		}
	}

	return ret
}
func testUnitPossibleSlots(t *testing.T) (exit bool) {
	// Any "early" exit is a failure
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	//
	// SETUP: Make some users, some discussions, and a timetable
	//
	t.Logf("Setting up users, discussion, timetable")
	testUserCount := 6
	users := make([]User, testUserCount)
	for i := range users {
		subexit := false
		users[i], subexit = testNewUser(t)
		if subexit {
			return
		}
	}

	testDiscussionCount := 3
	discussions := make([]Discussion, testDiscussionCount)

	for i := range discussions {
		subexit := false
		discussions[i], subexit = testNewDiscussion(t, users[i].UserID)
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
				{Time: Date(2020, 7, 7, 16, 00, 0, 0, time.UTC)},
				{Time: Date(2020, 7, 7, 16, 30, 0, 0, time.UTC)},
			}},
		},
	}
	totalSlots := 8

	possibleslots := make([][]bool, len(discussions))
	for i := range discussions {
		possibleslots[i] = make([]bool, totalSlots)
		for j := range possibleslots[i] {
			possibleslots[i][j] = true
		}
	}

	err := TimetableSet(&tt)
	if err != nil {
		t.Errorf("ERROR Basic TimetableSet: %v", err)
		return
	}

	//
	// TESTING
	//

	// Get slots for non-set discussions, should all be 'true'
	t.Logf("Checking to see that default is all slots")
	gotdisc, err := DiscussionFindByIdFull(discussions[0].DiscussionID)
	if err != nil {
		t.Errorf("Finding the discussion we just created by ID: %v", err)
		return
	}
	fmt.Printf("%v\n", gotdisc.PossibleSlots)
	if !comparePossibleSlots(gotdisc.PossibleSlots, possibleslots[0], t) {
		return
	}

	// Set some slots for some of the discussions, make sure everything else
	t.Logf("Restricting some slots")
	tmp := append([]DisplaySlot(nil), gotdisc.PossibleSlots...)
	for i := 0; i < 4; i++ {
		possibleslots[0][i] = false
		tmp[i].Checked = false
	}
	fmt.Printf(" [0] %v\n", possibleslots[0])
	err = DiscussionSetPossibleSlots(discussions[0].DiscussionID, tmp)
	if err != nil {
		t.Errorf("Setting possible slots: %v", err)
		return
	}

	tmp = append([]DisplaySlot(nil), gotdisc.PossibleSlots...)
	for i := 4; i < totalSlots; i++ {
		possibleslots[1][i] = false
		tmp[i].Checked = false
	}
	err = DiscussionSetPossibleSlots(discussions[1].DiscussionID, tmp)
	if err != nil {
		t.Errorf("Setting possible slots: %v", err)
		return
	}
	fmt.Printf(" [0] %v\n", possibleslots[0])

	for i := range discussions {
		gotdisc, err = DiscussionFindByIdFull(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Finding the discussion we just created by ID: %v", err)
			return
		}
		if !comparePossibleSlots(gotdisc.PossibleSlots, possibleslots[i], t) {
			return
		}
	}

	fmt.Printf(" [0] %v\n", possibleslots[0])

	// Try adding some slots, make sure we get what we expect
	t.Logf("Adding a slot, making sure we get what we expect")
	tt.Days[0].Slots = append(tt.Days[0].Slots,
		TimetableSlot{Time: Date(2020, 7, 6, 17, 15, 0, 0, time.UTC)})
	err = TimetableSet(&tt)
	if err != nil {
		t.Errorf("ERROR Day / slot add failed: %v", err)
		return
	}
	totalSlots++

	fmt.Printf(" [0] %v\n", possibleslots[0])

	for i := range possibleslots {
		newValue := !(i == 0 || i == 1)
		n := make([]bool, totalSlots)
		copy(n[:4], possibleslots[i][:4])
		n[4] = newValue
		copy(n[5:], possibleslots[i][4:])
		possibleslots[i] = n
	}
	for i := range discussions {
		gotdisc, err = DiscussionFindByIdFull(discussions[i].DiscussionID)
		if err != nil {
			t.Errorf("Finding the discussion we just created by ID: %v", err)
			return
		}
		if !comparePossibleSlots(gotdisc.PossibleSlots, possibleslots[i], t) {
			fmt.Printf("  Wanted %v\n  Got %v\n", possibleslots[i], gotdisc.PossibleSlots)
			t.Errorf("Mismatch in discussion[%d] after adding a slot", i)
			return
		}
	}

	tc.cleanup()

	return false
}
