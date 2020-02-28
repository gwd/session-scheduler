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

	err = os.Remove(sfname)
	if err != nil {
		t.Errorf("Removing temporary file: %v", err)
		return
	}

	// Test parallel open
	errChan := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			db, err := openDb(sfname)
			if err == nil {
				db.Close()
			}
			errChan <- err
		}()
	}

	for i := 0; i < 10; i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("Opening database: %v", err)
			return
		}
	}

	os.Remove(sfname)
}
