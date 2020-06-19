package main

import (
	"html/template"
	"log"

	"github.com/gwd/session-scheduler/event"
)

type UserProfile struct {
	RealName    string
	Email       string
	Company     string
	Description string // Raw description, suitable for editing
}

type UserDisplay struct {
	UserID      event.UserID
	Username    string
	IsAdmin     bool
	IsVerified  bool // Has entered the verification code
	MayEdit     bool
	Profile     UserProfile
	Description template.HTML // Sanitised description, suitable for displaying
	List        []*DiscussionDisplay
}

func UserGetDisplay(u *event.User, cur *event.User, long bool) (ud *UserDisplay) {
	ud = &UserDisplay{
		UserID:     u.UserID,
		Username:   u.Username,
		IsVerified: u.IsVerified,
	}
	// Only show profile information to registered users
	if cur != nil {
		ud.MayEdit = cur.MayEditUser(u)
		ud.IsAdmin = cur.IsAdmin
		// Only display profile information to people who are logged in
		ud.Profile.RealName = u.RealName
		ud.Profile.Email = u.Email
		ud.Profile.Company = u.Company
		ud.Profile.Description = u.Description
		ud.Description = ProcessText(u.Description)
	}
	// But show discussions to everyone.  (This is already available
	// from the 'sessions' list.)
	ud.List = DiscussionGetListUser(u, cur)
	return
}

type DiscussionDisplay struct {
	event.DiscussionFull

	DescriptionRaw  string
	DescriptionHTML template.HTML
	TitleDisplay    string
	IsUser          bool
	MayEdit         bool
	IsAdmin         bool
	TimeDisplay     string
	Interest        int

	AllUsers []event.User
}

func DiscussionGetDisplay(d *event.DiscussionFull, cur *event.User) *DiscussionDisplay {
	showMain := true

	// Only display a discussion if:
	// 1. It's pulbic, or...
	// 2. The current user is admin, or the discussion owner
	if !d.IsPublic &&
		(cur == nil || (!cur.IsAdmin && cur.UserID != d.Owner)) {
		if d.ApprovedTitle == "" {
			return nil
		} else {
			showMain = false
		}
	}

	dd := &DiscussionDisplay{
		DiscussionFull: *d,
	}

	if showMain {
		dd.TitleDisplay = d.Title
		dd.DescriptionRaw = d.Description
	} else {
		dd.Title = d.ApprovedTitle
		dd.DescriptionRaw = d.ApprovedDescription
	}

	dd.DescriptionHTML = ProcessText(dd.DescriptionRaw)

	if cur != nil {
		if cur.Username != event.AdminUsername {
			dd.IsUser = true
			dd.Interest, _ = cur.GetInterest(d)
		}
		dd.MayEdit = cur.MayEditDiscussion(d)
		if cur.IsAdmin {
			dd.IsAdmin = true
			var err error
			dd.AllUsers, err = event.UserGetAll()
			if err != nil {
				// Report error but continue
				log.Printf("INTERNAL ERROR: Getting all users: %v", err)
			}
		} else {
			dd.PossibleSlots = nil
		}

	}
	return dd
}

func DiscussionGetListUser(u *event.User, cur *event.User) (list []*DiscussionDisplay) {
	event.DiscussionIterateUser(u.UserID, func(d *event.Discussion) error {
		dd := DiscussionGetDisplay(d, cur)
		if dd != nil {
			list = append(list, dd)
		}
		return nil
	})

	return
}

func DiscussionGetList(cur *event.User) (list []*DiscussionDisplay) {
	event.DiscussionIterate(func(d *event.Discussion) error {
		dd := DiscussionGetDisplay(d, cur)
		if dd != nil {
			list = append(list, dd)
		}
		return nil
	})

	return
}

func UserGetUsersDisplay(cur *event.User) (users []*UserDisplay) {
	event.UserIterate(func(u *event.User) error {
		if u.Username != event.AdminUsername {
			users = append(users, UserGetDisplay(u, cur, false))
		}
		return nil
	})
	return
}
