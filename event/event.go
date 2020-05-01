package event

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/gwd/session-scheduler/id"
)

type EventStore struct {
	filename string
	*sqlx.DB
}

type EventOptions struct {
	AdminPwd   string
	dbFilename string
}

var event EventStore

const (
	DbFilename    = "data/event.sqlite"
	AdminUsername = "admin"
)

// handleAdminPwd: If there is no admin user, create it.
//  - If opt.AdminPwd is non-empty, use it; otherwise generate one.
//  - Generate one.  Either way, print the password.
//
// If there is no admin user, but opt.AdminPwd is non-zero, set the password.
func handleAdminPwd(adminPwd string) {
	admin, err := UserFindByUsername(AdminUsername)

	if err != nil {
		log.Fatalf("handleAdminPwd: Error finding admin user: %v", err)
	}

	if admin != nil {
		if adminPwd != "" {
			if err != nil || admin == nil {
				log.Fatalf("Can't find admin user: %v", err)
			}
			log.Printf("Resetting admin password")
			admin.setPassword(adminPwd)
		}
		return
	}

	// admin user doesn't exist; create it
	if adminPwd == "" {
		adminPwd = id.GenerateRawID(12)
	}

	_, err = NewUser(adminPwd, &User{Username: AdminUsername,
		IsAdmin:    true,
		IsVerified: true,
		RealName:   "Xen Schedule Administrator"})
	if err != nil {
		log.Fatalf("Error creating admin user: %v", err)
	}
	log.Printf("Administrator account: admin %s", adminPwd)
}

func (store *EventStore) Load(opt EventOptions) error {
	var err error
	event.DB, err = openDb(opt.dbFilename)
	if err != nil {
		return err
	}

	handleAdminPwd(opt.AdminPwd)

	// FIXME: Clean up any stale 'running' data

	return nil
}

func Load(opt EventOptions) error {
	if opt.dbFilename == "" {
		opt.dbFilename = DbFilename
	}
	return event.Load(opt)
}

func Close() {
	if event.DB != nil {
		event.DB.Close()
	}
	event = EventStore{}
}

func UserFind(userid UserID) (*User, error) {
	var user User
	for {
		err := event.Get(&user, `select * from event_users where userid=?`,
			userid)
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

func UserGetAll() (users []User, err error) {
	for {
		err = event.Select(&users, `select * from event_users order by userid`)
		switch {
		case shouldRetry(err):
			continue
		default:
			return users, err
		}
	}
}

func discussionIterateQuery(query string, args []interface{}, f func(*Discussion) error) error {
	for {
		rows, err := event.Queryx(query, args...)
		switch {
		case shouldRetry(err):
			continue
		case err != nil:
			return err
		}
		defer rows.Close()
		processed := 0
		for rows.Next() {
			var disc Discussion
			if err := rows.StructScan(&disc); err != nil {
				return err
			}
			err = f(&disc)
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

func DiscussionIterate(f func(*Discussion) error) error {
	return discussionIterateQuery(`select * from event_discussions order by discussionid`, nil, f)
}

// FIXME: This will simply do nothing if the userid doesn't exist.  It
// would be nice for the caller to distinguish between "User does not
// exist" and "User has no discussions".
func DiscussionIterateUser(userid UserID, f func(*Discussion) error) (err error) {
	return discussionIterateQuery(`select * from event_discussions where owner=? order by discussionid`, []interface{}{userid}, f)
}
