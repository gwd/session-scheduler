package keyvalue

import (
	"flag"
	"os"
	"testing"
)

func TestFlags(t *testing.T) {
	fname := os.TempDir() + "/flagvalue.db"
	t.Logf("Temporary keyvalue db filename: %v", fname)

	// Remove the file first, just in case
	os.Remove(fname)

	kvs, err := OpenFile(fname)
	if err != nil {
		t.Errorf("Opening temporary file: %v", err)
		return
	}

	flagSet := flag.NewFlagSet("", flag.ContinueOnError)

	flagSet.Var(kvs.GetFlagValue("AdminPassword"),
		"admin-password", "Set admin password")
	flagSet.Var(kvs.GetFlagValue("ServeAddress"),
		"address", "Address to serve http from")
	flagSet.Var(kvs.GetFlagValue("SearchTime"),
		"searchtime", "Duration to run search")

	t.Run("ValidFlags", func(t *testing.T) {
		expected := map[string]string{
			"ServeAddress": "localhost:8080",
			"SearchTime":   "60",
		}
		unexpected := []string{"AdminPassword"}
		err = flagSet.Parse([]string{"-address", "localhost:8080", "-searchtime", "60"})
		if err != nil {
			t.Errorf("Parsing argument list: %v", err)
			return
		}

		if VerifyExpected(t, expected, kvs) {
			return
		}

		if VerifyUnexpected(t, unexpected, kvs) {
			return
		}
	})
	t.Run("Help", func(t *testing.T) {
		err = flagSet.Parse([]string{"-help"})
		if err != flag.ErrHelp {
			t.Errorf("Parsing argument list: wanted ErrHelp, got %v", err)
			return
		}
	})

	// FIXME: Test String() and Get() methods of FlagValue

	// Only remove the file if we were successful
	os.Remove(fname)

}
