package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/event"
	"github.com/gwd/session-scheduler/sessions"
)

func HandleUserNew(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	RenderTemplate(w, r, "user/new", nil)
}

func parseProfile(r *http.Request) (profile *event.UserProfile) {
	profile = &event.UserProfile{
		RealName:    r.FormValue("RealName"),
		Company:     r.FormValue("Company"),
		Email:       r.FormValue("Email"),
		Description: r.FormValue("Description"),
	}
	return
}

func HandleUserCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var e string
	var isVerified bool
	var err error
	var uid event.UserID

	profile := parseProfile(r)
	username := r.FormValue("Username")

	{
		evcode, err := kvs.Get(VerificationCode)
		if err != nil {
			log.Printf("INTERNAL ERROR: Couldn't get event verification code: %v", err)
			e = "Internal error"
			goto fail
		}
		vcode := r.FormValue("Vcode")
		if vcode == evcode {
			isVerified = true
		} else if kvs.GetBoolDef(FlagRequireVerification) {
			log.Printf("New user failed: Bad vcode %s", vcode)
			e = "Incorrect Verification Code"
			goto fail
		}
	}

	uid, err = event.NewUser(r.FormValue("Password"), event.User{
		Username:   username,
		IsVerified: isVerified,
		Profile:    *profile})

	if err != nil {
		if event.IsValidationError(err) {
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
		"Username": username,
		"Profile":  profile,
	})
	return
}
