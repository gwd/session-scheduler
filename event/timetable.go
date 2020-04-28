package event

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
