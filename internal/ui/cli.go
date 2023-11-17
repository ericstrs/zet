package ui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iuiq/zet"
	"github.com/iuiq/zet/internal/config"
	"github.com/iuiq/zet/internal/meta"
	"github.com/iuiq/zet/internal/storage"
)

const (
	yellow = "\033[33m" // ANSI escape code for yellow
	red    = "\033[31m" // ANSI escape code for red
	reset  = "\033[0m"  // ANSI escape code to reset to default color

	searchUsage = `NAME

  search - searches for zettels.

USAGE

  zet search query|q [term]  - Print zettels given a search term.
  zet search browse|b [term] - Interactively search for a zettel.
  zet search help            - Print zettels given a search term.
	`
	splitUsage = `NAME

	split  - Splits up a given zettel into sub-zettels.

USAGE

	zet split          - Splits zettel content from stdin into sub-zettels.
	zet split [isosec] - Splits zettel content from README.md in isosec directory into sub-zettels.
	`
	contentUsage = `NAME

	content - Prints different sections of zettel content.

USAGE

	zet content title - Prints title from README.md in current directory or in given directory.
	zet content body  - Prints body from README.md in current directory or in given directory.
	zet content links - Prints links from README.md in current directory or in given directory.
	zet content tags  - Prints tags from README.md in current directory or in given directory.
	`
)

func SearchCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	s, err := storage.UpdateDB(c.ZetDir)
	if err != nil {
		return fmt.Errorf("Error syncing database and flat files: %v", err)
	}
	defer s.Close()
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

func SplitCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // no args
		stdin, err := getStdin()
		if err != nil {
			return fmt.Errorf("Error getting standard input: %v", err)
		}
		if stdin == "" {
			return nil
		}

		p, ok, err := meta.InZettel(c.ZetDir)
		if err != nil {
			return fmt.Errorf("Failed to check if user is in a zettel: %v", err)
		}
		if !ok {
			return errors.New("not in a zettel")
		}

		if err := zet.SplitZettel(c.ZetDir, p, stdin); err != nil {
			return fmt.Errorf("Error splitting zettel content: %v", err)
		}
	case 3: // one arg
		switch strings.ToLower(os.Args[2]) {
		case `help`:
			fmt.Printf(splitUsage)
		}
	}
	return nil
}

func getStdin() (string, error) {
	var stdin string
	fi, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("Failed to get info on stdin: %v", err)
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
			return "", fmt.Errorf("Error reading from stdin: %v", err)
		}
		// Remove the last newline character from stdin
		stdin = strings.TrimSuffix(stdin, "\n")
	}
	return stdin, nil
}

func ContentCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	if n < 3 {
		fmt.Fprintln(os.Stderr, "Error: Not enough arguments.")
		fmt.Fprintf(os.Stderr, contentUsage)
		os.Exit(1)
	}

	switch strings.ToLower(args[2]) {
	case `title`:
		if err := titleCmd(args[2:], c.ZetDir); err != nil {
			return err
		}
	case `body`:
		if err := bodyCmd(args[2:], c.ZetDir); err != nil {
			return err
		}
	case `links`:
	case `tags`:
	case `help`:
		fmt.Println(contentUsage)
	}

	return nil
}

func titleCmd(args []string, zetDir string) error {
	var t string
	n := len(args)
	switch n {
	case 1:
		p, ok, err := meta.InZettel(zetDir)
		if err != nil {
			return fmt.Errorf("Failed to check if user is in a zettel: %v", err)
		}
		if !ok {
			return errors.New("not in a zettel")
		}
		t, err = meta.Title(p)
		if err != nil {
			return err
		}
	default:
		var err error
		p := filepath.Join(zetDir, args[1])
		t, err = meta.Title(p)
		if err != nil {
			return err
		}
	}
	fmt.Println(t)
	return nil
}

func bodyCmd(args []string, zetDir string) error {
	var b string
	n := len(args)
	switch n {
	case 1:
		p, ok, err := meta.InZettel(zetDir)
		if err != nil {
			return fmt.Errorf("Error checking if user is in a zettel directory: %v", err)
		}
		if !ok {
			return errors.New("not in a zettel")
		}
		b, err = meta.Body(p)
		if err != nil {
			return err
		}
	default:
		var err error
		p := filepath.Join(zetDir, args[1])
		b, err = meta.Body(p)
		if err != nil {
			return err
		}
	}
	fmt.Println(b)
	return nil
}
