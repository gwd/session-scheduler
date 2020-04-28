package event

// import "github.com/gwd/session-scheduler/id"

type LocationID string

type Location struct {
	ID       LocationID
	Name     string
	IsPlace  bool
	Capacity int
}

const (
	locationIDLength = 16
)

func LocationGetAll() ([]Location, error) {
	// TODO: Locations
	return nil, nil
}
