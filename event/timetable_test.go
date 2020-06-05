package event

import (
	"testing"
	"time"
)

func compareTimetables(a *Timetable, b *Timetable, t *testing.T) bool {
	ret := true
	if len(a.Days) != len(b.Days) {
		t.Errorf("mismatch days: %v != %v", len(a.Days), len(b.Days))
		ret = false
	}

	for i := 0; i < len(a.Days) && i < len(b.Days); i++ {
		dayid := i + 1
		ad := &a.Days[i]
		bd := &b.Days[i]

		if ad.DayName != bd.DayName {
			t.Errorf("Day %d mismatch dayname: %v != %v",
				dayid, ad.DayName, bd.DayName)
			ret = false
		}

		if len(ad.Slots) != len(bd.Slots) {
			t.Errorf("Day %d mismatch slots: %d != %d!",
				dayid, len(ad.Slots), len(bd.Slots))
			ret = false
		}

		for j := 0; j < len(ad.Slots) && j < len(bd.Slots); j++ {
			as := &ad.Slots[j]
			bs := &bd.Slots[j]

			// Use == rather than Time.Equal() because we do want the locations to be the same
			if as.Time != bs.Time {
				t.Errorf("Day %d slot %d mismatch Time: %v != %v",
					i, j, as.Time, bs.Time)
				ret = false
			}
			if as.IsBreak != bs.IsBreak {
				t.Errorf("Day %d slot %d mismatch IsBreak: %v != %v",
					i, j, as.IsBreak, bs.IsBreak)
				ret = false
			}
		}
	}

	return ret

}

func testUnitTimetable(t *testing.T) (exit bool) {
	// Any "early" exit is a failure
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	gottt, err := GetTimetable()
	if err != nil {
		t.Errorf("ERROR Getting empty timetable: %v", err)
		return
	}
	if len(gottt.Days) != 0 {
		t.Errorf("ERROR Expected 0 days, got %v", len(gottt.Days))
		return
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

	t.Logf("Creating basic timetable with %d days", len(tt.Days))
	err = TimetableSet(&tt)
	if err != nil {
		t.Errorf("ERROR Basic TimetableSet: %v", err)
		return
	}
	gottt, err = GetTimetable()
	if err != nil {
		t.Errorf("ERROR Getting non-empty timetable: %v", err)
		return
	}
	if !compareTimetables(&tt, &gottt, t) {
		t.Errorf("Timetable mismatch")
		return
	}

	t.Logf("Updating to a break")
	tt.Days[1].Slots[2].IsBreak = true
	err = TimetableSet(&tt)
	if err != nil {
		t.Errorf("ERROR Basic TimetableSet update: %v", err)
		return
	}
	gottt, err = GetTimetable()
	if err != nil {
		t.Errorf("ERROR Getting non-empty timetable: %v", err)
		return
	}
	if !compareTimetables(&tt, &gottt, t) {
		t.Errorf("Timetable mismatch")
		return
	}

	t.Logf("Trying an invalid range (should fail)")
	tt.Days[1].Slots[3].Time = Date(2020, 7, 8, 16, 30, 0, 0, time.UTC)
	err = TimetableSet(&tt)
	if err == nil {
		t.Errorf("ERROR Invalid range succeeded!")
		return
	}
	tt.Days[1].Slots[3].Time = Date(2020, 7, 7, 16, 30, 0, 0, time.UTC)

	t.Logf("Adding an extra day, adding some slots")
	tt.Days = append(tt.Days, TimetableDay{DayName: "Wednesday", Slots: []TimetableSlot{
		{Time: Date(2020, 7, 8, 14, 30, 0, 0, time.UTC)},
		{Time: Date(2020, 7, 8, 15, 15, 0, 0, time.UTC), IsBreak: true},
		{Time: Date(2020, 7, 8, 15, 45, 0, 0, time.UTC)},
		{Time: Date(2020, 7, 8, 16, 30, 0, 0, time.UTC)}}})
	tt.Days[0].Slots = append(tt.Days[0].Slots,
		TimetableSlot{Time: Date(2020, 7, 6, 17, 15, 0, 0, time.UTC)})
	t.Logf("%v", tt)
	err = TimetableSet(&tt)
	if err != nil {
		t.Errorf("ERROR Day / slot add failed: %v", err)
		return
	}
	gottt, err = GetTimetable()
	if err != nil {
		t.Errorf("ERROR Getting non-empty timetable: %v", err)
		return
	}
	if !compareTimetables(&tt, &gottt, t) {
		t.Errorf("Timetable mismatch")
		return
	}

	t.Logf("Removing some days, removing some timeslots")
	tt.Days = tt.Days[1:]
	tt.Days[1].Slots = tt.Days[1].Slots[1:]
	t.Logf("%v", tt)
	err = TimetableSet(&tt)
	if err != nil {
		t.Errorf("ERROR Day / slot removal failed: %v", err)
		return
	}
	gottt, err = GetTimetable()
	if err != nil {
		t.Errorf("ERROR Getting non-empty timetable: %v", err)
		return
	}
	if !compareTimetables(&tt, &gottt, t) {
		t.Errorf("Timetable mismatch")
		return
	}

	tc.cleanup()

	return false
}
