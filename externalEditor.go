package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

var editcmd string

func ExternalEditorFile(filename string) error {
	if editcmd == "" {
		var prs bool
		editcmd, prs = os.LookupEnv("EDITOR")
		if !prs {
			return fmt.Errorf("Please set EDITOR environment variable")
		}
	}

	cmd := exec.Command(editcmd, filename)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("Running editor (%s): %v", editcmd, err)
		return err
	}

	return nil
}

func ExternalEditorBytes(out []byte) ([]byte, error) {
	// Open temporary file
	tmpfile, err := ioutil.TempFile("", "edit*.txt")
	if err != nil {
		return nil, fmt.Errorf("Creating temp file for segmentation: %v", err)
	}

	// Write the output to the file
	_, err = tmpfile.Write(out)
	if err != nil {
		return nil, fmt.Errorf("Writing text to file: %v", err)
	}

	// Close the file before editing...
	tmpfileName := tmpfile.Name()
	tmpfile.Close()

	// Run the external editor.  This will do its own errors if there's a failure.
	err = ExternalEditorFile(tmpfileName)
	if err != nil {
		return nil, err
	}

	// Read the file back in
	in, err := ioutil.ReadFile(tmpfileName)
	if err != nil {
		return nil, fmt.Errorf("Reading back tempfile after edit: %v", err)
	}

	return in, nil
}
