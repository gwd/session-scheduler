package event

import (
	"fmt"
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

type User struct {
	UserID         UserID
	HashedPassword string
	Username       string
	IsAdmin        bool
	IsVerified     bool // Has entered the verification code
	//Interest       map[DiscussionID]int
	RealName    string
	Email       string
	Company     string
	Description string
}

func (u *User) MayEditUser(tgt *User) bool {
	return u.IsAdmin || u.UserID == tgt.UserID
}

func (u *User) MayEditDiscussion(d *Discussion) bool {
	return u.IsAdmin || u.UserID == d.Owner
}

func NewUser(password string, user *User) (UserID, error) {
	log.Printf("New user post: '%s'", user.Username)

	if user.Username == "" || AllWhitespace(user.Username) {
		log.Printf("New user failed: no username")
		return user.UserID, errNoUsername
	}

	if IsEmailAddress(user.Username) {
		log.Printf("New user failed: Username looks like an email address")
		return user.UserID, errUsernameIsEmail
	}

	switch {
	case user.HashedPassword == "" && password == "":
		if password == "" {
			log.Printf("New user failed: no password")
			return user.UserID, errNoPassword
		}
	case password != "":
		if len(password) < passwordLength {
			log.Printf("New user failed: password too short")
			return user.UserID, errPasswordTooShort
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), hashCost)
		if err != nil {
			log.Printf("Hashing password failed?! %v", err)
			return user.UserID, ErrInternal
		}
		user.HashedPassword = string(hashedPassword)
	}
	user.UserID.generate()

	_, err := event.Exec(`
        insert into event_users(
            userid,
            hashedpassword,
            username,
            isadmin, isverified,
            realname, email, company, description) values(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.UserID,
		user.HashedPassword,
		user.Username,
		user.IsAdmin, user.IsVerified,
		user.RealName, user.Email, user.Company, user.Description)
	switch {
	case isErrorConstraintUnique(err):
		log.Printf("New user failed: user exists")
		return user.UserID, errUsernameExists
	case err != nil:
		log.Printf("New user failed: %v", err)
		return user.UserID, err
	}

	return user.UserID, err
}

func (u *User) CheckPassword(password string) bool {
	// Don't bother checking the password if it's empty
	if password == "" {
		return false
	}

	return bcrypt.CompareHashAndPassword(
		[]byte(u.HashedPassword),
		[]byte(password)) == nil
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
		// FIXME: Interest
		//user.Interest[disc.ID] = interest
		//disc.Interested[user.UserID] = true
	} else {
		// FIXME: Interest
		//delete(user.Interest, disc.ID)
		//delete(disc.Interested, user.UserID)
		disc.maxScoreValid = false // Lazily update this when it's wanted
	}
	event.ScheduleState.Modify()
	return nil
}

func (user *User) SetInterest(disc *Discussion, interest int) error {
	err := user.SetInterestNosave(disc, interest)
	if err == nil {
		event.Save()
	}
	return err
}

func (user *User) setPassword(newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), hashCost)
	if err != nil {
		return err
	}
	user.HashedPassword = string(hashedPassword)
	return nil
}

func (user *User) SetVerified(isVerified bool) error {
	_, err := event.Exec(`
    update event_users set isverified = ? where userid = ?`,
		isVerified, user.UserID)
	return err
}

func UserRemoveDiscussion(did DiscussionID) error {
	// FIXME: Interest
	return nil
}

// UserUpdate will update "user-facing" data associated with the user.
// This includes RealName, Email, Company, and Description.  It can
// also inlude the password.
//
// UserUpdate will *not* update Username, IsAdmin or IsVerified.  IsVerified
// should be updated with SetVerified instead.
//
// If newPassword is "", HashedPassword will not be changed. If
// newPassword is non-null, currentPassword will be checked against
// modifier.HashedPassword.
func UserUpdate(userNext, modifier *User, currentPassword, newPassword string) error {
	setPassword := false

	if newPassword != "" {
		// No current password? Don't try update the password.
		// FIXME: Huh?
		if currentPassword == "" {
			return nil
		}

		if bcrypt.CompareHashAndPassword(
			[]byte(modifier.HashedPassword),
			[]byte(currentPassword),
		) != nil {
			return errPasswordIncorrect
		}

		if len(newPassword) < passwordLength {
			return errPasswordTooShort
		}

		err := userNext.setPassword(newPassword)
		if err != nil {
			return err
		}

		setPassword = true
	}

	q := `update event_users set `
	args := []interface{}{}
	if setPassword {
		q += `hashedpassword = ?, `
		args = append(args, userNext.HashedPassword)
	}
	q += `realname = ?, email = ?, company = ?, description = ? where userid = ?`
	args = append(args, userNext.RealName)
	args = append(args, userNext.Email)
	args = append(args, userNext.Company)
	args = append(args, userNext.Description)
	args = append(args, userNext.UserID)

	res, err := event.Exec(q, args...)
	if err != nil {
		return err
	}

	rcount, err := res.RowsAffected()
	if err != nil {
		log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
		return ErrInternal
	}

	switch {
	case rcount == 0:
		return ErrUserNotFound
	case rcount > 1:
		log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
		return ErrInternal
	}

	return nil
}

func DeleteUser(userid UserID) error {
	for {
		tx, err := event.Beginx()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Starting transaction: %v", err)
		}
		defer tx.Rollback()

		_, err = tx.Exec(`
        delete from event_discussions
            where owner = ?`,
			userid)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Deleting discussions owned by %v: %v",
				userid, err)
		}

		// FIXME: Interest
		// DiscussionRemoveUser(userid)

		res, err := tx.Exec(`
        delete from event_users where userid=?`,
			userid)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Deleting record for user %v: %v",
				userid, err)
		}
		rcount, err := res.RowsAffected()
		if err != nil {
			log.Printf("ERROR Getting number of affected rows: %v; continuing", err)
		}
		switch {
		case rcount == 0:
			return ErrUserNotFound
		case rcount > 1:
			log.Printf("ERROR Expected to change 1 row, changed %d", rcount)
			return ErrInternal
		}

		err = tx.Commit()
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return err
		}
		return nil
	}
}
