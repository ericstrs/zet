package zet

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ericstrs/zet/internal/meta"
)

var (
	Perm           = 0700
	errNotInZettel = errors.New("not in a zettel")
)

// CreateAdd creates a new directory with a unique identifier and then
// creates a new file.
func CreateAdd(path, editor, title, body, stdin, link string, open bool) error {
	// Create new directory using the current isosec
	is := Isosec()
	newDirPath := filepath.Join(path, is)
	err := dir(newDirPath)
	if err != nil {
		return fmt.Errorf("Error creating new zettel directory: %v", err)
	}
	err = Add(newDirPath, editor, title, body, stdin, link, open)
	if err != nil {
		return fmt.Errorf("Error adding zettel: %v", err)
	}
	return nil
}

// Add adds a zettel (note) to an exiting zettel directory. Zettels are
// markdown by default. The path to the zet system is used instead of
// changing the zet directory to support zettel creation from scripts.
//
// The behavior is dependent on the arguments and input methods provided:
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
// All the above scenarios accept standard input. In which, content from
// Stdin is always appended after any argument data. Providing non-empty
// Stdin alongside `zet add` disables the interactive feature.
//
// If link argument is not empty, it will be included in the newly
// created zettel.
func Add(newDirPath, editor, title, body, stdin, link string, open bool) error {
	zfpath := filepath.Join(newDirPath, "README.md")

	// Create new zettel
	f, err := file(zfpath)
	if err != nil {
		return fmt.Errorf("Failed create new zettel file: %v", err)
	}
	defer f.Close()

	fullText := "# " + title + "\n"
	if body != "" {
		fullText += body
	}
	if stdin != "" {
		fullText += stdin
	}
	if link != "" {
		fullText += "See:\n\n" + link
	}
	fullText += "\n"

	// Write the zettel content
	writer := bufio.NewWriter(f)
	if _, err = writer.WriteString(fullText); err != nil {
		return fmt.Errorf("Failed full text write to new zettel %s: %v", zfpath, err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("Failed write buffered data to new zettel %s: %v", zfpath, err)
	}

	if open {
		if err := runCmd(newDirPath, editor, zfpath); err != nil {
			return fmt.Errorf("Failed to open new zettel: %v", err)
		}
		return nil
	}

	newLink, err := meta.Link(newDirPath)
	if err != nil {
		return fmt.Errorf("Error getting newly added zettel's link: %v", err)
	}
	fmt.Println(newLink)

	return nil
}

// Isosec returns the ISO date to the millisecond.
func Isosec() string {
	t := time.Now().UTC()
	return t.Format("20060102150405")
}

// dir creates a new directory with a given path. An error is
// returned if the directory is failed to be made.
func dir(p string) error {
	return os.Mkdir(p, fs.FileMode(Perm))
}

// file creates and returns a file. An error is returned if the file
// is failed to be made.
func file(s string) (*os.File, error) {
	// Create a new file or append if file exists
	file, err := os.OpenFile(s, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// runCmd runs an external command given the path to directory command
// should be executed in, path to command, and command arguments.
func runCmd(execPath, cmdPath string, args ...string) error {
	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = execPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
