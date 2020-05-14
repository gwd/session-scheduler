package event

import (
	//"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

const codeSchemaVersion = 2

func isSqliteErrorCode(err error, queries ...error) bool {
	if err == nil {
		return false
	}
	sqliteErr, ok := err.(sqlite3.Error)
	if !ok {
		return false
	}
	for _, qerr := range queries {
		switch v := qerr.(type) {
		case sqlite3.ErrNo:
			if sqliteErr.Code == v {
				return true
			}
		case sqlite3.ErrNoExtended:
			if sqliteErr.ExtendedCode == v {
				return true
			}
		default:
			log.Printf("INTERNAL ERROR: isSqliteErrorCode passed invalid type %T", qerr)
		}
	}
	return false
}

func shouldRetry(err error) bool {
	return isSqliteErrorCode(err, sqlite3.ErrBusy, sqlite3.ErrLocked)
}

func isErrorConstraintUnique(err error) bool {
	return isSqliteErrorCode(err, sqlite3.ErrConstraintUnique)
}

func isErrorForeignKey(err error) bool {
	return isSqliteErrorCode(err, sqlite3.ErrConstraintForeignKey)
}

func errOrRetry(comment string, err error) error {
	if shouldRetry(err) {
		return err
	}
	return fmt.Errorf("%s: %v", comment, err)
}

func openDb(filename string) (*sqlx.DB, error) {

	db, err := sqlx.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&_foreign_keys=on", filename))
	if err != nil {
		return nil, err
	}

	// Check for existence of tables
	tx, err := db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var dbSchemaVersion int
	err = tx.Get(&dbSchemaVersion, "pragma user_version")
	if err != nil {
		return nil, fmt.Errorf("Getting schema version: %v", err)
	}

	commit := false

	switch dbSchemaVersion {
	case 0:
		err = initDb(tx)
		if err != nil {
			return nil, fmt.Errorf("Initializing database: %v", err)
		}
		commit = true
	case 1:
		err = upgradeDbv1(tx)
		if err != nil {
			return nil, fmt.Errorf("Upgrading database: %v", err)
		}

		commit = true
	case codeSchemaVersion:
		break
	default:
		return nil, fmt.Errorf("Wrong schema version (code %d, db %d)",
			codeSchemaVersion, dbSchemaVersion)
	}

	// No need to commit in common case
	if commit {
		err = tx.Commit()
		if err != nil {
			return nil, fmt.Errorf("Committing database init/upgrade: %v", err)
		}
	}

	return db, nil
}

func upgradeDbv1(ext sqlx.Ext) error {
	_, err := ext.Exec(fmt.Sprintf("pragma user_version=%d", 2))
	if err != nil {
		return errOrRetry("Updating user_version", err)
	}

	// NB when adding a column with a 'not null' restriction, we have
	// to add a default, even if we don't expect there to be any rows
	// to update.  Testing for the non-upgaded case should catch any
	// insertions which don't include this column.
	_, err = ext.Exec(`alter table event_locations add column locationdescription text not null default ""`)
	if err != nil {
		return errOrRetry("Adding event_locations:locationdescription: %v", err)
	}
	return nil
}

func initDb(ext sqlx.Ext) error {
	_, err := ext.Exec(fmt.Sprintf("pragma user_version=%d", codeSchemaVersion))
	if err != nil {
		return errOrRetry("Setting user_version", err)
	}

	_, err = ext.Exec(`
CREATE TABLE event_users(
    userid          text primary key,
    hashedpassword text not null,
    username        text not null unique,
    isadmin         boolean not null,
    isverified      boolean not null,
    realname        text,
    email           text,
    company         text,
    description     text,
    location        text not null /* Parsable by time.LoadLocation() */)`)
	if err != nil {
		return errOrRetry("Creating table event_users", err)
	}

	_, err = ext.Exec(`
CREATE TABLE event_discussions(
    discussionid        text primary key,
    owner               text not null,
    title               text not null,
    description         text,
    approvedtitle       text,
    approveddescription text,
    ispublic            boolean not null,
    foreign key(owner) references event_users(userid),
    unique(title))`)
	if err != nil {
		return errOrRetry("Creating table event_discussions", err)
	}

	_, err = ext.Exec(`
CREATE TABLE event_interest(
    userid text not null,
    discussionid text not null,
    interest integer not null,
    foreign key(userid) references event_users(userid),
    foreign key(discussionid) references event_discussions(discussionid),
    unique(userid, discussionid))`)
	if err != nil {
		return errOrRetry("Creating table event_interest", err)
	}

	_, err = ext.Exec(`
CREATE TABLE event_locations(
    locationid   integer primary key,
    locationname text not null,
    locationdescription text not null,
    isplace      boolean not null,
    capacity     integer not null)`)
	if err != nil {
		return errOrRetry("Creating table event_locations", err)
	}

	_, err = ext.Exec(`
CREATE TABLE event_days(
    dayid integer primary key,
    dayname  text not null)`)
	if err != nil {
		return errOrRetry("Creating table event_days", err)
	}

	_, err = ext.Exec(`
CREATE TABLE event_slots(
    slotid   integer primary key,
    dayid    integer not null,
    slottime string not null, /* Output of time.MarshalText() */
    isbreak  boolean not null,
    islocked boolean not null,
    foreign  key(dayid) references event_days(dayid),
    unique(dayid, slotid))`)
	if err != nil {
		return errOrRetry("Creating table event_slots", err)
	}

	_, err = ext.Exec(`
CREATE TABLE event_schedule(
    discussionid text not null,
    slotid       text not null,
    locationid   integer not null,
    foreign key(discussionid) references event_discussions(discussionid),
    foreign key(slotid) references event_slots(slotid),
    foreign key(locationid) references event_slots(locationid),
    unique(slotid, locationid))`)
	if err != nil {
		return errOrRetry("Creating table event_schedule", err)
	}

	return nil
}
