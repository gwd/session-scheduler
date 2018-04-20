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

// Annotated for display to an individual user
type DiscussionDisplay struct {
	ID  DiscussionID
	Title string
	Description string
	Owner   *User
	Attendees []*User
	IsMe bool
	AmAttending bool
}


func (d *Discussion) GetURL() string {
	return "/discussion/by-id/" + string(d.ID) + "/view"
}

func (d *Discussion) GetDisplay(cur *User) *DiscussionDisplay {
	dd := &DiscussionDisplay{
		ID: d.ID,
		Title: d.Title,
		Description: d.Description,
	}
	dd.Owner, _ = globalUserStore.Find(d.Owner)
	if cur != nil && dd.Owner.ID == cur.ID {
		dd.IsMe = true
	}
	for i := range d.Attendees {
		a, _ := globalUserStore.Find(d.Attendees[i])
		if a != nil {
			dd.Attendees = append(dd.Attendees, a)
			if cur != nil && a.ID == cur.ID {
				dd.AmAttending = true
			}
		}
	}
	return dd
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

func DiscussionGetList(cur *User) (list []*DiscussionDisplay) {
	for _, d := range globalDiscussionStore.Discussions {
		dd := d.GetDisplay(cur)
		if dd != nil {
			list = append(list, dd)
		}
	}
	return
}
