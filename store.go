package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
)

type ScheduleStore struct {
	Users       UserStore
	Discussions DiscussionStore
	filename    string
}

var Schedule ScheduleStore

const StoreFilename = "data/schedule.json"

func (store *ScheduleStore) Load() error {
	if store.filename == "" {
		store.filename = StoreFilename
	}
	contents, err := ioutil.ReadFile(store.filename)

	if err != nil {
		if os.IsNotExist(err) {
			store.Users.Init()
			store.Discussions.Init()
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

func (store *ScheduleStore) Save() error {
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
	return Schedule.Save()
}

func (ustore UserStore) Find(id UserID) (*User, error) {
	user, ok := ustore[id]
	if ok {
		return user, nil
	}
	return nil, nil
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
		if strings.ToLower(email) == strings.ToLower(user.Email) {
			return user, nil
		}
	}
	return nil, nil
}

type DiscussionStore map[DiscussionID]*Discussion

func (dstore *DiscussionStore) Init() {
	*dstore = DiscussionStore(make(map[DiscussionID]*Discussion))
}

func (dstore DiscussionStore) Find(id string) (*Discussion, error) {
	discussion, exists := (dstore)[DiscussionID(id)]
	if !exists {
		return nil, nil
	}

	return discussion, nil
}

func (dstore DiscussionStore) Save(discussion *Discussion) error {
	dstore[discussion.ID] = discussion
	return Schedule.Save()
}

func (dstore DiscussionStore) Delete(discussion *Discussion) error {
	delete(dstore, discussion.ID)
	return Schedule.Save()
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
