package event

import (
	"database/sql"
	"fmt"
	"log"
)

type TimetableDiscussion struct {
	ID        DiscussionID
	Title     string
	Attendees int
	Score     int
	// Copy of the "canonical" location, updated every time the
	// schedule is run
	LocationInfo Location
}

type TimetableSlot struct {
	Time    string
	IsBreak bool

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

func TimetableGetLockedSlots() []DisplaySlot {
	// FIXME: Timetable
	return nil
}

func GetTimetable() Timetable {
	// FIXME: Timetable
	return Timetable{}
}

type DayID int

type Day struct {
	DayID
	DayName string
}

func checkDayParams(d *Day) error {
	if d.DayName == "" {
		return errDayNoName
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

		// Find the highest dayid.  Returning 0 if no rows means the
		// next one chosen will be 1, as we intend.
		var maxdayid int
		err = tx.Get(&maxdayid, `select ifnull(max(dayid), 0) from event_days`)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return d.DayID, fmt.Errorf("Getting  max dayid: %v", err)
		}

		d.DayID = DayID(maxdayid + 1)
		_, err = tx.Exec(`
            insert into event_days(dayid, dayname)
                values (?, ?)`,
			d.DayID,
			d.DayName)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return d.DayID, fmt.Errorf("Inserting day: %v", err)
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
func DayFindById(did DayID) (*Day, error) {
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

		// TODO: Delete (nullify?) the schedule as well
		res, err := event.Exec(`delete from event_days where dayid=?`, did)
		switch {
		case shouldRetry(err):
			continue
		case err == nil:
			break
		default:
			return fmt.Errorf("Deleting day from event_days: %v", err)
		}

		rcount, err := res.RowsAffected()
		if shouldRetry(err) {
			continue
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

		err = tx.Commit()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return err
		}
		return nil
	}
}

// DayUpdate
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

		// TODO: Delete (nullify?) the schedule if changing isplace or capacity

		_, err = tx.Exec(`
            update event_days
                set dayname =?
                where dayid = ?`,
			d.DayName, d.DayID)
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
