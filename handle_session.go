package main

import (
	"log"
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"

	"github.com/gwd/session-scheduler/event"
	"github.com/gwd/session-scheduler/sessions"
)

func HandleSessionDestroy(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := sessions.DeleteSessionByRequest(r); err != nil {
		panic(err)
	}
	RenderTemplate(w, r, "sessions/destroy", nil)
}

func HandleSessionNew(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	next := r.URL.Query().Get("next")
	RenderTemplate(w, r, "sessions/new", map[string]interface{}{
		"Next": next,
	})
}

func FindUser(username, password string) (*event.User, error) {
	existingUser, err := event.UserFindByUsername(username)
	if err != nil {
		log.Printf("INTERNAL ERROR: UserFindByUsername: %v")
		return nil, event.ErrInternal
	}
	if existingUser == nil {
		// Same error for no user / wrong password to avoid username fishing
		return nil, event.ErrCredentialsIncorrect
	}

	if !existingUser.CheckPassword(password) {
		return nil, event.ErrCredentialsIncorrect
	}

	return existingUser, nil
}

func HandleSessionCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	next := r.FormValue("next")

	user, err := FindUser(username, password)
	if err != nil {
		if event.IsValidationError(err) {
			RenderTemplate(w, r, "sessions/new", map[string]interface{}{
				"Error": err,
				"User":  event.User{Username: username},
				"Next":  next,
			})
			return
		}
		panic(err)
	}

	if !kvs.GetBoolDef(FlagActive) && !user.IsAdmin {
		http.Redirect(w, r, "/login?flash=Website+Inactive", http.StatusFound)
		return
	}

	_, err = sessions.FindOrCreateSession(w, r, string(user.UserID))
	if err != nil {
		panic(err)
	}

	if next == "" {
		next = "/"
	} else {
		next, err = url.QueryUnescape(next)
		if err != nil {
			next = "/"
		}
	}

	http.Redirect(w, r, next+"?flash=Signed+in", http.StatusFound)
}
