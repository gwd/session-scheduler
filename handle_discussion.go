package main

import (
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"strconv"
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
	itype := ps.ByName("itype")

	if itype == "user" && uid == "self" {
		if cur != nil {
			uid = string(cur.ID)
		} else {
			return
		}
	}
	
	// OK: discussion / user => view; discussion / user -> edit 
	if !((action == "view" || action == "edit") &&
		(itype == "user" || itype == "discussion")) {
		return
	}
		

	var display interface{}

	switch itype {
	case "discussion":
		disc, _ := DiscussionFindById(uid)

		if disc != nil && (action != "edit" || cur.MayEditDiscussion(disc)) {
			switch action {
			case "edit":
				display = disc
			case "view":
				display = disc.GetDisplay(cur)
			}
		}
	case "user":
		user, _ := Event.Users.Find(UserID(uid))
		if user != nil && (action != "edit" || cur.MayEditUser(UserID(uid))) {
			display = user.GetDisplay(cur)
		}
	default:
		return
	}

	if display == nil {
		RenderTemplate(w, r, "uid/notfound", map[string]interface{}{
			"Utype": itype,
		})
		return
	}

	log.Printf("%s/%s: Display %v", itype, action, display)
	
	RenderTemplate(w, r, itype+"/"+action, map[string]interface{}{
		"Display": display,
	})
}

func HandleUidPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cur := RequestUser(r)

	if cur == nil {
		return
	}

	itype := ps.ByName("itype")
	uid := ps.ByName("uid")
	if itype == "user" && uid == "self" {
		if cur != nil {
			uid = string(cur.ID)
		} else {
			return
		}
	}
	
	action := ps.ByName("action")

	log.Printf("POST %s %s %s", uid, action, itype)
	
	if !((itype == "discussion" &&
		(action == "setinterest" || action == "edit")) ||
		(itype == "user" && action == "edit")) {
		log.Printf(" Disallowed action")
		return
	}

	switch itype {
	case "discussion":
		disc, _ := DiscussionFindById(uid)
		if disc == nil {
			return
		}
		switch action {
		case "setinterest":
			// Administrators can't express interest in discussions
			if cur.Username == AdminUsername {
				return
			}

			interestString := r.FormValue("interest")
			interest, err := strconv.Atoi(interestString)
			if err != nil || !(interest >= 0){
				return
			}
			cur.SetInterest(disc, interest)
		case "edit":
			if !cur.MayEditDiscussion(disc) {
				return
			}

			title := r.FormValue("title")
			description := r.FormValue("description")
			
			discussionNext, err := UpdateDiscussion(disc, title, description)
			if err != nil {
				if IsValidationError(err) {
					RenderTemplate(w, r, "edit", map[string]interface{}{
						"Error":      err.Error(),
						"Display": discussionNext,
					})
					return
				}
				panic(err)
			}
		default:
			return
		}
	case "user":
		// Only allowed to edit our own profile unless you're an admin
		if !cur.MayEditUser(UserID(uid)) {
			log.Printf(" uid %s tried to edit uid %s", string(cur.ID), uid)
			return
		}

		switch action {
		case "edit":
			user, _ := Event.Users.Find(UserID(uid))
			if user == nil {
				return
			}

			currentPassword := r.FormValue("currentPassword")
			newPassword := r.FormValue("newPassword")
			profile := parseProfile(r)

			log.Printf(" new profile %v", *profile)

			userNext, err := UpdateUser(user, currentPassword, newPassword, profile)

			if err != nil {
				if IsValidationError(err) {
					RenderTemplate(w, r, "user/edit", map[string]interface{}{
						"Error": err.Error(),
						"User":  userNext,
					})
					return
				}
				panic(err)
			}
		}

	default:
		return
	}

	http.Redirect(w, r, "view", http.StatusFound)

}

func HandleList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cur := RequestUser(r)

	itype := ps.ByName("itype")

	var displayList interface{}

	switch itype {
	case "discussion":
		displayList = DiscussionGetList(cur)
	case "user":
		displayList = Event.Users.GetUsersDisplay(cur)
	default:
		return
	}

	RenderTemplate(w, r, itype+"/list", map[string]interface{}{
		"List": displayList,
	})
}
