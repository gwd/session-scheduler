package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

var globalDiscussionStore *FileDiscussionStore

func init() {
	store, err := NewFileDiscussionStore("./data/discussions.json")
	if err != nil {
		panic(fmt.Errorf("Error creating discussion store: %s", err))
	}
	globalDiscussionStore = store
}

type FileDiscussionStore struct {
	filename string
	Discussions map[string]*Discussion
}

func NewFileDiscussionStore(name string) (*FileDiscussionStore, error) {
	store := &FileDiscussionStore{
		Discussions: map[string]*Discussion{},
		filename: name,
	}

	contents, err := ioutil.ReadFile(name)

	if err != nil {
		// If it's a matter of the file not existing, that's ok
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	err = json.Unmarshal(contents, store)
	if err != nil {
		return nil, err
	}
	return store, err
}

func (s *FileDiscussionStore) Find(id string) (*Discussion, error) {
	discussion, exists := s.Discussions[id]
	if !exists {
		return nil, nil
	}

	return discussion, nil
}

func (store *FileDiscussionStore) Save(discussion *Discussion) error {
	store.Discussions[string(discussion.ID)] = discussion
	contents, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(store.filename, contents, 0660)
}

func (store *FileDiscussionStore) Delete(discussion *Discussion) error {
	delete(store.Discussions, string(discussion.ID))
	contents, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(store.filename, contents, 0660)
}
