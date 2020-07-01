package event

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
	//"github.com/hako/durafmt"
)

type SearchAlgo string

const (
	SearchHeuristicOnly = SearchAlgo("heuristic")
	//SearchGenetic       = SearchAlgo("genetic")
	SearchRandom = SearchAlgo("random")
)

type SearchOptions struct {
	Async          bool
	Algo           SearchAlgo
	Validate       bool
	DebugLevel     int
	SearchDuration time.Duration
	Debug          *log.Logger
}

var opt SearchOptions

func SchedLastUpdate() string {
	lastUpdate := "Never"
	// if event.ScheduleV2 != nil {
	// 	lastUpdate = durafmt.ParseShort(time.Since(event.ScheduleV2.Created)).String() + " ago"
	// }
	return lastUpdate
}

type SchedState int

const (
	SchedStateCurrent = SchedState(iota)
	SchedStateModified
	SchedStateRunning
)

func SchedGetState() SchedState {
	return SchedStateCurrent
}

type scheduleSlot struct {
	SlotID SlotID

	// These should be sorted in order of expected attendees, high to
	// low.
	Discussions []*searchDiscussion
}

type schedule struct {
	Slots               []scheduleSlot
	UnplacedDiscussions []*searchDiscussion
}

type searchDiscussion struct {
	DiscussionID  DiscussionID
	Owner         UserID
	PossibleSlots map[SlotID]bool
	UserInterest  []struct {
		UserID   UserID
		Interest int
	}
	MaxInterest int

	// Filled in by placement
	SlotID     SlotID
	LocationID LocationID
}

type searchStore struct {
	// All unlocked slots
	Slots []SlotID

	// All users
	Users []User

	// Discussions not assigned to locked slots (either not assigned
	// at all, or assigned to unlocked slots); along with maps to user
	// interest, and fordbidden slots
	Discussions []searchDiscussion

	// All locations
	Locations []LocationID

	CurrentSchedule *schedule
}

