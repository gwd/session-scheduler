package main

import (
	"io/ioutil"
	"log"
	"sort"
	"math/rand"
	"os"
	"time"
	"github.com/MaxHalford/gago"
)

type Slot struct {
	// What are all the concurrent discussions happening during this slot?
	Discussions map[DiscussionID]bool

	Users map[UserID]DiscussionID
}

func (slot *Slot) Clone() (nslot *Slot) {
	nslot = &Slot{}
	nslot.Init()
	for k, v := range slot.Discussions {
		nslot.Discussions[k] = v
	}
	for k, v := range slot.Users {
		nslot.Users[k] = v
	}
	return
}

// Change this to os.Stderr to enable
var SchedDebug *log.Logger
var OptSchedDebug = false
var OptSchedDebugVerbose = true

func ScheduleInit() {
	if OptSchedDebug {
		SchedDebug = log.New(os.Stderr, "schedule.go ", log.LstdFlags)
	} else {
		SchedDebug = log.New(ioutil.Discard, "schedule.go ", log.LstdFlags)
	}
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
			}
			if OptSchedDebugVerbose {
				SchedDebug.Printf("  User %s %d -> %d (+%d)",
					user.Username, oInterest, tInterest, tInterest - oInterest)
			}
		} else if oInterest > 0 && OptSchedDebugVerbose {
			SchedDebug.Printf("  User %s will stay where they are (%d > %d)",
				user.Username, oInterest, tInterest)
		}
	}
	if commit {
		slot.Discussions[disc.ID] = true
	}

	return
}

