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

  split - splits up a given zettel into sub-zettels.

USAGE

  zet split          - Splits zettel content from stdin into sub-zettels.
  zet split [isosec] - Splits zettel content from README.md in isosec directory into sub-zettels.`
	contentUsage = `NAME

  content - prints different sections of zettel content.

USAGE

  zet content title - Prints title from README.md in current directory or in given directory.
  zet content body  - Prints body from README.md in current directory or in given directory.
  zet content links - Prints links from README.md in current directory or in given directory.
  zet content tags  - Prints tags from README.md in current directory or in given directory.
`
	mergeUsage = `NAME

  merge - Merges the contents of split zettel's into single body of text.

USAGE

  zet merge [isosec] - Merges contents of split linked zettel's at given isosec directory or using stdin.

DESCRIPTION

  The non-linear nature of a Zettelkasten is one of its main strengths,
  but sometimes a linear representation is more suitable.

  Printing to standard output rather than writing to a file creates
  flexibility is how you can use this command. It allows the user to
  view the merged content in various ways (viewing, redirecting to a
  file, further processing with other tools).

  Example usage:

  Print merged content to pager:

  ` + "`" + `$ zet merge 20231118194243 | less` + "`" + `

  Piped merged content to file:

  ` + "`" + `$ zet merge 20231118194243 > output.md` + "`" + `

  The file output.md can then be used to retrieve the next level of
  sub-zettels:

  ` + "`" + `$zet merge < output.md > output.md` + "`" + `
`
	configUsage = `NAME

  config - displays configuration properties.

USAGE

  zet config     - prints configuration file.
  zet config dir - Prints path to configuration directory.
`
	listUsage = `NAME

  list - lists all the zettels.

USAGE

  zet list|ls          - Prints all zettels to stdout sorted by creation date.
  zet list|ls recent   - Prints all zettels sorted by modification time.
  zet list|ls length   - Prints all zettels sorted by word count.
  zet list|ls alpha    - Prints all zettels by alphabetically sorted titles.
  zet list|ls help     - Provides command information.

DESCRIPTION

  The list command serves as a tool viewing a collection of zettels. This
  command displays a list of all zettels stored in the system. Its main
  purpose is to output all zettels in an organized manner, in ascending
  order.
`
	linkUsage = `NAME
  link - prints zettel link

USAGE:

  zet link          - Prints zettel link for the current dir.
  zet link [isosec] - Prints zettel link for the given dir isosec.
  zet link help     - Provides command information.
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

	if n < 3 {
		fmt.Fprintln(os.Stderr, "Error: Not enough arguments.")
		fmt.Fprintf(os.Stderr, searchUsage)
		os.Exit(1)
	}

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
					hashedTags := "    #" + strings.ReplaceAll(z.TagsSnippet, " ", " #")
					fmt.Println(hashedTags)
				}
			}
		case `browse`, `b`:
			if err := NewSearchUI(s, query, c.ZetDir, c.Editor).Run(); err != nil {
				return fmt.Errorf("Error running search ui: %v", err)
			}
		default:
			fmt.Printf(searchUsage)
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
	case 2:
		stdin, err := getStdin()
		if err != nil {
			return fmt.Errorf("Error getting standard input: %v", err)
		}
		if stdin == "" {
			return nil
		}
		b := meta.ParseBody(stdin)

		p, ok, err := meta.InZettel(c.ZetDir)
		if err != nil {
			return fmt.Errorf("Failed to check if user is in a zettel: %v", err)
		}
		if !ok {
			return errors.New("not in a zettel")
		}

		if err := zet.SplitZettel(c.ZetDir, p, strings.Join(b, "\n")); err != nil {
			return fmt.Errorf("Error splitting zettel content: %v", err)
		}
	default:
		if strings.ToLower(os.Args[2]) == `help` {
			fmt.Printf(splitUsage)
			return nil
		}
		p := filepath.Join(c.ZetDir, args[2])

		b, err := meta.Body(p)
		if err != nil {
			return fmt.Errorf("Error parsing out zettel body: %v", err)
		}

		if err := zet.SplitZettel(c.ZetDir, p, b); err != nil {
			return fmt.Errorf("Error splitting zettel content: %v", err)
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
		if err := linksCmd(args[2:], c.ZetDir); err != nil {
			return err
		}
	case `tags`:
		if err := tagsCmd(args[2:], c.ZetDir); err != nil {
			return err
		}
	case `help`:
		fmt.Printf(contentUsage)
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
	if t != "" {
		fmt.Println(t)
	}
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
	if b != "" {
		fmt.Println(b)
	}
	return nil
}

func linksCmd(args []string, zetDir string) error {
	var l string
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
		l, err = meta.Links(p)
		if err != nil {
			return err
		}
	default:
		var err error
		p := filepath.Join(zetDir, args[1])
		l, err = meta.Links(p)
		if err != nil {
			return err
		}
	}
	if l != "" {
		fmt.Println(l)
	}
	return nil
}

