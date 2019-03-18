package main

import (
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"reflect"
	"strconv"
)

func HandleDiscussionNew(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if Event.RequireVerification {
		cur := RequestUser(r)
		if cur == nil || !cur.IsVerified {
			http.Redirect(w, r, "/uid/user/self/view", http.StatusFound)
			return
		}
	}

	RenderTemplate(w, r, "discussion/new", nil)
}

func HandleDiscussionNotFound(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	RenderTemplate(w, r, "discussion/notfound", nil)
}

func HandleDiscussionCreate(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	owner := RequestUser(r)

	if Event.RequireVerification && (owner == nil || !owner.IsVerified) {
		http.Redirect(w, r, "/uid/user/self/view", http.StatusFound)
		return
	}

	disc, err := NewDiscussion(
		owner,
		r.FormValue("title"),
		r.FormValue("description"),
	)

	if err != nil {
		if IsValidationError(err) {
			RenderTemplate(w, r, "discussion/new", map[string]interface{}{
				"Error":      err.Error(),
				"Discussion": disc.GetDisplay(owner),
			})
			return
		}
		panic(err)
	}

	http.Redirect(w, r, disc.GetURL()+"?flash=Session+Created", http.StatusFound)
}

// A safety catch to DTRT if either user or discussion are nil
func MayEditDiscussion(u *User, d *Discussion) bool {
	if u == nil || d == nil {
		return false
	}
	return u.MayEditDiscussion(d)
}

func MayEditUser(cur *User, tgt *User) bool {
	if cur == nil || tgt == nil {
		return false
	}
	return cur.MayEditUser(tgt)
}

func IsAdmin(u *User) bool {
	return u != nil && u.IsAdmin
}

func IsVerified(u *User) bool {
	return u != nil && u.IsVerified
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

	if !((action == "view" || action == "edit" || action == "delete") &&
		(itype == "user" || itype == "discussion")) {
		return
	}

	// Modifying things always requires a login
	if (action == "edit" || action == "delete") && cur == nil {
		RequireLogin(w, r)
		return
	}

	var display interface{}

	switch itype {
	case "discussion":
		disc, _ := DiscussionFindById(uid)

		if disc == nil {
			break
		}

		// Unverified accounts can't create or edit sessions
		if action == "edit" && Event.RequireVerification && !IsVerified(cur) {
			http.Redirect(w, r, "/uid/user/self/view", http.StatusFound)
			return
		}

		// Only display edit or delete confirmation pages if the
		// current user can perform that action
		if (action == "edit" || action == "delete") &&
			!MayEditDiscussion(cur, disc) {
			break
		}

		display = disc.GetDisplay(cur)
	case "user":
		user, _ := Event.Users.Find(UserID(uid))

		if user == nil {
			break
		}

		// Only display edit confirmation page if the current user can edit.
		if action == "edit" && !MayEditUser(cur, user) {
			break
		}

		// Only display a delete confirmation page for admins
		if action == "delete" && !IsAdmin(cur) {
			break
		}

		display = user.GetDisplay(cur, true)
	default:
		return
	}

	// Check for nil return values from GetDisplay functions, as will as nil interface value
	if display == nil || reflect.ValueOf(display).IsNil() {
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
		(itype == "user" && (action == "edit" || action == "setverified" || action == "verify" || action == "delete"))) {
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

			// Unverified accounts can't create or edit sessions
			if Event.RequireVerification &&
				(cur == nil || !cur.IsVerified) {
				http.Redirect(w, r, "/uid/user/self/view", http.StatusFound)
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
				// When making something public, keep track of the
				// "approved" value
				disc.IsPublic = true
				disc.ApprovedTitle = disc.Title
				disc.ApprovedDescription = disc.Description
			} else {
				// To actually hide something, the ApprovedTitle needs
				// to be false as well.
				disc.IsPublic = false
				disc.ApprovedTitle = ""
				disc.ApprovedDescription = ""
			}

			if tmp := r.FormValue("redirectURL"); tmp != "" {
				redirectURL = tmp
			}

			Event.Discussions.Save(disc)

		default:
			return
		}
	case "user":
		user, _ := Event.Users.Find(UserID(uid))
		if user == nil {
			log.Printf("Invalid user: %s", uid)
			return
		}

		// Only allowed to edit our own profile unless you're an admin
		if !cur.MayEditUser(user) {
			log.Printf(" uid %s tried to edit uid %s", string(cur.ID), uid)
			return
		}

		switch action {
		case "edit":
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
		case "setverified":
			// Only administrators can change verification status
			if !cur.IsAdmin {
				log.Printf("%s isn't an admin")
				return
			}

			newValueString := r.FormValue("newvalue")
			if newValueString == "true" {
				user.IsVerified = true
			} else {
				user.IsVerified = false
			}

			if tmp := r.FormValue("redirectURL"); tmp != "" {
				redirectURL = tmp
			}

			Event.Users.Save(user)
		case "verify":
			vcode := r.FormValue("Vcode")

			if vcode != Event.VerificationCode {
				redirectURL = "view?flash=Invalid+Validation+Code"
			} else {
				user.IsVerified = true
				Event.Users.Save(user)
				redirectURL = "view?flash=Account+Verified"
			}
		case "delete":
			if !cur.IsAdmin {
				log.Printf("WARNING user %s isn't an admin", cur.Username)
				return
			}

			DeleteUser(user.ID)

			// Can't redirect to 'view' as it's been deleted
			http.Redirect(w, r, "/list/user", http.StatusFound)
			return
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
