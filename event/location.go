package event

import "github.com/gwd/session-scheduler/id"

type LocationID string

type Location struct {
	ID       LocationID
	Name     string
	IsPlace  bool
	Capacity int
}

type LocationDisplay struct {
}

func (location *Location) GetDisplay(cur *User) (ld *LocationDisplay) {
	return &LocationDisplay{}
}

type LocationStore []*Location

const (
	locationIDLength = 16
)

func (lstore *LocationStore) Init() {
	*lstore = make([]*Location, 0)
	// For now, hardcode 3 actual rooms, and one "generic"
	// NB these must be in capacity order, from highest to lowest.
	lstore.Save(&Location{
		ID:       LocationID(id.GenerateID("loc", locationIDLength)),
		Name:     "Contemporary, 6th Floor",
		IsPlace:  true,
		Capacity: 45,
	})
	lstore.Save(&Location{
		ID:       LocationID(id.GenerateID("loc", locationIDLength)),
		Name:     "Gallery 2/3, 5th Floor",
		IsPlace:  true,
		Capacity: 40,
	})
	lstore.Save(&Location{
		ID:       LocationID(id.GenerateID("loc", locationIDLength)),
		Name:     "Gallery 1, 5th Floor",
		IsPlace:  true,
		Capacity: 20,
	})
	lstore.Save(&Location{
		ID:      LocationID(id.GenerateID("loc", locationIDLength)),
		Name:    "Ad-hoc meetings",
		IsPlace: false,
	})

}

func (lstore LocationStore) Find(id LocationID) (*Location, error) {
	for _, loc := range lstore {
		if loc.ID == id {
			return loc, nil
		}
	}
	return nil, nil
}

func (lstore *LocationStore) Save(location *Location) error {
	// FIXME: For now, add in order.  Later sort by some metric.
	*lstore = append(*lstore, location)
	return Event.Save()
}

func (lstore LocationStore) GetLocations() []*Location {
	return []*Location(lstore)
}

func (lstore LocationStore) GetList(cur *User) (list []*LocationDisplay) {
	for _, l := range lstore {
		ld := l.GetDisplay(cur)
		if ld != nil {
			list = append(list, ld)
		}
	}
	return
}
