package event

import (
	"log"
	"math/rand"
	"sort"
	"time"
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
	if !d.IsPresent(did) {
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
	sched *Schedule
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
			if opt.DebugLevel > 1 {
				opt.Debug.Printf("  User %s %d -> %d (+%d)",
					user.Username, oInterest, tInterest, tInterest-oInterest)
			}
		} else if oInterest > 0 && opt.DebugLevel > 1 {
			opt.Debug.Printf("  User %s will stay where they are (%d > %d)",
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
			if opt.DebugLevel > 1 {
				opt.Debug.Printf("  User %s already going to a different discussion, no change",
					user.Username)
			}
		} else {
			best := struct {
				interest int
				did      DiscussionID
			}{}

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
				if opt.DebugLevel > 1 {
					opt.Debug.Printf("  User %s has no other discussions of interest",
						user.Username)
				}
				if commit {
					slot.Users.Delete(uid)
				}
			} else {
				if opt.DebugLevel > 1 {
					opt.Debug.Printf("  User %s %d -> %d (%d)",
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

func (slot *Slot) DiscussionScore(did DiscussionID) (score, missed int) {
	us, ds := slot.sched.GetStores()

	// For every discussion in this slot...
	disc, _ := ds.Find(did)

	for uid := range disc.Interested {
		// Find out how much each user was interested in it
		user, _ := us.Find(uid)
		if user == nil {
			log.Printf("INTERNAL ERROR: disc.ID %v has interest from non-existent user %v",
				disc.ID, uid)
			continue
		}

		interest := user.Interest[disc.ID]

		// If they're going, add it to the score;
		// if not, add it to the 'missed' category
		adid := slot.Users.Get(uid) // "Attending" discussion id
		if adid == disc.ID {
			score += interest
		} else {
			// Check to make sure the discussion they're attending is actually more
			ainterest := interest
			if opt.Validate {
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

type ScheduleState struct {
	ChangesSinceLastSchedule      bool
	ChangesSinceSchedulerSnapshot bool
	SchedulerRunning              bool
}

func (state *ScheduleState) Modify() {
	state.ChangesSinceLastSchedule = true
	if state.SchedulerRunning {
		state.ChangesSinceSchedulerSnapshot = true
	}
}

func (state *ScheduleState) StartSearch() {
	state.ChangesSinceSchedulerSnapshot = false
	state.SchedulerRunning = true
}

func (state *ScheduleState) SearchSucceeded() {
	state.SchedulerRunning = false
	if !state.ChangesSinceSchedulerSnapshot {
		state.ChangesSinceLastSchedule = false
	}
	state.ChangesSinceSchedulerSnapshot = false
}

// The search failed, either due to an error or being cancelled..  If
// the state is still InProgress, restore the state; otherwise, leave
// it be.
func (state *ScheduleState) SearchFailed() {
	state.SchedulerRunning = false
	state.ChangesSinceSchedulerSnapshot = false
}

func (state *ScheduleState) IsRunning() bool {
	return state.SchedulerRunning
}

func (state *ScheduleState) IsModified() bool {
	return state.ChangesSinceLastSchedule
}

// Pure scheduling: Only slots
type Schedule struct {
	Slots   []*Slot
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

func (sched *Schedule) Init(store *SearchStore) {
	sched.Slots = make([]*Slot, store.ScheduleSlots)
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

func (sched *Schedule) Validate() error {
	if !opt.Validate {
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
			if opt.DebugLevel > 0 {
				opt.Debug.Printf("  Assigning discussion %v to slot %d", disc.ID, slotIndex)
			}
			sched.Slots[slotIndex].Assign(disc, true)
			break
		}
	}
}

func ScheduleFactoryTemplate(ss *SearchStore) (sched *Schedule) {
	sched = &Schedule{}
	sched.Init(ss)

	// Clone locked slots
	if ss.LockedSlots != nil && ss.CurrentSchedule != nil {
		for i := range sched.Slots {
			if ss.LockedSlots[i] {
				sched.Slots[i] = ss.CurrentSchedule.Slots[i].Clone(sched)
			}
		}
	}

	return
}

func ScheduleFactoryInner(ss *SearchStore, rng *rand.Rand) (sched *Schedule) {
	opt.Debug.Printf("Making new random schedule")

	// Clone the locked discussions
	sched = ScheduleFactoryTemplate(ss)

	// Put each non-locked discussion in a random slot
	for _, disc := range ss.dList {
		opt.Debug.Printf("  Placing discussion `%s`", disc.Title)
		sched.AssignRandom(disc, rng)
	}

	sched.Validate()

	return
}

func (sched *Schedule) Clone() (new *Schedule) {
	opt.Debug.Print("Clone")
	new = &Schedule{store: sched.store}
	for i := range sched.Slots {
		new.Slots = append(new.Slots, sched.Slots[i].Clone(new))
	}

	new.Validate()

	if opt.Validate {
		sscore, smissed := sched.Score()
		nscore, nmissed := new.Score()
		if sscore != nscore || smissed != nmissed {
			log.Panicf(" Schedule clone has different score: (%d, %d) != (%d, %d)!",
				sscore, smissed, nscore, nmissed)
		}
	}

	return
}

func (sched *Schedule) Evaluate() (float64, error) {
	opt.Debug.Print("Evaluate")
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

	if opt.DebugLevel > 0 {
		sScore, _ = sched.Score()
		opt.Debug.Print("Mutate")
	}
	replace := []*Discussion{}

	// Remove a random number of discussions
	rmCount := rng.Intn(len(sched.store.Discussions)*25/100 + 1)
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
		if opt.DebugLevel > 0 {
			opt.Debug.Printf(" Removing discussion %v from slot %d",
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

	if opt.DebugLevel > 0 {
		eScore, _ := sched.Score()
		if eScore > sScore {
			opt.Debug.Printf("Mutated from %d to %d mplus", sScore, eScore)
		} else {
			opt.Debug.Printf("Mutated from %d to %d", sScore, eScore)
		}
	}
}

// A snapshot of users and discussions to use while reference while
// searching for an optimum schedule, but not holding the lock.
type SearchStore struct {
	Users       UserStore
	Discussions DiscussionStore
	LockedSlots
	ScheduleSlots   int
	CurrentSchedule *Schedule

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
	stop := start.Add(opt.SearchDuration)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	best = ScheduleFactoryInner(ss, rng)
	bestScore, _ := best.Score()
	bestTime := time.Now()

	log.Printf("Random schedule time %v score %d", bestTime.Sub(start), bestScore)
	for time.Now().Before(stop) {
		next := ScheduleFactoryInner(ss, rng)
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
		opt.Debug.Printf("Scheduling discussion %s (max score %d)",
			disc.Title, disc.GetMaxScore())

		// Find the slot that increases the score the most
		best := struct{ score, index int }{score: 0, index: -1}
		for i := range sched.Slots {
			opt.Debug.Printf(" Evaluating slot %d", i)
			if Event.LockedSlots[i] {
				opt.Debug.Printf("  Locked, skipping")
				continue
			}
			if !disc.PossibleSlots[i] {
				opt.Debug.Printf("  Impossible, skipping")
				continue
			}
			// OK, how much will we increase the score by putting this discussion here?
			score := sched.Slots[i].Assign(disc, false)
			opt.Debug.Printf("  Total value: %d", score)
			if score > best.score {
				best.score = score
				best.index = i
			}
		}

		// If we've found a slot, put it there
		if best.index < 0 {
			// FIXME: Do something useful (least-busy slot?)
			opt.Debug.Printf(" Can't find a good slot!")
		} else {
			opt.Debug.Printf(" Putting discussion in slot %d",
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
	SearchGenetic       = SearchAlgo("genetic")
	SearchRandom        = SearchAlgo("random")
)

func makeScheduleAsync(ss *SearchStore) {
	var Hscore, Hmissed, Sscore, Smissed int
	var new, newS *Schedule
	var err error

	newH, err := MakeScheduleHeuristic(ss)
	if err != nil {
		log.Printf("Schedule generator failed: %v", err)
		goto out
	}

	Hscore, Hmissed = newH.Score()

	switch opt.Algo {
	case SearchRandom:
		newS, err = MakeScheduleRandom(ss)
	default:
		log.Printf("ERROR: Unknown search algorithm %s", opt.Algo)
		goto out
	}

	if err != nil {
		log.Printf("Schedule generator failed: %v", err)
		goto out
	}

	if newS != nil {
		Sscore, Smissed = newS.Score()
	}

	log.Printf("Heuristic schedule happiness: %d, sadness %d", Hscore, Hmissed)
	log.Printf("Search (%s) schedule happiness: %d, sadness %d", opt.Algo, Sscore, Smissed)

	new = newH
	if Sscore > Hscore {
		new = newS
	}

	new.Created = time.Now()

	// Temporary stores only used during search
	new.store = nil

out:
	if new != nil {
		Event.ScheduleV2 = new
		Event.Timetable.Place(new)
		Event.ScheduleState.SearchSucceeded()
	} else {
		Event.ScheduleState.SearchFailed()
	}

	Event.Save()

	return
}

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
	if Event.ScheduleState.IsRunning() {
		return errInProgress
	}

	// FIXME: Ignore async for now

	err := Event.Discussions.Iterate(func(disc *Discussion) error {
		if !disc.IsPublic {
			return errModeratedDiscussions
		}
		return nil
	})

	if err != nil {
		return err
	}

	ss := &SearchStore{}

	if err := ss.Snapshot(&Event); err != nil {
		return err
	}

	Event.ScheduleState.StartSearch()

	opt = optArg

	makeScheduleAsync(ss)

	return nil
}
