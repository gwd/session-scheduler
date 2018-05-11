package main

import "errors"

type ValidationError error

var (
	errNoUsername           = ValidationError(errors.New("You must supply a username"))
	errNoEmail              = ValidationError(errors.New("You must supply an email"))
	errNoPassword           = ValidationError(errors.New("You must supply a password"))
	errPasswordTooShort     = ValidationError(errors.New("Your password is too short"))
	errPasswordIncorrect    = ValidationError(errors.New("Password did not match"))
	errUsernameExists       = ValidationError(errors.New("That username is taken"))
	errEmailExists          = ValidationError(errors.New("That email address has an account"))
	errCredentialsIncorrect = ValidationError(errors.New("We couldn’t find a user with the supplied username and password combination"))
	errNoTitle              = ValidationError(errors.New("You must provide a title"))
	errTitleExists          = ValidationError(errors.New("That title exists"))
	errNoDesc               = ValidationError(errors.New("You must provide a description"))
	errInvalidInterest      = ValidationError(errors.New("Interest value out of range"))
	errInvalidVcode         = ValidationError(errors.New("Incorrect verification code"))
	errTooManyDiscussions   = ValidationError(errors.New("You have too many discussions"))
)

func IsValidationError(err error) bool {
	_, ok := err.(ValidationError)
	return ok
}
