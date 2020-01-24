package sessions

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type SessionStore interface {
	Find(string) (*Session, error)
	Save(*Session) error
	Delete(*Session) error
	Close()
}

var store SessionStore

type SQLiteSessionStore struct {
	*sqlx.DB
}

type schemaVersion int

const (
	dbSchemaVersion = schemaVersion(1)
)

const (
	attributeSchemaVersion     = "SchemaVersion"
	attributeSessionCookieName = "SessionCookieName"
	attributeDefaultExpiry     = "DefaultExpiry"
)

func getSchemaVersion(tx *sqlx.Tx) (schemaVersion, error) {
	var tableCount int

	// Does the 'attributes' table exist?
	err := tx.Get(&tableCount,
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='attributes';")
	if err != nil {
		return -1, err
	}

	// No tables named 'attributes'; return 0 to indicate no schema
	if tableCount == 0 {
		return 0, nil
	}

	var fileSchemaVersionString string

	// The current schema version
	err = tx.Get(&fileSchemaVersionString,
		"select value from attributes where key=?", attributeSchemaVersion)
	if err != nil {
		return -1, err
	}

	fileSchemaVersion, err := strconv.Atoi(fileSchemaVersionString)

	return schemaVersion(fileSchemaVersion), err
}

func newSQLiteSessionStore(name string) (SessionStore, error) {
	store := &SQLiteSessionStore{}

	var err error

	store.DB, err = sqlx.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared", name))
	if err != nil {
		return nil, err
	}

	// Check for existence of tables
	tx, err := store.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	fileSchemaVersion, err := getSchemaVersion(tx)
	if err != nil {
		return nil, err
	}

	// No 'attributes' table?  Assume an empty database
	if fileSchemaVersion == 0 {
		_, err = tx.Exec(`
        create table attributes(
            key   text primay key,
            value text not null)`)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(`
        create table sessions(
            id       text primary key,
            userid   string not null,
            expiryts integer not null /* in Unix time */)`)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(`
        insert into attributes(key, value) values(?, ?)`, attributeSchemaVersion, dbSchemaVersion)
		if err != nil {
			return nil, err
		}
	} else if fileSchemaVersion != dbSchemaVersion {
		return nil, fmt.Errorf("Database version mismatch: %d != %d", fileSchemaVersion, dbSchemaVersion)
	}

	tx.Commit()

	return store, nil
}

func (store *SQLiteSessionStore) Save(session *Session) error {
	_, err := store.Exec(
		`insert into sessions(id, userid, expiryts) values(?, ?, ?)`,
		session.ID, session.UserID, session.Expiry.Unix())
	return err
}

func (store *SQLiteSessionStore) Find(id string) (*Session, error) {
	var val struct {
		UserID   string
		ExpiryTS int64
	}
	err := sqlx.Get(store, &val,
		`select userid, expiryts from sessions where id = ?`,
		id)
	if err == nil {
		return &Session{
			ID:     SessionID(id),
			UserID: val.UserID,
			Expiry: time.Unix(val.ExpiryTS, 0)}, nil
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return nil, err
}

func (store *SQLiteSessionStore) Delete(session *Session) error {
	_, err := store.Exec(
		`delete from sessions where id = ?`,
		session.ID)
	return err
}

func (store *SQLiteSessionStore) Close() {
	store.DB.Close()
}

func OpenSessionStore(name string) error {
	var err error
	store, err = newSQLiteSessionStore(name)
	return err
}

func CloseSessionStore() {
	store.Close()
	store = nil
}
