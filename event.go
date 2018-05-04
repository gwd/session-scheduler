package main

import (
	"encoding/json"
	"fmt"
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

var Event EventStore

const (
	StoreFilename = "data/event.json"
	DefaultSlots = 10
)

func (store *EventStore) Init(slots int) {
	store.Users.Init()
	store.Discussions.Init()
	store.ScheduleSlots = slots
	store.Schedule = nil
}

func (store *EventStore) Load() error {
	if store.filename == "" {
		store.filename = StoreFilename
	}
	contents, err := ioutil.ReadFile(store.filename)

	if err != nil {
		if os.IsNotExist(err) {
			store.Init(DefaultSlots)
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
	l := len(ustore)

	if l == 0 {
		err = fmt.Errorf("No users!")
		return
	}

	// Choose a random value from [0,l) and 
	i := rand.Int31n(int32(l))
	for _, user = range ustore {
		if i == 0 {
			return
		}
		i--
	}
	// We shouldn't be able to get here, but the compiler doesn't know that
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
