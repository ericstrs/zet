package zet

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/iuiq/zet/internal/config"
	"github.com/iuiq/zet/internal/meta"
)

var Perm = 0700
var errNotInZettel = errors.New("not in a zettel")
var isoUsage = `NAME

	isosec - prints the current ISO date to the millisecond.

USAGE

	zet isosec      - Print current ISO date to the millisecond.
	zet isosec help - Provides command information.`
var addUsage = `NAME

	add - adds a new zettel with the given title and content.

USAGE

	zet add|a                - Adds new zettel and opens for editing.
	zet add|a [title]        - Adds new zettel with provided title.
	zet add|a [title] [body] - Adds new zettel with provided title and body.
	zet add|a help           - Provides command information.

DESCRIPTION

	All the above scenarios accept standard input. In which, content from
	Stdin is always appended after any argument data. Providing non-empty
	Stdin alongside ` + "`zet add`" + ` disables the interactive feature.

	Auto-linking is enabled by default. If calling the add command from
	an existing zettel directory, the newly created zettel will have link
	to existing zettel.`

// AddCmd parses and validates user arguments for the add command.
// If arguments are valid, it calls the desired operation.
func AddCmd(args []string) error {
	var title, body, stdin string
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	// Assign title and body based on positional arguments
	if n > 2 {
		if strings.ToLower(args[2]) == `help` {
			fmt.Println(addUsage)
			return nil
		}
		title = args[2]
	}
	if n > 3 {
		body = args[3]
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("Failed to get info on stdin: %v", err)
	}
	// If the Stdin is from a pipe
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// Read stdin content, if available
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			stdin += line + "\n"
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("Error reading from stdin: %v", err)
		}
		// Remove the last newline character from stdin
		stdin = strings.TrimSuffix(stdin, "\n")
	}

	// If current link cannot be found, skip auto-linking
	currLink, err := meta.CurrLink(c.ZetDir)
	if err != nil {
		currLink = ""
	}

	var openZettel bool
	// If no title and no body and no stdin, then open newly created zettel.
	if title == "" && body == "" && stdin == "" {
		openZettel = true
	}

	// Otherwise, just create the zettel without opening it.
	if err := Add(c.ZetDir, c.Editor, title, body, stdin, currLink, openZettel); err != nil {
		return err
	}

	return nil
}

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

	fullText := "# " + title
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

// IsosecCmd parses and validates user arguments for the isosec command.
// If arguments are valid, it calls the desired operation.
func IsosecCmd(args []string) {
	var iso string
	n := len(args)
	switch n {
	case 2:
		iso = Isosec()
	case 3:
		if strings.ToLower(args[2]) == `help` {
			fmt.Println(isoUsage)
			return
		}
	}
	fmt.Println(iso)
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
