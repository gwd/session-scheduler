package main

import (
	"fmt"
	"log"
	"sort"
)

// *ScheduleDisplay: Leftover from previous -- may be useful later
type UserScheduleDisplay struct {
	Username string
	Interest int
}

type DiscussionScheduleDisplay struct {
	ID DiscussionID
	Title string
	Description string
	Url string
	Attending []UserScheduleDisplay
	Missing []UserScheduleDisplay
}

type TimetableDiscussion struct {
	ID DiscussionID
	Title string
	Attendees int
	Score int
	// Copy of the "canonical" location, updated every time the
	// schedule is run
	LocationInfo Location
}

type TimetableSlot struct {
	Time string
	IsBreak bool

	// Which room will each discussion be in?
	// (Separate because placement and scheduling are separate steps)
	Discussions []TimetableDiscussion
}

func (ts *TimetableSlot) PlaceSlot(slot *Slot, dayName string) {
	// For now, just list the discussions.  Place into locations later.
	ts.Discussions = []TimetableDiscussion{}
	for did := range slot.Discussions {
		disc, _ := Event.Discussions.Find(did)
		tdisc := TimetableDiscussion{
			ID: did,
			Title: disc.Title,
			Attendees: slot.DiscussionAttendeeCount(did),
		}
		tdisc.Score, _ = slot.DiscussionScore(did)
		
		ts.Discussions = append(ts.Discussions, tdisc)

		disc.time = dayName + " " + ts.Time
	}
	
	// Sort by number of attendees
	sort.Slice(ts.Discussions, func(i, j int) bool {
		return ts.Discussions[i].Attendees > ts.Discussions[j].Attendees
	})

	// And place them in locations, using the non-place as a catch-all
	locations := Event.Locations.GetLocations()
	lidx := 0
	for i := range ts.Discussions {
		tdisc := &ts.Discussions[i]
		if lidx < len(locations) {
			loc := locations[lidx]
			disc, _ := Event.Discussions.Find(tdisc.ID)

			log.Printf("Setting discussion %s room to id %d (%s)",
				tdisc.Title, lidx, tdisc.LocationInfo.Name)

			tdisc.LocationInfo = *loc
			disc.location = loc

			if loc.IsPlace {
				lidx++
			} else {
				if lidx + 1 != len(locations) {
					log.Fatalf("Non-place not last in list! lidx %d len %d",
						lidx, len(locations))
				}
			}
		} else {
			log.Printf("Out of locations")
		}
	}
}

type TimetableDay struct {
	DayName string
	IsFinal bool
	// Date?
	
	Slots []*TimetableSlot
}

// Placement: Specific days, times, rooms
type Timetable struct {
	Days []*TimetableDay
}


func (tt *Timetable) Init() {
	// Clear out any old data which may be there
	*tt = Timetable{}
	
	// For now, hardcode 3 days (W Th F), 3 time slots
	// 4 locations: 
	for _, day := range []string{"Wednesday", "Thursday", "Friday"} {
		td := &TimetableDay{ DayName: day }
		for _, time := range []string{"1:50", "2:40", "3:25", "4:00"} {
			ts := &TimetableSlot{ Time: time }
			if time == "3:25" {
				ts.IsBreak = true
			}
			// FIXME: Init Discussions
			td.Slots = append(td.Slots, ts)
		}
		tt.Days = append(tt.Days, td)
	}
}

func (tt *Timetable) GetSlots() int {
	count := 0
	for _, td := range tt.Days {
		for _, ts := range td.Slots {
			if !ts.IsBreak {
				count++
			}
		}
	}
	return count
}

// Take a "Schedule" (consisting only of slots arranged for minimal
// conflicts) and map it into a "Timetable" (consisting of actual
// times and locations)
func (tt *Timetable) Place(sched *Schedule) (err error) {
	ttSlots := tt.GetSlots()
	if len(sched.Slots) != ttSlots {
		err = fmt.Errorf("Internal error: Schedule slots %d, timetable slots %d!",
			len(sched.Slots), ttSlots)
		return
	}

	count := 0
	for _, td := range tt.Days {
		for _, ts := range td.Slots {
			if ts.IsBreak {
				continue
			}

			slot := sched.Slots[count]

			ts.PlaceSlot(slot, td.DayName)

			count++
		}
	}
	
	return
}

func (tt *Timetable) UpdateIsFinal(ls LockedSlots) {
	count := 0
	for _, td := range tt.Days {
		// Start by assuming the day is finalized...
		td.IsFinal = true
		for _, ts := range td.Slots {
			if ts.IsBreak {
				continue
			}

			// And only clear if it we find one slot in this day that's not.
			// Note we still need to finish all the loops though so the slots
			// line up with the right days.
			if !ls[count] {
				td.IsFinal = false
			}

			count++
		}
	}
}

func (tt *Timetable) FillDisplaySlots(bslot []bool) (dss []DisplaySlot) {
	count := 0
	for _, td := range tt.Days {
		for _, ts := range td.Slots {
			if ts.IsBreak {
				continue
			}

			ds := DisplaySlot{
				Checked: bslot[count],
				Label: td.DayName+" "+ts.Time,
				Index: count,
			}

			dss = append(dss, ds)
			
			count++
		}
	}
	return
}
