package event

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"

	"github.com/gwd/session-scheduler/id"
)

type TimetableDiscussion struct {
	DiscussionID DiscussionID
	Title        string
	Attendees    int
	Score        int
	LocationName string
	LocationURL  string
}

type TimetableSlot struct {
	Time        Time // NB: Must duplicate this so that sqlx's StructScan doesn't get confused
	TimeDisplay string
	IsBreak     bool

	// Which room will each discussion be in?
	// (Separate because placement and scheduling are separate steps)
	Discussions []TimetableDiscussion
}

type TimetableDay struct {
	DayName string
	IsFinal bool

	Slots []TimetableSlot
}

// Placement: Specific days, times, rooms
type Timetable struct {
	Days []TimetableDay
}

// DisplaySlot used both for discussion possible slots, as well as for
// schedule locked slots.
type DisplaySlot struct {
	SlotID      SlotID
	SlotTime    Time
	TimeDisplay string
	Checked     bool
}

// FIXME: Add testing for [GS]etLockedSlots

func TimetableGetLockedSlots() []DisplaySlot {
	var ds []DisplaySlot
	err := txLoop(func(eq sqlx.Ext) error {
		err := sqlx.Select(eq, &ds, `
            select slotid,
                   slottime,
                   islocked as checked
                from event_slots
                where isbreak = false
                order by dayid, slotidx`)
		if err != nil {
			return errOrRetry("Getting locked slots", err)
		}
		return nil
	})
	if err != nil {
		log.Printf("INTERNAL ERROR getting locked slots: %v", err)
		ds = nil
	}
	return ds
}

func TimetableSetLockedSlots(pslots []SlotID) error {
	return txLoop(func(eq sqlx.Ext) error {
		var q string
		var args []interface{}
		var err error
		if len(pslots) > 0 {
			q, args, err = sqlx.In(`
            update event_slots
                set islocked = (slotid in (?))`, pslots)
			if err != nil {
				return err
			}
		} else {
			q = `update event_slots set islocked = false`
		}
		_, err = eq.Exec(q, args...)
		if err != nil {
			return errOrRetry("Updating locked slots", err)
		}
		return nil
	})
}

// GetTimetable will get a structured form of the entire timetable.
// If tfmt is non-empty, TimetableStot.TimeDisplay will be formatted
// with the specified time.  If tzl is non-nil, the location will be
// converted to that location before displaying.
func GetTimetable(tfmt string, tzl *TZLocation) (tt Timetable, err error) {
	err = txLoop(func(eq sqlx.Ext) error {
		err := sqlx.Select(eq, &tt.Days,
			`select dayname from event_days order by dayid asc`)
		if err != nil {
			return errOrRetry("Getting day list", err)
		}

		for i := range tt.Days {
			td := &tt.Days[i]
			dayID := i + 1

			err := sqlx.Get(eq, &td.IsFinal,
				`select count(*)==0
                     from event_slots
                     where dayid=? and isbreak=false and islocked=false`, dayID)
			if err != nil {
				return errOrRetry("Getting day finality", err)
			}

			err = sqlx.Select(eq, &td.Slots,
				`select slottime as time, isbreak
                     from event_slots
                     where dayid=?
                     order by slotidx asc`, dayID)
			if err != nil {
				return errOrRetry("Getting slots for one day", err)
			}

			for j := range td.Slots {
				ts := &td.Slots[j]
				err = sqlx.Select(eq, &ts.Discussions, `
with intjoin (userid, discussionid, interest, locationname, locationurl) as
  (select userid, discussionid, interest, locationname, locationurl
       from event_interest
           natural join event_schedule
	       natural join event_slots
           natural join event_locations
       where dayid=? and slotidx=?),
maxint (userid, discussionid, maxint, locationname, locationurl) as
    (select x.userid, discussionid, maxint, locationname, locationurl
     from intjoin x
        join (select userid, max(interest) as maxint
                    from intjoin
               group by userid) y
	on x.userid = y.userid and x.interest = y.maxint),
discint (discussionid, attendees, score, locationname, locationurl) as
	(select discussionid, count(*) as attendees, sum(maxint) as score, locationname, locationurl
             from maxint
    	     group by discussionid)
select discussionid, title, attendees, score, locationname, locationurl
    from discint natural join event_discussions`, dayID, j+1)
				if err != nil {
					return errOrRetry("Getting discussion info for slot", err)
				}
			}

			if tfmt != "" {
				for j := range td.Slots {
					t := td.Slots[j].Time.Time
					if tzl.Location != nil {
						t = t.In(tzl.Location)
					}
					td.Slots[j].TimeDisplay = t.Format(tfmt)
				}
			}
		}

		return nil
	})

	return tt, err
}

type DayID int

type Day struct {
	DayID
	DayName string
}

const slotIDLength = 8

type SlotID string

type Slot struct {
	SlotID
	DayID
	SlotIDX  int
	SlotTime string
	IsBreak  bool
	IsLocked bool
}

func checkDayParams(d *Day) error {
	if d.DayName == "" {
		return errDayNoName
	}
	return nil
}

