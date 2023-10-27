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
	if len(os.Args) == 1 {
		return fmt.Errorf("Not enough arguments.\n%s", usage)
	}

	switch strings.ToLower(os.Args[1]) {
	case "add": // add a new zettel
		c := new(config.C)
		if err := c.Init(); err != nil {
			return err
		}
		if err := z.Add(c.ZetDir); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid argument.\n%s", usage)
	}

	return nil
}
