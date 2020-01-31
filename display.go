package main

import (
	"html/template"
	"sort"

	disc "github.com/gwd/session-scheduler/discussions"
)

type UserDisplay struct {
	ID          disc.UserID
	Username    string
	IsAdmin     bool
	IsVerified  bool // Has entered the verification code
	MayEdit     bool
	Profile     *disc.UserProfile
	Description template.HTML
	List        []*DiscussionDisplay
}

func UserGetDisplay(u *disc.User, cur *disc.User, long bool) (ud *UserDisplay) {
	ud = &UserDisplay{
		ID:         u.ID,
		Username:   u.Username,
		IsVerified: u.IsVerified,
	}
	if cur != nil {
		ud.MayEdit = cur.MayEditUser(u)
		ud.IsAdmin = cur.IsAdmin
		// Only display profile information to people who are logged in
		ud.Profile = &u.Profile
		ud.Description = ProcessText(u.Profile.Description)
		ud.List = DiscussionGetListUser(Event.Discussions, u, cur)
	}
	return
}

type DiscussionDisplay struct {
	ID             disc.DiscussionID
	Title          string
	Description    template.HTML
	DescriptionRaw string
	Owner          *disc.User
	Interested     []*disc.User
	IsPublic       bool
	// IsUser: Used to determine whether to display 'interest'
	IsUser bool
	// MayEdit: Used to determine whether to show edit / delete buttons
	MayEdit bool
	// IsAdmin: Used to determine whether to show slot scheduling options
	IsAdmin       bool
	Interest      int
	Location      disc.Location
	Time          string
	IsFinal       bool
	PossibleSlots []disc.DisplaySlot
	AllUsers      []*disc.User
}

func DiscussionGetDisplay(d *disc.Discussion, cur *disc.User) *DiscussionDisplay {
	showMain := true

	// Only display a discussion if:
	// 1. It's pulbic, or...
	// 2. The current user is admin, or the discussion owner
	if !d.IsPublic &&
		(cur == nil || (!cur.IsAdmin && cur.ID != d.Owner)) {
		if d.ApprovedTitle == "" {
			return nil
		} else {
			showMain = false
		}
	}

	dd := &DiscussionDisplay{
		ID:       d.ID,
		IsPublic: d.IsPublic,
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

	dd.Owner, _ = Event.Users.Find(d.Owner)
	if cur != nil {
		if cur.Username != disc.AdminUsername {
			dd.IsUser = true
			dd.Interest = cur.Interest[d.ID]
		}
		dd.MayEdit = cur.MayEditDiscussion(d)
		if cur.IsAdmin {
			dd.IsAdmin = true
			dd.PossibleSlots = Event.Timetable.FillDisplaySlots(d.PossibleSlots)
			dd.AllUsers = Event.Users.GetUsers()
		}
	}
	for uid := range d.Interested {
		a, _ := Event.Users.Find(uid)
		if a != nil {
			dd.Interested = append(dd.Interested, a)
		}
	}
	return dd
}

func DiscussionGetListUser(dstore disc.DiscussionStore,
	u *disc.User, cur *disc.User) (list []*DiscussionDisplay) {
	dstore.Iterate(func(d *disc.Discussion) error {
		if d.Owner == u.ID {
			dd := DiscussionGetDisplay(d, cur)
			if dd != nil {
				list = append(list, dd)
			}
		}
		return nil
	})

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return
}

func DiscussionGetList(dstore disc.DiscussionStore, cur *disc.User) (list []*DiscussionDisplay) {
	dstore.Iterate(func(d *disc.Discussion) error {
		dd := DiscussionGetDisplay(d, cur)
		if dd != nil {
			list = append(list, dd)
		}
		return nil
	})

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return
}

func UserGetUsersDisplay(ustore disc.UserStore, cur *disc.User) (users []*UserDisplay) {
	ustore.Iterate(func(u *disc.User) error {
		if u.Username != disc.AdminUsername {
			users = append(users, UserGetDisplay(u, cur, false))
		}
		return nil
	})

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})
	return
}
