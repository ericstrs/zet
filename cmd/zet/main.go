/*
Zet is a command-line utility for managing a Zettelkasten.
It employs a command-line interface (CLI) for core and batch features and a text-based user interface (TUI) for searching and visualization functionalities.

Usage:

	zet [command] [arguments]

Commands:

	add    - Adds a new zettel with the given title and content.
	search - Searches for zettels given a query string.
	merge  - Merges linked notes to form a single note.
	list   - Lists all existing zettels.
	title  - Prints the title of a zettel file.
	link   - Prints the link of a zettel.
	isosec - Prints the current ISO date to the millisecond.
	commit - Performs a git commit using zettel's title.

Appending "help" after any command will print command info.
*/
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	z "github.com/iuiq/zet"
	"github.com/iuiq/zet/internal/meta"
)

const usage = `Usage:

  zet [command] [arguments]

Commands:

  add    - Adds a new zettel with the given title and content.
  search - Searches for zettels given a query string.
  merge  - Merges linked notes to form a single note.
  list   - Lists all existing zettels.
  title  - Prints the title of a zettel file.
  link   - Prints the link of a zettel.
  isosec - Prints the current ISO date to the millisecond.
	commit - Performs a git commit using zettel's title.

Appending "help" after any command will print more command info.
`

func main() {
	if err := Run(); err != nil {
		log.Println(err)
	}
}

func Run() error {
	args := os.Args
	if len(args) == 1 {
		fmt.Fprintln(os.Stderr, "Error: Not enough arguments.")
		fmt.Fprintf(os.Stderr, usage)
		os.Exit(1)
	}

	switch strings.ToLower(os.Args[1]) {
	case `add`: // add a new zettel
		if err := z.AddCmd(args); err != nil {
			return fmt.Errorf("Failed to add a zettel: %v", err)
		}
	case `title`: // get zettel title
		if err := meta.TitleCmd(args); err != nil {
			return fmt.Errorf("Failed to retrieve zettel title: %v", err)
		}
	case `link`: // get zettel link
		if err := meta.LinkCmd(args); err != nil {
			return fmt.Errorf("Failed to retrieve zettel link: %v", err)
		}
	case `commit`:
		if err := z.CommitCmd(args); err != nil {
			return err
		}
	case `isosec`:
		z.IsosecCmd(args)
	default:
		return fmt.Errorf("Invalid argument.\n%s", usage)
	}

	return nil
}
