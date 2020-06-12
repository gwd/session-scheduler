package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/event"
)

func HandleScheduleView(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if kvs.GetBoolDef(FlagScheduleActive) {
		// FIXME: Handle the error
		tt, _ := event.GetTimetable()
		RenderTemplate(w, r, "schedule/view", map[string]interface{}{
			"Timetable": tt,
		})
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
