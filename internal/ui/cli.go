package ui

import (
	"fmt"
	"strings"

	"github.com/iuiq/zet/internal/config"
	"github.com/iuiq/zet/internal/storage"
)

const (
	yellow = "\033[33m" // ANSI escape code for yellow
	red    = "\033[31m" // ANSI escape code for red
	reset  = "\033[0m"  // ANSI escape code to reset to default color
)

var searchUsage = `NAME

  search - searches for zettels.

USAGE

  zet search query|q [term]  - Print zettels given a search term.
  zet search browse|b [term] - Interactively search for a zettel.
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

	if n >= 3 {
		query := strings.Join(args[3:], " ")
		switch strings.ToLower(args[2]) {
		case `query`, `q`:
			if query == "" {
				return nil
			}
			zettels, err := s.SearchZettels(query, red, reset)
			if err != nil {
				zettels = []storage.ResultZettel{storage.ResultZettel{TitleSnippet: "Incorrect syntax"}}
			}
			for _, z := range zettels {
				fmt.Println(yellow + z.DirName + reset + " " + z.TitleSnippet)
				if z.BodySnippet != "" {
					fmt.Println(removeEmptyLines(z.BodySnippet))
				}
				if z.TagsSnippet != "" {
					hashedTags := "\t\t#" + strings.ReplaceAll(z.TagsSnippet, " ", " #")
					fmt.Println(hashedTags)
				}
			}
		case `browse`, `b`:
			if err := NewSearchUI(s, query, c.ZetDir, c.Editor).Run(); err != nil {
				return fmt.Errorf("Error running search ui: %v", err)
			}
		}
	}

	return nil
}

func removeEmptyLines(str string) string {
	lines := strings.Split(str, "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}
	return strings.Join(nonEmptyLines, "\n")
}
