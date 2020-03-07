package event

import (
	_ "fmt"
	"os"
	"testing"
)

func TestVersion(t *testing.T) {
	sfname := os.TempDir() + "/event.sqlite3"
	t.Logf("Temporary session store filename: %v", sfname)

	// Remove the file first, just in case
	os.Remove(sfname)

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
	// _, err = db.Exec(fmt.Sprintf("pragma user_version=%d", codeSchemaVersion+1))
	// if err != nil {
	// 	t.Errorf("Messing up user version: %v", err)
	// 	return
	// }

	db.Close()

	// db, err = openDb(sfname)
	// if err == nil {
	// 	t.Errorf("Opening database with wrong version succeeded!")
	// 	return
	// }

	os.Remove(sfname)
}
