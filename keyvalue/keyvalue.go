package keyvalue

import (
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

type KeyValueStore struct {
	*sqlx.DB
	closeFileOnClose bool
}

func newKVStore(db *sqlx.DB) (*KeyValueStore, error) {
	kv := &KeyValueStore{DB: db}

	for {
		// Check for existence of tables
		tx, err := kv.Beginx()
		if err != nil {
			return nil, err
		}
		// Rollback on all paths where a commit hasn't taken place
		defer tx.Rollback()

		// Does the 'attributes' table exist?
		var tableCount int
		err = tx.Get(&tableCount,
			"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='keyvalue_keyvalues';")
		if err != nil {
			return nil, fmt.Errorf("Checking for existence of tables: %v", err)
		}
		if tableCount > 1 {
			return nil, fmt.Errorf("Unexpected number of tables: %d", tableCount)
		}

		if tableCount == 0 {
			_, err = tx.Exec(`
            create table keyvalue_keyvalues(
                key text primary key,
                value text not null)`)
			if err != nil {
				return nil, fmt.Errorf("Creating tables: %v", err)
			}

			err = tx.Commit()
			if err == sqlite3.ErrBusy {
				// Racing with someone else; try the transaction again
				continue
			} else if err != nil {
				return nil, err
			}
		}

		return kv, nil
	}
}

// OpenDB accepts *sql.DB and driver name to an open
// github.com/mattn/go-sqlite3 database.  It creates the appropriate
// table if not present, and returns a KeyValueStore.  Tables are
// created with the `keyvalue_` prefix; no tables starting with
// `keyvalue_` should be in this database if using this mode.
func OpenDB(db *sql.DB, driverName string) (*KeyValueStore, error) {
	return newKVStore(sqlx.NewDb(db, driverName))
}

// OpenFile takes a filename.  It will open the file as a SQLite
// database, creating it if it doesn't exist.
func OpenFile(name string) (*KeyValueStore, error) {
	db, err := sqlx.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared", name))
	if err != nil {
		return nil, err
	}
	kv, err := newKVStore(db)
	if err != nil {
		kv.closeFileOnClose = true
	}
	return kv, err
}

// Close cleans up the database.  If KeyValueStore was created by
// OpenFile, the backing file is closed; otherwise, the references to
// the underlying database are just dropped.
func (store *KeyValueStore) Close() {
	if store.closeFileOnClose {
		store.DB.Close()
	}
	store.DB = nil
}

func get(q sqlx.Queryer, key string, value *string) error {
	return sqlx.Get(q, value, `select value from keyvalue_keyvalues where key=?`,
		key)
}

func set(e sqlx.Execer, key, value string) error {
	_, err := e.Exec(`
    insert into keyvalue_keyvalues(key, value) values(?, ?)
        on conflict(key) do
            update set value=excluded.value`, key, value)
	return err
}

// Set "key" to "value" in the store.
func (kv KeyValueStore) Set(key, value string) error {
	return set(kv.DB, key, value)
}

var ErrNoRows = sql.ErrNoRows

// Get looks the value of "key" in the store.  If found, it returns
// the value; if not found, it will return ErrNoRows.
func (kv KeyValueStore) Get(key string) (string, error) {
	var value string
	err := get(kv.DB, key, &value)
	return value, err
}

// Sets "key" to "value", returning the old value, or an empty string
// if there is no previous value.
func (kv KeyValueStore) Exchange(key, value string) (string, error) {
	tx, err := kv.Beginx()
	if err != nil {
		return "", fmt.Errorf("Starting transaction: %v", err)
	}
	defer tx.Rollback()

	var oldval string
	err = get(tx, key, &oldval)
	if err != nil && err != ErrNoRows {
		return "", fmt.Errorf("Getting old value for key %s: %v", key, err)
	}

	err = set(tx, key, value)
	if err != nil {
		return "", fmt.Errorf("Setting key %s: %v", key, err)
	}
	tx.Commit()

	return oldval, nil
}

// Delete a key from the store.  Future Get() calls will return
// ErrNoRows.
func (kv KeyValueStore) Unset(key string) error {
	_, err := kv.Exec(`delete from keyvalue_keyvalues where key=?`, key)
	return err
}

func boolToString(val bool) string {
	if val {
		return "true"
	} else {
		return "false"
	}
}

func stringToBool(val string) bool {
	if val == "true" {
		return true
	}
	return false
}

// GetBoolDef looks for the value of "key" in the store.  If found it
// returns the value; on any error it returns "false".
func (kv KeyValueStore) GetBoolDef(key string) bool {
	val, _ := kv.Get(key)
	return stringToBool(val)
}

// SetBool sets "key" to a stringified boolean of "val".
func (kv KeyValueStore) SetBool(key string, val bool) error {
	return kv.Set(key, boolToString(val))
}

func (kv KeyValueStore) ExchangeBool(key string, val bool) (bool, error) {
	ovs, err := kv.Exchange(key, boolToString(val))
	return stringToBool(ovs), err
}

// FlagValue implements the flag.Value interface.  See
// KeyValueStore.GetFlagValue for usage.
type FlagValue struct {
	key       string
	kvs       *KeyValueStore
	validator func(string) error
}

// Set the key associated with fv to "value".  If a validator was
// passed, it will be called; if the validaotr returns an error, Set
// will return an error.
func (fv FlagValue) Set(value string) error {
	if fv.validator != nil {
		err := fv.validator(value)
		if err != nil {
			return err
		}
	}
	return fv.kvs.Set(fv.key, value)
}

// Get the key associated with fv.  If the key was not set, return "".
func (fv FlagValue) String() string {
	if fv.kvs == nil {
		return ""
	}
	value, err := fv.kvs.Get(fv.key)
	if err != nil {
		return ""
	}
	return value
}

// Get the key associated with fv, as a string.  If the key was not
// set, return "".
func (fv FlagValue) Get() interface{} {
	return fv.String()
}

// GetFlagValue returns a structure of type FlagValue, which can be
// passed in to flag.Var*() functions.  When the flag is set, the key
// will be set to the value of the flag.
//
// For example:
//
//     flag.Var(kv.GetFlagValue("ServerAddress"), "address", "Address to serve http from")
//
// For now, keyvalue databases must be opened before flags are read,
// with a fixed filename.  Allowing the filename to be one of the
// flags is a future feature.
func (kvs *KeyValueStore) GetFlagValue(key string, validators ...func(string) error) FlagValue {
	fv := FlagValue{key: key, kvs: kvs}
	if validators != nil {
		fv.validator = validators[0]
	}
	return fv
}
