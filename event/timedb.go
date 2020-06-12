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

type Time struct {
	time.Time
}

func Date(year int, month time.Month, day, hour, min, sec, nsec int, loc *time.Location) Time {
	return Time{Time: time.Date(year, month, day, hour, min, sec, nsec, loc)}
}

func ParseInLocation(layout, value string, loc TZLocation) (Time, error) {
	t, err := time.ParseInLocation(layout, value, loc.Location)
	return Time{Time: t}, err
}

func (t *Time) Scan(src interface{}) error {
	var ts []byte

	switch v := src.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("Invalid time [empty string]")
		}
		ts = []byte(v)
	case []byte:
		ts = v
	default:
		return fmt.Errorf("TimeScan: Cannot convert type %t into Time", src)
	}

	if err := t.UnmarshalText(ts); err != nil {
		return fmt.Errorf("TimeScan: UnmarshalText returned for %s returned %v",
			string(ts), err)
	}
	return nil
}

func (t Time) Value() (driver.Value, error) {
	return t.MarshalText()
}
