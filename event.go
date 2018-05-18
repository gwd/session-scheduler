package main

import (
	"encoding/json"
	"fmt"
	"log"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
)

type EventStore struct {
	Users       UserStore
	Discussions DiscussionStore
	Locations   LocationStore
	Schedule    *Schedule
	Timetable   Timetable
	ScheduleSlots int
	LockedSlots []bool
	TestMode    bool
	VerificationCode string
	filename    string
}

type EventOptions struct {
	Slots            int
	AdminPassword    string
	VerificationCode string
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
	store.Locations.Init()
	store.Schedule = nil

	store.ScheduleSlots = opt.Slots
	store.LockedSlots = make([]bool, opt.Slots)

	store.VerificationCode = opt.VerificationCode
	if store.VerificationCode == "" {
		store.VerificationCode = GenerateRawID(8)
	}

	// Create the admin user
	pwd := opt.AdminPassword
	if pwd == "" {
		pwd = GenerateRawID(12)
		log.Printf("Administrator account: admin %s", pwd)
	}
	admin, err := NewUser(AdminUsername, pwd, Event.VerificationCode,
		&UserProfile{ RealName: "Xen Schedule Administrator" })
	if err != nil {
		log.Fatalf("Error creating admin user: %v", err)
	}
	admin.IsAdmin = true
	Event.Users.Save(admin)
}

// Reset "event" data, without touching users or discussions
func (store *EventStore) ResetEventData() {
	store.Timetable.Init()
	store.Locations.Init()
	store.Schedule = nil
	store.LockedSlots = make([]bool, store.ScheduleSlots)

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
				Slots: DefaultSlots,
				AdminPassword: OptAdminPassword})
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

	// Run timetable placement to update discussion info
	if Event.Schedule != nil {
		Event.Timetable.Place(Event.Schedule)
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

func (ustore UserStore) FindRandom() (user * User, err error) {
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

func (ustore UserStore) Iterate(f func (u *User) error) (err error) {
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
	// FIXME: Sort?
	return
}

func (ustore UserStore) GetUsersDisplay(cur *User) (users []*UserDisplay) {
	ustore.Iterate(func(u *User) error {
		if u.Username != AdminUsername {
			users = append(users, u.GetDisplay(cur, false))
		}
		return nil
	})
	// FIXME: Sort?
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

type DiscussionStore map[DiscussionID]*Discussion

func (dstore *DiscussionStore) Init() {
	*dstore = DiscussionStore(make(map[DiscussionID]*Discussion))
}

func (dstore DiscussionStore) Find(id DiscussionID) (*Discussion, error) {
	discussion, exists := (dstore)[id]
	if !exists {
		return nil, nil
	}

	return discussion, nil
}

func (dstore DiscussionStore) Iterate(f func (d *Discussion) error) (err error) {
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
	return Event.Save()
}

func (dstore DiscussionStore) GetListUser(u *User, cur *User) (list []*DiscussionDisplay) {
	dstore.Iterate(func (d *Discussion) error {
		if d.Owner == u.ID {
			dd := d.GetDisplay(cur)
			if dd != nil {
				list = append(list, dd)
			}
		}
		return nil
	})
	return
}

func (dstore DiscussionStore) GetList(cur *User) (list []*DiscussionDisplay) {
	dstore.Iterate(func (d *Discussion) error {
		dd := d.GetDisplay(cur)
		if dd != nil {
			list = append(list, dd)
		}
		return nil
	})
	return
}

