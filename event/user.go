package event

import (
	"log"

	"golang.org/x/crypto/bcrypt"

	"github.com/gwd/session-scheduler/id"
)

const (
	hashCost       = 10
	passwordLength = 6
	userIDLength   = 16
	InterestMax    = 100
)

type UserID string

func (uid *UserID) generate() {
	*uid = UserID(id.GenerateID("usr", userIDLength))
}

type UserProfile struct {
	RealName    string
	Email       string
	Company     string
	Description string
}

type User struct {
	ID             UserID
	HashedPassword string
	Username       string
	IsAdmin        bool
	IsVerified     bool // Has entered the verification code
	Interest       map[DiscussionID]int
	// Profile: Informational only
	Profile UserProfile
}

func (u *User) MayEditUser(tgt *User) bool {
	return u.IsAdmin || u.ID == tgt.ID
}

func (u *User) MayEditDiscussion(d *Discussion) bool {
	return u.IsAdmin || u.ID == d.Owner
}

func NewUser(username, password string, isVerified bool, profile *UserProfile) (*User, error) {
	user := &User{
		Username:   username,
		Profile:    *profile,
		IsVerified: isVerified,
	}

	log.Printf("New user post: '%s'", username)

	if username == "" || AllWhitespace(username) {
		log.Printf("New user failed: no username")
		return user, errNoUsername
	}

	if IsEmailAddress(username) {
		log.Printf("New user failed: Username looks like an email address")
		return user, errUsernameIsEmail
	}

	if password == "" {
		log.Printf("New user failed: no password")
		return user, errNoPassword
	}

	if len(password) < passwordLength {
		log.Printf("New user failed: password too short")
		return user, errPasswordTooShort
	}

	// Check if the username exists
	existingUser, err := event.Users.FindByUsername(username)
	if err != nil {
		log.Printf("New user failed: %v", err)
		return user, err
	}
	if existingUser != nil {
		log.Printf("New user failed: user exists")
		return user, errUsernameExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), hashCost)
	user.HashedPassword = string(hashedPassword)
	user.ID.generate()

	user.Interest = make(map[DiscussionID]int)

	event.Users.Save(user)

	return user, err
}

func FindUser(username, password string) (*User, error) {
	out := &User{
		Username: username,
	}

	existingUser, err := event.Users.FindByUsername(username)
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

// Use when you plan on setting a large sequence in a row and can save
// the state yourself
func (user *User) SetInterestNosave(disc *Discussion, interest int) error {
	log.Printf("Setinterest: %s '%s' %d", user.Username, disc.Title, interest)
	if interest > InterestMax || interest < 0 {
		log.Printf("SetInterest failed: Invalid interest")
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
	event.ScheduleState.Modify()
	return nil
}

func (user *User) SetInterest(disc *Discussion, interest int) (err error) {
	err = user.SetInterestNosave(disc, interest)
	if err == nil {
		event.Save()
	}
	return
}

func (user *User) SetPassword(newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), hashCost)
	if err != nil {
		return err
	}
	user.HashedPassword = string(hashedPassword)
	return nil
}

func (user *User) SetVerified(isVerified bool) error {
	user.IsVerified = isVerified
	event.Users.Save(user)
	return nil
}

func UserRemoveDiscussion(did DiscussionID) error {
	return event.Users.Iterate(func(u *User) error {
		delete(u.Interest, did)
		return nil
	})
}

func UpdateUser(user, modifier *User, currentPassword, newPassword string,
	profile *UserProfile) (User, error) {
	out := *user
	out.Profile = *profile

	if newPassword != "" {
		// No current password? Don't try update the password.
		if currentPassword == "" {
			return out, nil
		}

		if bcrypt.CompareHashAndPassword(
			[]byte(modifier.HashedPassword),
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

	event.Users.Save(user)

	return out, nil
}

func DeleteUser(uid UserID) error {
	dlist := event.Discussions.GetDidListUser(uid)

	for _, did := range dlist {
		DeleteDiscussion(did)
	}

	DiscussionRemoveUser(uid)

	event.Users.Delete(uid)

	return nil
}
