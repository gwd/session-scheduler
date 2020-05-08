package event

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func initDbV1(ext sqlx.Ext) error {
	_, err := ext.Exec(fmt.Sprintf("pragma user_version=%d", 1))
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

func openDbV1(filename string) (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&_foreign_keys=on", filename))
	if err != nil {
		return nil, err
	}

	// Check for existence of tables
	tx, err := db.Beginx()
	if err != nil {
		return nil, err
	}

	err = initDbV1(tx)
	if err != nil {
		return nil, fmt.Errorf("Initializing v1 database: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("Committing database init/upgrade: %v", err)
	}

	return db, nil
}

func TestUpgradev1v2(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "event")
	if err != nil {
		t.Errorf("Creating temporary directory: %v", err)
		return
	}

	sfname := tmpdir + "/event.sqlite3"
	t.Logf("Temporary session store filename: %v", sfname)

	// Init v1 of the database, then close it
	db, err := openDbV1(sfname)
	if err != nil {
		t.Errorf("Opening database v1: %v", err)
		return
	}

	db.Close()

	// Test simple open / close
	db, err = openDb(sfname)
	if err != nil {
		t.Errorf("Opening database: %v", err)
		return
	}

	// TODO: Do some basic location operations

	db.Close()

	// Open it again, to make sure the version was upgraded properly
	db, err = openDb(sfname)
	if err != nil {
		t.Errorf("Opening database: %v", err)
		return
	}

	// TODO: Do some basic location operations

	db.Close()
}
