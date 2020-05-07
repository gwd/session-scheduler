package event

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/gwd/session-scheduler/id"
)

type EventStore struct {
	filename string
	*sqlx.DB
	defaultLocation *time.Location
}

type EventOptions struct {
	AdminPwd        string
	DefaultLocation string
	dbFilename      string
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
			if admin == nil {
				log.Fatalf("Cannot set admin password: No such user")
			}
			log.Printf("Resetting admin password")
			err = admin.setPassword(adminPwd)
			if err != nil {
				log.Fatalf("resetting admin password: %v", err)
			}
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
	if opt.DefaultLocation == "" {
		log.Fatalf("No default location!")
	}

	var err error

	event.defaultLocation, err = time.LoadLocation(opt.DefaultLocation)
	if err != nil {
		return err
	}

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
