package event

import (
	"database/sql"
	"log"

	"github.com/jmoiron/sqlx"
)

// import "github.com/gwd/session-scheduler/id"

type LocationID int

type Location struct {
	LocationID   LocationID
	LocationName string
	LocationURL  string
	IsPlace      bool
	Capacity     int
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

	err = txLoop(func(eq sqlx.Ext) error {
		// Find the highest locationid.  Returning 0 if no rows means the
		// next one chosen will be 1, as we intend.
		var maxlocid int
		err := sqlx.Get(eq, &maxlocid, `select ifnull(max(locationid), 0) from event_locations`)
		if err != nil {
			return errOrRetry("Getting  max locationid", err)
		}

		l.LocationID = LocationID(maxlocid + 1)
		_, err = eq.Exec(`
            insert into event_locations(locationid, locationname, locationurl, isplace, capacity)
                values (?, ?, ?, ?, ?)`,
			l.LocationID,
			l.LocationName,
			l.LocationURL,
			l.IsPlace,
			l.Capacity)
		if err != nil {
			return errOrRetry("Inserting location", err)
		}

		return nil
	})

	return l.LocationID, err
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
	return txLoop(func(eq sqlx.Ext) error {
		// TODO: Delete (nullify?) the schedule as well
		res, err := event.Exec(`delete from event_locations where locationid=?`, lid)
		if err != nil {
			return errOrRetry("Deleting location from event_locations", err)
		}

		rcount, err := res.RowsAffected()
		if shouldRetry(err) {
			return err
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

		return nil
	})
}

// LocationUpdate
func LocationUpdate(l *Location) error {
	if err := checkLocationParams(l); err != nil {
		return err
	}

	err := txLoop(func(eq sqlx.Ext) error {
		// TODO: Delete (nullify?) the schedule if changing isplace or capacity
		_, err := eq.Exec(`
            update event_locations
                set locationname =?,
                    locationurl = ?,
                    isplace = ?,
                    capacity = ?
                where locationid = ?`,
			l.LocationName, l.LocationURL, l.IsPlace, l.Capacity, l.LocationID)
		return err
	})

	return err
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
