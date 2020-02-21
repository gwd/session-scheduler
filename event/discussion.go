package event

import (
	"log"

	"github.com/gwd/session-scheduler/id"
)

const (
	discussionIDLength = 20
)

type DiscussionID string

func (did *DiscussionID) generate() {
	*did = DiscussionID(id.GenerateID("disc", discussionIDLength))
}

type Discussion struct {
	ID    DiscussionID
	Owner UserID

	Title               string
	Description         string
	ApprovedTitle       string
	ApprovedDescription string

	Interested    map[UserID]bool
	PossibleSlots []bool

	// Is this discussion publicly visible?
	// If true, 'Title' and 'Description' should be shown to everyone.
	// If false:
	//   admin and owner should see 'Title' and 'Description'
	//   Everyone else should either see 'Approved*', or nothing at all (if nothing has been approved)
	IsPublic bool

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
			user, err := event.Users.Find(uid)
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

func (d *Discussion) Location() Location {
	if d.location != nil {
		return *d.location
	}
	return Location{}
}

func (d *Discussion) Slot() (IsFinal bool, Time string) {
	if d.slot != nil {
		IsFinal = d.slot.day.IsFinal
		Time = d.slot.day.DayName + " " + d.slot.Time
	}
	return IsFinal, Time
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
		event.ScheduleState.Modify()
	}

	if newOwnerID != "" && newOwnerID != disc.Owner {
		newOwner, _ := event.Users.Find(newOwnerID)
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
	owner, _ := event.Users.Find(disc.Owner)
	disc.IsPublic = (owner != nil && owner.IsVerified)

	err := event.Discussions.Save(disc)

	return disc, err
}

func DiscussionSetPublic(uid string, public bool) error {
	d, err := DiscussionFindById(uid)
	if err != nil {
		return err
	}
	if public {
		// When making something public, keep track of the
		// "approved" value
		d.IsPublic = true
		d.ApprovedTitle = d.Title
		d.ApprovedDescription = d.Description
	} else {
		// To actually hide something, the ApprovedTitle needs
		// to be false as well.
		d.IsPublic = false
		d.ApprovedTitle = ""
		d.ApprovedDescription = ""
	}
	event.Discussions.Save(d)

	return nil
}

func DeleteDiscussion(did DiscussionID) {
	log.Printf("Deleting discussion %s", did)

	// Remove it from the schedule before removing it from user list
	// so we still have the 'Interest' value in case we decide to
	// maintain a score at a given time.
	if event.ScheduleV2 != nil {
		event.ScheduleV2.RemoveDiscussion(did)

		// Removing a discussion means updating attendees, and
		// possibly moving rooms as well.  Run the placement again.
		event.Timetable.Place(event.ScheduleV2)
	}

	UserRemoveDiscussion(did)

	event.Discussions.Delete(did)
}

func DiscussionRemoveUser(uid UserID) error {
	return event.Discussions.Iterate(func(d *Discussion) error {
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
		PossibleSlots: MakePossibleSlots(event.ScheduleSlots),
	}

	log.Printf("%s New discussion post: '%s'",
		owner.Username, title)

	if title == "" || AllWhitespace(title) {
		log.Printf("%s New discussion failed: no title",
			owner.Username)
		return disc, errNoTitle
	}

	if description == "" || AllWhitespace(description) {
		log.Printf("%s New discussion failed: no description",
			owner.Username)
		return disc, errNoDesc
	}

	// Check for duplicate titles and too many discussions (admins are exempt)
	count := 0
	err := event.Discussions.Iterate(func(check *Discussion) error {
		if check.Title == title {
			log.Printf("%s New discussion failed: duplicate title",
				owner.Username)
			return errTitleExists
		}
		if !owner.IsAdmin && disc.Owner == check.Owner {
			count++
			// Normal users are not allowed to propose more
			// discussions than they can personally attend
			if count > event.ScheduleSlots {
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

	return disc, event.Discussions.Save(disc)
}

func DiscussionFindById(id string) (*Discussion, error) {
	return event.Discussions.Find(DiscussionID(id))
}
