package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func HandleScheduleView(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	RenderTemplate(w, r, "schedule/view", map[string]interface{}{
		"Timetable": &Event.Timetable,
	})
}
