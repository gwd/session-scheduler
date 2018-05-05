package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func HandleDiscussionNew(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	RenderTemplate(w, r, "discussion/new", nil)
}

func HandleDiscussionNotFound(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	RenderTemplate(w, r, "discussion/notfound", nil)
}

func HandleDiscussionCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	owner := RequestUser(r)

	disc, err := NewDiscussion(
		owner,
		r.FormValue("title"),
		r.FormValue("description"),
	)

	if err != nil {
		if IsValidationError(err) {
			RenderTemplate(w, r, "discussion/new", map[string]interface{}{
				"Error":      err.Error(),
				"Discussion": disc,
			})
			return
		}
		panic(err)
	}

	http.Redirect(w, r, disc.GetURL()+"?flash=Session+Created", http.StatusFound)
}

func HandleUid(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cur := RequestUser(r)

	uid := ps.ByName("uid")
	
	action := ps.ByName("action")
	if action != "view" {
		return
	}

	itype := ps.ByName("itype")
	var display interface{}
	switch itype {
	case "discussion":
		disc, _ := DiscussionFindById(uid)
		if disc != nil {
			display = disc.GetDisplay(cur)
		}
	case "user":
		display, _ = Event.Users.Find(UserID(uid))
	default:
		return
	}

	if display == nil {
		RenderTemplate(w, r, "uid/notfound", map[string]interface{}{
			"Utype": itype,
		})
		return
	}

	RenderTemplate(w, r, itype+"/"+action, map[string]interface{}{
		"Display": display,
	})
}

func HandleList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cur := RequestUser(r)

	itype := ps.ByName("itype")

	var displayList interface{}

	switch itype {
	case "discussion":
		displayList = DiscussionGetList(cur)
	case "user":
		displayList = Event.Users.GetUsers()
	default:
		return
	}

	RenderTemplate(w, r, itype+"/list", map[string]interface{}{
		"List": displayList,
	})
}
