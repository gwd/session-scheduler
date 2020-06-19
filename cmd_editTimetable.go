package main

import (
	"log"

	"encoding/json"
	"github.com/hjson/hjson-go"

	"github.com/gwd/session-scheduler/event"
)

type TimetableEditSlot struct {
	Time    string
	IsBreak bool `json:",omitempty"`
}

type TimetableEditDay struct {
	DayName string
	Slots   []TimetableEditSlot
}

type TimetableEdit struct {
	Location string
	Days     []TimetableEditDay
}

var header = `// Location is the normal timezone location; e.g., 'Europe/Berlin'.
// Time should be in the format "2020 Jan 2 15:04". 
// Add 'IsBreak: true' to any slot which should be labeled as a break.
`

var format = "2006 Jan 2 15:04"

func EditTimetable() {
	// Get timetable.  If it's not empty, marshal it to json (perhaps
	// with a comment at the top?).  Otherwise, use the starter schedule
	tt, err := event.GetTimetable("", nil)
	if err != nil {
		log.Fatalf("Getting timetable: %v")
	}

	ett := TimetableEdit{Location: DefaultLocation}

	if len(tt.Days) > 0 {
		// If the timetable is non-empty, copy from tt into ett
		ett.Days = make([]TimetableEditDay, len(tt.Days))
		for i := range tt.Days {
			ed := &ett.Days[i]
			td := &tt.Days[i]
			ed.DayName = td.DayName
			ed.Slots = make([]TimetableEditSlot, len(td.Slots))

			for j := range td.Slots {
				ed.Slots[j].Time = td.Slots[j].Time.Format(format)
				ed.Slots[j].IsBreak = td.Slots[j].IsBreak
			}
		}
	} else {
		// Set up an example timetable
		ett.Days = []TimetableEditDay{
			{"Monday",
				[]TimetableEditSlot{
					{"2020 Jul 6 14:00", false},
					{"2020 Jul 6 14:30", false},
					{"2020 Jul 6 15:00", true},
					{"2020 Jul 6 15:30", false}}}}
	}

	outb, err := hjson.MarshalWithOptions(ett, hjson.EncoderOptions{BracesSameLine: true, Eol: "\n", IndentBy: "  "})
	if err != nil {
		log.Fatalf("Marshalling structure: %v", err)
	}

	outb = append([]byte(header), outb...)

	// Execute the editor
	inb, err := ExternalEditorBytes(outb)
	if err != nil {
		log.Fatalf("Editing json: %v", err)
	}

	// Unmarshal the modified data.

	// HACK: hjson seems to have trouble marshalling into
	// actual structs, so send it through encoding/json.
	ints := map[string]interface{}{}
	err = hjson.Unmarshal(inb, &ints)
	if err != nil {
		log.Fatalf("Importing structure: %v", err)
	}

	intb, err := json.Marshal(ints)
	if err != nil {
		log.Fatalf("Intermediate marshal: %v", err)
	}

	err = json.Unmarshal(intb, &ett)
	if err != nil {
		log.Fatalf("Intermediate unmarshal: %v", err)
	}

	// FIXME: It might be nice to feed back the error to the use with
	// the same file so they can fix it up.
	loc, err := event.LoadLocation(ett.Location)
	if err != nil {
		log.Fatalf("Loading location: %v", err)
	}

	tt = event.Timetable{}
	tt.Days = make([]event.TimetableDay, len(ett.Days))
	for i := range ett.Days {
		td := &tt.Days[i]
		ed := &ett.Days[i]
		td.DayName = ed.DayName
		td.Slots = make([]event.TimetableSlot, len(ed.Slots))

		for j := range ed.Slots {
			td.Slots[j].Time, err = event.ParseInLocation(format, ed.Slots[j].Time, loc)
			if err != nil {
				log.Fatalf("Error parsing time: %v", err)
			}

			td.Slots[j].IsBreak = ed.Slots[j].IsBreak
		}
	}

	err = event.TimetableSet(&tt)
	if err != nil {
		log.Fatalf("Error setting timetable: %v", err)
	}
}
