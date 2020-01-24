package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/gwd/session-scheduler/id"
)

var deepCopyBuffer bytes.Buffer
var deepCopyEncoder = gob.NewEncoder(&deepCopyBuffer)
var deepCopyDecoder = gob.NewDecoder(&deepCopyBuffer)

type EventStore struct {
	ServeAddress string
	filename     string

	TestMode             bool
	Active               bool
	ScheduleActive       bool
	VerificationCodeSent bool
	RequireVerification  bool

	Timetable Timetable

	VerificationCode string

	Users       UserStore
	Discussions DiscussionStore
	Locations   LocationStore

	ScheduleV2    *Schedule
	ScheduleSlots int
	LockedSlots

	ScheduleState
}

type EventOptions struct {
	AdminPassword    string
	VerificationCode string
	ServeAddress     string
}

var Event EventStore

const (
	StoreFilename = "data/event.json"
	AdminUsername = "admin"
	// 3 days * 3 slots per day
	DefaultSlots = 9
)

var OptAdminPassword string

func (store *EventStore) Init(opt EventOptions) {
	store.Users.Init()
	store.Discussions.Init()
	store.Timetable.Init()
	store.ScheduleSlots = store.Timetable.GetSlots()

	store.Locations.Init()
	store.ScheduleV2 = nil

	store.LockedSlots = make([]bool, store.ScheduleSlots)

	store.VerificationCode = opt.VerificationCode
	if store.VerificationCode == "" {
		store.VerificationCode = id.GenerateRawID(8)
	}

	// Create the admin user
	pwd := opt.AdminPassword
	if pwd == "" {
		pwd = id.GenerateRawID(12)
		log.Printf("Administrator account: admin %s", pwd)
	}
	admin, err := NewUser(AdminUsername, pwd, Event.VerificationCode,
		&UserProfile{RealName: "Xen Schedule Administrator"})
	if err != nil {
		log.Fatalf("Error creating admin user: %v", err)
	}
	admin.IsAdmin = true
	Event.Users.Save(admin)

	Event.ServeAddress = opt.ServeAddress
	if Event.ServeAddress == "" {
		// Generate a raw port between 1024 and 32768
		Event.ServeAddress = fmt.Sprintf("localhost:%d",
			rand.Int31n(32768-1024)+1024)
	}

	Event.Save()
}

// Reset "event" data, without touching users or discussions
func (store *EventStore) ResetEventData() {
	store.Timetable.Init()
	store.ScheduleSlots = store.Timetable.GetSlots()

	store.Locations.Init()
	store.ScheduleV2 = nil
	store.LockedSlots = make([]bool, store.ScheduleSlots)
	store.Discussions.ResetEventData()

	Event.Save()
}

// Reset "user" data -- users, discussions, and interest (keeping admin user).
// This also resets the 'event' data, as it won't make much sense anymore with the
// users and discussions gone.
// This should only be done in test mode!
func (store *EventStore) ResetUserData() {
	admin, err := store.Users.FindByUsername(AdminUsername)
	if err != nil || admin == nil {
		log.Fatal("Can't find admin user: %v", err)
	}

	store.Users.Init()
	store.Discussions.Init()
	store.ResetEventData()

	Event.Users.Save(admin)
}

func (store *EventStore) Load() error {
	if store.filename == "" {
		store.filename = StoreFilename
	}
	contents, err := ioutil.ReadFile(store.filename)

	if err != nil {
		if os.IsNotExist(err) {
			store.Init(EventOptions{
				AdminPassword: OptAdminPassword,
				ServeAddress:  OptServeAddress})
			return nil
		}
		return err
	}
	err = json.Unmarshal(contents, store)
	if err != nil {
		return err
	}

	// Someone has request resetting the admin password
	if OptAdminPassword != "" {
		admin, err := store.Users.FindByUsername(AdminUsername)
		if err != nil || admin == nil {
			log.Fatal("Can't find admin user: %v", err)
		}
		log.Printf("Resetting admin password")
		admin.SetPassword(OptAdminPassword)
	}

	if OptServeAddress != "" && OptServeAddress != Event.ServeAddress {
		log.Printf("Changing default serve address to %s",
			OptServeAddress)
		Event.ServeAddress = OptServeAddress
		Event.Save()
	}

	// Clean up any stale 'running' data
	if Event.ScheduleState.IsRunning() {
		Event.ScheduleState.SearchFailed()
	}

	// Run timetable placement to update discussion info
	if Event.ScheduleV2 != nil {
		Event.ScheduleV2.LoadPost()
		Event.Timetable.Place(Event.ScheduleV2)
	}
	return nil
}

func (store *EventStore) Save() error {
	contents, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(store.filename, contents, 0660)
}

type LockedSlots []bool

func (ls *LockedSlots) Set(new LockedSlots) {
	*ls = new
	Event.Timetable.UpdateIsFinal(new)
}

type UserStore map[UserID]*User

