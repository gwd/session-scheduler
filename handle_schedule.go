package main

import (
	"net/http"
	//"time"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/event"
)

const (
	locationCookieName = "XenSummitTZLocation"
)

func HandleScheduleView(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !kvs.GetBoolDef(FlagScheduleActive) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	cur := RequestUser(r)

	curLocationString := DefaultLocation
	curLocationTZ := DefaultLocationTZ

	if loc := r.FormValue("location"); loc != "" {
		// If we've gone to a specific URL, use that location
		tz, err := event.LoadLocation(loc)
		if err == nil {
			curLocationString = loc
			curLocationTZ = tz
		}
	} else if cur != nil {
		// Otherwise, if we're logged in, use the user's preference
		tz, err := cur.GetLocationTZ()
		if err == nil {
			curLocationString = tz.String()
			curLocationTZ = tz
		}
	}

	// FIXME: Handle the error
	tt, _ := event.GetTimetable("3:04pm Jan 2", &curLocationTZ)
	RenderTemplate(w, r, "schedule/view", map[string]interface{}{
		"Timetable":       tt,
		"CurrentLocation": curLocationString,
		"Locations":       TimezoneList,
	})
}