func tagsCmd(args []string, zetDir string) error {
	var t string
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
		t, err = meta.Tags(p)
		if err != nil {
			return err
		}
	default:
		var err error
		p := filepath.Join(zetDir, args[1])
		t, err = meta.Tags(p)
		if err != nil {
			return err
		}
	}
	if t != "" {
		fmt.Println(t)
	}
	return nil
}

// MergeCmd merges the contents of split zettel's into single body of text.
//
// The non-linear nature of a Zettelkasten is one of its main strengths,
// but sometimes a linear representation is more suitable.
//
// Printing to standard output rather than writing to a file creates
// flexibility is how you can use this command. It allows the user to
// view the merged content in various ways (viewing, redirecting to a
// file, further processing with other tools).
//
// Example usage:
//
// Print merged content to pager:
//
// `$ zet merge 20231118194243 | less`
//
// Piped merged content to file:
//
// `$ zet merge 20231118194243 > output.md`
//
// The file output.md can then be used to retrieve the next level of
// sub-zettels:
//
// `$zet merge < output.md > output.md`
func MergeCmd(args []string) error {
	var mc string
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // Root zettel content comes from stdin
		s, err := storage.UpdateDB(c.ZetDir)
		if err != nil {
			return fmt.Errorf("Error syncing database and flat files: %v", err)
		}
		defer s.Close()

		stdin, err := getStdin()
		if err != nil {
			return fmt.Errorf("Error getting standard input: %v", err)
		}
		if stdin == "" {
			return nil
		}

		mc, err = s.Merge(stdin)
		if err != nil {
			return fmt.Errorf("Error splitting zettel content: %v", err)
		}
	default:
		if strings.ToLower(os.Args[2]) == `help` {
			fmt.Printf(mergeUsage)
			break
		}
		s, err := storage.UpdateDB(c.ZetDir)
		if err != nil {
			return fmt.Errorf("Error syncing database and flat files: %v", err)
		}
		defer s.Close()

		p := filepath.Join(c.ZetDir, args[2])

		// This essentially locks support to just readme files.
		if !strings.HasSuffix(p, `README.md`) {
			p = filepath.Join(p, `README.md`)
		}
		// Does the file exist?
		ok, err := meta.IsFile(p)
		if err != nil {
			if err == meta.ErrPathDoesNotExist {
				return err
			}
			return fmt.Errorf("Failed to ensure file exists: %v", err)
		}
		if !ok {
			return errors.New("path corresponds to a directory")
		}
		cb, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("Error reading zettel content: %v", err)
		}
		c := string(cb)

		mc, err = s.Merge(c)
		if err != nil {
			return fmt.Errorf("Error merging linked zettel content: %v", err)
		}
	}
	if mc != "" {
		fmt.Println(mc)
	}
	return nil
}

func ConfigCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	if n == 2 {
		fmt.Printf("ZET_DIR=%s\n", c.ZetDir)
		fmt.Printf("EDITOR=%s\n", c.Editor)
		return nil
	}

	switch strings.ToLower(os.Args[2]) {
	case `dir`:
		fmt.Println(filepath.Join(c.ConfDir, c.File))
	default:
		fmt.Printf(configUsage)
	}
	return nil
}

// ListCmd parses and validates user arguments for the list command.
// If arguments are valid, it calls the desired operation.
func ListCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	var zettels []storage.Zettel
	var err error
	switch n {
	case 2: // no args
		zettels, err = meta.List(c.ZetDir, `dir_name ASC`)
		if err != nil {
			return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
		}
	case 3: // one arg
		switch strings.ToLower(args[2]) {
		case `recent`:
			zettels, err = meta.List(c.ZetDir, `mtime ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
		case `alpha`:
			zettels, err = meta.List(c.ZetDir, `title ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
		case `length`:
			zettels, err = meta.List(c.ZetDir, `LENGTH(body) ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
		case `help`:
			fmt.Printf(listUsage)
			return nil
		default:
			fmt.Fprintln(os.Stderr, "Error: incorrect sub-command.")
			fmt.Fprintf(os.Stderr, listUsage)
			os.Exit(1)
		}
	}
	for _, z := range zettels {
		fmt.Println(yellow + z.DirName + reset + " " + z.Title)
	}
	return nil
}

// LinkCmd parses and validates user arguments for the link command.
// If arguments are valid, it calls the desired operation.
func LinkCmd(args []string) error {
	var l string
	var err error
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // no args, use pwd as path
		l, err = meta.CurrLink(c.ZetDir)
		if err != nil {
			return err
		}
	case 3: // one arg, use c.ZetDir/arg as path
		if strings.ToLower(args[2]) == `help` {
			fmt.Println(linkUsage)
			return nil
		}
		p := filepath.Join(c.ZetDir, args[2])
		l, err = meta.Link(p)
		if err != nil {
			return err
		}
	}

	fmt.Println(l)
	return nil
}
