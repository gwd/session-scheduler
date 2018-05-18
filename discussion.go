package main

import (
	"log"
	"html/template"
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
	PossibleSlots []bool

	// Cached information from a schedule
	location    *Location
	time        string
	
	// Things to add at some point:
	// Session Length (30m, 1hr, &c)
	// Invitees?

	maxScore    int
	maxScoreValid bool
}

// Annotated for display to an individual user

type DisplaySlot struct {
	Label string
	Index int
	Possible bool
}

type DiscussionDisplay struct {
	ID          DiscussionID
	Title       string
	Description template.HTML
	DescriptionRaw string
	Owner       *User
	Interested  []*User
	// IsUser: Used to determine whether to display 'interest'
	IsUser      bool
	// MayEdit: Used to determine whether to show edit / delete buttons
	MayEdit     bool
	// IsAdmin: Used to determine whether to show slot scheduling options
	IsAdmin     bool
	Interest int
	Location    Location
	Time        string
	PossibleSlots []DisplaySlot
	AllUsers   []*User
}

func (d *Discussion) GetURL() string {
	return "/uid/discussion/" + string(d.ID) + "/view"
}

func (d *Discussion) GetMaxScore() int {
	if !d.maxScoreValid {
		d.maxScore = 0
		for uid := range d.Interested {
			if !d.Interested[uid] {
				log.Fatalf("INTERNAL ERROR: Discussion %s Interested[%s] false!",
					d.ID, uid)
			}
			user, err := Event.Users.Find(uid)
			if err != nil {
				log.Fatalf("Finding user %s: %v", uid, err)
			}
			interest, prs := user.Interest[d.ID]
			if !prs {
				log.Fatalf("INTERNAL ERROR: User %s has no interest in discussion %s",
					user.ID, d.ID)
			}
			d.maxScore += interest
		}
		d.maxScoreValid = true
	}

	return d.maxScore
}

func (d *Discussion) GetDisplay(cur *User) *DiscussionDisplay {
	dd := &DiscussionDisplay{
		ID:          d.ID,
		Title:       d.Title,
		DescriptionRaw: d.Description,
		Description:  ProcessText(d.Description),
		Time: d.time,
	}

	if d.location != nil {
		dd.Location = *d.location
	}
	
	dd.Owner, _ = Event.Users.Find(d.Owner)
	if cur != nil {
		if cur.Username != AdminUsername {
			dd.IsUser = true
			dd.Interest = cur.Interest[d.ID]
		}
		dd.MayEdit = cur.MayEditDiscussion(d)
		if cur.IsAdmin {
			dd.IsAdmin = true
			dd.PossibleSlots = Event.Timetable.FillPossibleSlots(d.PossibleSlots)
			dd.AllUsers = Event.Users.GetUsers()
		}
	}
	for uid := range d.Interested {
		a, _ := Event.Users.Find(uid)
		if a != nil {
			dd.Interested = append(dd.Interested, a)
		}
	}
	return dd
}

func UpdateDiscussion(disc *Discussion, title, description string, pSlots []bool,
	newOwnerID UserID) (*Discussion, error) {
	out := *disc

	out.Title = title
	out.Description = description
	
	if title == "" {
		return &out, errNoTitle
	}

	if description == "" {
		return &out, errNoDesc
	}

	disc.Title = title
	disc.Description = description

	if pSlots != nil {
		disc.PossibleSlots = pSlots
	}

	if newOwnerID != "" && newOwnerID != disc.Owner {
		newOwner, _ := Event.Users.Find(newOwnerID)
		if newOwner != nil {
			// All we need to do is set the owner's interest to max,
			// and set the new owner.
			newOwner.SetInterest(disc, InterestMax)
			disc.Owner = newOwnerID
		} else {
			log.Printf("Ignoring non-existing user %v", newOwnerID)
		}
	}
	
	err := Event.Discussions.Save(disc)

	return disc, err
}

func DeleteDiscussion(did DiscussionID) {
	// Remove it from the schedule before removing it from user list
	// so we still have the 'Interest' value in case we decide to
	// maintain a score at a given time.
	if Event.Schedule != nil {
		Event.Schedule.RemoveDiscussion(did)

		// Removing a discussion means updating attendees, and
		// possibly moving rooms as well.  Run the placement again.
		Event.Timetable.Place(Event.Schedule)
	}
	
	UserRemoveDiscussion(did)
	
	Event.Discussions.Delete(did)
}

func MakePossibleSlots(len int) []bool {
	pslots := make([]bool, len)
	for i := range pslots {
		pslots[i] = true
	}
	return pslots
}

func NewDiscussion(owner *User, title, description string) (*Discussion, error) {
	disc := &Discussion{
		Owner:       owner.ID,
		Title:       title,
		Description: description,
		PossibleSlots: MakePossibleSlots(Event.ScheduleSlots),
	}

	log.Printf("Got new discussion: '%s' '%s' '%s'",
		string(owner.ID), title, description)

	if title == "" {
		return disc, errNoTitle
	}

	// Check for duplicate titles and too many discussions
	count := 0
	err := Event.Discussions.Iterate(func(check* Discussion) error {
		if check.Title == title {
			return errTitleExists
		}
		if disc.Owner == check.Owner {
			count++
			if count > Event.ScheduleSlots {
				return errTooManyDiscussions
			}
		}
		return nil
	})
	if err != nil {
		return disc, err
	}
		
	if description == "" {
		return disc, errNoDesc
	}

	disc.ID.generate()

	disc.Interested = make(map[UserID]bool)

	// SetInterest will mark the schedule stale
	owner.SetInterest(disc, 100)

	err = Event.Discussions.Save(disc)

	return disc, err
}

func DiscussionFindById(id string) (*Discussion, error) {
	return Event.Discussions.Find(DiscussionID(id))
}

func DiscussionGetList(cur *User) (list []*DiscussionDisplay) {
	return Event.Discussions.GetList(cur)
}
