package main

import (
	"net/http"

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

	curLocationString := DefaultLocation
	curLocationTZ := DefaultLocationTZ
	if loc := r.FormValue("location"); loc != "" {
		tz, err := event.LoadLocation(loc)
		if err == nil {
			curLocationString = loc
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