// getMaxDay: Find the highest dayid.  Returning 0 if no rows means the
// next one chosen will be 1, as we intend.
func getMaxDay(q sqlx.Queryer) (int, error) {
	var maxdayid int
	err := sqlx.Get(q, &maxdayid, `select ifnull(max(dayid), 0) from event_days`)
	return maxdayid, err
}

// getMaxSlotIdx: Find the highest slot index for a given day.  Returning
// 0 if no rows means the next one chosen will be 1, as we intend.
func getMaxSlotIdx(q sqlx.Queryer, dayid DayID) (int, error) {
	var maxSlotIdx int
	err := sqlx.Get(q, &maxSlotIdx, `
        select ifnull(max(slotidx), 0)
            from event_slots
            where dayid=?`, dayid)
	return maxSlotIdx, err
}

func dayAddTx(eq sqlx.Ext, d *Day) error {
	_, err := eq.Exec(`
            insert into event_days(dayid, dayname)
                values (?, ?)`,
		d.DayID,
		d.DayName)
	if err != nil {
		return errOrRetry("Inserting day", err)
	}
	return nil
}

func NewDay(d *Day) (DayID, error) {
	err := checkDayParams(d)
	if err != nil {
		return d.DayID, err
	}

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return d.DayID, fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		maxdayid, err := getMaxDay(tx)
		if err != nil {
			return d.DayID, errOrRetry("Getting  max dayid", err)
		}

		d.DayID = DayID(maxdayid + 1)

		err = dayAddTx(tx, d)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return d.DayID, err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			err = fmt.Errorf("Commiting transaction: %v", err)
		}
		return d.DayID, err
	}
}

/// DayFindById
func DayFindByID(did DayID) (*Day, error) {
	day := &Day{}
	for {
		err := event.Get(day,
			`select * from event_days where dayid = ?`,
			did)
		switch {
		case shouldRetry(err):
			continue
		case err == sql.ErrNoRows:
			return nil, nil
		default:
			return day, err
		}
	}
}

func daySlotsDeleteTx(eq sqlx.Ext, did DayID, firstDelIdx int) error {
	log.Printf("Attempting to delete dayid %d slotidx %d",
		did, firstDelIdx)

	// Check to make sure none of the slots are locked
	var lockedSlots int
	err := sqlx.Get(eq, &lockedSlots,
		`select count(*) 
             from event_slots 
             where dayid=? and slotidx >= ? and islocked = true`,
		did, firstDelIdx)
	if err != nil {
		return errOrRetry("Looking for locked slots", err)
	}

	if lockedSlots > 0 {
		return fmt.Errorf("Cannot delete slot range, %d are locked", lockedSlots)
	}

	// Delete schedule entries for slots we're about to delete
	_, err = eq.Exec(
		`delete from event_schedule
             where slotid in
                 (select slotid from event_slots
                      where dayid=? and slotidx >= ?)`, did, firstDelIdx)
	if err != nil {
		return errOrRetry("Deleting schedule entries for slot range", err)
	}

	// Delete the slots
	res, err := eq.Exec(`delete from event_slots where dayid=? and slotidx >= ?`,
		did, firstDelIdx)
	if err != nil {
		return errOrRetry("Deleting slots range", err)
	}

	rowsdeleted, err := res.RowsAffected()
	if err != nil {
		return errOrRetry("Getting rows affected by slot deletion", err)
	}
	log.Printf("Deleted %d slots", rowsdeleted)

	return nil
}

func deleteDayTx(eq sqlx.Ext, did DayID) error {
	// First delete the slots associated with this day
	err := daySlotsDeleteTx(eq, did, 1)
	if err != nil {
		return err
	}

	// Delete the day
	res, err := eq.Exec(`delete from event_days where dayid=?`, did)
	if err != nil {
		return errOrRetry("Deleting day from event_days", err)
	}

	rcount, err := res.RowsAffected()
	if shouldRetry(err) {
		return err
	} else if err != nil {
		log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
	}
	switch {
	case rcount == 0:
		return ErrDayNotFound
	case rcount > 1:
		log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
		return ErrInternal
	}
	return nil
}

// DeleteDay
func DeleteDay(did DayID) error {
	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		err = deleteDayTx(tx, did)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return err
		}
		return nil
	}
}

func dayUpdateTx(e sqlx.Execer, d *Day) error {
	_, err := e.Exec(`
            update event_days
                set dayname =?
                where dayid = ?`,
		d.DayName, d.DayID)
	return err
}

// DayUpdate: Set d.DayID's fields
func DayUpdate(d *Day) error {
	if err := checkDayParams(d); err != nil {
		return err
	}

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		err = dayUpdateTx(tx, d)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return err
		}

		return nil
	}

}

func timetableSlotAddTx(eq sqlx.Ext, dayid DayID, slotIdx int, slot *TimetableSlot) error {
	slotId := SlotID(id.GenerateID("slot", slotIDLength))
	_, err := eq.Exec(`
        insert into event_slots(slotid, slotidx, dayid, slottime, isbreak, islocked)
            values(?, ?, ?, ?, ?, FALSE)`,
		slotId, slotIdx, dayid, slot.Time, slot.IsBreak)
	if err != nil {
		return errOrRetry("Inserting new slot", err)
	}
	return nil
}

