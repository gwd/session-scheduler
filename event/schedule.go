package event

import (
	"errors"
	"fmt"
	"log"
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

func MakeSchedule(optArg SearchOptions) error {
	// FIXME: Schedule

	return errors.New("Not Implemented!")
}

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
	SlotID      SlotID
	Discussions []*searchDiscussion
}

type schedule struct {
	Slots               []scheduleSlot
	UnplacedDiscussions []*searchDiscussion
}

type searchDiscussion struct {
	DiscussionID  DiscussionID
	Owner         UserID
	PossibleSlots []SlotID
	UserInterest  []struct {
		UserID   UserID
		Interest int
	}
	MaxInterest int
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
	//Locations []Location

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
			if count == 0 {
				d.PossibleSlots = append([]SlotID(nil), ss.Slots...)
			} else {
				err = sqlx.Select(eq, &d.PossibleSlots,
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
				if len(d.PossibleSlots) == 0 {
					return fmt.Errorf("Unscheduled discussion %v restricted to locked slots",
						d.DiscussionID)
				}
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
			ds := []struct {
				SlotID       SlotID
				DiscussionID DiscussionID
				LocationID   LocationID
			}{}
			for j := range ss.Discussions {
				ds = append(ds, struct {
					SlotID       SlotID
					DiscussionID DiscussionID
					LocationID   LocationID
				}{
					SlotID:       ss.SlotID,
					DiscussionID: ss.Discussions[j].DiscussionID,
					LocationID:   LocationID(j + 1) /* FIXME */})
			}
			_, err = sqlx.NamedExec(eq, `
                insert into event_schedule
                    values(:discussionid, :slotid, :locationid)`,
				ds)
			if err != nil {
				log.Printf("Failed inserting schedule entries %v", ds)
				return errOrRetry("Adding new schedule entries", err)
			}
		}
		return nil
	})
	return err
}
