package main

import (
	"io/ioutil"
	"log"
	"sort"
	"time"
)

type SlotAttendance map[UserID]DiscussionID

type Slot struct {
	// What are all the concurrent discussions happening during this slot?
	Discussions map[DiscussionID]bool

	Users SlotAttendance
}

// Change this to os.Stderr to enable
var SchedDebug = log.New(ioutil.Discard, "schedule.go ", log.LstdFlags)

func (slot *Slot) Init() {
	slot.Discussions = make(map[DiscussionID]bool)
	slot.Users = SlotAttendance(make(map[UserID]DiscussionID))
}

func (slot *Slot) Assign(disc *Discussion, commit bool) (delta int) {
	for uid := range disc.Interested {
		user, _ := Event.Users.Find(uid)

		// How interested is the user in this?
		tInterest, iprs := user.Interest[disc.ID]
		if !iprs {
			log.Fatalf("Internal error: interest not symmetric")
		}
		
		// See if the user is currently scheduled to do something else
		oInterest := 0
		odid, prs := slot.Users[uid]
		if prs {
			i, iprs := user.Interest[odid]
			if !iprs {
				log.Fatalf("Internal error: interest not symmetric")
			}
			oInterest = i
		}

		if tInterest > oInterest {
			delta += tInterest - oInterest
			if commit {
				slot.Users[uid] = disc.ID
			} else {
				SchedDebug.Printf("  User %s %d -> %d (+%d)",
					user.Username, oInterest, tInterest, tInterest - oInterest)
			}
		} else if oInterest > 0 && !commit {
			SchedDebug.Printf("  User %s will stay where they are (%d > %d)",
				user.Username, oInterest, tInterest)
		}
	}
	if commit {
		slot.Discussions[disc.ID] = true
	}

	return
}

func (slot *Slot) DiscussionScore(did DiscussionID) (score, missed int) {
	// For every discussion in this slot...
	disc, _ := Event.Discussions.Find(did)

	for uid := range disc.Interested {
		// Find out how much each user was interested in it
		user, _ := Event.Users.Find(uid)

		interest := user.Interest[disc.ID]

		// If they're going, add it to the score;
		// if not, add it to the 'missed' category
		if slot.Users[uid] == disc.ID {
			score += interest
		} else {
			missed += interest
		}
	}
	return
}

func (slot *Slot) DiscussionAttendeeCount(did DiscussionID) (count int) {
	for _, attendingID := range slot.Users {
		if attendingID == did {
			count++
		}
	}
	return
}

func (slot *Slot) Score() (score, missed int) {
	for did := range slot.Discussions {
		ds, dm := slot.DiscussionScore(did)
		score += ds
		missed += dm
	}
	return
}

func (slot *Slot) RemoveDiscussion(did DiscussionID) error {
	// Delete the discussion from the map
	delete(slot.Discussions, did)
			
	// Find all users attending this discussion and have them go
	// somewhere else
 	for uid, attendingDid := range slot.Users {
		if attendingDid != did {
			continue
		}
		user, _ := Event.Users.Find(uid)
		altDid := DiscussionID("")
		altInterest := 0
		for candidateDid := range slot.Discussions {
			if user.Interest[candidateDid] > altInterest {
				altDid = candidateDid
				altInterest = user.Interest[candidateDid]
			}
		}
		if altInterest > 0 {
			// Found something -- change the user to going to this session
			slot.Users[uid] = altDid
		} else {
			// User isn't interested in anything in this session -- remove them
			delete(slot.Users, uid)
		}
	}
	
	return nil
}

// Pure scheduling: Only slots
type Schedule struct {
	Slots []*Slot
	Created time.Time
	IsStale bool
}

func (sched *Schedule) Init(slots int) {
	sched.Slots = make([]*Slot, slots)
	for i := range sched.Slots {
		sched.Slots[i] = &Slot{}
		sched.Slots[i].Init()
	}
}

func (sched *Schedule) Score() (score, missed int) {
	for i := range sched.Slots {
		sscore, smissed := sched.Slots[i].Score()
		score += sscore
		missed += smissed
	}
	return
}

func (sched *Schedule) RemoveDiscussion(did DiscussionID) error {
	for _, slot := range sched.Slots {
		if slot.Discussions[did] {
			slot.RemoveDiscussion(did)
			break
		}
	}
	return nil
}

func MakeSchedule() (err error) {
	sched := &Schedule{}

	sched.Init(Event.ScheduleSlots)
	
	// First, sort our discussions by total potential score
	dList := []*Discussion{}

	dLocked := make(map[DiscussionID]bool)

	// If a slot is locked:
	// - Copy the old one verbatim into the new schedule
	// - Exclude the discussion in it from placement
	if Event.Schedule != nil && Event.LockedSlots != nil {
		for i := range Event.Schedule.Slots {
			if Event.LockedSlots[i] {
				sched.Slots[i] = Event.Schedule.Slots[i]
				for did := range sched.Slots[i].Discussions {
					dLocked[did] = true
				}
			}
		}
	}

	for _, disc := range Event.Discussions {
		if !dLocked[disc.ID] {
			dList = append(dList, disc)
		}
	}

	dListMaxIsLess := func(i, j int) bool {
		return dList[i].GetMaxScore() < dList[j].GetMaxScore()
	}
	sort.Slice(dList, dListMaxIsLess)

	// Starting at the top, look for a slot to put it in which will maximize this score
	for _, disc := range dList {
		SchedDebug.Printf("Scheduling discussion %s (max score %d)",
			disc.Title, disc.GetMaxScore())

		// Find the slot that increases the score the most
		best := struct { score, index int }{ score: 0, index: -1 }
		for i := range sched.Slots {
			SchedDebug.Printf(" Evaluating slot %d", i)
			if Event.LockedSlots[i] {
				SchedDebug.Printf("  Locked, skipping")
				continue
			}
			if !disc.PossibleSlots[i] {
				SchedDebug.Printf("  Impossible, skipping")
				continue
			}
			// OK, how much will we increase the score by putting this discussion here?
			score := sched.Slots[i].Assign(disc, false)
			SchedDebug.Printf("  Total value: %d", score)
			if score > best.score {
				best.score = score
				best.index = i
			}
		}

		// If we've found a slot, put it there
		if best.index < 0 {
			SchedDebug.Printf(" Can't find a good slot!")
		} else {
			SchedDebug.Printf(" Putting discussion in slot %d",
				best.index)

			// Make it so
			sched.Slots[best.index].Assign(disc, true)
		}
	}

	score, missed := sched.Score()

	log.Printf("New schedule happiness: %d, sadness %d", score, missed)

	sched.Created = time.Now()
	
	Event.Schedule = sched

	Event.Timetable.Place(sched)
	
	Event.Save()
	
	return
}

