package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func HandleAdminConsole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if !user.IsAdmin {
		return
	}

	template := ps.ByName("template")
	switch template {
	case "console", "test":
		RenderTemplate(w, r, "admin/"+template, map[string]interface{}{
			"User": user,
		})
	}

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

func HandleTestAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if !user.IsAdmin {
		return
	}

	flash := "Success"
	
	action := ps.ByName("action")

	if !Event.TestMode {
		if action != "enabletest" {
			return
		}

		if r.FormValue("confirm") != "SafetyOff" {
			RenderTemplate(w, r, "admin/test", map[string]interface{}{
				"MustConfirm": true,
			})
			return
		}
		
		Event.TestMode = true
		Event.Save()

		flash = "Test+mode+enabled"
	} else {
		switch action {
		case "disabletest":
			Event.TestMode = false
			Event.Save()
			flash = "Test+mode+disabled"
		case "enabletest":
			flash = "Test+mode+already+disabled"
		case "reset":
			Event.Reset()
			flash = "Data+reset"
		case "genuser":
			countString := r.FormValue("count")
			count, err := strconv.Atoi(countString)
			if err != nil || !(count > 0){
				flash = "Bad+input"
			} else {
				for i := 0; i < count; i++ {
					NewTestUser()
				}
				flash = countString+" users generated"
			}
		case "gendiscussion":
			countString := r.FormValue("count")
			count, err := strconv.Atoi(countString)
			if err != nil || !(count > 0){
				flash = "Bad+input"
			} else {
				for i := 0; i < count; i++ {
					NewTestDiscussion(nil)
				}
				flash = countString+" discussions generated"
			}
		case "geninterest":
			TestGenerateInterest()
			flash = "Interest generated"
		default:
			return
		}
	}

	http.Redirect(w, r, "/admin/test?flash="+flash, http.StatusFound)
}
