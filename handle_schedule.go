package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/event"
)

func HandleScheduleView(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if kvs.GetBoolDef(FlagScheduleActive) {
		RenderTemplate(w, r, "schedule/view", map[string]interface{}{
			"Timetable": event.GetTimetable(),
		})
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
