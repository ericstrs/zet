package zet

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var Perm = 0700

// Add adds a zettel (note). This consists of creating a new
// directory with a unique identifier and then creating a new
// file. Zettels are markdown by default. The behavior is dependent
// on the arguments and input methods provided:
//
//  1. `zet add` with no arguments:
//     * Creates a	new, empty zettel.
//     * Opens newly created zettel for editing.
//  2. `zet add "title"` with one argument:
//     * Creates a	new zettel with the provided title.
//     * Does not open the zettel for editing.
//  3. `zet add "title" "body"` with two arguments:
//     * Creates a new zettel with the provided title and body.
//     * Does not open the zettel for editing.
//
// All the above scenarios can be used along with stdin. In which, the
// content from stdin is always appended after any argument data.
//
// The path to the zet system is used instead of changing the zet
// directory to support zettel creation from scripts.
//
// TODO:
// * handle error conditions like file already exists, disk full.
// * should I implement auto-linking?
// * smart modification detection where a git commit is made if file was
// modified.
func Add(path, editor, title, body, stdin string) error {
	// Create new directory using the current isosec
	is := isosec()
	zpath := filepath.Join(path, is)
	err := dir(zpath)
	if err != nil {
		return fmt.Errorf("Failed create new zettel directory: %v", err)
	}

	zpath = filepath.Join(zpath, "README.md")

	// Create new zettel
	f, err := file(zpath)
	if err != nil {
		return fmt.Errorf("Failed create new zettel file: %v", err)
	}
	defer f.Close()

	fullText := "# " + title
	if body != "" {
		fullText += "\n\n" + body
	}
	if stdin != "" {
		fullText += "\n\n" + stdin
	}
	fullText += "\n"

	// Write the zettel content
	writer := bufio.NewWriter(f)
	_, err = writer.WriteString(fullText)
	if err != nil {
		return fmt.Errorf("Failed full text write to new zettel %s: %v", zpath, err)
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("Failed write buffered data to new zettel %s: %v", zpath, err)
	}

	// Check current path:
	// If current path is in a zettel, then create doubly link.
	// Otherwise, don't create any links.

	// If there was no title or body arguments, open newly created zettel.
	if title == "" && body == "" {
		if err := openFile(editor, zpath); err != nil {
			return fmt.Errorf("Failed to open new zettel: %v", err)
		}
	}

	return nil
}

// Isosec returns the ISO date to the millisecond.
func isosec() string {
	// Get the current time in UTC
	t := time.Now().UTC()

	// Format the current time into a string
	return t.Format("20060102150405")
}

// Dir creates a new directory with a given path. An error is
// returned if the directory is failed to be made.
func dir(p string) error {
	return os.Mkdir(p, fs.FileMode(Perm))
}

// File creates and returns a file. An error is returned if the file
// is failed to be made.
func file(s string) (*os.File, error) {
	// Create a new file or append if file exists
	file, err := os.OpenFile(s, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// OpenFile opens a file.
func openFile(editorPath, filePath string) error {
	cmd := exec.Command(editorPath, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
