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
				"Discussion": disc.GetDisplay(nil),
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
	if !(((action == "view" || action == "edit") &&
		(itype == "user" || itype == "discussion")) ||
		(itype == "discussion" && action == "delete")) {
		return
	}

	var display interface{}

	switch itype {
	case "discussion":
		disc, _ := DiscussionFindById(uid)

		if disc != nil && (action != "edit" || cur.MayEditDiscussion(disc)) {
			display = disc.GetDisplay(cur)
		}
	case "user":
		user, _ := Event.Users.Find(UserID(uid))
		if user != nil && (action != "edit" || cur.MayEditUser(UserID(uid))) {
			display = user.GetDisplay(cur, true)
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

	//log.Printf("%s/%s: Display %v", itype, action, display)

	RenderTemplate(w, r, itype+"/"+action, map[string]interface{}{
		"Display": display,
	})
}

func FormCheckToBool(formData []string) (bslot []bool, err error) {
	bslot = make([]bool, Event.ScheduleSlots)
	for _, iString := range formData {
		i, err := strconv.Atoi(iString)
		if err != nil {
			return nil, err
		}
		bslot[i] = true
	}
	return
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

	//log.Printf("POST %s %s %s", uid, action, itype)

	if !((itype == "discussion" &&
		(action == "setinterest" || action == "edit" || action == "delete" || action == "setpublic")) ||
		(itype == "user" && action == "edit")) {
		log.Printf(" Disallowed action")
		return
	}

	// By default, redirect to the current Uid's 'view'.  This will be
	// overriden if necessary.
	redirectURL := "view"

	switch itype {
	case "discussion":
		disc, _ := DiscussionFindById(uid)
		if disc == nil {
			log.Printf("Invalid discussion: %s", uid)
			return
		}
		switch action {
		case "setinterest":
			// Administrators can't express interest in discussions
			if cur.Username == AdminUsername {
				log.Printf("%s user can't express interest",
					AdminUsername)
				return
			}

			interestString := r.FormValue("interest")
			interest, err := strconv.Atoi(interestString)
			if err != nil {
				log.Printf("Error parsing interest: %v", err)
				return
			}
			if !(interest >= 0) {
				log.Printf("Negative interest (%d)", interest)
				return
			}
			cur.SetInterest(disc, interest)

			if tmp := r.FormValue("redirectURL"); tmp != "" {
				redirectURL = tmp
			}

		case "edit":
			if !cur.MayEditDiscussion(disc) {
				log.Printf("WARNING user %s doesn't have permission to edit discussion %s",
					cur.Username, disc.ID)
				return
			}

			title := r.FormValue("title")
			description := r.FormValue("description")
			var possibleSlots []bool
			owner := disc.Owner

			if cur.IsAdmin {
				var err error
				possibleSlots, err = FormCheckToBool(r.Form["possible"])
				if err != nil {
					return
				}
				owner = UserID(r.FormValue("owner"))
			}

			discussionNext, err := UpdateDiscussion(disc, title, description, possibleSlots, owner)
			if err != nil {
				if IsValidationError(err) {
					RenderTemplate(w, r, "edit", map[string]interface{}{
						"Error":   err.Error(),
						"Display": discussionNext,
					})
					return
				}
				panic(err)
			}
		case "delete":
			if !cur.MayEditDiscussion(disc) {
				log.Printf("WARNING user %s doesn't have permission to edit discussion %s",
					cur.Username, disc.ID)
				return
			}

			DeleteDiscussion(disc.ID)

			// Can't redirect to 'view' as it's been deleted
			http.Redirect(w, r, "/list/discussion", http.StatusFound)
			return
		case "setpublic":
			// Only administrators can change public
			if !cur.IsAdmin {
				log.Printf("%s isn't an admin")
				return
			}

			newValueString := r.FormValue("newvalue")
			if newValueString == "true" {
				disc.IsPublic = true
			} else {
				disc.IsPublic = false
			}

			if tmp := r.FormValue("redirectURL"); tmp != "" {
				redirectURL = tmp
			}

			Event.Discussions.Save(disc)

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
				log.Printf("Invalid user: %s", uid)
				return
			}

			currentPassword := r.FormValue("currentPassword")
			newPassword := r.FormValue("newPassword")
			profile := parseProfile(r)

			log.Printf(" new profile %v", *profile)

			userNext, err := UpdateUser(user, cur, currentPassword, newPassword, profile)

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

	http.Redirect(w, r, redirectURL, http.StatusFound)

}

func HandleList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cur := RequestUser(r)

	itype := ps.ByName("itype")

	templateArgs := make(map[string]interface{})

	switch itype {
	case "discussion":
		templateArgs["List"] = DiscussionGetList(cur)
		templateArgs["redirectURL"] = ""
	case "user":
		templateArgs["List"] = Event.Users.GetUsersDisplay(cur)
	default:
		return
	}

	RenderTemplate(w, r, itype+"/list", templateArgs)
}
