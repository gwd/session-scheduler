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

type SlotDiscussions []DiscussionID

func (d *SlotDiscussions) Index(did DiscussionID) (bool, int) {
	for i, v := range *d {
		if v == did {
			return true, i
		}
	}
	return false, -1
}

func (d *SlotDiscussions) IsPresent(did DiscussionID) bool {
	prs, _ := d.Index(did)
	return prs
}

func (d *SlotDiscussions) Delete(did DiscussionID) {
	prs, i := d.Index(did)
	if !prs {
		return
	}
	new := append((*d)[:i], (*d)[i+1:]...)
	*d = new
}

func (d *SlotDiscussions) InsertOnce(did DiscussionID) {
	if ! d.IsPresent(did) {
		*d = append((*d), did)
	}
}

type SlotUser struct {
	Uid UserID
	Did DiscussionID
}

type SlotUsers []SlotUser

func (su *SlotUsers) Index(uid UserID) (bool, int) {
	for i, v := range *su {
		if v.Uid == uid {
			return true, i
		}
	}
	return false, -1
}

func (su *SlotUsers) Get(uid UserID) (did DiscussionID) {
	prs, i := su.Index(uid)
	if prs {
		did = (*su)[i].Did
	}
	return
}

func (su *SlotUsers) GetPrs(uid UserID) (did DiscussionID, prs bool) {
	p, i := su.Index(uid)
	if p {
		prs = true
		did = (*su)[i].Did
	}
	return
}

func (su *SlotUsers) Set(uid UserID, did DiscussionID) {
	prs, i := su.Index(uid)
	if prs {
		(*su)[i].Did = did
	} else {
		*su = append(*su, SlotUser{Uid: uid, Did: did})
	}
}

func (su *SlotUsers) Delete(uid UserID) {
	prs, i := su.Index(uid)
	if !prs {
		return
	}
	*su = append((*su)[:i], (*su)[i+1:]...)
}

type Slot struct {
	// What are all the concurrent discussions happening during this slot?
	Discussions SlotDiscussions
	Users       SlotUsers

	// Up-pointer to schedule of which we're a part
	sched       *Schedule
}

func (slot *Slot) Clone(sched *Schedule) (nslot *Slot) {
	nslot = &Slot{}
	nslot.Init(sched)
	nslot.Discussions = make([]DiscussionID, len(slot.Discussions))
	copy(nslot.Discussions, slot.Discussions)
	nslot.Users = make([]SlotUser, len(slot.Users))
	copy(nslot.Users, slot.Users)
	return
}

// Change this to os.Stderr to enable
var SchedDebug *log.Logger
var OptSchedDebug = false
var OptSchedDebugVerbose = false

func ScheduleInit() {
	if OptSchedDebug {
		SchedDebug = log.New(os.Stderr, "schedule.go ", log.LstdFlags)
	} else {
		SchedDebug = log.New(ioutil.Discard, "schedule.go ", log.LstdFlags)
	}

	var err error
	OptSearchDuration, err = time.ParseDuration(OptSearchDurationString)
	if err != nil {
		log.Fatalf("Error parsing search time: %v", err)
	}
}

func (slot *Slot) Init(sched *Schedule) {
	slot.Discussions = SlotDiscussions([]DiscussionID{})
	slot.Users = SlotUsers([]SlotUser{})
	slot.sched = sched
}

func (slot *Slot) Assign(disc *Discussion, commit bool) (delta int) {
	if slot.Discussions.IsPresent(disc.ID) {
		return
	}
	for uid := range disc.Interested {
		user, _ := slot.sched.store.Users.Find(uid)

		// How interested is the user in this?
		tInterest, iprs := user.Interest[disc.ID]
		if !iprs {
			log.Fatalf("Internal error: interest not symmetric")
		}
		
		// See if the user is currently scheduled to do something else
		oInterest := 0
		odid, prs := slot.Users.GetPrs(uid)
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
				slot.Users.Set(uid, disc.ID)
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
		slot.Discussions = append(slot.Discussions, disc.ID)
	}

	return
}