func (ustore *UserStore) Init() {
	*ustore = UserStore(make(map[UserID]*User))
}

func (ustore UserStore) Save(user *User) error {
	ustore[user.ID] = user
	return Event.Save()
}

func (ustore UserStore) Find(id UserID) (*User, error) {
	user, ok := ustore[id]
	if ok {
		return user, nil
	}
	return nil, nil
}

func (ustore UserStore) FindRandom() (user *User, err error) {
	// Don't count the admin user
	l := len(ustore) - 1

	if l == 0 {
		err = fmt.Errorf("No users!")
		return
	}

	// Choose a random value from [0,l) and
	i := rand.Int31n(int32(l))
	for _, user = range ustore {
		if user.Username == AdminUsername {
			continue
		}
		if i == 0 {
			return
		}
		i--
	}
	// We shouldn't be able to get here, but the compiler doesn't know that
	return
}

func (ustore UserStore) Iterate(f func(u *User) error) (err error) {
	for _, user := range ustore {
		err = f(user)
		if err != nil {
			return
		}
	}
	return
}

func (ustore UserStore) GetUsers() (users []*User) {
	ustore.Iterate(func(u *User) error {
		if u.Username != AdminUsername {
			users = append(users, u)
		}
		return nil
	})

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})

	return
}

func (ustore UserStore) GetUsersDisplay(cur *User) (users []*UserDisplay) {
	ustore.Iterate(func(u *User) error {
		if u.Username != AdminUsername {
			users = append(users, u.GetDisplay(cur, false))
		}
		return nil
	})

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})
	return
}

func (ustore UserStore) FindByUsername(username string) (*User, error) {
	if username == "" {
		return nil, nil
	}

	for _, user := range ustore {
		if strings.ToLower(username) == strings.ToLower(user.Username) {
			return user, nil
		}
	}
	return nil, nil
}

func (ustore UserStore) FindByEmail(email string) (*User, error) {
	if email == "" {
		return nil, nil
	}

	for _, user := range ustore {
		if strings.ToLower(email) == strings.ToLower(user.Profile.Email) {
			return user, nil
		}
	}
	return nil, nil
}

func (ustore *UserStore) DeepCopy(ucopy *UserStore) (err error) {
	if err = deepCopyEncoder.Encode(ustore); err != nil {
		return err
	}
	if err = deepCopyDecoder.Decode(ucopy); err != nil {
		return err
	}
	return nil
}

func (ustore UserStore) Delete(uid UserID) error {
	delete(ustore, uid)
	Event.ScheduleState.Modify()
	return Event.Save()
}

func (dstore *DiscussionStore) DeepCopy(dcopy *DiscussionStore) (err error) {
	if err = deepCopyEncoder.Encode(dstore); err != nil {
		return err
	}
	if err = deepCopyDecoder.Decode(dcopy); err != nil {
		return err
	}
	return nil
}

type DiscussionStore map[DiscussionID]*Discussion

func (dstore *DiscussionStore) Init() {
	*dstore = DiscussionStore(make(map[DiscussionID]*Discussion))
}

// Update PossibleSlot size while retaining other information
func (dstore *DiscussionStore) ResetEventData() {
	dstore.Iterate(func(disc *Discussion) error {
		disc.PossibleSlots = MakePossibleSlots(Event.ScheduleSlots)
		return nil
	})
}

func (dstore DiscussionStore) Find(id DiscussionID) (*Discussion, error) {
	discussion, exists := (dstore)[id]
	if !exists {
		return nil, nil
	}

	return discussion, nil
}

func (dstore DiscussionStore) Iterate(f func(d *Discussion) error) (err error) {
	for _, disc := range dstore {
		err = f(disc)
		if err != nil {
			return
		}
	}
	return
}

func (dstore DiscussionStore) Save(discussion *Discussion) error {
	dstore[discussion.ID] = discussion
	return Event.Save()
}

func (dstore DiscussionStore) Delete(did DiscussionID) error {
	delete(dstore, did)
	Event.ScheduleState.Modify()
	return Event.Save()
}

func (dstore DiscussionStore) GetListUser(u *User, cur *User) (list []*DiscussionDisplay) {
	dstore.Iterate(func(d *Discussion) error {
		if d.Owner == u.ID {
			dd := d.GetDisplay(cur)
			if dd != nil {
				list = append(list, dd)
			}
		}
		return nil
	})

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return
}

func (dstore DiscussionStore) GetDidListUser(uid UserID) (list []DiscussionID) {
	dstore.Iterate(func(d *Discussion) error {
		if d.Owner == uid {
			list = append(list, d.ID)
		}
		return nil
	})

	return
}

func (dstore DiscussionStore) GetList(cur *User) (list []*DiscussionDisplay) {
	dstore.Iterate(func(d *Discussion) error {
		dd := d.GetDisplay(cur)
		if dd != nil {
			list = append(list, dd)
		}
		return nil
	})

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return
}
