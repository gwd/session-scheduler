package main

import "golang.org/x/crypto/bcrypt"

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

func NewUser(username, password string, profile *UserProfile) (*User, error) {
	user := &User{
		Username: username,
		Profile: *profile,
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

func UpdateUser(user *User, currentPassword, newPassword string,
                 profile *UserProfile) (User, error) {
	out := *user
	out.Profile = *profile

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

	if newPassword == "" {
		return out, errNoPassword
	}

	if len(newPassword) < passwordLength {
		return out, errPasswordTooShort
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), hashCost)
	user.HashedPassword = string(hashedPassword)

	user.Profile = *profile

	Event.Users.Save(user)

	return out, err
}
