package event

import (
	"database/sql"
	"fmt"
	"log"
)

// import "github.com/gwd/session-scheduler/id"

type LocationID int

type Location struct {
	LocationID          LocationID
	LocationName        string
	LocationDescription string
	IsPlace             bool
	Capacity            int
}

const (
	locationIDLength = 16
)

func checkLocationParams(l *Location) error {
	if l.LocationName == "" {
		return errLocationNoName
	}

	if l.Capacity < 1 {
		return errLocationInvalidCapacity
	}

	return nil
}

//
func NewLocation(l *Location) (LocationID, error) {
	err := checkLocationParams(l)
	if err != nil {
		return l.LocationID, err
	}

	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return l.LocationID, fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		// Find the highest locationid.  Returning 0 if no rows means the
		// next one chosen will be 1, as we intend.
		var maxlocid int
		err = tx.Get(&maxlocid, `select ifnull(max(locationid), 0) from event_locations`)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return l.LocationID, fmt.Errorf("Getting  max locationid: %v", err)
		}

		l.LocationID = LocationID(maxlocid + 1)
		_, err = tx.Exec(`
            insert into event_locations(locationid, locationname, locationdescription, isplace, capacity)
                values (?, ?, ?, ?, ?)`,
			l.LocationID,
			l.LocationName,
			l.LocationDescription,
			l.IsPlace,
			l.Capacity)
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return l.LocationID, fmt.Errorf("Inserting location: %v", err)
		}

		err = tx.Commit()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			err = fmt.Errorf("Commiting transaction: %v", err)
		}
		return l.LocationID, err
	}
}

/// LocationFindById
func LocationFindById(lid LocationID) (*Location, error) {
	loc := &Location{}
	for {
		err := event.Get(loc,
			`select * from event_locations where locationid = ?`,
			lid)
		switch {
		case shouldRetry(err):
			continue
		case err == sql.ErrNoRows:
			return nil, nil
		default:
			return loc, err
		}
	}
}

// DeleteLocation
func DeleteLocation(lid LocationID) error {
	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		// TODO: Delete (nullify?) the schedule as well
		res, err := event.Exec(`delete from event_locations where locationid=?`, lid)
		switch {
		case shouldRetry(err):
			continue
		case err == nil:
			break
		default:
			return fmt.Errorf("Deleting location from event_locations: %v", err)
		}

		rcount, err := res.RowsAffected()
		if shouldRetry(err) {
			continue
		} else if err != nil {
			log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
		}
		switch {
		case rcount == 0:
			return ErrLocationNotFound
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

// LocationUpdate
func LocationUpdate(l *Location) error {
	if err := checkLocationParams(l); err != nil {
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
            update event_locations
                set locationname =?,
                    locationdescription = ?,
                    isplace = ?,
                    capacity = ?
                where locationid = ?`,
			l.LocationName, l.LocationDescription, l.IsPlace, l.Capacity, l.LocationID)
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

func LocationGetAll() (locations []Location, err error) {
	for {
		err = event.Select(&locations, `select * from event_locations order by locationid`)
		switch {
		case shouldRetry(err):
			continue
		default:
			return locations, err
		}
	}
}
