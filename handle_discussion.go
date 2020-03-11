package main

import (
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gwd/session-scheduler/event"
)

func HandleDiscussionNew(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if kvs.GetBoolDef(FlagRequireVerification) {
		cur := RequestUser(r)
		if !IsVerified(cur) {
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

	if kvs.GetBoolDef(FlagRequireVerification) && !IsVerified(owner) {
		http.Redirect(w, r, "/uid/user/self/view", http.StatusFound)
		return
	}

	d := event.Discussion{
		Owner:       owner.UserID,
		Title:       r.FormValue("title"),
		Description: r.FormValue("description")}
	err := event.NewDiscussion(&d)

	if err != nil {
		if event.IsValidationError(err) {
			RenderTemplate(w, r, "discussion/new", map[string]interface{}{
				"Error":      err.Error(),
				"Discussion": DiscussionGetDisplay(&d, owner),
			})
			return
		}
		panic(err)
	}

	http.Redirect(w, r, d.GetURL()+"?flash=Session+Created", http.StatusFound)
}

// A safety catch to DTRT if either user or discussion are nil
func MayEditDiscussion(u *event.User, d *event.Discussion) bool {
	if u == nil || d == nil {
		return false
	}
	return u.MayEditDiscussion(d)
}

func MayEditUser(cur *event.User, tgt *event.User) bool {
	if cur == nil || tgt == nil {
		return false
	}
	return cur.MayEditUser(tgt)
}

func IsAdmin(u *event.User) bool {
	return u != nil && u.IsAdmin
}

func IsVerified(u *event.User) bool {
	return u != nil && (u.IsVerified || u.IsAdmin)
}

func HandleUid(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cur := RequestUser(r)

	uid := ps.ByName("uid")
	action := ps.ByName("action")
	itype := ps.ByName("itype")

	if itype == "user" && uid == "self" {
		if cur != nil {
			uid = string(cur.UserID)
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
		d, _ := event.DiscussionFindById(event.DiscussionID(uid))

		if d == nil {
			break
		}

		// Unverified accounts can't create or edit sessions
		if action == "edit" && kvs.GetBoolDef(FlagRequireVerification) && !IsVerified(cur) {
			http.Redirect(w, r, "/uid/user/self/view", http.StatusFound)
			return
		}

		// Only display edit or delete confirmation pages if the
		// current user can perform that action
		if (action == "edit" || action == "delete") &&
			!MayEditDiscussion(cur, d) {
			break
		}

		display = DiscussionGetDisplay(d, cur)
	case "user":
		user, _ := event.UserFind(event.UserID(uid))

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

		display = UserGetDisplay(user, cur, true)
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
	bslot = make([]bool, event.ScheduleGetSlots())
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
			uid = string(cur.UserID)
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
		d, _ := event.DiscussionFindById(event.DiscussionID(uid))
		if d == nil {
			log.Printf("Invalid discussion: %s", uid)
			return
		}
		switch action {
		case "setinterest":
			// Administrators can't express interest in discussions
			if cur.Username == event.AdminUsername {
				log.Printf("%s user can't express interest",
					event.AdminUsername)
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
			cur.SetInterest(d, interest)

			if tmp := r.FormValue("redirectURL"); tmp != "" {
				redirectURL = tmp
			}

		case "edit":
			if !cur.MayEditDiscussion(d) {
				log.Printf("WARNING user %s doesn't have permission to edit discussion %s",
					cur.Username, d.DiscussionID)
				return
			}

			// Unverified accounts can't create or edit sessions
			if kvs.GetBoolDef(FlagRequireVerification) &&
				!IsVerified(cur) {
				http.Redirect(w, r, "/uid/user/self/view", http.StatusFound)
				return
			}

			discussionNext := *d
			discussionNext.Title = r.FormValue("title")
			discussionNext.Description = r.FormValue("description")

			//var possibleSlots []bool

			if cur.IsAdmin {
				var err error
				//possibleSlots, err = FormCheckToBool(r.Form["possible"])
				if err != nil {
					return
				}
				discussionNext.Owner = event.UserID(r.FormValue("owner"))
			}

			err := event.DiscussionUpdate(&discussionNext)
			if err != nil {
				if event.IsValidationError(err) {
					RenderTemplate(w, r, "edit", map[string]interface{}{
						"Error":   err.Error(),
						"Display": discussionNext,
					})
					return
				}
				panic(err)
			}
		case "delete":
			if !cur.MayEditDiscussion(d) {
				log.Printf("WARNING user %s doesn't have permission to edit discussion %s",
					cur.Username, d.DiscussionID)
				return
			}

			event.DeleteDiscussion(d.DiscussionID)

			// Can't redirect to 'view' as it's been deleted
			http.Redirect(w, r, "/list/discussion", http.StatusFound)
			return
		case "setpublic":
			// Only administrators can change public
			if !cur.IsAdmin {
				log.Printf("%s isn't an admin")
				return
			}

			if err := event.DiscussionSetPublic(d.DiscussionID, r.FormValue("newvalue") == "true"); err != nil {
				// FIXME
				log.Printf("DiscussionSetPublic: %v", err)
			}

			if tmp := r.FormValue("redirectURL"); tmp != "" {
				redirectURL = tmp
			}

		default:
			return
		}
	case "user":
		user, _ := event.UserFind(event.UserID(uid))
		if user == nil {
			log.Printf("Invalid user: %s", uid)
			return
		}

		// Only allowed to edit our own profile unless you're an admin
		if !cur.MayEditUser(user) {
			log.Printf(" uid %s tried to edit uid %s", string(cur.UserID), uid)
			return
		}

		switch action {
		case "edit":
			currentPassword := r.FormValue("currentPassword")
			newPassword := r.FormValue("newPassword")
			// Make a copy of the current user data, then update the copy to be the new
			// user data.  We do this so that in the event of an error, we can fill in the form
			// with the data the user entered (rather than losing it).
			userNext := *user
			parseProfile(r, &userNext)

			log.Printf(" new user info %v", userNext)

			err := event.UserUpdate(&userNext, cur, currentPassword, newPassword)

			if err != nil {
				if event.IsValidationError(err) {
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
			user.SetVerified(newValueString == "true")

			if tmp := r.FormValue("redirectURL"); tmp != "" {
				redirectURL = tmp
			}
		case "verify":
			vcode := r.FormValue("Vcode")

			evcode, _ := kvs.Get(VerificationCode)
			if vcode != evcode {
				redirectURL = "view?flash=Invalid+Validation+Code"
			} else {
				user.SetVerified(true)
				redirectURL = "view?flash=Account+Verified"
			}
		case "delete":
			if !cur.IsAdmin {
				log.Printf("WARNING user %s isn't an admin", cur.Username)
				return
			}

			event.DeleteUser(user.UserID)

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
		templateArgs["List"] = UserGetUsersDisplay(cur)
	default:
		return
	}

	RenderTemplate(w, r, itype+"/list", templateArgs)
}
