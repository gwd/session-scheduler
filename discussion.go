package main

import (
	"log"
)

const (
	discussionIDLength = 20
)

type DiscussionID string

func (did *DiscussionID) generate() {
	*did = DiscussionID(GenerateID("disc", discussionIDLength))
}

type Discussion struct {
	ID          DiscussionID
	Owner       UserID
	Title       string
	Description string
	Attendees   []UserID

	// Things to add at some point:
	// Session Length (30m, 1hr, &c)
	// Invitees?
}

func (d *Discussion) GetURL() string {
	return "/discussion/by-id/" + string(d.ID) + "/view"
}

func NewDiscussion(owner UserID, title, description string) (*Discussion, error) {
	disc := &Discussion{
		Owner: owner,
		Title: title,
		Description: description,
	}

	log.Printf("Got new discussion: '%s' '%s' '%s'",
		string(owner), title, description)

	if title == "" {
		return disc, errNoTitle
	}

	// FIXME: Check for duplicate titles

	if description == "" {
		return disc, errNoDesc
	}
	
	disc.ID.generate()

	disc.Attendees = append(disc.Attendees, owner)

	globalDiscussionStore.Save(disc)
	
	return disc, nil
}

func DiscussionFindById(id string) (*Discussion, error) {
	return globalDiscussionStore.Find(id)
}
