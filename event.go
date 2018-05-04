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
	Schedule    *Schedule
	ScheduleSlots int
	filename    string
}

type EventOptions struct {
	Slots         int
	AdminPassword string
}

var Event EventStore

const (
	StoreFilename = "data/event.json"
	AdminUsername = "admin"
	DefaultSlots = 10
)

var OptAdminPassword string

func (store *EventStore) Init(opt EventOptions) {
	store.Users.Init()
	store.Discussions.Init()
	store.Schedule = nil

	store.ScheduleSlots = opt.Slots

	// Create the admin user
	pwd := opt.AdminPassword
	if pwd == "" {
		pwd = GenerateID("pwd", 12)
		log.Printf("Administrator account: admin %s", pwd)
	}
	admin, err := NewUser(AdminUsername, pwd, &UserProfile{ RealName: "Xen Schedule Administrator" })
	if err != nil {
		log.Fatalf("Error creating admin user: %v", err)
	}
	admin.IsAdmin = true
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

func (ustore UserStore) GetUsers() (users []*User) {
	for _, u := range ustore {
		if u.Username != AdminUsername {
			users = append(users, u)
		}
	}
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

func (dstore DiscussionStore) Save(discussion *Discussion) error {
	dstore[discussion.ID] = discussion
	return Event.Save()
}

func (dstore DiscussionStore) Delete(discussion *Discussion) error {
	delete(dstore, discussion.ID)
	return Event.Save()
}

func (dstore DiscussionStore) GetList(cur *User) (list []*DiscussionDisplay) {
	for _, d := range dstore {
		dd := d.GetDisplay(cur)
		if dd != nil {
			list = append(list, dd)
		}
	}
	return
}
