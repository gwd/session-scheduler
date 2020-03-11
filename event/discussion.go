package event

import (
	"database/sql"
	"fmt"
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

// display.go:DiscussionGetDisplay()
//
// !IsPublic && ApprovedTitle == ""
//  -> Real title only available to admin or owner, hidden to everyone else
// !IsPublic && ApprovedTitle != ""
//  -> Real title shown to admin or owner, approved title to everyone else
// IsPublic
//  -> Real title to everyone
//
// SetPublic(true)
// - IsPublic = true, ApprovedX = X
// SetPublic(false)
// - IsPublic = false, ApprovedX = ""
//
// DiscussionUpdate(), owner.IsVerified
// - IsPublic = true, ApprovedX = newX
// DiscussionUpdate(), !owner.IsVerified
// -
//
// discussion.html:item-full
// IsPublic: Shows alert, "This discussion has changes awaiting moderation"
// "SetPublic" checkbox
//

type Discussion struct {
	DiscussionID DiscussionID
	Owner        UserID

	Title               string
	Description         string
	ApprovedTitle       string
	ApprovedDescription string

	// Interested    map[UserID]bool
	// PossibleSlots []bool

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
	return "/uid/discussion/" + string(d.DiscussionID) + "/view"
}

// FIXME
const maxDiscussionsPerUser = 5

func checkDiscussionParams(disc *Discussion) error {
	if disc.Title == "" || AllWhitespace(disc.Title) {
		log.Printf("%s New/Update discussion failed: no title",
			disc.Owner)
		return errNoTitle
	}

	if disc.Description == "" || AllWhitespace(disc.Description) {
		log.Printf("%s New/Update discussion failed: no description",
			disc.Owner)
		return errNoDesc
	}
	return nil
}

// Restrictions:
// - Can't already have too many discussions
// - Title can't be empty
// - Description can't be empty
// - Title unique (enforced by SQL)
func NewDiscussion(disc *Discussion) error {
	owner := disc.Owner

	log.Printf("%s New discussion post: '%s'",
		owner, disc.Title)

	if err := checkDiscussionParams(disc); err != nil {
		return err
	}

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		count := 0
		err = tx.Get(&count,
			`select count(*) from event_discussions where owner=?`,
			disc.Owner)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Getting discussion count for user %v: %v",
				disc.Owner, err)
		}

		if count >= maxDiscussionsPerUser {
			return errTooManyDiscussions
		}

		disc.DiscussionID.generate()

		//disc.Interested = make(map[UserID]bool)

		// SetInterest will mark the schedule stale
		//owner.SetInterest(disc, 100)

		// New discussions are non-public by default unless owner is verified
		err = tx.Get(&disc.IsPublic,
			`select isverified from event_users where userid = ?`,
			owner)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		if disc.IsPublic {
			disc.ApprovedTitle = disc.Title
			disc.ApprovedDescription = disc.Description
		} else {
			disc.ApprovedTitle = ""
			disc.ApprovedDescription = ""
		}
		_, err = tx.Exec(
			`insert into event_discussions values (?, ?, ?, ?, ?, ?, ?)`,
			disc.DiscussionID, disc.Owner, disc.Title, disc.Description,
			disc.ApprovedTitle, disc.ApprovedDescription,
			disc.IsPublic)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		return nil
	}
}

