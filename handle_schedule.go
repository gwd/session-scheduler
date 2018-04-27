package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func HandleScheduleView(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cur := RequestUser(r)

	sched := Event.Schedule
	
	if sched == nil {
		http.Redirect(w, r, "schedule/notfound", http.StatusFound)
		return
	}

	RenderTemplate(w, r, "schedule/view", map[string]interface{}{
		"Schedule": sched.GetDisplay(cur),
	})
}
