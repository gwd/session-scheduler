package main

import (
	"html/template"
	"log"

	"github.com/gwd/session-scheduler/event"
)

type UserProfile struct {
	RealName string
	Email    string
	Company  string
}

type UserDisplay struct {
	UserID      event.UserID
	Username    string
	IsAdmin     bool
	IsVerified  bool // Has entered the verification code
	MayEdit     bool
	Profile     UserProfile
	Description template.HTML
	List        []*DiscussionDisplay
}

func UserGetDisplay(u *event.User, cur *event.User, long bool) (ud *UserDisplay) {
	ud = &UserDisplay{
		UserID:     u.UserID,
		Username:   u.Username,
		IsVerified: u.IsVerified,
	}
	if cur != nil {
		ud.MayEdit = cur.MayEditUser(u)
		ud.IsAdmin = cur.IsAdmin
		// Only display profile information to people who are logged in
		ud.Profile.RealName = u.RealName
		ud.Profile.Email = u.Email
		ud.Profile.Company = u.Company
		ud.Description = ProcessText(u.Description)
		ud.List = DiscussionGetListUser(u, cur)
	}
	return
}

type DiscussionDisplay struct {
	DiscussionID   event.DiscussionID
	Title          string
	Description    template.HTML
	DescriptionRaw string
	Owner          *event.User
	// Interested     []*event.User // Doesn't seem to be used
	IsPublic bool
	// IsUser: Used to determine whether to display 'interest'
	IsUser bool
	// MayEdit: Used to determine whether to show edit / delete buttons
	MayEdit bool
	// IsAdmin: Used to determine whether to show slot scheduling options
	IsAdmin       bool
	Interest      int
	Location      event.Location
	Time          string
	IsFinal       bool
	PossibleSlots []event.DisplaySlot
	// AllUsers: Used to generate a dropdown for admins to change the
	// owner.  Only geneated for admin user.
	AllUsers []event.User
}

func DiscussionGetDisplay(d *event.Discussion, cur *event.User) *DiscussionDisplay {
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
		DiscussionID: d.DiscussionID,
		IsPublic:     d.IsPublic,
	}

	if showMain {
		dd.Title = d.Title
		dd.DescriptionRaw = d.Description
	} else {
		dd.Title = d.ApprovedTitle
		dd.DescriptionRaw = d.ApprovedDescription
	}

	dd.Description = ProcessText(dd.DescriptionRaw)

	dd.Location = d.Location()

	dd.IsFinal, dd.Time = d.Slot()

	dd.Owner, _ = event.UserFind(d.Owner)
	if cur != nil {
		if cur.Username != event.AdminUsername {
			dd.IsUser = true
			dd.Interest = cur.GetInterest(d)
		}
		dd.MayEdit = cur.MayEditDiscussion(d)
		if cur.IsAdmin {
			dd.IsAdmin = true
			//dd.PossibleSlots = event.TimetableFillDisplaySlots(d.PossibleSlots)
			var err error
			dd.AllUsers, err = event.UserGetAll()
			if err != nil {
				// Report error but continue
				log.Printf("INTERNAL ERROR: Getting all users: %v", err)
			}
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
