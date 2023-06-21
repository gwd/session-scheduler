package event

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"

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

	// PossibleSlots []bool

	// Is this discussion publicly visible?
	// If true, 'Title' and 'Description' should be shown to everyone.
	// If false:
	//   admin and owner should see 'Title' and 'Description'
	//   Everyone else should either see 'Approved*', or nothing at all (if nothing has been approved)
	IsPublic bool
}

type DiscussionFull struct {
	Discussion
	OwnerInfo     User
	Location      Location
	Time          Time
	IsFinal       bool
	PossibleSlots []DisplaySlot
}

func (d *Discussion) GetURL() string {
	return "/uid/discussion/" + string(d.DiscussionID) + "/view"
}

// FIXME
const maxDiscussionsPerUser = 12

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

	return txLoop(func(eq sqlx.Ext) error {
		count := 0
		err := sqlx.Get(eq, &count,
			`select count(*) from event_discussions where owner=?`,
			disc.Owner)
		if err != nil {
			return errOrRetry("Getting discussion count for user", err)
		}

		if count >= maxDiscussionsPerUser {
			// Admins aren't affected by the discussion limit.
			isAdmin := false
			err := sqlx.Get(eq, &isAdmin,
				`select isadmin from event_users where userid=?`,
				owner)
			if err != nil {
				return errOrRetry("Getting isadmin for user", err)
			}
			if !isAdmin {
				return errTooManyDiscussions
			}
		}

		disc.DiscussionID.generate()

		// New discussions are non-public by default unless owner is verified
		err = sqlx.Get(eq, &disc.IsPublic,
			`select isverified from event_users where userid = ?`,
			owner)
		if err != nil {
			return err
		}

		if disc.IsPublic {
			disc.ApprovedTitle = disc.Title
			disc.ApprovedDescription = disc.Description
		} else {
			disc.ApprovedTitle = ""
			disc.ApprovedDescription = ""
		}
		_, err = eq.Exec(
			`insert into event_discussions values (?, ?, ?, ?, ?, ?, ?)`,
			disc.DiscussionID, disc.Owner, disc.Title, disc.Description,
			disc.ApprovedTitle, disc.ApprovedDescription,
			disc.IsPublic)
		if err != nil {
			return err
		}

		// Owners are assumed to want to attend their own session
		return setInterestTx(eq, disc.Owner, disc.DiscussionID, InterestMax)
	})
}

// GetMaxScore returns the maximum possible score a discussion could
// have if everyone attended; that is, the sum of all the interests
// expressed.
func (d *Discussion) GetMaxScore() (int, error) {
	var maxscore int
	// Theoretically the owner should always have non-zero interest,
	// so sum(interest) should never be NULL; but better to be robust.
	for {
		err := event.Get(&maxscore, `
            select IFNULL(sum(interest), 0)
                from event_interest
                where discussionid = ?`,
			d.DiscussionID)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
			log.Printf("INTERNAL ERROR: Getting max score for discussion %v: %v",
				d.DiscussionID, err)
			return 0, err
		default:
			return maxscore, err
		}
	}
}

func (d *Discussion) Location() Location {
	// FIXME: Location
	return Location{}
}

