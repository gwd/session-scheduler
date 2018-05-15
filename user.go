package main

import (
	"html/template"

	"golang.org/x/crypto/bcrypt"
)

const (
	hashCost       = 10
	passwordLength = 6
	userIDLength   = 16
	InterestMax = 100
)

type UserID string

func (uid *UserID) generate() {
	*uid = UserID(GenerateID("usr", userIDLength))
}

type UserProfile struct {
	RealName       string
	Email          string
	Company        string
	Description    string
}

type User struct {
	ID             UserID
	HashedPassword string
	Username       string
	IsAdmin        bool
	Interest       map[DiscussionID]int
	// Profile: Informational only
	Profile        UserProfile
}

type UserDisplay struct {
	ID       UserID
	Username string
	IsAdmin  bool
	MayEdit bool
	Profile  *UserProfile
	Description template.HTML
}

func (u *User) MayEditUser(ID UserID) bool {
	return u.IsAdmin || u.ID == ID
}

func (u *User) MayEditDiscussion(d *Discussion) bool {
	return u.IsAdmin || u.ID == d.Owner
}

func (u *User) GetDisplay(cur *User) (ud *UserDisplay) {
	ud = &UserDisplay{
		ID: u.ID,
		Username: u.Username,
		IsAdmin: u.IsAdmin,
	}
	if cur != nil {
		ud.MayEdit = cur.MayEditUser(u.ID)
		// Only display profile information to people who are logged in
		ud.Profile = &u.Profile
		ud.Description = ProcessText(u.Profile.Description)
	}
	return
}

func NewUser(username, password, vcode string, profile *UserProfile) (*User, error) {
	user := &User{
		Username: username,
		Profile: *profile,
	}

	if vcode != Event.VerificationCode {
		return user, errInvalidVcode
	}
	
	if username == "" {
		return user, errNoUsername
	}

	if password == "" {
		return user, errNoPassword
	}

	if len(password) < passwordLength {
		return user, errPasswordTooShort
	}

	// Check if the username exists
	existingUser, err := Event.Users.FindByUsername(username)
	if err != nil {
		return user, err
	}
	if existingUser != nil {
		return user, errUsernameExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), hashCost)
	user.HashedPassword = string(hashedPassword)
	user.ID.generate()

	user.Interest = make(map[DiscussionID]int)

	Event.Users.Save(user)

	return user, err
}

func FindUser(username, password string) (*User, error) {
	out := &User{
		Username: username,
	}

	existingUser, err := Event.Users.FindByUsername(username)
	if err != nil {
		return out, err
	}
	if existingUser == nil {
		return out, errCredentialsIncorrect
	}

	// Don't bother checking the password if it's empty
	if password == "" ||
		bcrypt.CompareHashAndPassword(
			[]byte(existingUser.HashedPassword),
			[]byte(password),
		) != nil {
		return out, errCredentialsIncorrect
	}

	return existingUser, nil
}

func (user *User) SetInterest(disc *Discussion, interest int) (error) {
	if interest > InterestMax || interest < 0 {
		return errInvalidInterest
	}
	if interest > 0 {
		user.Interest[disc.ID] = interest
		disc.Interested[user.ID] = true
	} else {
		delete(user.Interest, disc.ID)
		delete(disc.Interested, user.ID)
		disc.maxScoreValid = false // Lazily update this when it's wanted
	}
	Event.Save()
	return nil
}

func (user *User) SetPassword(newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), hashCost)
	if err != nil {
		return err
	}
	user.HashedPassword = string(hashedPassword)
	return nil
}

func UserRemoveDiscussion(did DiscussionID) (error) {
	return Event.Users.Iterate(func(u *User) error {
		delete(u.Interest, did)
		return nil
	})
}

func UpdateUser(user *User, currentPassword, newPassword string,
                 profile *UserProfile) (User, error) {
	out := *user
	out.Profile = *profile

	if newPassword != "" {
		// No current password? Don't try update the password.
		if currentPassword == "" {
			return out, nil
		}

		if bcrypt.CompareHashAndPassword(
			[]byte(user.HashedPassword),
			[]byte(currentPassword),
		) != nil {
			return out, errPasswordIncorrect
		}

		if len(newPassword) < passwordLength {
			return out, errPasswordTooShort
		}

		err := user.SetPassword(newPassword)
		if err != nil {
			return out, err
		}
	}

	user.Profile = *profile

	Event.Users.Save(user)

	return out, nil
}
