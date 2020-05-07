package event

import (
	"database/sql/driver"
	"fmt"
	"log"
	"time"
)

// Location is database-scannable wrapper around time.Location
type TZLocation struct {
	*time.Location
}

// This is a wrapper around time.LoadLocation which returns a
// database-scannable Location
func LoadLocation(name string) (TZLocation, error) {
	l, err := time.LoadLocation(name)
	return TZLocation{Location: l}, err
}

func (l *TZLocation) Scan(src interface{}) error {
	switch ts := src.(type) {
	case string:
		var err error
		if ts == "" {
			log.Printf("INTERNAL ERROR: Empty string for location; using default %v",
				event.defaultLocation)
			l.Location = event.defaultLocation
		} else {
			l.Location, err = time.LoadLocation(ts)
		}
		if err != nil {
			return fmt.Errorf("LocationScan: LoadLocation for %s returned %v",
				ts, err)
		}
		return nil
	default:
		return fmt.Errorf("LocationScan: Cannot convert type %t into Location", src)
	}
}

func (l TZLocation) Value() (driver.Value, error) {
	return driver.Value(l.String()), nil
}
