package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func HandleUserNew(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	RenderTemplate(w, r, "user/new", nil)
}

func parseProfile(r *http.Request) (profile *UserProfile) {
	profile = &UserProfile{
		RealName:    r.FormValue("RealName"),
		Company:     r.FormValue("Company"),
		Email:       r.FormValue("Email"),
		Description: r.FormValue("Description"),
	}
	return
}

func HandleUserCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user, err := NewUser(
		r.FormValue("Username"),
		r.FormValue("Password"),
		r.FormValue("Vcode"),
		parseProfile(r),
	)

	if err != nil {
		if IsValidationError(err) {
			RenderTemplate(w, r, "user/new", map[string]interface{}{
				"Error": err.Error(),
				"User":  user,
			})
			return
		}
		panic(err)
		return
	}

	err = Event.Users.Save(user)
	if err != nil {
		panic(err)
		return
	}

	// Create a new session
	session := NewSession(w)
	session.UserID = user.ID
	err = globalSessionStore.Save(session)
	if err != nil {
		panic(err)
	}

	http.Redirect(w, r, "/?flash=User+created", http.StatusFound)
}
