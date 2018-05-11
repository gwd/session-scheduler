package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/hako/durafmt"
	"github.com/julienschmidt/httprouter"
)

func HandleAdminConsole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	content := map[string]interface{}{ "User": user }

	tmpl := ps.ByName("template")
	switch tmpl {
	case "console":
		content["Vcode"] = Event.VerificationCode
		lastUpdate := "Never"
		if Event.Schedule != nil {
			lastUpdate = durafmt.ParseShort(time.Since(Event.Schedule.Created)).String()+" ago"
		}
		content["SinceLastSchedule"] = lastUpdate
		fallthrough
	case "test":
		content[tmpl] = true
		RenderTemplate(w, r, "admin/"+tmpl, content)
	}

}

func HandleAdminAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	action := ps.ByName("action")
	if !(action == "runschedule" || action == "setvcode") {
		return
	}

	switch action {
	case "runschedule":
		err := MakeSchedule()
		if err == nil {
			http.Redirect(w, r, "console?flash=Schedule+Generated", http.StatusFound)
		} else {
			log.Printf("Error generating schedule: %v", err)
			http.Redirect(w, r, "console?flash=Error+generating+schedule", http.StatusFound)
		}
	case "setvcode":
		newvcode := r.FormValue("vcode")
		if newvcode == "" {
			RenderTemplate(w, r, "console?flash=Invalid+Vcode",
				map[string]interface{}{
					"User": user,
					"console": true,
					"Vcode": Event.VerificationCode,
				})
			return
		}

		Event.VerificationCode = newvcode
		Event.Save()
		http.Redirect(w, r, "console?flash=Verification+code+updated", http.StatusFound)
		return
	}
}

func HandleTestAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
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
