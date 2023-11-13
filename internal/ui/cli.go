package ui

import (
	"fmt"
	"strings"

	"github.com/iuiq/zet/internal/config"
	"github.com/iuiq/zet/internal/storage"
)

var searchUsage = `NAME

  search - searches for zettels.

USAGE

  zet search [term]          - Print zettels given a simple search term.
  zet search browse|b        - Interactively search for a zettel.
  zet search history [clear] - Print search history or optionally delete it.
  zet search help            - Print zettels given a search term.`

func SearchCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	s, err := storage.UpdateDB(c.ZetDir)
	if err != nil {
		return fmt.Errorf("Error syncing database and flat files: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // print results to stdout
	case 3:
		switch strings.ToLower(args[2]) {
		case `browse`, `b`:
			if err := NewSearchUI(s, c.ZetDir, c.Editor).Run(); err != nil {
				return fmt.Errorf("Error running search ui: %v", err)
			}
		}
	}

	return nil
}
