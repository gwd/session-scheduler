package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/event"
)

const (
	FlagTestMode             = "ServeTestMode"
	FlagActive               = "ServeActive"
	FlagScheduleActive       = "ServeScheduleActive"
	FlagVerificationCodeSent = "ServeVerificationCodeSent"
	FlagRequireVerification  = "ServeRequireVerification"
)

func HandleAdminConsole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	content := map[string]interface{}{"User": user}

	tmpl := ps.ByName("template")
	switch tmpl {
	case "console":
		content["Vcode"], _ = kvs.Get(VerificationCode)
		content["SinceLastSchedule"] = event.SchedLastUpdate()
		switch event.SchedGetState() {
		case event.SchedStateRunning:
			content["IsInProgress"] = true
		case event.SchedStateModified:
			content["IsStale"] = true
		default:
			content["IsCurrent"] = true
		}
		content["LockedSlots"] = event.TimetableGetLockedSlots()
		fallthrough
	case "test":
		content[tmpl] = true
		RenderTemplate(w, r, "admin/"+tmpl, content)
	}

}

var OptSearchAlgo string

func HandleAdminAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	action := ps.ByName("action")
	if !(action == "runschedule" ||
		action == "setvcode" ||
		action == "setstatus" ||
		action == "resetEventData" ||
		action == "setLocked") {
		return
	}

	switch action {
	case "runschedule":
		err := MakeSchedule(true)
		if err == nil {
			http.Redirect(w, r, "console?flash=Schedule+Started", http.StatusFound)
		} else {
			log.Printf("Error generating schedule: %v", err)
			http.Redirect(w, r, "console?flash=Error+starting+schedule: See Log", http.StatusFound)
		}
	case "resetEventData":
		event.ResetData()
		http.Redirect(w, r, "console?flash=Event+data+reset", http.StatusFound)
		return
	case "setvcode":
		newvcode := r.FormValue("vcode")
		if newvcode == "" {
			vcode, _ := kvs.Get(VerificationCode)
			RenderTemplate(w, r, "console?flash=Invalid+Vcode",
				map[string]interface{}{
					"User":    user,
					"console": true,
					"Vcode":   vcode,
				})
			return
		}

		log.Printf("New vcode: %s", newvcode)
		err := kvs.Set(VerificationCode, newvcode)
		flash := "Verification+code+updated"
		if err != nil {
			flash = "Verification+code+not+updated"
			log.Printf("Error setting verification code: %v", err)
		}
		http.Redirect(w, r, "console?flash="+flash, http.StatusFound)
		return
	case "setstatus":
		r.ParseForm()
		statuses := r.Form["status"]
		newval := map[string]bool{
			"website":      false,
			"schedule":     false,
			"verification": false,
			"vcodesent":    false}
		for _, status := range statuses {
			switch status {
			case "websiteActive":
				newval["website"] = true
			case "scheduleActive":
				newval["schedule"] = true
			case "vcodeSent":
				newval["vcodesent"] = true
			case "requireVerification":
				newval["verification"] = true
			default:
				log.Printf("Unexpected status value: %v", status)
				flash := "Invalid form result: Report this error to the admin"
				http.Redirect(w, r, "console?flash="+flash, http.StatusFound)
				return
			}
		}

		flashAccumulator := func(flash string, err error, fkey string, dbkey string, ftt string, ttf string) (string, error) {
			if err != nil {
				return flash, err
			}
			nv := newval[fkey]
			ov, err := kvs.ExchangeBool(dbkey, nv)
			if ov != nv {
				if flash != "" {
					flash += ", "
				}
				if nv {
					flash += ftt
				} else {
					flash += ttf
				}
			}
			return flash, nil
		}

		flash, err := flashAccumulator("", nil, "website",
			FlagActive, "Website+Activated", "Website+Deactivated")

		flash, err = flashAccumulator(flash, err, "schedule",
			FlagScheduleActive, "Schedule+Activated", "Schedule+Deactivated")

		flash, err = flashAccumulator(flash, err, "vcodesent",
			FlagVerificationCodeSent, "Verification+Code+Sent", "Verification+Code+Not+Sent")

		flash, err = flashAccumulator(flash, err, "verification",
			FlagRequireVerification, "Verification+Required", "Verification+Not+Required")

		http.Redirect(w, r, "console?flash="+flash, http.StatusFound)
		return
	case "setLocked":
		r.ParseForm()
		locked, err := FormCheckToBool(r.Form["locked"])
		if err != nil {
			return
		}
		log.Printf("New locked slots: %v", locked)
		event.LockedSlotsSet(locked)
		http.Redirect(w, r, "console?flash=Locked+slots+updated", http.StatusFound)
	}
}

func HandleTestAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := RequestUser(r)

	if user == nil || !user.IsAdmin {
		return
	}

	flash := "Success"

	action := ps.ByName("action")

	if !kvs.GetBoolDef(FlagTestMode) {
		if action != "enabletest" {
			return
		}

		if r.FormValue("confirm") != "SafetyOff" {
			RenderTemplate(w, r, "admin/test", map[string]interface{}{
				"MustConfirm": true,
			})
			return
		}

		err := kvs.SetBool(FlagTestMode, true)
		if err != nil {
			log.Printf("ERROR: Setting test mode failed: %v", err)
			flash = "Set+test+mode+failed"
		} else {
			flash = "Test+mode+enabled"
		}
	} else {
		switch action {
		case "disabletest":
			err := kvs.SetBool(FlagTestMode, false)
			if err != nil {
				log.Printf("ERROR: Setting test mode failed: %v", err)
				flash = "Set+test+mode+failed"
			} else {
				flash = "Test+mode+disabled"
			}
		case "enabletest":
			flash = "Test+mode+already+disabled"
		// case "gendiscussion":
		// 	countString := r.FormValue("count")
		// 	count, err := strconv.Atoi(countString)
		// 	if err != nil || !(count > 0) {
		// 		flash = "Bad+input"
		// 	} else {
		// 		for i := 0; i < count; i++ {
		// 			event.NewTestDiscussion(nil)
		// 		}
		// 		flash = countString + " discussions generated"
		// 	}
		// case "geninterest":
		// 	event.TestGenerateInterest()
		// 	flash = "Interest generated"
		default:
			return
		}
	}

	http.Redirect(w, r, "/admin/test?flash="+flash, http.StatusFound)
}
