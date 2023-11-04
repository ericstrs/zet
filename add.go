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

// AddCmd parses and validates user arguments for the add command.
// If arguments are valid, it calls the desired operation.
// TODO: add `help` sub-command. Check args and print usage.
func AddCmd(args []string) error {
	var title, body, stdin string
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}

	// Assign title and body based on positional arguments
	if len(args) > 2 {
		title = args[2]
	}
	if len(args) > 3 {
		body = args[3]
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("Failed to get info on stdin: %v", err)
	}
	// If the Stdin is from a pipe
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)

		// Read stdin content, if available
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
// All the above scenarios can be used along with stdin. In which, the
// content from stdin is always appended after any argument data.
//
// Auto-linking is enabled by default. If user is in a zettel, then
// adding will result in appending the current zettels link in the newly
// created zettel. This does not create a link for the current zettel.
// After the new zettel has a title and been saved, its link can be
// retrieved by doing `zet link last` from the current zettel.
//
// TODO:
// * smart modification detection where a git commit is made if file was
// modified.
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

	// If there was no title or body arguments, open newly created zettel.
	if title == "" && body == "" {
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
