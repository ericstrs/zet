/*
Zet is a command-line utility for managing a Zettelkasten.
It employs a command-line interface (CLI) for core and batch features and a text-based user interface (TUI) for searching and visualization functionalities.

Usage:

	zet [command] [arguments]

Commands:

	add    - Adds a new zettel with the given title and content.
	remove - Removes a zettel by its ID.
	search - Searches for zettels given a query string.
	merge  - Merges linked notes to form a single note.
	list   - Lists all existing zettels.
*/
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	z "github.com/iuiq/zet"
	"github.com/iuiq/zet/internal/config"
)

const usage = `Usage:

  zet [command] [arguments]

Commands:

  add    - Adds a new zettel with the given title and content.
  remove - Removes a zettel by its ID.
  search - Searches for zettels given a query string.
  merge  - Merges linked notes to form a single note.
  list   - Lists all existing zettels.`

func main() {
	if err := Run(); err != nil {
		log.Println(err)
	}
}

func Run() error {
	args := os.Args
	if len(args) == 1 {
		return fmt.Errorf("Not enough arguments.\n%s", usage)
	}

	switch strings.ToLower(os.Args[1]) {
	case "add": // add a new zettel
		if err := addCmd(args); err != nil {
			return fmt.Errorf("Failed to add a zettel: %v", err)
		}
	default:
		return fmt.Errorf("Invalid argument.\n%s", usage)
	}

	return nil
}

func addCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}

	var title, body, stdin string

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

	if err := z.Add(c.ZetDir, c.Editor, title, body, stdin); err != nil {
		return err
	}

	return nil
}
