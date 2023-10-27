package zet

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var Perm = 0700

// Add adds a zettels to the zet system. A zettel is made by adding a
// new directory with a unique identifier and then creating a markdown
// file.
//
// The path to the zet system is used instead of changing the zet
// directory to work better with scripts.
func Add(path string) error {
	// Create new directory using the current isosec
	is := isosec()
	zpath := filepath.Join(path, is)
	err := dir(zpath)
	if err != nil {
		return err
	}

	zpath = filepath.Join(zpath, "README.md")

	// Create new zettel
	_, err = file(zpath)
	if err != nil {
		return err
	}

	// Check current path:
	// If current path is in a zettel, then create doubly link.
	// Otherwise, don't create any links.

	// If not bulk operation, Open zettel using $EDITOR.

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
func dir(d string) error {
	return os.Mkdir(d, fs.FileMode(Perm))
}

// File creates and returns the file for a zettel. An error is
// returned if the file is failed to be made.
func file(s string) (*os.File, error) {
	// Create a new file or append if file exists
	file, err := os.OpenFile(s, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file, nil
}
