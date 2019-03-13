package main

import (
	"html/template"
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
	ID            DiscussionID
	Owner         UserID
	Title         string
	Description   string
	Interested    map[UserID]bool
	PossibleSlots []bool
	IsPublic      bool // Is this discussion publicly visible?

	// Cached information from a schedule
	location *Location
	slot     *TimetableSlot

	// Things to add at some point:
	// Session Length (30m, 1hr, &c)
	// Invitees?

	maxScore      int
	maxScoreValid bool
}

// Annotated for display to an individual user

type DisplaySlot struct {
	Label   string
	Index   int
	Checked bool
}

type DiscussionDisplay struct {
	ID             DiscussionID
	Title          string
	Description    template.HTML
	DescriptionRaw string
	Owner          *User
	Interested     []*User
	IsPublic       bool
	// IsUser: Used to determine whether to display 'interest'
	IsUser bool
	// MayEdit: Used to determine whether to show edit / delete buttons
	MayEdit bool
	// IsAdmin: Used to determine whether to show slot scheduling options
	IsAdmin       bool
	Interest      int
	Location      Location
	Time          string
	IsFinal       bool
	PossibleSlots []DisplaySlot
	AllUsers      []*User
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
	// Only display a discussion if:
	// 1. It's pulbic, or...
	// 2. The current user is admin, or the discussion owner
	if !d.IsPublic &&
		(cur == nil || (!cur.IsAdmin && cur.ID != d.Owner)) {
		return nil
	}

	dd := &DiscussionDisplay{
		ID:             d.ID,
		Title:          d.Title,
		DescriptionRaw: d.Description,
		Description:    ProcessText(d.Description),
		IsPublic:       d.IsPublic,
	}

	if d.location != nil {
		dd.Location = *d.location
	}

	if d.slot != nil {
		dd.IsFinal = d.slot.day.IsFinal
		dd.Time = d.slot.day.DayName + " " + d.slot.Time
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
			dd.PossibleSlots = Event.Timetable.FillDisplaySlots(d.PossibleSlots)
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

	log.Printf("Update discussion post: '%s'", title)

	if title == "" {
		log.Printf("Update discussion failed: no title", title)
		return &out, errNoTitle
	}

	if description == "" {
		log.Print("Update discussion failed: no description")
		return &out, errNoDesc
	}

	disc.Title = title
	disc.Description = description

	if pSlots != nil {
		disc.PossibleSlots = pSlots
		Event.ScheduleState.Modify()
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

	// Editing a discussion takes it non-public unless the owner is verified.
	owner, _ := Event.Users.Find(disc.Owner)
	disc.IsPublic = (owner != nil && owner.IsVerified)

	err := Event.Discussions.Save(disc)

	return disc, err
}

func DeleteDiscussion(did DiscussionID) {
	log.Printf("Deleting discussion %s", did)

	// Remove it from the schedule before removing it from user list
	// so we still have the 'Interest' value in case we decide to
	// maintain a score at a given time.
	if Event.ScheduleV2 != nil {
		Event.ScheduleV2.RemoveDiscussion(did)

		// Removing a discussion means updating attendees, and
		// possibly moving rooms as well.  Run the placement again.
		Event.Timetable.Place(Event.ScheduleV2)
	}

	UserRemoveDiscussion(did)

	Event.Discussions.Delete(did)
}

func DiscussionRemoveUser(uid UserID) error {
	return Event.Discussions.Iterate(func(d *Discussion) error {
		delete(d.Interested, uid)
		return nil
	})
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
		Owner:         owner.ID,
		Title:         title,
		Description:   description,
		PossibleSlots: MakePossibleSlots(Event.ScheduleSlots),
	}

	log.Printf("%s New discussion post: '%s'",
		owner.Username, title)

	if title == "" {
		log.Printf("%s New discussion failed: no title",
			owner.Username, title)
		return disc, errNoTitle
	}

	if description == "" {
		log.Printf("%s New discussion failed: no description",
			owner.Username)
		return disc, errNoDesc
	}

	// Check for duplicate titles and too many discussions (admins are exempt)
	count := 0
	err := Event.Discussions.Iterate(func(check *Discussion) error {
		if check.Title == title {
			log.Printf("%s New discussion failed: duplicate title",
				owner.Username)
			return errTitleExists
		}
		if !owner.IsAdmin && disc.Owner == check.Owner {
			count++
			// Normal users are not allowed to propose more
			// discussions than they can personally attend
			if count > Event.ScheduleSlots {
				log.Printf("%s New discussion failed: Too many discussions (%d)",
					owner.Username, count)
				return errTooManyDiscussions
			}
		}
		return nil
	})
	if err != nil {
		return disc, err
	}

	disc.ID.generate()

	disc.Interested = make(map[UserID]bool)

	// SetInterest will mark the schedule stale
	owner.SetInterest(disc, 100)

	// New discussions are non-public by default unless owner is verified
	disc.IsPublic = owner.IsVerified

	return disc, Event.Discussions.Save(disc)
}

func DiscussionFindById(id string) (*Discussion, error) {
	return Event.Discussions.Find(DiscussionID(id))
}

func DiscussionGetList(cur *User) (list []*DiscussionDisplay) {
	return Event.Discussions.GetList(cur)
}
