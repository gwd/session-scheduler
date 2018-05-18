package main

type LocationID string

type Location struct {
	ID LocationID
	Name string
	IsPlace bool
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
	lstore.Save(&Location{
		ID: LocationID(GenerateID("loc", locationIDLength)),
		Name: "Jiangning",
		IsPlace: true,
		Capacity: 100,
	})
	lstore.Save(&Location{
		ID: LocationID(GenerateID("loc", locationIDLength)),
		Name: "Meeting Room 4",
		IsPlace: true,
		Capacity: 50,
	})
	lstore.Save(&Location{
		ID: LocationID(GenerateID("loc", locationIDLength)),
		Name: "Meeting Room 5",
		IsPlace: true,
		Capacity: 50,
	})
	lstore.Save(&Location{
		ID: LocationID(GenerateID("loc", locationIDLength)),
		Name: "Ad-hoc meetings",
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

func (lstore LocationStore) GetLocations() ([]*Location) {
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
