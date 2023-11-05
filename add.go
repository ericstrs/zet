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
var addUsage = `add adds a new zettel with the given title and content

Usage:

	zet add                - Creates a new zettel and opens it for editing.
	zet add [title]        - Creates a new zettel with provided title.
	zet add [title] [body] - Creates a new zettel with provided title and body.

All the above scenarios accept standard input. In which, content from
Stdin is always appended after any argument data. Providing non-empty
Stdin alongside ` + "`zet add`" + ` disables the interactive feature.

Auto-linking is enabled by default. If calling the add command from
an existing zettel directory, the newly created zettel will have link
to existing zettel.
`

// AddCmd parses and validates user arguments for the add command.
// If arguments are valid, it calls the desired operation.
func AddCmd(args []string) error {
	n := len(args)
	if n < 3 {
		fmt.Println(addUsage)
		return nil
	}
	var title, body, stdin string
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}

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

	if err := Add(c.ZetDir, c.Editor, title, body, stdin); err != nil {
		return err
	}

	return nil
}

// Add adds a zettel (note). This consists of creating a new directory
// with a unique identifier and then creating a new file. Zettels are
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
// Auto-linking is enabled by default. If calling the add command from
// an existing zettel directory, the newly created zettel will have link
// to existing zettel.
func Add(path, editor, title, body, stdin string) error {
	// Create new directory using the current isosec
	is := Isosec()
	zpath := filepath.Join(path, is)
	err := dir(zpath)
	if err != nil {
		return fmt.Errorf("Failed create new zettel directory: %v", err)
	}

	zfpath := filepath.Join(zpath, "README.md")

	// Create new zettel
	f, err := file(zfpath)
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

	autoLinkEnabled := true
	if autoLinkEnabled {
		// If current link cannot be found, skip auto-linking
		currLink, err := meta.CurrLink()
		if err == nil {
			fullText += "\n\nSee:\n\n" + currLink
		}
	}

	fullText += "\n"

	// Write the zettel content
	writer := bufio.NewWriter(f)
	_, err = writer.WriteString(fullText)
	if err != nil {
		return fmt.Errorf("Failed full text write to new zettel %s: %v", zfpath, err)
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("Failed write buffered data to new zettel %s: %v", zfpath, err)
	}

	// If no title and no body and no stdin, then open newly created zettel.
	if title == "" && body == "" && stdin == "" {
		if err := openFile(editor, zfpath); err != nil {
			return fmt.Errorf("Failed to open new zettel: %v", err)
		}
	}

	return nil
}

// Isosec returns the ISO date to the millisecond.
// TODO: add info on `help` arg. Variadic func may be useful here.
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

// openFile opens a file.
func openFile(editorPath, filePath string) error {
	cmd := exec.Command(editorPath, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