func (slot *Slot) Remove(disc *Discussion, commit bool) (delta int) {
	
	if !slot.Discussions.IsPresent(disc.ID) {
		log.Printf("ERROR: Trying to remove non-assigned discussion %s", disc.ID)
		return
	}
	
	if commit {
		slot.Discussions.Delete(disc.ID)
	}

	for uid := range disc.Interested {
		user, _ := slot.sched.store.Users.Find(uid)

		// How interested is the user in this?
		tInterest, iprs := user.Interest[disc.ID]
		if !iprs {
			log.Fatalf("Internal error: interest not symmetric")
		}
		
		// Is this their current favorite? If not, removing it won't have an effect
		if slot.Users.Get(uid) != disc.ID {
			if OptSchedDebugVerbose {
				SchedDebug.Printf("  User %s already going to a different discussion, no change",
					user.Username)
			}
		} else {
			best := struct { interest int; did DiscussionID }{}
			
			// This user is currently going to this discussion.  See
			// if they have somewhere else they want to go.
			for _, did := range slot.Discussions {
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
					slot.Users.Delete(uid)
				}
			} else {
				if OptSchedDebugVerbose {
					SchedDebug.Printf("  User %s %d -> %d (%d)",
						user.Username, tInterest, best.interest, delta)
				}
				if commit {
					slot.Users.Set(uid, disc.ID)
				}
			}
		}
	}
	return
}

var OptValidate = false

func (slot *Slot) DiscussionScore(did DiscussionID) (score, missed int) {
	us, ds := slot.sched.GetStores()
	
	// For every discussion in this slot...
	disc, _ := ds.Find(did)

	for uid := range disc.Interested {
		// Find out how much each user was interested in it
		user, _ := us.Find(uid)

		interest := user.Interest[disc.ID]

		// If they're going, add it to the score;
		// if not, add it to the 'missed' category
		adid := slot.Users.Get(uid) // "Attending" discussion id
		if adid == disc.ID {
			score += interest
		} else {
			// Check to make sure the discussion they're attending is actually more
			ainterest := interest
			if OptValidate {
				ainterest = user.Interest[adid]
			}
			if ainterest < interest {
				adisc, _ := ds.Find(adid)
				log.Printf("User %v attending wrong discussion (%s %d < %s %d)!",
					user.Username, adisc.Title, ainterest, disc.Title, interest)
				score += interest
				slot.Users.Set(uid, disc.ID)
			} else {
				missed += interest
			}
		}
	}
	return
}

func (slot *Slot) DiscussionAttendeeCount(did DiscussionID) (count int) {
	for _, attending := range slot.Users {
		if attending.Did == did {
			count++
		}
	}
	return
}

func (slot *Slot) Score() (score, missed int) {
	for _, did := range slot.Discussions {
		ds, dm := slot.DiscussionScore(did)
		score += ds
		missed += dm
	}
	return
}

func (slot *Slot) RemoveDiscussion(did DiscussionID) error {
	us, _ := slot.sched.GetStores()

	// Delete the discussion from the map
	slot.Discussions.Delete(did)
			
	// Find all users attending this discussion and have them go
	// somewhere else
 	for _, attending := range slot.Users {
		if attending.Did != did {
			continue
		}
		user, _ := us.Find(attending.Uid)
		altDid := DiscussionID("")
		altInterest := 0
		for _, candidateDid := range slot.Discussions {
			if user.Interest[candidateDid] > altInterest {
				altDid = candidateDid
				altInterest = user.Interest[candidateDid]
			}
		}
		if altInterest > 0 {
			// Found something -- change the user to going to this session
			slot.Users.Set(attending.Uid, altDid)
		} else {
			// User isn't interested in anything in this session -- remove them
			slot.Users.Delete(attending.Uid)
		}
	}
	
	return nil
}

/*
 * Schedule state:
 * - StateCurrent
 *   Invariant: schedule matches discussions / users/ preferences, no scheduling process
 *   On modification of above: -> StateStale
 *   On start of scheduling: -> StateInProgress
 * - StateInProgress
 *   Invariant: Local copy of state equivalent to global copy, scheduling routine in process
 *   On modification: -> StateStale
 *   On routine finishing: -> StateCurrent
 * - StateStale
 *   Invariant: Updates since last successful schedule
 *   ON start of sheduling: -> StateInProgress
 */

type ScheduleState int

const (
	StateCurrent    = ScheduleState(0)
	StateStale      = ScheduleState(1)
	StateInProgress = ScheduleState(2)
)

func (state *ScheduleState) Modify() {
	*state = StateStale
}

func (state *ScheduleState) StartSearch() {
	*state = StateInProgress
}

func (state *ScheduleState) SearchSucceeded() {
	if *state == StateInProgress {
		log.Printf("No changes since start of scheduling, marking state clean")
		*state = StateCurrent
	} else {
		log.Printf("Schedule done, but changes made since; not marking state clean")
	}
}

