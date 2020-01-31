package main

import (
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"

	disc "github.com/gwd/session-scheduler/discussions"
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

func HandleSessionCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	next := r.FormValue("next")

	user, err := disc.FindUser(username, password)
	if err != nil {
		if disc.IsValidationError(err) {
			RenderTemplate(w, r, "sessions/new", map[string]interface{}{
				"Error": err,
				"User":  user,
				"Next":  next,
			})
			return
		}
		panic(err)
	}

	if !Event.Active && !user.IsAdmin {
		http.Redirect(w, r, "/login?flash=Website+Inactive", http.StatusFound)
		return
	}

	_, err = sessions.FindOrCreateSession(w, r, string(user.ID))
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
