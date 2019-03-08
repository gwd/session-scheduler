package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func HandleScheduleView(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if Event.ScheduleActive {
		RenderTemplate(w, r, "schedule/view", map[string]interface{}{
			"Timetable": &Event.Timetable,
		})
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