// The search failed.  If the state is still InProgress, restore the state; otherwise,
// leave it be.
func (state *ScheduleState) SearchFailed(oldstate ScheduleState) {
	if *state == StateInProgress {
		log.Printf("No changes since start of scheduling, restoring old state")
		*state = oldstate
	} else {
		log.Printf("Schedule done, but changes made since; not restoring old state")
	}
}


// Pure scheduling: Only slots
type Schedule struct {
	Slots []*Slot
	Created time.Time
	// Snapshot of user / discussion / locked data we can use without holding the lock
	// This is only set while a search is going on
	store *SearchStore
}

func (sched *Schedule) GetStores() (us *UserStore, ds *DiscussionStore) {
	if sched.store != nil {
		ds = &sched.store.Discussions
		us = &sched.store.Users
	} else {
		ds = &Event.Discussions
		us = &Event.Users
	}
	return
}

func (sched *Schedule) Init(slots int, store *SearchStore) {
	sched.Slots = make([]*Slot, slots)
	for i := range sched.Slots {
		sched.Slots[i] = &Slot{}
		sched.Slots[i].Init(sched)
	}
	sched.store = store
}

func (sched *Schedule) LoadPost() {
	// Restore "internal" pointers after load.
	for i := range sched.Slots {
		sched.Slots[i].sched = sched
	}
	// NB that we specificaly do *not* initialize sched.store -- that should
	// only be set during scheduling.
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
	if !OptValidate {
		return nil
	}

	if sched.store != nil {
		log.Panicf("Validate called outside of schedule search!")
	}

	ss := sched.store
	
	// Make sure that this schedule has the following properties:
	// - Every discussion placed exactly once
	// - 'Locked' sessions match current scheduler locked session

	// First, find the location of all the locked discussions in the 'master'
	lockedD := make(map[DiscussionID]int)
	if ss.CurrentSchedule != nil && ss.LockedSlots != nil {
		for i, slot := range ss.CurrentSchedule.Slots {
			if ss.LockedSlots[i] {
				for _, did := range slot.Discussions {
					lockedD[did] = i
				}
			}
		}
	}

	dMap := make(map[DiscussionID]int)
	// Now go through each slot and find where discussions are mapped; checking that
	// 1) No duplicates, and 2) Locked discussions in the right place
	for i, slot := range sched.Slots {
		for _, did := range slot.Discussions {
			t, prs := dMap[did]
			if prs {
				log.Panicf("Found duplicate discussion! %v -> %d, %d",
					did, t, i)
			}
			t, prs = lockedD[did]
			if prs && t != i {
				log.Panicf("Locked discussion moved! %v in slot %d, should be %d",
					did, i, t)
			}
			dMap[did] = i
		}
	}

	// Now go through all the discussions and make sure each one was mapped once
	err := ss.Discussions.Iterate(func(disc *Discussion) error {
		_, prs := dMap[disc.ID]
		if !prs {
			log.Panicf("Discussion %v missing!", disc.ID)
		}
		return nil
	})
	
	return err
}