func (d *Discussion) GetMaxScore() int {
	if !d.maxScoreValid {
		// FIXME: Interest
		d.maxScore = 0
		// for uid := range d.Interested {
		// 	if !d.Interested[uid] {
		// 		log.Fatalf("INTERNAL ERROR: Discussion %s Interested[%s] false!",
		// 			d.ID, uid)
		// 	}
		// 	user, err := event.Users.Find(uid)
		// 	if err != nil {
		// 		log.Fatalf("Finding user %s: %v", uid, err)
		// 	}
		// 	interest, prs := user.Interest[d.ID]
		// 	if !prs {
		// 		log.Fatalf("INTERNAL ERROR: User %s has no interest in discussion %s",
		// 			user.ID, d.ID)
		// 	}
		// 	d.maxScore += interest
		// }
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

// Updates discussion's Title, Description, and Owner.
//
// If the owner (or the new owner, if that's being changed) is verifed, then
// IsPublic will be set to 'true', and ApprovedTitle and ApprovedDescription will be set from
// Title and Description as well.
//
// If the owner (or new owner) is not verified, then IsPublic will be
// set to false, and only Title and Description will be modified.
func DiscussionUpdate(disc *Discussion) error {
	log.Printf("Update discussion post: '%s'", disc.Title)

	if err := checkDiscussionParams(disc); err != nil {
		return err
	}

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		var curOwner UserID
		var ownerIsVerified bool
		row := tx.QueryRow(
			`select owner, isverified
                 from event_discussions
                   join event_users on owner = userid
                 where discussionid = ?`, disc.DiscussionID)
		err = row.Scan(&curOwner, &ownerIsVerified)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Getting info for owner of discussion %v: %v",
				disc.DiscussionID, err)
		}

		// NB: We don't check for number owned discussions. Only the
		// admin can re-assign a discussion; they can be allowed the
		// latitude.

		if disc.Owner != curOwner {
			// FIXME: Interest
			// newOwner, _ := event.Users.Find(newOwnerID)
			// if newOwner != nil {
			// 	// All we need to do is set the owner's interest to max,
			// 	// and set the new owner.
			// 	newOwner.SetInterest(disc, InterestMax)
			// 	disc.Owner = newOwnerID
			// } else {
			// 	log.Printf("Ignoring non-existing user %v", newOwnerID)
			// }

			// If we're changing owner, we need to see whether the new owner is verified
			err := tx.Get(&ownerIsVerified,
				`select isverified from event_users where userid = ?`,
				disc.Owner)
			if shouldRetry(err) {
				tx.Rollback()
				continue
			} else if err != nil {
				return fmt.Errorf("Getting IsVerified for new owner %v: %v",
					disc.Owner, err)
			}
		}

		// Editing a discussion takes it non-public unless the owner is verified.
		disc.IsPublic = ownerIsVerified

		q :=
			`update event_discussions set
                 owner = ?,
                 title = ?,
                 description = ?,
                 ispublic = ?`
		args := []interface{}{disc.Owner, disc.Title, disc.Description, disc.IsPublic}

		if disc.IsPublic {
			disc.ApprovedTitle = disc.Title
			disc.ApprovedDescription = disc.Description
			q += `,
                 approvedtitle = ?,
                 approveddescription = ?`
			args = append(args, disc.ApprovedTitle)
			args = append(args, disc.ApprovedDescription)
		}

		q += `where discussionid = ?`
		args = append(args, disc.DiscussionID)

		_, err = tx.Exec(q, args...)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}

		return nil
	}
}

// Sets the given discussion ID to public or private.
//
// If public is true, it copies the title and description into the
// "approved" title and description, so that it will be visible even
// after being modified.
//
// If public is false, it hides the discussion entirely, by both
// setting 'IsPublic' to false, but also clearing the approved title
// and description.
func DiscussionSetPublic(discussionid DiscussionID, public bool) error {
	if public {
		res, err := event.Exec(`
        update event_discussions
            set (approvedtitle, approveddescription, ispublic)
            = (select title, description, true
                   from event_discussions
                   where discussionid = :did)
            where discussionid = :did`,
			discussionid)
		if err != nil {
			log.Printf("Setting event discussion %v public: %v", discussionid, err)
			return ErrInternal
		}
		rcount, err := res.RowsAffected()
		if err != nil {
			log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
		}
		switch {
		case rcount == 0:
			return ErrUserNotFound
		case rcount > 1:
			log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
			return ErrInternal
		}
		return nil

	} else {
		res, err := event.Exec(`
        update event_discussions
            set ispublic = false,
                approvedtitle = "",
                approveddescription = ""
            where discussionid = ?`,
			discussionid)
		if err != nil {
			log.Printf("Setting event discussion %v non-public: %v", discussionid, err)
			return ErrInternal
		}
		rcount, err := res.RowsAffected()
		if err != nil {
			log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
		}
		switch {
		case rcount == 0:
			return ErrUserNotFound
		case rcount > 1:
			log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
			return ErrInternal
		}
		return nil
	}
}

func DeleteDiscussion(did DiscussionID) error {
	log.Printf("Deleting discussion %s", did)

	// Remove it from the schedule before removing it from user list
	// so we still have the 'Interest' value in case we decide to
	// maintain a score at a given time.
	// if event.ScheduleV2 != nil {
	// 	event.ScheduleV2.RemoveDiscussion(did)

	// 	// Removing a discussion means updating attendees, and
	// 	// possibly moving rooms as well.  Run the placement again.
	// 	event.Timetable.Place(event.ScheduleV2)
	// }

	// FIXME: Interest
	// UserRemoveDiscussion(did)

	res, err := event.Exec(`
    delete from event_discussions
        where discussionid = ?`, did)
	if err != nil {
		return err
	}

	rcount, err := res.RowsAffected()
	if err != nil {
		log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
	}
	switch {
	case rcount == 0:
		return ErrDiscussionNotFound
	case rcount > 1:
		log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
		return ErrInternal
	}
	return nil
}

func MakePossibleSlots(len int) []bool {
	pslots := make([]bool, len)
	for i := range pslots {
		pslots[i] = true
	}
	return pslots
}

func DiscussionFindById(discussionid DiscussionID) (*Discussion, error) {
	disc := &Discussion{}
	err := event.Get(disc,
		`select * from event_discussions where discussionid = ?`,
		discussionid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return disc, err
}
