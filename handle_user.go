package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/event"
	"github.com/gwd/session-scheduler/sessions"
)

func HandleUserNew(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	RenderTemplate(w, r, "user/new", map[string]interface{}{
		"DefaultLocation": DefaultLocation,
		"Locations":       TimezoneList,
	})
}

func parseProfile(r *http.Request, user *event.User) {
	user.RealName = r.FormValue("RealName")
	user.Company = r.FormValue("Company")
	user.Email = r.FormValue("Email")
	user.Description = r.FormValue("Description")
	if loc := r.FormValue("Location"); loc != "" {
		tzl, err := event.LoadLocation(loc)
		if err == nil {
			user.Location = tzl
		}
	}
}

func HandleUserCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var e string
	var err error
	var uid event.UserID
	var user event.User

	user.Username = r.FormValue("Username")
	parseProfile(r, &user)

	{
		evcode, err := kvs.Get(VerificationCode)
		if err != nil {
			log.Printf("INTERNAL ERROR: Couldn't get event verification code: %v", err)
			e = "Internal error"
			goto fail
		}
		vcode := r.FormValue("Vcode")
		if vcode == evcode {
			user.IsVerified = true
		} else if kvs.GetBoolDef(FlagRequireVerification) {
			log.Printf("New user failed: Bad vcode %s", vcode)
			e = "Incorrect Verification Code"
			goto fail
		}
	}

	uid, err = event.NewUser(r.FormValue("Password"), &user)

	if err != nil {
		if event.IsValidationError(err) {
			e = err.Error()
			goto fail
		}
		panic(err)
		return
	}

	if err != nil {
		panic(err)
		return
	}

	// Create a new session
	_, err = sessions.NewSession(w, string(uid))
	if err != nil {
		panic(err)
	}

	http.Redirect(w, r, "/?flash=User+created", http.StatusFound)
	return

fail:
	RenderTemplate(w, r, "user/new", map[string]interface{}{
		"Error":    e,
		"Username": user.Username,
		"Profile": map[string]string{
			"RealName":    user.RealName,
			"Email":       user.Email,
			"Company":     user.Company,
			"Description": user.Description,
		},
	})
	return
}
