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

	curLocationString := DefaultLocation
	curLocationTZ := DefaultLocationTZ

	if loc := r.FormValue("location"); loc != "" {
		// If we've gone to a specific URL, use that location, and set a
		// cookie to remember it.
		tz, err := event.LoadLocation(loc)
		if err == nil {
			curLocationString = loc
			curLocationTZ = tz
			// http.SetCookie(w, &http.Cookie{
			// 	Name:    locationCookieName,
			// 	Value:   loc,
			// 	Expires: time.Now().Add(time.Hour * 24 * 365)})
		}
	} else if cookie, err := r.Cookie(locationCookieName); err == nil {
		// Otherwise, if there's a cookie, use that value.
		tz, err := event.LoadLocation(cookie.Value)
		if err == nil {
			curLocationString = cookie.Value
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
