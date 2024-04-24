/*
Zet is a command-line utility for managing a Zettelkasten.
It employs a command-line interface (CLI) for core and batch features and a text-based user interface (TUI) for searching and visualization functionalities.

Usage:

	zet [command] [arguments]

Commands:

	add     - Adds a new zettel with the given title and content.
	search  - Searches for zettels given a query string.
	split   - Splits up a given zettel into sub-zettels.
	content - Prints different sections of zettel content.
	merge   - Merges linked notes to form a single note.
	list    - Lists all existing zettels.
	link    - Prints the link of a zettel.
	isosec  - Prints the current ISO date to the millisecond.
	commit  - Performs a git commit using zettel's title.
	config  - Displays configuration directory path.

Appending "help" after any command will print command info.
*/
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ericstrs/zet/internal/ui"
)

const usage = `USAGE

	zet [command] [arguments]

COMMANDS

	add, a  - Adds a new zettel with the given title and content.
	search  - Searches for zettels given a query string.
	split   - Splits up a given zettel into sub-zettels.
	content - Prints different sections of zettel content.
	merge   - Merges linked notes to form a single note.
	list    - Lists all existing zettels.
	link, l - Prints the link of a zettel.
	isosec  - Prints the current ISO date to the millisecond.
	commit  - Performs a git commit using zettel's title.
	config  - Displays configuration directory path.

DESCRIPTION

	Zet is a command-line utility for managing a Zettelkasten.
  It employs a command-line interface (CLI) for core and batch features
  and a text-based user interface (TUI) for searching and visualization
	functionalities.

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
		args = append(args, `search`, `browse`)
		if err := ui.SearchCmd(args); err != nil {
			return err
		}
		return nil
	}

	switch strings.ToLower(os.Args[1]) {
	case `add`, `a`: // add a new zettel
		if err := ui.AddCmd(args); err != nil {
			return fmt.Errorf("Failed to add a zettel: %v", err)
		}
	case `link`, `l`: // get zettel link
		if err := ui.LinkCmd(args); err != nil {
			return fmt.Errorf("Failed to retrieve zettel link: %v", err)
		}
	case `commit`:
		if err := ui.CommitCmd(args); err != nil {
			return err
		}
	case `list`, `ls`:
		if err := ui.ListCmd(args); err != nil {
			return err
		}
	case `search`:
		if err := ui.SearchCmd(args); err != nil {
			return err
		}
	case `split`:
		if err := ui.SplitCmd(args); err != nil {
			return fmt.Errorf("Error splitting zettel: %v", err)
		}
	case `merge`:
		if err := ui.MergeCmd(args); err != nil {
			return fmt.Errorf("Error merging zettels: %v", err)
		}
	case `content`:
		if err := ui.ContentCmd(args); err != nil {
			return fmt.Errorf("Error getting content from zettel: %v", err)
		}
	case `isosec`:
		ui.IsosecCmd(args)
	case `config`:
		if err := ui.ConfigCmd(args); err != nil {
			return fmt.Errorf("Error getting config: %v", err)
		}
	case `help`:
		fmt.Printf(usage)
	default:
		return fmt.Errorf("Invalid argument.\n%s", usage)
	}

	return nil
}