func (sched *Schedule) RemoveDiscussion(did DiscussionID) error {
	for _, slot := range sched.Slots {
		if slot.Discussions.IsPresent(did) {
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
	ss := sched.store 
	for slotn := range disc.PossibleSlots {
		if disc.PossibleSlots[slotn] && !ss.LockedSlots[slotn] {
			found = true
			break
		}
	}
	if !found {
		log.Fatalf("Discussion %s has no possible slots! %v %v",
			disc.ID, disc.PossibleSlots, ss.LockedSlots)
	}

	for {
		slotIndex := rng.Intn(len(sched.Slots))
		if disc.PossibleSlots[slotIndex] && !ss.LockedSlots[slotIndex] {
			if OptSchedDebug {
				SchedDebug.Printf("  Assigning discussion %v to slot %d", disc.ID, slotIndex)
			}
			sched.Slots[slotIndex].Assign(disc, true)
			break
		}
	}
}

func ScheduleFactoryTemplate(ss *SearchStore) (sched *Schedule) {
	template := ss.CurrentSchedule
	
	sched = &Schedule{}
	sched.Init(len(template.Slots), ss)

	// Clone locked slots
	if ss.LockedSlots != nil {
		for i := range sched.Slots {
			if ss.LockedSlots[i] {
				sched.Slots[i] = template.Slots[i].Clone(sched)
			}
		}
	}

	return
}

func ScheduleFactoryInner(ss *SearchStore, rng *rand.Rand) gago.Genome {
	SchedDebug.Printf("Making new random schedule")

	// Clone the locked discussions
	sched := ScheduleFactoryTemplate(ss)

	// Put each non-locked discussion in a random slot
	for _, disc := range ss.dList {
		SchedDebug.Printf("  Placing discussion `%s`", disc.Title)
		sched.AssignRandom(disc, rng)
	}

	sched.Validate()

	return gago.Genome(sched)
}

func (sched *Schedule) Clone() gago.Genome {
	SchedDebug.Print("Clone")
	new := &Schedule{ store: sched.store }
	for i := range sched.Slots {
		new.Slots = append(new.Slots, sched.Slots[i].Clone(new))
	}

	new.Validate()

	if OptValidate {
		sscore, smissed := sched.Score()
		nscore, nmissed := new.Score()
		if sscore != nscore || smissed != nmissed {
			log.Panicf(" Schedule clone has different score: (%d, %d) != (%d, %d)!",
				sscore, smissed, nscore, nmissed)
		}
	}
	
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
		if sched.store.LockedSlots == nil || !sched.store.LockedSlots[slotn] {
			slot = sched.Slots[slotn]
			return
		}
	}
}

func (sched *Schedule) Mutate(rng *rand.Rand) {
	var sScore int
	
	if OptSchedDebug {
		sScore, _ = sched.Score()
		SchedDebug.Print("Mutate")
	}
	replace := []*Discussion{}

	// Remove a random number of discussions
	rmCount := rng.Intn(len(sched.store.Discussions) * 25 / 100 + 1)
	if rmCount < 1 {
		rmCount = 1
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
		did := slot.Discussions[dnum]
		if OptSchedDebug {
			SchedDebug.Printf(" Removing discussion %v from slot %d",
				did, slotn)
		}
		disc, _ := sched.store.Discussions.Find(did)
		replace = append(replace, disc)
		slot.Remove(disc, true)
	}
	
	// And put them back somewhere else
	for _, disc := range replace {
		sched.AssignRandom(disc, rng)
	}

	sched.Validate()

	if OptSchedDebug {
		eScore, _ := sched.Score()
		if eScore > sScore {
			SchedDebug.Printf("Mutated from %d to %d mplus", sScore, eScore)
		} else {
			SchedDebug.Printf("Mutated from %d to %d", sScore, eScore)
		}
	}
}

var OptCrossover = true

func (sched *Schedule) Crossover(Genome gago.Genome, rng *rand.Rand) {
	if !OptCrossover {
		return
	}

	var sScore int
	if OptSchedDebug {
		sScore, _ = sched.Score()
		SchedDebug.Print("Crossover")
	}
	
	osched := Genome.(*Schedule)
	
	// Keep track of the discussions that don't get placed.  Keep a
	// pointer so we don't have to look it up again when we decide to
	// use it.
	displaced := SlotDiscussions{}

	// For each slot, replace a random number of discussions
	for i := range sched.Slots {
		slot := sched.Slots[i]
		oslot := osched.Slots[i]

		// Nothing to do for slots with no discussions already, or locked slots
		if len(slot.Discussions) == 0 ||
			(Event.LockedSlots != nil && Event.LockedSlots[i]) {
			continue
		}

		maxIndex := 64
		if len(slot.Discussions) < maxIndex {
			maxIndex = len(slot.Discussions)
		}
		
		var remIndexes uint64
		remDisc := []*Discussion{}

		toRemove := rng.Intn(len(slot.Discussions))

		// Choose a random set of "indexes" to remove
		for n := 0; n < toRemove ; n++ {
			for {
				i := uint(rng.Intn(maxIndex))
				if remIndexes & (1 << i) == 0 {
					remIndexes |= 1 << i

					did := slot.Discussions[i]
					disc, _ := Event.Discussions.Find(did)
					displaced.InsertOnce(did)
					remDisc = append(remDisc, disc)
					break
				}
			}
		}

		var addIndexes uint64
		toMove := toRemove
		if toMove > len(oslot.Discussions) {
			toMove = len(oslot.Discussions)
		}
		
		maxIndex = 64
		if len(oslot.Discussions) < maxIndex {
			maxIndex = len(oslot.Discussions)
		}

		// And a set to move from the other parent
		for n := 0; n < toMove ; n++ {
			for {
				i := uint(rng.Intn(maxIndex))
				if addIndexes & (1 << i) == 0 {
					addIndexes |= 1 << i
					break
				}
			}
		}

		// Do the same for the discussions to add, but add the
		// non-added ones to the 'displaced' discussion list
		addDisc := []*Discussion{}
		for i, did := range slot.Discussions {
			disc, _ := Event.Discussions.Find(did)
			if addIndexes & (1 << uint(i)) != 0 {
				addDisc = append(addDisc, disc)
			} else {
				displaced.InsertOnce(did)
			}
		}

		// Now remove the removed ones, add the added ones
		for _, disc := range remDisc {
			slot.Remove(disc, true)
		}
		for _, disc := range addDisc {
			slot.Assign(disc, true)
		}

		// And finally, go through the list cleaning out the 'displaced' list
		for _, did := range slot.Discussions {
			displaced.Delete(did)
		}
	}

	// Finally, put all the displaced discussions somewhere random
	for _, did := range displaced {
		disc, _ := Event.Discussions.Find(did)
		sched.AssignRandom(disc, rng)
	}

	sched.Validate()

	if OptSchedDebug {
		eScore, _ := sched.Score()
		if eScore > sScore {
			SchedDebug.Printf("Crossover from %d to %d cplus", sScore, eScore)
		} else {
			SchedDebug.Printf("Crossover from %d to %d", sScore, eScore)
		}
	}
}

// A snapshot of users and discussions to use while reference while
// searching for an optimum schedule, but not holding the lock.
type SearchStore struct {
	Users UserStore
	Discussions DiscussionStore
	LockedSlots
	ScheduleSlots int
	CurrentSchedule *Schedule
	OldState ScheduleState

	// List of discussions that need to be placed.  NB some schedulers
	// reorder this.
	dList []*Discussion
}

// Collect all relevant data needed to do the search without holding the lock.
// This involves:
// - Having a copy of Event data Users, Events, LockedSlots
// - Holding a ref to the schedule current at the time of the
// - Creating a list of discussions that need to be placed (i.e., not in locked slots)

func (ss *SearchStore) Snapshot(event *EventStore) (err error) {
	if err = event.Users.DeepCopy(&ss.Users); err != nil {
		return
	}
	if err = event.Discussions.DeepCopy(&ss.Discussions); err != nil {
		return
	}
	if event.LockedSlots != nil {
		ss.LockedSlots = append(LockedSlots(nil), event.LockedSlots...)
	}
	ss.ScheduleSlots = event.ScheduleSlots
	ss.CurrentSchedule = event.ScheduleV2
	ss.OldState = event.ScheduleState

	// All the schedule search functions need to know which discussions they need
	// to place (i.e., which ones are not in locked slots).  Make that list once.
	dLocked := make(map[DiscussionID]bool)

	// If a slot is locked:
	// - Copy the old one verbatim into the new schedule
	// - Exclude the discussion in it from placement
	//
	// Also while we're here, check to make sure there are unlocked slots (otherwise
	// we can't really do a schedule.)
	allLocked := true
	if ss.LockedSlots != nil {
		if ss.CurrentSchedule != nil {
			for i := range ss.CurrentSchedule.Slots {
				if ss.LockedSlots[i] {
					for _, did := range ss.CurrentSchedule.Slots[i].Discussions {
						dLocked[did] = true
					}
				} else {
					allLocked = false
				}
			}
		} else {
			// No schedule; but we still need to check for unlocked slots
			for _, l := range ss.LockedSlots {
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

	ss.dList = nil
	for _, disc := range ss.Discussions {
		if !dLocked[disc.ID] {
			ss.dList = append(ss.dList, disc)
		}
	}

	return 
}

func MakeScheduleRandom(ss *SearchStore) (best *Schedule, err error) {
	start := time.Now()
	stop := start.Add(OptSearchDuration)
	
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	best = ScheduleFactoryInner(ss, rng).(*Schedule)
	bestScore, _ := best.Score()
	bestTime := time.Now()

	log.Printf("Random schedule time %v score %d", bestTime.Sub(start), bestScore)
	for time.Now().Before(stop) {
		next := ScheduleFactoryInner(ss, rng).(*Schedule)
		nextScore, _ := next.Score()
		if nextScore > bestScore {
			best = next
			bestScore = nextScore
			bestTime = time.Now()
			log.Printf("Random schedule time %v score %d", bestTime.Sub(start), bestScore)
		}
	}

	return
}

func MakeScheduleGenetic(ss *SearchStore) (*Schedule, error) {
	ScheduleFactory := func(rng *rand.Rand) gago.Genome {
		return ScheduleFactoryInner(ss, rng)
	}

	ga := gago.GA {
		NewGenome: ScheduleFactory,
		NPops:     2,
		PopSize:   200,
		Model: gago.ModDownToSize{
			NOffsprings: 100,
			SelectorA: gago.SelElitism{},
			SelectorB: gago.SelElitism{},
			MutRate:   0.5,
			CrossRate: 0.7,
		},
	}
	ga.Logger = SchedDebug

	start := time.Now()
	stop := start.Add(OptSearchDuration)


	if err := ga.Initialize(); err != nil {
		log.Printf("Error initalizing ga: %v", err)
		return nil, err
	}

	bestScore := ga.HallOfFame[0].Fitness
	bestTime := time.Now()
	log.Printf("Genetic schedule time %v generations %v score %v\n", bestTime.Sub(start),
		ga.Generations, bestScore)
	for time.Now().Before(stop) {
        if err := ga.Evolve(); err != nil {
            log.Println("Error evolving: %v", err)
			return nil, err
        }
		if ga.HallOfFame[0].Fitness < bestScore {
			bestScore = ga.HallOfFame[0].Fitness
			bestTime = time.Now()
			log.Printf("Genetic schedule time %v generations %v score %v\n", bestTime.Sub(start),
				ga.Generations, bestScore)
		}
    }

	return ga.HallOfFame[0].Genome.(*Schedule), nil
}

func MakeScheduleHeuristic(ss *SearchStore) (*Schedule, error) {
	// Clone the locked discussions from the template
	sched := ScheduleFactoryTemplate(ss)
	
	// Sort discussion list by interest, high to low
	dListMaxIsLess := func(i, j int) bool {
		return ss.dList[i].GetMaxScore() < ss.dList[j].GetMaxScore()
	}
	sort.Slice(ss.dList, dListMaxIsLess)

	// Starting at the top, look for a slot to put it in which will maximize this score
	for _, disc := range ss.dList {
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

type SearchAlgo string
const (
	SearchHeuristicOnly = SearchAlgo("heuristic")
	SearchGenetic = SearchAlgo("genetic")
	SearchRandom = SearchAlgo("random")
)

var OptSearchAlgo string
var OptSearchDurationString string
var OptSearchDuration time.Duration

// async indicates that MakeScheduleAsync should attempt to grab
// the event mutex before updating the schedule
func MakeScheduleAsync(ss *SearchStore, algo SearchAlgo, async bool) {
	var Hscore, Hmissed, Sscore, Smissed int
	var new, newS *Schedule
	var err error

	newH, err := MakeScheduleHeuristic(ss)
	if err != nil {
		log.Printf("Schedule generator failed: %v", err)
		goto out
	}

	Hscore, Hmissed = newH.Score()

	switch algo {
	case SearchGenetic:
		newS, err = MakeScheduleGenetic(ss)
	case SearchRandom:
		newS, err = MakeScheduleRandom(ss)
	}
	
	if err != nil {
		log.Printf("Schedule generator failed: %v", err)
		goto out
	}

	if newS != nil {
		Sscore, Smissed = newS.Score()
	}

	log.Printf("Heuristic schedule happiness: %d, sadness %d", Hscore, Hmissed)
	log.Printf("Search (%s) schedule happiness: %d, sadness %d", algo, Sscore, Smissed)

	new = newH
	if Sscore > Hscore {
		new = newS
	}
	
	new.Created = time.Now()

	// Temporary stores only used during search
	new.store = nil

out:
	if async {
		log.Printf("MakeScheduleAsync: Grabbing mutex")
		lock.Lock()
		defer lock.Unlock()
	}

	if new != nil {
		Event.ScheduleV2 = new
		Event.Timetable.Place(new)
		Event.ScheduleState.SearchSucceeded()
	} else {
		Event.ScheduleState.SearchFailed(ss.OldState)
	}
	
	Event.Save()
	
	return
}

func MakeSchedule(algo SearchAlgo, async bool) (error) {
	if Event.ScheduleState == StateInProgress {
		return errInProgress
	}
	
	ss := &SearchStore{}

	if err := ss.Snapshot(&Event); err != nil {
		return err
	}

	Event.ScheduleState.StartSearch()

	if async {
		go MakeScheduleAsync(ss, algo, async)
	} else {
		MakeScheduleAsync(ss, algo, async)
	}

	return nil
}

