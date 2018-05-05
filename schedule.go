package main

import (
	"log"
	"sort"
)

// Start simple: 10 slots, any amount of parallelism
type Slot struct {
	// What are all the concurrent discussions happening during this slot?
	Discussions map[DiscussionID]bool
	
	// Where is everyone during this slot?
	Users map[UserID]DiscussionID
}

func (slot *Slot) Init() {
	slot.Discussions = make(map[DiscussionID]bool)
	slot.Users = make(map[UserID]DiscussionID)
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
				log.Printf("  User %s %d -> %d (+%d)",
					user.Username, oInterest, tInterest, tInterest - oInterest)
			}
		} else if oInterest > 0 && !commit {
			log.Printf("  User %s will stay where they are (%d > %d)",
				user.Username, oInterest, tInterest)
		}
	}
	if commit {
		slot.Discussions[disc.ID] = true
	}

	return
}

func (slot *Slot) Score() (score, missed int) {
	for did := range slot.Discussions {
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
	}
	return
}

type Schedule struct {
	Slots []Slot
}

func (sched *Schedule) Init(slots int) {
	sched.Slots = make([]Slot, slots)
	for i := range sched.Slots {
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

type SlotDisplay struct {
	Discussions []*DiscussionScheduleDisplay
	// Time?
}

func (slot *Slot) GetDisplay(cur *User) (sd SlotDisplay) {
	for did := range slot.Discussions {
		dsd := &DiscussionScheduleDisplay{}
		
		// For every discussion in this slot...
		disc, _ := Event.Discussions.Find(did)

		log.Printf(" Packing display for discussion %s", disc.Title)

		dsd.ID = disc.ID
		dsd.Title = disc.Title
		dsd.Description = disc.Description
		dsd.Url = disc.GetURL()

		for uid := range disc.Interested {
			user, _ := Event.Users.Find(uid)

			usd := UserScheduleDisplay{
				Username: user.Username,
				Interest: user.Interest[disc.ID],
			}

			log.Printf("  Placing user %s in appropriate list", user.Username)
			
			// If they're going to this discussion, add them to the attending list;
			// otherwise, add them to the missing list
			if slot.Users[uid] == disc.ID {
				dsd.Attending = append(dsd.Attending, usd)
			} else {
				dsd.Missing = append(dsd.Missing, usd)
			}
		}

		sd.Discussions = append(sd.Discussions, dsd)
	}
	return

}

type ScheduleDisplay struct {
	Slots []SlotDisplay
}

func (sched *Schedule) GetDisplay(cur *User) (sd *ScheduleDisplay) {
	sd = &ScheduleDisplay{}
	for i := range sched.Slots {
		log.Printf("Packing display for slot %d", i)
		sd.Slots = append(sd.Slots, sched.Slots[i].GetDisplay(cur))
	}
	return
}

func MakeSchedule() (err error) {
	sched := &Schedule{}

	// Hard-code ten slots for now
	sched.Init(Event.ScheduleSlots)
	
	// First, sort our discussions by total potential score
	dList := []*Discussion{}

	for _, disc := range Event.Discussions {
		dList = append(dList, disc)
	}

	dListMaxIsLess := func(i, j int) bool {
		return dList[i].GetMaxScore() < dList[j].GetMaxScore()
	}
	sort.Slice(dList, dListMaxIsLess)

	// Starting at the top, look for a slot to put it in which will maximize this score
	for _, disc := range dList {
		log.Printf("Scheduling discussion %s (max score %d)",
			disc.Title, disc.GetMaxScore())

		// Find the slot that increases the score the most
		best := struct { score, index int }{ score: 0, index: -1 }
		for i := range sched.Slots {
			log.Printf(" Evaluating slot %d", i)
			// OK, how much will we increase the score by putting this discussion here?
			score := sched.Slots[i].Assign(disc, false)
			log.Printf("  Total value: %d", score)
			if score > best.score {
				best.score = score
				best.index = i
			}
		}

		// If we've found a slot, put it there
		if best.index < 0 {
			log.Printf(" Can't find a good slot!")
		} else {
			log.Printf(" Putting discussion in slot %d",
				best.index)

			// Make it so
			sched.Slots[best.index].Assign(disc, true)
		}
	}

	score, missed := sched.Score()

	log.Printf("Happiness: %d, sadness %d", score, missed)

	Event.Schedule = sched

	Event.Save()
	
	return
}

