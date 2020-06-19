package event

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
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
	Location       TZLocation
	RealName       string
	Email          string
	Company        string
	Description    string
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

	if user.Location.Location == nil {
		user.Location.Location = event.defaultLocation
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

	for {
		_, err := event.Exec(`
        insert into event_users(
            userid,
            hashedpassword,
            username,
            isadmin, isverified,
            realname, email, company, description,
            location) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			user.UserID,
			user.HashedPassword,
			user.Username,
			user.IsAdmin, user.IsVerified,
			user.RealName, user.Email, user.Company, user.Description,
			user.Location)
		switch {
		case shouldRetry(err):
			continue
		case isErrorConstraintUnique(err):
			log.Printf("New user failed: user exists")
			return user.UserID, errUsernameExists
		case err != nil:
			log.Printf("New user failed: %v", err)
			return user.UserID, err
		}
		break
	}

	return user.UserID, nil
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

func setInterestTx(ext sqlx.Ext, uid UserID, did DiscussionID, interest int) error {
	_, err := ext.Exec(`
        insert into event_interest(userid, discussionid, interest)
            values(:userid, :discussionid, :interest)
        on conflict(userid, discussionid) do update set interest=:interest`,
		uid, did, interest)
	return err
}

func (user *User) SetInterest(disc *Discussion, interest int) error {
	switch {
	case interest > InterestMax || interest < 0:
		return errInvalidInterest
	case interest == 0:
		for {
			_, err := event.Exec(`
            delete from event_interest
                where discussionid = ? and userid = ?`, disc.DiscussionID, user.UserID)
			switch {
			case shouldRetry(err):
				continue
			case err == sql.ErrNoRows:
				return nil
			default:
				return err
			}
		}
	default:
		for {
			err := setInterestTx(event, user.UserID, disc.DiscussionID, interest)
			switch {
			case shouldRetry(err):
				continue
			case isErrorForeignKey(err):
				return ErrUserOrDiscussionNotFound
			default:
				return err
			}
		}
	}
}

func userGetInterestTx(q sqlx.Queryer, uid UserID, did DiscussionID, interest *int) error {
	return sqlx.Get(q, interest, `
		select interest
            from event_interest
            where userid=? and discussionid=?`,
		uid, did)
}

func (user *User) GetInterest(disc *Discussion) (int, error) {
	// NB this will return 0 even for non-existent users and
	// discussions.  If we wanted to change this, we'd have to return
	// an error code.  We'd also need to either set interest to 0
	// proactively (which would mean setting up interest for all users
	// every time a new discussion was created, and setting up
	// interest for all discussions every time a new user is created)
	// or setting up a query such that we could distinguish between
	// "user/discussion pair exists but has no interest entry" and
	// "user/discussion pair does not exist".
	var interest int
	err := txLoop(func(eq sqlx.Ext) error {
		err := userGetInterestTx(eq, user.UserID, disc.DiscussionID, &interest)
		if err == sql.ErrNoRows {
			err = nil
		} else if err != nil {
			err = errOrRetry("Error getting interest for user", err)
		}
		return err
	})
	return interest, err
}

func passwordHash(newPassword string) (string, error) {
	hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), hashCost)
	return string(hashedPasswordBytes), err
}

func (user *User) setPassword(newPassword string) error {
	hashedPassword, err := passwordHash(newPassword)
	if err != nil {
		return err
	}

	for {
		_, err := event.Exec(`
        update event_users set hashedpassword = ? where userid = ?`,
			hashedPassword, user.UserID)
		switch {
		case shouldRetry(err):
			continue
		case err == nil:
			user.HashedPassword = hashedPassword
			fallthrough
		default:
			return err
		}
	}
}

func (user *User) SetVerified(isVerified bool) error {
	for {
		_, err := event.Exec(`
        update event_users set isverified = ? where userid = ?`,
			isVerified, user.UserID)
		switch {
		case shouldRetry(err):
			continue
		default:
			return err
		}
	}
}

// UserUpdate will update "user-facing" data associated with the user.
// This includes RealName, Email, Company, Description, and Location.
// It can also inlude the password *via* the new/currentPassword
// fields (not the HashedPassword field).
//
// UserUpdate will *not* update Username, IsAdmin or IsVerified.  IsVerified
// should be updated with SetVerified instead.
//
// If newPassword is "", HashedPassword will not be changed. If
// newPassword is non-null, currentPassword will be checked against
// modifier.HashedPassword.
func UserUpdate(userNext, modifier *User, currentPassword, newPassword string) error {
	setPassword := false

	var hashedPassword string

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

		var err error
		hashedPassword, err = passwordHash(newPassword)
		if err != nil {
			return err
		}

		setPassword = true
	}

	q := `update event_users set `
	args := []interface{}{}
	if setPassword {
		q += `hashedpassword = ?, `
		args = append(args, hashedPassword)
	}
	q += `realname = ?, email = ?, company = ?, description = ?, location = ? where userid = ?`
	args = append(args, userNext.RealName)
	args = append(args, userNext.Email)
	args = append(args, userNext.Company)
	args = append(args, userNext.Description)
	args = append(args, userNext.Location)
	args = append(args, userNext.UserID)

	for {
		res, err := event.Exec(q, args...)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
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

		// Only update the password hash if we succeeded in the update
		if setPassword {
			userNext.HashedPassword = hashedPassword
		}

		return nil
	}
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

		// Delete foreign key references first

		// Delete interest of this user in any discussion
		_, err = tx.Exec(`
           delete from event_interest
               where userid = ?`, userid)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Deleting discussion from event_interest: %v", err)
		}

		// Delete interest in any users in discussions owned by this user
		_, err = tx.Exec(`
           delete from event_interest
               where discussionid in (
                   select discussionid
                       from event_discussions
                       where owner = ?)`, userid)
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return fmt.Errorf("Deleting interest in discussions owned by %v: %v",
				userid, err)
		}

		// And delete any discussions owned by this user
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

func userGetTx(q sqlx.Queryer, userid UserID, user *User) error {
	return sqlx.Get(q, user,
		`select * from event_users where userid=?`,
		userid)
}

func UserFind(userid UserID) (*User, error) {
	user := &User{}
	err := txLoop(func(eq sqlx.Ext) error {
		err := userGetTx(eq, userid, user)
		if err == sql.ErrNoRows {
			user = nil
			err = nil
		}
		return err
	})
	return user, err
}

// Return nil for user not present
func UserFindByUsername(username string) (*User, error) {
	var user User
	// FIXME: Consider making usernames case-insensitive.  This
	// involves making the whole column case-insensitive (with collate
	// nocase) so that case-insensitive-uniqueness is maintianed; it
	// also means adding unit tests to ensure that case-differeng
	// usernames collide, and that case-differeng usernames are found
	// by the various searches.
	for {
		err := event.Get(&user, `select * from event_users where username=?`,
			username)
		switch {
		case shouldRetry(err):
			continue
		case err == sql.ErrNoRows:
			return nil, nil
		default:
			return &user, err
		}
	}
}

func UserFindRandom() (*User, error) {
	var user User
	for {
		err := event.Get(&user, `
        select * from event_users
           where username != ?
           order by RANDOM() limit 1`, AdminUsername)
		switch {
		case shouldRetry(err):
			continue
		case err == sql.ErrNoRows:
			return nil, nil
		case err != nil:
			return nil, err
		default:
			return &user, nil
		}
	}
}

// Iterate over all users, calling f(u) for each user.
func UserIterate(f func(u *User) error) error {
	for {
		rows, err := event.Queryx(`select * from event_users order by userid`)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
			return err
		}
		defer rows.Close()
		processed := 0
		for rows.Next() {
			var user User
			if err := rows.StructScan(&user); err != nil {
				return err
			}
			err = f(&user)
			if err != nil {
				return err
			}
			processed++
		}

		// For some reason we often get the transaction conflict error
		// from rows.Err() rather than from the original Queryx.
		// Retrying is fine as long as we haven't actually processed
		// any rows yet.  If we have, throw an error.  (There's an
		// argument to makign this a panic() instead.)
		err = rows.Err()
		if shouldRetry(err) {
			if processed == 0 {
				rows.Close()
				continue
			}
			err = fmt.Errorf("INTERNAL ERROR: Got transaction retry error after processing %d callbacks",
				processed)
		}

		return err
	}
}

func userGetAllTx(q sqlx.Queryer, usersp *[]User) error {
	return sqlx.Select(q, usersp, `select * from event_users order by userid`)
}

func UserGetAll() ([]User, error) {
	var users []User
	err := txLoop(func(eq sqlx.Ext) error {
		err := userGetAllTx(eq, &users)
		if err != nil {
			return errOrRetry("Getting list of all users", err)
		}
		return nil
	})
	return users, err
}
