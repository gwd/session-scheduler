package event

import (
	"errors"
	"regexp"
	"unicode"
)

type ValidationError error

var (
	ErrInternal             = ValidationError(errors.New("Internal server error"))
	errNoUsername           = ValidationError(errors.New("You must supply a username"))
	errUsernameIsEmail      = ValidationError(errors.New("Username cannot be an email address"))
	errNoEmail              = ValidationError(errors.New("You must supply an email"))
	errNoPassword           = ValidationError(errors.New("You must supply a password"))
	errPasswordTooShort     = ValidationError(errors.New("Your password is too short"))
	errPasswordIncorrect    = ValidationError(errors.New("Password did not match"))
	errUsernameExists       = ValidationError(errors.New("That username is taken"))
	errEmailExists          = ValidationError(errors.New("That email address has an account"))
	ErrCredentialsIncorrect = ValidationError(errors.New("We couldn’t find a user with the supplied username and password combination"))
	errNoTitle              = ValidationError(errors.New("You must provide a title"))
	errTitleExists          = ValidationError(errors.New("That title exists"))
	errNoDesc               = ValidationError(errors.New("You must provide a description"))
	errInvalidInterest      = ValidationError(errors.New("Interest value out of range"))
	errTooManyDiscussions   = ValidationError(errors.New("You have too many discussions"))
	errAllSlotsLocked       = ValidationError(errors.New("All slots are locked"))
	errInProgress           = ValidationError(errors.New("Schedule already in progress"))
	errModeratedDiscussions = ValidationError(errors.New("Moderated discussions present: Please unmoderate or delete"))
	ErrUserNotFound         = errors.New("UserID not found")
)

func IsValidationError(err error) bool {
	_, ok := err.(ValidationError)
	return ok
}

func AllWhitespace(s string) bool {
	for _, ch := range s {
		if !unicode.IsSpace(ch) {
			return false
		}
	}
	return true
}

var emailRE = regexp.MustCompile("^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\\.[a-zA-Z0-9-.]+$")

func IsEmailAddress(s string) bool {
	return emailRE.MatchString(s)
}
