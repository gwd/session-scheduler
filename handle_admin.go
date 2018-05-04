package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func HandleAdminConsole(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := RequestUser(r)

	if !user.IsAdmin {
		return
	}

	RenderTemplate(w, r, "admin/console", map[string]interface{}{
		"User": user,
	})
}

func HandleAdminRunSchedule(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := RequestUser(r)

	if !user.IsAdmin {
		return
	}
	
	err := MakeSchedule()
	if err == nil {
		http.Redirect(w, r, "/admin/console?flash=Schedule+Generated", http.StatusFound)
	} else {
		log.Printf("Error generating schedule: %v", err)
		http.Redirect(w, r, "/admin/console?flash=Error+generating+schedule", http.StatusFound)
	}
}