func (slot *Slot) Remove(disc *Discussion, commit bool) (delta int) {
	if !slot.Discussions[disc.ID] {
		log.Printf("ERROR: Trying to remove non-assigned discussion %s", disc.ID)
		return
	}
	
	if commit {
		delete(slot.Discussions, disc.ID)
	}

	for uid := range disc.Interested {
		user, _ := Event.Users.Find(uid)

		// How interested is the user in this?
		tInterest, iprs := user.Interest[disc.ID]
		if !iprs {
			log.Fatalf("Internal error: interest not symmetric")
		}
		
		// Is this their current favorite? If not, removing it won't have an effect
		if slot.Users[uid] != disc.ID {
			if OptSchedDebugVerbose {
				SchedDebug.Printf("  User %s already going to a different discussion, no change",
					user.Username)
			}
		} else {
			best := struct { interest int; did DiscussionID }{}
			
			// This user is currently going to this discussion.  See
			// if they have somewhere else they want to go.
			for did := range slot.Discussions {
				i, iprs := user.Interest[did]
				if iprs && i > best.interest {
					best.interest = i
					best.did = did
				}
			}

			delta = tInterest - best.interest
			if best.did == "" {
				if OptSchedDebugVerbose {
					SchedDebug.Printf("  User %s has no other discussions of interest",
						user.Username)
				}
				if commit {
					delete(slot.Users, uid)
				}
			} else {
				if OptSchedDebugVerbose {
					SchedDebug.Printf("  User %s %d -> %d (%d)",
						user.Username, tInterest, best.interest, delta)
				}
				if commit {
					slot.Users[uid] = disc.ID
				}
			}
		}
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
		adid := slot.Users[uid] // "Attending" discussion id
		if adid == disc.ID {
			score += interest
		} else {
			// Check to make sure the discussion they're attending is actually more
			ainterest := user.Interest[adid]
			if ainterest < interest {
				adisc, _ := Event.Discussions.Find(adid)
				log.Printf("User %v attending wrong discussion (%s %d < %s %d)!",
					user.Username, adisc.Title, ainterest, disc.Title, interest)
				score += interest
				slot.Users[uid] = disc.ID
			} else {
				missed += interest
			}
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

func (sched *Schedule) Validate() (error) {
	// Make sure that this schedule has the following properties:
	// - Every discussion placed exactly once
	// - 'Locked' sessions match current scheduler locked session

	// First, find the location of all the locked discussions in the 'master'
	lockedD := make(map[DiscussionID]int)
	if Event.Schedule != nil && Event.LockedSlots != nil {
		for i, slot := range Event.Schedule.Slots {
			if Event.LockedSlots[i] {
				for did := range slot.Discussions {
					lockedD[did] = i
				}
			}
		}
	}

	dMap := make(map[DiscussionID]int)
	// Now go through each slot and find where discussions are mapped; checking that
	// 1) No duplicates, and 2) Locked discussions in the right place
	for i, slot := range sched.Slots {
		for did := range slot.Discussions {
			t, prs := dMap[did]
			if prs {
				log.Fatalf("Found duplicate discussion! %v -> %d, %d",
					did, t, i)
			}
			t, prs = lockedD[did]
			if prs && t != i {
				log.Fatalf("Locked discussion moved! %v in slot %d, should be %d",
					did, i, t)
			}
			dMap[did] = i
		}
	}

	// Now go through all the discussions and make sure each one was mapped once
	err := Event.Discussions.Iterate(func(disc *Discussion) error {
		_, prs := dMap[disc.ID]
		if !prs {
			log.Fatalf("Discussion %v missing!", disc.ID)
		}
		return nil
	})
	
	return err
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

// type Genome interface {
//     Evaluate() (float64, error)
//     Mutate(rng *rand.Rand)
//     Crossover(genome Genome, rng *rand.Rand)
//     Clone() Genome
// }
// type NewGenome func(rng *rand.Rand) Genome

func (sched *Schedule) AssignRandom(disc *Discussion, rng *rand.Rand) {
	// First check to make sure there are at least some possible slots
	found := false
	for slotn := range disc.PossibleSlots {
		if disc.PossibleSlots[slotn] && !Event.LockedSlots[slotn] {
			found = true
			break
		}
	}
	if !found {
		log.Fatalf("Discussion %s has no possible slots! %v %v",
			disc.ID, disc.PossibleSlots, Event.LockedSlots)
	}

	for {
		slotIndex := rng.Intn(len(sched.Slots))
		if disc.PossibleSlots[slotIndex] && !Event.LockedSlots[slotIndex] {
			SchedDebug.Printf("  Assigning discussion %v to slot %d", disc.ID, slotIndex)
			sched.Slots[slotIndex].Assign(disc, true)
			break
		}
	}
}

func ScheduleFactoryInner(template *Schedule, dList []*Discussion, rng *rand.Rand) gago.Genome {
	SchedDebug.Printf("Making new random schedule")
	sched := &Schedule{}
	sched.Init(len(template.Slots))

	// Clone locked slots
	if Event.LockedSlots != nil {
		for i := range sched.Slots {
			if Event.LockedSlots[i] {
				sched.Slots[i] = template.Slots[i].Clone()
			}
		}
	}

	// Put each discussion in a random slot
	for _, disc := range dList {
		SchedDebug.Printf("  Placing discussion `%s`", disc.Title)
		sched.AssignRandom(disc, rng)
	}

	sched.Validate()

	return gago.Genome(sched)
}

func (sched *Schedule) Clone() gago.Genome {
	SchedDebug.Print("Clone")
	new := &Schedule{}
	for i := range sched.Slots {
		new.Slots = append(new.Slots, sched.Slots[i].Clone())
	}

	new.Validate()
	
	return gago.Genome(new)
}

func (sched *Schedule) Evaluate() (float64, error) {
	SchedDebug.Print("Evaluate")
	score, _ := sched.Score()
	return -float64(score), nil
}

func (sched *Schedule) RandomUnlockedSlot(rng *rand.Rand) (slotn int, slot *Slot) {
	for {
		slotn = rng.Intn(len(sched.Slots))
		if Event.LockedSlots == nil || !Event.LockedSlots[slotn] {
			slot = sched.Slots[slotn]
			return
		}
	}
}

func (sched *Schedule) Mutate(rng *rand.Rand) {
	sScore, _ := sched.Score()
	
	SchedDebug.Print("Mutate")
	replace := []*Discussion{}

	// Remove a random number of discussions
	rmCount := rng.Intn(len(Event.Discussions) * 100 / 25 + 1)
	if rmCount < 2 {
		rmCount = 2
	}
	for n := 0; n < rmCount; n++ {
		// Choose a random slot
		slotn, slot := sched.RandomUnlockedSlot(rng)

		// Nothing to do for empty slots
		if len(slot.Discussions) == 0 {
			continue
		}

		// Choose a random discussion
		dnum := rng.Intn(len(slot.Discussions))
		for did := range slot.Discussions {
			if dnum == 0 {
				SchedDebug.Printf(" Removing discussion %v from slot %d",
					did, slotn)
				disc, _ := Event.Discussions.Find(did)
				replace = append(replace, disc)
				slot.Remove(disc, true)
				break
			}
			dnum--
		}
	}
	
	// And put them back somewhere else
	for _, disc := range replace {
		sched.AssignRandom(disc, rng)
	}

	sched.Validate()

	eScore, _ := sched.Score()
	if eScore > sScore {
		log.Printf("Mutated from %d to %d mplus", sScore, eScore)
	} else {
		log.Printf("Mutated from %d to %d", sScore, eScore)
	}
}

func (sched *Schedule) Crossover(Genome gago.Genome, rng *rand.Rand) {
	sScore, _ := sched.Score()
	
	SchedDebug.Print("Crossover")
	osched := Genome.(*Schedule)
	
	// Keep track of the discussions that don't get placed.  Keep a
	// pointer so we don't have to look it up again when we decide to
	// use it.
	displaced := make(map[DiscussionID]*Discussion)

	// For each slot, replace a random number of discussions
	for i := range sched.Slots {
		slot := sched.Slots[i]
		oslot := osched.Slots[i]

		// Nothing to do for slots with no discussions already, or locked slots
		if len(slot.Discussions) == 0 ||
			(Event.LockedSlots != nil && Event.LockedSlots[i]) {
			continue
		}
		
		remIndexes := make(map[int]bool)

		toRemove := rng.Intn(len(slot.Discussions))

		// Choose a random set of "indexes" to remove
		for n := 0; n < toRemove ; n++ {
			for {
				i := rng.Intn(len(slot.Discussions))
				if ! remIndexes[i] {
					remIndexes[i] = true
					break
				}
			}
		}

		addIndexes := make(map[int]bool)
		toMove := toRemove
		if toMove > len(oslot.Discussions) {
			toMove = len(oslot.Discussions)
		}
		
		// And a set to move from the other parent
		for n := 0; n < toMove ; n++ {
			for {
				i := rng.Intn(len(oslot.Discussions))
				if ! addIndexes[i] {
					addIndexes[i] = true
					break
				}
			}
		}

		// Make that into a list of discussions to remove, adding them
		// to the list of 'displaced' discussions
		remDisc := []*Discussion{}
		n := 0
		for did := range slot.Discussions {
			if remIndexes[n] {
				disc, _ := Event.Discussions.Find(did)
				displaced[did] = disc
				remDisc = append(remDisc, disc)
			}
			n++
		}

		// Do the same for the discussions to add, but add the
		// non-added ones to the 'displaced' discussion list
		addDisc := []*Discussion{}
		n = 0
		for did := range slot.Discussions {
			disc, _ := Event.Discussions.Find(did)
			if addIndexes[n] {
				addDisc = append(addDisc, disc)
			} else {
				displaced[did] = disc
			}
			n++
		}

		// Now remove the removed ones, add the added ones
		for _, disc := range remDisc {
			slot.Remove(disc, true)
		}
		for _, disc := range addDisc {
			slot.Assign(disc, true)
		}

		// And finally, go through the list cleaning out the 'displaced' list
		for did := range slot.Discussions {
			if displaced[did] != nil {
				delete(displaced, did)
			}
		}
	}

	// Finally, put all the displaced discussions somewhere random
	for _, disc := range displaced {
		sched.AssignRandom(disc, rng)
	}

	sched.Validate()

	eScore, _ := sched.Score()
	if eScore > sScore {
		log.Printf("Crossover from %d to %d cplus", sScore, eScore)
	} else {
		log.Printf("Crossover from %d to %d", sScore, eScore)
	}
}


func MakeScheduleGenetic(sched *Schedule, dList []*Discussion) (*Schedule, error) {
	ScheduleFactory := func(rng *rand.Rand) gago.Genome {
		return ScheduleFactoryInner(sched, dList, rng)
	}

	ga := gago.Generational(ScheduleFactory)
	ga.Logger = SchedDebug
	if err := ga.Initialize(); err != nil {
		log.Printf("Error initalizing ga: %v", err)
		return nil, err
	}

	log.Printf("Generation %d age %v Best fitness %v\n", ga.Generations,
		ga.Age, ga.HallOfFame[0].Fitness)
    for i := 1; i < 100; i++ {
        if err := ga.Evolve(); err != nil {
            log.Println("Error evolving: %v", err)
			return nil, err
        }
		log.Printf("Generation %d age %v Best fitness %v\n", ga.Generations,
			ga.Age, ga.HallOfFame[0].Fitness)
    }

	return ga.HallOfFame[0].Genome.(*Schedule), nil
}

func MakeScheduleHeuristic(sched *Schedule, dList []*Discussion) (*Schedule, error) {
	// Sort discussion list by interest, high to low
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
			// FIXME: Do something useful (least-busy slot?)
			SchedDebug.Printf(" Can't find a good slot!")
		} else {
			SchedDebug.Printf(" Putting discussion in slot %d",
				best.index)

			// Make it so
			sched.Slots[best.index].Assign(disc, true)
		}
	}

	return sched, nil
}

func MakeSchedule() (error) {
	sched := &Schedule{}

	sched.Init(Event.ScheduleSlots)
	
	// First, sort our discussions by total potential score
	dList := []*Discussion{}

	dLocked := make(map[DiscussionID]bool)

	// If a slot is locked:
	// - Copy the old one verbatim into the new schedule
	// - Exclude the discussion in it from placement
	//
	// Also while we're here, check to make sure there are unlocked slots (otherwise
	// we can't really do a schedule.)
	allLocked := true
	if Event.LockedSlots != nil {
		if Event.Schedule != nil {
			for i := range Event.Schedule.Slots {
				if Event.LockedSlots[i] {
					sched.Slots[i] = Event.Schedule.Slots[i]
					for did := range sched.Slots[i].Discussions {
						dLocked[did] = true
					}
				} else {
					allLocked = false
				}
			}
		} else {
			// No schedule; but we still need to check for unlocked slots
			for _, l := range Event.LockedSlots {
				if !l {
					allLocked = false
					break
				}
			}
		}
	} else {
		// All slots unlocked
		allLocked = false
	}

	if allLocked {
		log.Printf("Can't make a schedule -- all slots are locked")
		return errAllSlotsLocked
	}

	for _, disc := range Event.Discussions {
		if !dLocked[disc.ID] {
			dList = append(dList, disc)
		}
	}

	var Hscore, Hmissed, Gscore, Gmissed int
	var newH, newG *Schedule
	var err error

	newH, err = MakeScheduleHeuristic(sched, dList)
	if err != nil {
		log.Printf("Schedule generator failed")
		return err
	}

	Hscore, Hmissed = newH.Score()

	newG, err = MakeScheduleGenetic(sched, dList)
	if err != nil {
		log.Printf("Schedule generator failed")
		return err
	}

	Gscore, Gmissed = newG.Score()

	log.Printf("Heuristic schedule happiness: %d, sadness %d", Hscore, Hmissed)
	log.Printf("Genetic schedule happiness: %d, sadness %d", Gscore, Gmissed)

	new := newH
	if Gscore > Hscore {
		new = newG
	}
	
	new.Created = time.Now()
	
	Event.Schedule = new

	Event.Timetable.Place(new)
	
	Event.Save()
	
	return nil
}