func timetableSlotUpdateTx(eq sqlx.Ext, dayID DayID, slotidx int, slot *TimetableSlot) error {
	// FIXME: Also need to delete schedule entries if going isBreak ==
	// false => true (and return an error if that slot is locked)
	_, err := eq.Exec(`
        update event_slots
            set slottime=?, isbreak=?
            where dayid=? and slotidx=?`,
		slot.Time, slot.IsBreak, dayID, slotidx)
	if err != nil {
		return errOrRetry("Updating slot", err)
	}
	return nil
}

func timetableDayAddTx(eq sqlx.Ext, d *Day, slots []TimetableSlot) error {
	// Add day
	err := dayAddTx(eq, d)
	if err != nil {
		return errOrRetry("Adding day", err)
	}

	// Add slots
	for i := range slots {
		if err := timetableSlotAddTx(eq, d.DayID, i+1, &slots[i]); err != nil {
			return err
		}
	}

	return nil
}

func timetableDayUpdateTx(eq sqlx.Ext, d *Day, slots []TimetableSlot) error {
	// Update day
	err := dayUpdateTx(eq, d)
	if err != nil {
		return errOrRetry("Updating day", err)
	}

	curMaxSlotIdx, err := getMaxSlotIdx(eq, d.DayID)
	if err != nil {
		return err
	}

	// Remove extraneous slots, if any
	if curMaxSlotIdx > len(slots) {
		err = daySlotsDeleteTx(eq, d.DayID, len(slots)+1)
		if err != nil {
			return err
		}
	}

	// Update / add slots
	for i := range slots {
		slotIdx := i + 1
		if i < curMaxSlotIdx {
			// Update slot
			err = timetableSlotUpdateTx(eq, d.DayID, slotIdx, &slots[i])
		} else {
			// Add slot
			err = timetableSlotAddTx(eq, d.DayID, slotIdx, &slots[i])
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func timetableSetTx(eq sqlx.Ext, tt *Timetable) error {
	// NB DayIDs start at 1, so there's an offset of 1 between
	// tt.Days index and dayid.
	curmaxdayid, err := getMaxDay(eq)
	if err != nil {
		return err
	}

	log.Printf("TimetableSet: curmaxdayid %d", curmaxdayid)

	////
	// First, delete all extraneous days.

	// To make this more concrete, suppose tt.Days[] has length 3,
	// and curmaxdayid is 5.  That means dayids for tt.Days[] goes
	// from 1 to 3, and we want to delete dayids 4-5, and then set
	// curmaxdayid to 3.
	for dayid := len(tt.Days) + 1; dayid <= curmaxdayid; dayid++ {
		log.Printf("TimetableSet: Deleting day %d", dayid)
		err = deleteDayTx(eq, DayID(dayid))
		if err != nil {
			return err
		}
	}
	if curmaxdayid > len(tt.Days) {
		curmaxdayid = len(tt.Days)
	}

	////
	// Now update or add days

	// Suppose tt.Days[] is 5 and curmaxdayid is 3; so we want to
	// update day ids 1-3, corresponding to indexes 0-2, and add day ids 4-5,
	// corresponding to indexes 3-4.
	log.Printf("Days to process: %d", len(tt.Days))
	for i := range tt.Days {
		log.Printf("Processing day index %d", i)
		td := &tt.Days[i]
		day := Day{
			DayID:   DayID(i + 1),
			DayName: td.DayName,
		}
		err = checkDayParams(&day)
		if err != nil {
			return err
		}

		log.Printf("day index %d, passed check", i)

		// Sanity-check: Distance between last slot and first slot must be < 24 hours
		if len(td.Slots) > 2 {
			if td.Slots[len(td.Slots)-1].Time.Sub(td.Slots[0].Time.Time).Hours() > 24.0 {
				return fmt.Errorf("Slots span more than 24 hours!")
			}
		}

		log.Printf("day index %d, passed slot check", i)

		if i < curmaxdayid {
			log.Printf("day index %d, Updating day", i)
			err = timetableDayUpdateTx(eq, &day, td.Slots)
		} else {
			log.Printf("day index %d, Adding day", i)
			err = timetableDayAddTx(eq, &day, td.Slots)
		}

		if err != nil {
			log.Printf("didx %d, Thing returned %v", i, err)
			return err
		}
		log.Printf("Done processing day %d", i)
	}
	return nil
}

// Set the timetable.  This will compare the timetable to the one
// currently in the database, creating, deleting, or updating days and
// slots as necessary.
//
// If a <day, slot> combination disappears, the schedule entries for
// that day will be deleted; otherwise they will remain.  If a <day,
// slot> combination which is locked would be deleted, an error will
// be returned instead.
//
// Dealing with time zones and so on is the concern of the caller.
func TimetableSet(tt *Timetable) error {
	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		err = timetableSetTx(tx, tt)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return err
		}

		err = tx.Commit()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return err
		}

		return nil
	}

}
