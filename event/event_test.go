package event

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestVersion(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "event")
	if err != nil {
		t.Errorf("Creating temporary directory: %v", err)
		return
	}

	sfname := tmpdir + "/event.sqlite3"
	t.Logf("Temporary session store filename: %v", sfname)

	// Test simple open / close
	db, err := openDb(sfname)
	if err != nil {
		t.Errorf("Opening database: %v", err)
		return
	}

	db.Close()

	db, err = openDb(sfname)
	if err != nil {
		t.Errorf("Opening database a second time: %v", err)
		return
	}

	// Manually break the schema version
	_, err = db.Exec(fmt.Sprintf("pragma user_version=%d", codeSchemaVersion+1))
	if err != nil {
		t.Errorf("Messing up user version: %v", err)
		return
	}

	db.Close()

	db, err = openDb(sfname)
	if err == nil {
		t.Errorf("Opening database with wrong version succeeded!")
		return
	}

	os.RemoveAll(tmpdir)
}

type testContext struct {
	tmpdir  string
	dbfname string
}

func dataInit(t *testing.T) *testContext {
	tc := &testContext{}
	var err error

	tc.tmpdir, err = ioutil.TempDir("", "event")
	if err != nil {
		t.Errorf("Creating temporary directory: %v", err)
		return nil
	}

	tc.dbfname = tc.tmpdir + "/event.sqlite3"
	t.Logf("Temporary session store filenames: %s", tc.dbfname)

	// Remove the file first, just in case
	os.Remove(tc.dbfname)

	// Test simple open / close
	err = Load(EventOptions{dbFilename: tc.dbfname})
	if err != nil {
		t.Errorf("Opening stores: %v", err)
		return nil
	}

	return tc
}

func (tc testContext) cleanup() {
	os.RemoveAll(tc.tmpdir)
	Close()
}

func TestEvent(t *testing.T) {
	t.Logf("testUnitUser")
	if testUnitUser(t) {
		return
	}

	if testUnitDiscussion(t) {
		return
	}

	if testUnitInterest(t) {
		return
	}

	if testTransaction(t) {
		return
	}
}