func (d *Discussion) Slot() (IsFinal bool, Time string) {
	// FIXME: Slot
	return false, ""
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

	return txLoop(func(eq sqlx.Ext) error {
		var curOwner UserID
		var ownerIsVerified bool
		row := eq.QueryRowx(
			`select owner, isverified
                 from event_discussions
                   join event_users on owner = userid
                 where discussionid = ?`, disc.DiscussionID)
		err := row.Scan(&curOwner, &ownerIsVerified)
		if err != nil {
			return errOrRetry("Getting info for owner of discussion", err)
		}

		// NB: We don't check for number owned discussions. Only the
		// admin can re-assign a discussion; they can be allowed the
		// latitude.

		if disc.Owner != curOwner {
			err := setInterestTx(eq, disc.Owner, disc.DiscussionID, InterestMax)
			if err != nil {
				return errOrRetry("Setting interest for new owner", err)
			}

			// If we're changing owner, we need to see whether the new owner is verified
			err = sqlx.Get(eq, &ownerIsVerified,
				`select isverified from event_users where userid = ?`,
				disc.Owner)
			if err != nil {
				return errOrRetry("Getting IsVerified for new owner", err)
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

		_, err = eq.Exec(q, args...)
		return err
	})
}

// No entries at all means all entries are OK
func possibleSlotsDBToDisplay(pslots []DisplaySlot) {
	anyTrue := false
	for i := range pslots {
		anyTrue = anyTrue || pslots[i].Checked
	}
	if !anyTrue {
		for i := range pslots {
			pslots[i].Checked = true
		}
	}
}

func DiscussionSetPossibleSlots(discussionid DiscussionID, pslots []SlotID) error {
	checked := []struct {
		DiscussionID DiscussionID
		SlotID       SlotID
	}{}

	// Make a list of all possible slots
	for i := range pslots {
		checked = append(checked, struct {
			DiscussionID DiscussionID
			SlotID       SlotID
		}{
			DiscussionID: discussionid,
			SlotID:       pslots[i],
		})
	}

	err := txLoop(func(eq sqlx.Ext) error {
		var nslots int
		err := sqlx.Get(eq, &nslots, `
            select count(*) from event_slots where isbreak = false`)
		if err != nil {
			return errOrRetry("Getting number of slots", err)
		}

		// Always drop all restrictions
		_, err = eq.Exec(`
            delete from event_discussions_possible_slots
                where discussionid=?`, discussionid)
		if err != nil {
			return errOrRetry("Dropping discussion slot restrictions", err)
		}

		// Only need to add any back if there are negative values
		if len(checked) < nslots {
			_, err = sqlx.NamedExec(eq, `
                insert into event_discussions_possible_slots
                    values(:discussionid, :slotid)`, checked)
			if err != nil {
				return errOrRetry("Adding new slots", err)
			}
		}

		return nil
	})

	return err
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
	var query, errlogfmt string
	if public {
		query = `
        update event_discussions
            set (approvedtitle, approveddescription, ispublic)
            = (select title, description, true
                   from event_discussions
                   where discussionid = :did)
            where discussionid = :did`
		errlogfmt = "Setting event discussion %v public: %v"
	} else {
		query = `
        update event_discussions
            set ispublic = false,
                approvedtitle = "",
                approveddescription = ""
            where discussionid = ?`
		errlogfmt = "Setting event discussion %v non-public: %v"
	}

	for {
		res, err := event.Exec(query, discussionid)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
			log.Printf(errlogfmt, discussionid, err)
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

func deleteDiscussionCommon(eq sqlx.Ext, where string, arg interface{}) (int64, error) {
	_, err := eq.Exec(`
           delete from event_interest where `+where,
		arg)
	if err != nil {
		return 0, errOrRetry("Deleting discussion from event_interest", err)
	}

	_, err = eq.Exec(`
           delete from event_discussions_possible_slots where `+where, arg)
	if err != nil {
		return 0, errOrRetry("Deleting discussion from event_discussions_possible_slots", err)
	}

	_, err = eq.Exec(`
           delete from event_schedule where `+where, arg)
	if err != nil {
		return 0, errOrRetry("Deleting discussion from event_schedule", err)
	}

	res, err := eq.Exec(`
            delete from event_discussions
                where `+where, arg)
	if err != nil {
		return 0, errOrRetry("Deleting discussion from event_discussions", err)
	}

	rcount, err := res.RowsAffected()
	if err != nil {
		log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
	}

	return rcount, nil
}

func DeleteDiscussion(did DiscussionID) error {
	log.Printf("Deleting discussion %s", did)

	return txLoop(func(eq sqlx.Ext) error {
		rcount, err := deleteDiscussionCommon(eq, "discussionid = ?", did)

		if err == nil {
			switch {
			case rcount == 0:
				return ErrDiscussionNotFound
			case rcount > 1:
				log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
				return ErrInternal
			}
		}

		return err
	})

}

func MakePossibleSlots(len int) []bool {
	pslots := make([]bool, len)
	for i := range pslots {
		pslots[i] = true
	}
	return pslots
}

func discussionGetPossibleSlotsTx(q sqlx.Queryer, did DiscussionID, psp *[]DisplaySlot) error {
	err := sqlx.Select(q, psp, `
		    select slotid,
		           slottime,
		           (discussionid is not null) as checked
		        from event_slots natural left outer join
                     (select * 
                        from event_discussions_possible_slots
		                where discussionid=?)
                where isbreak = false
                order by dayid, slotidx`, did)
	if err == nil {
		possibleSlotsDBToDisplay(*psp)
	} else if err == sql.ErrNoRows {
		err = nil
	}
	return err
}

func DiscussionGetPossibleSlots(did DiscussionID) ([]DisplaySlot, error) {
	var ds []DisplaySlot
	err := txLoop(func(eq sqlx.Ext) error {
		err := discussionGetPossibleSlotsTx(eq, did, &ds)
		if err != nil {
			return errOrRetry("Getting possible slots for discussion", err)
		}
		return nil
	})

	return ds, err
}

func discussionFindByIdFullTx(q sqlx.Queryer, did DiscussionID) (*DiscussionFull, error) {
	var disc *DiscussionFull
	err := txLoop(func(eq sqlx.Ext) error {
		disc = &DiscussionFull{}
		err := sqlx.Get(eq, disc,
			`select * from event_discussions where discussionid = ?`,
			did)
		if err == sql.ErrNoRows {
			disc = nil
			return nil
		} else if err != nil {
			return errOrRetry("Getting discussion data", err)
		}

		err = userGetTx(eq, disc.Owner, &disc.OwnerInfo)
		if err == sql.ErrNoRows {
			return fmt.Errorf("INTERNAL ERROR: Could not get info for owner %v!", disc.Owner)
		} else if err != nil {
			return errOrRetry("Getting discussion owner info", err)
		}

		// Get the schedule info
		row := eq.QueryRowx(`
            select locationid,
                   locationname,
                   locationurl,
                   isplace,
                   capacity,
                   slottime,
                   islocked
                from event_locations
                    natural join event_schedule
                    natural join event_slots
                where discussionid=?`, disc.DiscussionID)
		err = row.Scan(&disc.Location.LocationID,
			&disc.Location.LocationName,
			&disc.Location.LocationURL,
			&disc.Location.IsPlace,
			&disc.Location.Capacity,
			&disc.Time,
			&disc.IsFinal)
		if err != nil && err != sql.ErrNoRows {
			return errOrRetry("Getting schedule information for discussion", err)
		}

		err = discussionGetPossibleSlotsTx(q, disc.DiscussionID, &disc.PossibleSlots)
		if err != nil {
			return errOrRetry("Getting possible slots for discussion", err)
		}

		return nil
	})
	return disc, err
}

func DiscussionFindByIdFull(discussionid DiscussionID) (*DiscussionFull, error) {
	return discussionFindByIdFullTx(event.DB, discussionid)
}

func discussionIterateQuery(query string, args []interface{}, f func(*DiscussionFull) error) error {
	return txLoop(func(eq sqlx.Ext) error {
		// First, get a list of all the appropriate discussion IDs
		dids := []DiscussionID{}
		err := sqlx.Select(eq, &dids, query, args...)
		if err != nil {
			return errOrRetry("Getting list of discussions to process", err)
		}

		for _, did := range dids {
			// For each discussion, load up "full" discussion information...
			disc, err := discussionFindByIdFullTx(eq, did)
			if err != nil {
				return err
			}
			err = f(disc)
			if err != nil {
				// FIXME: This may return a retry, in which case
				// "accumulative" operations will get duplicate
				// entries
				return err
			}
		}
		return nil
	})
}

func DiscussionIterate(f func(*DiscussionFull) error) error {
	return discussionIterateQuery(`select discussionid from event_discussions order by discussionid`, nil, f)
}

// FIXME: This will simply do nothing if the userid doesn't exist.  It
// would be nice for the caller to distinguish between "User does not
// exist" and "User has no discussions".
func DiscussionIterateUser(userid UserID, f func(*DiscussionFull) error) (err error) {
	return discussionIterateQuery(
		`select discussionid from event_discussions where owner=? order by discussionid`,
		[]interface{}{userid}, f)
}