// makeSnapshot will take a snapshot of all the data necessary to make a transaction.
//
// Should fail if:
// - Any discussions are non-public
// - There are no unlocked slots
func makeSnapshot() (*searchStore, error) {
	var ss *searchStore
	err := txLoop(func(eq sqlx.Ext) error {
		// Make sure there are no non-public discussion
		{
			var dcount int
			err := sqlx.Get(eq, &dcount, `
				select count(*) from event_discussions where ispublic = false`)
			if err != nil {
				return errOrRetry("Getting non-public discussion count", err)
			}
			if dcount != 0 {
				return fmt.Errorf("Cannot run scheduler with non-public discussions")
			}
		}

		ss = &searchStore{}

		// Get all unlocked slots
		err := sqlx.Select(eq, &ss.Slots, `
            select slotid
                from event_slots
                where isbreak == false and islocked == false
                order by dayid, slotidx`)
		if err != nil {
			return errOrRetry("Getting schedule-able slots", err)
		}

		if len(ss.Slots) == 0 {
			return fmt.Errorf("No schedulable slots found!")
		}

		// Get all users
		err = userGetAllTx(eq, &ss.Users)
		if err != nil {
			return err
		}

		// Get all locations
		err = sqlx.Select(eq, &ss.Locations,
			`select locationid from event_locations`)
		if err != nil {
			return errOrRetry("Getting locations", err)
		}

		// Get Discussions not scheduled to locked slots
		err = sqlx.Select(eq, &ss.Discussions,
			`select discussionid, owner
                 from event_discussions
                     natural left join event_schedule
                     natural left join event_slots
                 where ifnull(islocked, false) = false
                 order by discussionid`)
		if err != nil {
			return errOrRetry("Getting unlocked discussions", err)
		}

		for i := range ss.Discussions {
			d := &ss.Discussions[i]

			// Get unlocked slots into which this can be scheduled.
			// First, see if unrestricted.
			var count int
			err = sqlx.Get(eq, &count,
				`select count(*)
                     from event_discussions_possible_slots
                     where discussionid = ?`, d.DiscussionID)
			if err != nil {
				return errOrRetry("Getting possible slot count", err)
			}

			// If unrestricted, jus tcopy ss.Slots; otherwise, get a list
			// of unlocked slots
			var pslots []SlotID
			if count == 0 {
				pslots = append([]SlotID(nil), ss.Slots...)
			} else {
				err = sqlx.Select(eq, &pslots,
					`select slotid
                         from event_discussions_possible_slots
                             natural join event_slots
                         where discussionid = ?
                               and islocked = false 
                               and isbreak = false
                         order by dayid, slotidx`, d.DiscussionID)
				if err != nil {
					return errOrRetry("Getting possible unlocked slots", err)
				}
				if len(pslots) == 0 {
					return fmt.Errorf("Unscheduled discussion %v restricted to locked slots",
						d.DiscussionID)
				}
			}
			d.PossibleSlots = make(map[SlotID]bool)
			for _, slotid := range pslots {
				d.PossibleSlots[slotid] = true
			}

			// Get user interest in this discussion
			err = sqlx.Select(eq, &d.UserInterest,
				`select userid, interest
                     from event_interest
                     where discussionid = ?`, d.DiscussionID)
			if err != nil {
				return errOrRetry("Error getting interest for discussion", err)
			}

			d.MaxInterest = 0
			for i := range d.UserInterest {
				d.MaxInterest += d.UserInterest[i].Interest
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return ss, nil
}

// FIXME: This doesn't handle IsPlace == false
func placeDiscussions(ss *searchStore) error {
	s := ss.CurrentSchedule

	for i := range s.Slots {
		slot := &s.Slots[i]
		if len(slot.Discussions) > len(ss.Locations) {
			return fmt.Errorf("Invalid schedule: %d discussions in slot, but only %d locations",
				len(slot.Discussions), len(ss.Locations))
		}

		// Set SlotID, LocationID
		for j := range slot.Discussions {
			slot.Discussions[j].SlotID = slot.SlotID
			slot.Discussions[j].LocationID = ss.Locations[j]
		}
	}

	return nil
}

func scheduleSet(s *schedule) error {
	err := txLoop(func(eq sqlx.Ext) error {
		for i := range s.Slots {
			ss := &s.Slots[i]
			// Check to see if this slot is locked
			{
				var info struct {
					IsLocked bool
					IsBreak  bool
				}
				err := sqlx.Get(eq, &info,
					`select islocked, isbreak
                         from event_slots
                         where slotid = ?`, ss.SlotID)
				if err != nil {
					return errOrRetry("Getting slot islocked / isbreak statuses", err)
				}
				if info.IsLocked || info.IsBreak {
					return fmt.Errorf("Cannot set schedule for slot %v: islocked %v isbreak %v",
						ss.SlotID, info.IsLocked, info.IsBreak)
				}
			}

			// Drop schedule entries for this slotid
			_, err := eq.Exec(
				`delete from event_schedule where slotid = ?`,
				ss.SlotID)
			if err != nil {
				return errOrRetry("Deleting schedule entries", err)
			}

			// Add new schedule entries
			if len(ss.Discussions) > 0 {
				_, err = sqlx.NamedExec(eq, `
                insert into event_schedule
                    values(:discussionid, :slotid, :locationid)`,
					ss.Discussions)
				if err != nil {
					log.Printf("Failed inserting schedule entries %v", ss.Discussions)
					return errOrRetry("Adding new schedule entries", err)
				}
			}
		}
		return nil
	})
	return err
}

func scheduleMakeEmpty(ss *searchStore) *schedule {
	sched := &schedule{}
	for i := range ss.Slots {
		sched.Slots = append(sched.Slots, scheduleSlot{SlotID: ss.Slots[i]})
	}
	for i := range ss.Discussions {
		sched.UnplacedDiscussions = append(sched.UnplacedDiscussions, &ss.Discussions[i])
	}
	return sched
}

func addDiscussionInterest(userMaxInt map[UserID]int, disc *searchDiscussion) {
	for j := range disc.UserInterest {
		ui := &disc.UserInterest[j]
		if ui.Interest > userMaxInt[ui.UserID] {
			userMaxInt[ui.UserID] = ui.Interest
		}
	}
}

// How much would we increase the score by adding hyp to this discussion?
func scoreSlotDelta(discussions []*searchDiscussion, hyp *searchDiscussion) int {
	userMaxInt := map[UserID]int{}

	for i := range discussions {
		addDiscussionInterest(userMaxInt, discussions[i])
	}

	pre := 0
	for _, interest := range userMaxInt {
		pre += interest
	}

	addDiscussionInterest(userMaxInt, hyp)

	post := 0

	for _, interest := range userMaxInt {
		post += interest
	}
	return post - pre
}

func makeScheduleHeuristic(ss *searchStore) (*schedule, error) {
	sched := scheduleMakeEmpty(ss)
	unplaced := []*searchDiscussion(nil)

	// Sort discussion list by interest, high to low
	sort.Slice(sched.UnplacedDiscussions, func(i, j int) bool {
		return sched.UnplacedDiscussions[i].MaxInterest > sched.UnplacedDiscussions[j].MaxInterest
	})

	// Starting at the top, look for a slot to put it in which will maximize this score
	for _, disc := range sched.UnplacedDiscussions {
		log.Printf("Scheduling discussion %v (max score %d)",
			disc.DiscussionID, disc.MaxInterest)

		// Find the slot that increases the score the most
		best := struct{ score, index int }{score: 0, index: -1}
		for i := range sched.Slots {
			log.Printf(" Evaluating slot %d", i)
			if !disc.PossibleSlots[sched.Slots[i].SlotID] {
				log.Printf("  Slot disallowed, skipping")
				continue
			}
			if len(sched.Slots[i].Discussions) >= len(ss.Locations) {
				log.Printf("  Slot full, skipping")
				continue
			}

			// OK, how much will we increase the score by putting this discussion here?
			score := scoreSlotDelta(sched.Slots[i].Discussions, disc)
			log.Printf("  Total value: %d", score)
			if score > best.score {
				best.score = score
				best.index = i
			}
		}

		// If we've found a slot, put it there
		if best.index < 0 {
			// FIXME: Do something useful (least-busy slot?)
			log.Printf(" Can't find a good slot!")
			unplaced = append(unplaced, disc)
		} else {
			log.Printf(" Putting discussion in slot %d",
				best.index)

			// Make it so
			sched.Slots[best.index].Discussions = append(sched.Slots[best.index].Discussions, disc)
		}
	}

	sched.UnplacedDiscussions = unplaced

	return sched, nil
}

func MakeSchedule(opt SearchOptions) error {
	ss, err := makeSnapshot()
	if err != nil {
		return err
	}

	ss.CurrentSchedule, err = makeScheduleHeuristic(ss)
	if err != nil {
		return err
	}

	err = placeDiscussions(ss)
	if err != nil {
		return err
	}

	err = scheduleSet(ss.CurrentSchedule)
	if err != nil {
		return err
	}

	return nil
}
