package main

import (
	"net/http"
	"github.com/julienschmidt/httprouter"
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
		owner.ID,
		r.FormValue("title"),
		r.FormValue("description"),
	)

	if err != nil {
		if IsValidationError(err) {
			RenderTemplate(w, r, "discussion/new", map[string]interface{}{
				"Error": err.Error(),
				"Discussion":  disc,
			})
			return
		}
		panic(err)
	}

	
	http.Redirect(w, r, disc.GetURL()+"?flash=Session+Created", http.StatusFound)
}

func HandleDiscussionView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	disc, _ := DiscussionFindById(ps.ByName("discid"))

	if disc == nil {
		http.Redirect(w, r, "discussion/notfound", http.StatusFound)
		return
	}
	
	RenderTemplate(w, r, "discussion/view", map[string]interface{}{
		"Discussion": disc,
	})
}
