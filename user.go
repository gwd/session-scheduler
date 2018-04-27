package main

import "golang.org/x/crypto/bcrypt"

const (
	hashCost       = 10
	passwordLength = 6
	userIDLength   = 16
)

type UserID string

func (uid *UserID) generate() {
	*uid = UserID(GenerateID("usr", userIDLength))
}

type User struct {
	ID             UserID
	Email          string
	HashedPassword string
	Username       string
}

func NewUser(username, email, password string) (*User, error) {
	user := &User{
		Email:    email,
		Username: username,
	}
	if username == "" {
		return user, errNoUsername
	}

	if email == "" {
		return user, errNoEmail
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

	// Check if the email exists
	existingUser, err = Event.Users.FindByEmail(email)
	if err != nil {
		return user, err
	}
	if existingUser != nil {
		return user, errEmailExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), hashCost)
	user.HashedPassword = string(hashedPassword)
	user.ID.generate()

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

func UpdateUser(user *User, email, currentPassword, newPassword string) (User, error) {
	out := *user
	out.Email = email

	// Check if the email exists
	existingUser, err := Event.Users.FindByEmail(email)
	if err != nil {
		return out, err
	}
	if existingUser != nil && existingUser.ID != user.ID {
		return out, errEmailExists
	}

	// At this point, we can update the email address
	user.Email = email

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

	Event.Users.Save(user)

	return out, err
}
