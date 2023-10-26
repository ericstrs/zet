package zet

import (
	"os"
	"time"
)

func add() error {
	// Create new directory using the current isosec
	d, err := dir(isosec())
	_ = d
	if err != nil {
		return err
	}
	// Create new zettel

	// Check current path:
	// If current path is in a zettel, then create doubly link.
	// Otherwise, don't create any links.

	// Open zettel using $EDITOR.
	return nil
}

// Isosec returns the ISO date to the millisecond.
func isosec() string {
	// Get the current time in UTC
	t := time.Now().UTC()

	// Format the current time into a string
	return t.Format("20060102150405")
}

// Dir creates a new directory to house a new zettel. An error is
// returned if the directory is failed to be made.
func dir(s string) (*os.File, error) {
	// Create a new file or append if file exists
	file, err := os.OpenFile(s, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file, nil
}
