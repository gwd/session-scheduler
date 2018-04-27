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
	Interested   map[UserID]bool

	// Things to add at some point:
	// Session Length (30m, 1hr, &c)
	// Invitees?
}

// Annotated for display to an individual user
type DiscussionDisplay struct {
	ID          DiscussionID
	Title       string
	Description string
	Owner       *User
	Interested  []*User
	IsMe        bool
	AmAttending bool
}

func (d *Discussion) GetURL() string {
	return "/discussion/by-id/" + string(d.ID) + "/view"
}

func (d *Discussion) GetDisplay(cur *User) *DiscussionDisplay {
	dd := &DiscussionDisplay{
		ID:          d.ID,
		Title:       d.Title,
		Description: d.Description,
	}
	dd.Owner, _ = Event.Users.Find(d.Owner)
	if cur != nil && dd.Owner.ID == cur.ID {
		dd.IsMe = true
	}
	for uid := range d.Interested {
		a, _ := Event.Users.Find(uid)
		if a != nil {
			dd.Interested = append(dd.Interested, a)
			if cur != nil && a.ID == cur.ID {
				dd.AmAttending = true
			}
		}
	}
	return dd
}

func NewDiscussion(owner *User, title, description string) (*Discussion, error) {
	disc := &Discussion{
		Owner:       owner.ID,
		Title:       title,
		Description: description,
	}

	log.Printf("Got new discussion: '%s' '%s' '%s'",
		string(owner.ID), title, description)

	if title == "" {
		return disc, errNoTitle
	}

	// FIXME: Check for duplicate titles

	if description == "" {
		return disc, errNoDesc
	}

	disc.ID.generate()

	disc.Interested = make(map[UserID]bool)

	owner.SetInterest(disc, 100)

	err := Event.Discussions.Save(disc)

	return disc, err
}

func DiscussionFindById(id string) (*Discussion, error) {
	return Event.Discussions.Find(id)
}

func DiscussionGetList(cur *User) (list []*DiscussionDisplay) {
	return Event.Discussions.GetList(cur)
}
