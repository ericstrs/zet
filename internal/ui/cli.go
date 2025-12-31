package ui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/ericstrs/zet"
	"github.com/ericstrs/zet/internal/config"
	"github.com/ericstrs/zet/internal/meta"
	"github.com/ericstrs/zet/internal/storage"
)

const (
	yellow = "\033[33m" // ANSI escape code for yellow
	red    = "\033[31m" // ANSI escape code for red
	reset  = "\033[0m"  // ANSI escape code to reset to default color

	searchUsage = `NAME

  search - searches for zettels.

USAGE

  zet search query|q <term>  - Print zettels given a search term.
  zet search browse|b <term> - Interactively search for a zettel.
  zet search help            - Print zettels given a search term.
`
	splitUsage = `NAME

  split - splits up a given zettel into sub-zettels.

USAGE

  zet split          - Splits zettel content from stdin into sub-zettels.
  zet split <isosec> - Splits zettel content from README.md in isosec
                       directory into sub-zettels.`
	contentUsage = `NAME

  content - prints different sections of zettel content.

USAGE

  zet content title - Prints title from README.md in current directory
                      or in given directory.
  zet content body  - Prints body from README.md in current directory or
                      in given directory.
  zet content links - Prints links from README.md in current directory
                      or in given directory.
  zet content tags  - Prints tags from README.md in current directory or
                      in given directory.
`
	mergeUsage = `NAME

  merge - Merges prints the contents of split zettel's into single body of text.

USAGE

  zet merge <isosec> - Merges contents of split linked zettel's at given
                       isosec directory or using stdin.

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

  zet list|ls               - Prints all zettels to stdout sorted by creation date.
  zet list|ls modified [-d] - Prints all zettels sorted by modification time.
  zet list|ls length        - Prints all zettels sorted by word count.
  zet list|ls alpha         - Prints all zettels by alphabetically sorted titles.
  zet list|ls help          - Provides command information.

FLAGS

  -d, --desc   Descending order (most recent first). Applies to modified and its subcommands.
  -p, --plain  Plain output (no ANSI color codes).

SUBCOMMANDS

  Temporal subcommands filter by creation date:

  day [YYYY-MM-DD] [-N] [-N:]   - Print daily view. -N for N days ago, -N: for range.
  week [YYYY-MM-DD] [-N] [-N:]  - Print weekly view. -N for N weeks ago, -N: for range.
  month [YYYY-MM] [-N] [-N:]    - Print monthly view. -N for N months ago, -N: for range.
  year [YYYY] [-N] [-N:]        - Print yearly view. -N for N years ago, -N: for range.

  Temporal subcommands under modified filter by modification time:

  modified day [YYYY-MM-DD] [-N] [-N:]   - Zettels modified on day.
  modified week [YYYY-MM-DD] [-N] [-N:]  - Zettels modified in week.
  modified month [YYYY-MM] [-N] [-N:]    - Zettels modified in month.
  modified year [YYYY] [-N] [-N:]        - Zettels modified in year.

EXAMPLES

  zet list day              - Show zettels created today.
  zet list day 2024-12-25   - Show zettels created on Dec 25, 2024.
  zet list day -3           - Show zettels created 3 days ago.
  zet list day -3:          - Show zettels created from 3 days ago to today.
  zet list week             - Show zettels created this week.
  zet list modified         - Show all zettels sorted by modification time.
  zet list modified -d      - Show all zettels, most recently modified first.
  zet list modified day     - Show zettels modified today.
  zet list modified day -3  - Show zettels modified 3 days ago.
  zet list modified week -d - Show zettels modified this week, most recent first.

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
  zet link <isosec> - Prints zettel link for the given dir isosec.
  zet link annotate - Prints zettel content with annotated links.
  zet link help     - Provides command information.
`
	isoUsage = `NAME

    isosec - prints the current ISO date to the millisecond.

  USAGE

    zet isosec      - Print current ISO date to the millisecond.
    zet isosec help - Provides command information.
`
	addUsage = `NAME

    add - adds a new zettel with the given title and content.

  USAGE

    zet add|a                - Adds new zettel and opens for editing.
    zet add|a <title>        - Adds new zettel with provided title.
    zet add|a <title> <body> - Adds new zettel with provided title and body.
    zet add|a help           - Provides command information.

  DESCRIPTION

    All the above scenarios accept standard input. In which, content from
    Stdin is always appended after any argument data. Providing non-empty
    Stdin alongside ` + "`zet add`" + ` disables the interactive feature.

    Auto-linking is enabled by default. That is, if you are calling the
		add command from an existing zettel directory, the newly created zettel
		will have link to existing zettel.
`
	commitUsage = `NAME

    commit - performs a git commit using zettel's title.

  USAGE

    zet commit      - Commits the README.md file in current directory.
    zet commit all  - Commits all modified/new README.md files.
    zet commit help - Provides command information.
`
	relatedUsage = `NAME

    related - returns related zettels to a given zettel.

  USAGE

    zet related create    - Creates index and indexes all zettels.
		zet related           - Find related zettels for the zettel content
		                        provided by stdin.
    zet related <content> - Find related zettels for the zettel content
		                        provide by the argument.

	DESCRIPTION

		This command expects the zettel content to be passed in either through
		standard input or in an argument.
`
	annotateUsage = `NAME

  annotate - annotates zettel links

USAGE

  zet link annotate      - Annotates zettel links with a reason why someone
	                       should follow the link to the referenced content.
  zet link annotate help - Provides command information.

DESCRIPTION

  Example usage:

  Print a zettel with all links being annotated:

  ` + "`" + `$zet link annotate < README.md` + "`" + ``
)

func SearchCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}

	s, err := storage.UpdateDB(c.ZetDir, c.DBPath)
	if err != nil {
		return fmt.Errorf("Error syncing database and flat files: %v", err)
	}
	defer s.Close()
	n := len(args)

	if n < 3 {
		fmt.Fprintln(os.Stderr, "ERROR: Not enough arguments.")
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
		s, err := storage.UpdateDB(c.ZetDir, c.DBPath)
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
		s, err := storage.UpdateDB(c.ZetDir, c.DBPath)
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
		fmt.Printf("ZET_DB_PATH=%s\n", c.DBPath)
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

	// Parse -p/--plain flag (plain output) and remove from args
	plain := false
	var filteredArgs []string
	for _, arg := range args {
		if arg == "-p" || arg == "--plain" {
			plain = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs
	n := len(args)

	var zettels []storage.Zettel
	var err error

	if n == 2 {
		// no args
		zettels, err = meta.List(c.ZetDir, c.DBPath, `dir_name ASC`)
		if err != nil {
			return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
		}
	} else if n >= 3 {
		subcmd := strings.ToLower(args[2])
		switch subcmd {
		case `modified`, `recent`:
			zettels, err = handleModifiedCmd(c, args[3:])
			if err != nil {
				return err
			}
		case `alpha`:
			zettels, err = meta.List(c.ZetDir, c.DBPath, `title ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
		case `length`:
			zettels, err = meta.List(c.ZetDir, c.DBPath, `LENGTH(body) ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
		case `help`:
			fmt.Printf(listUsage)
			return nil
		case `day`, `week`, `month`, `year`:
			start, end, err := parseDateArgs(subcmd, args[3:])
			if err != nil {
				return fmt.Errorf("Failed to parse date arguments: %v", err)
			}
			zettels, err = meta.ListByDateRange(c.ZetDir, c.DBPath, start, end)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
		default:
			fmt.Fprintln(os.Stderr, "Error: incorrect sub-command.")
			fmt.Fprintf(os.Stderr, listUsage)
			os.Exit(1)
		}
	}
	for _, z := range zettels {
		if plain {
			fmt.Println(z.DirName + " " + z.Title)
		} else {
			fmt.Println(yellow + z.DirName + reset + " " + z.Title)
		}
	}
	return nil
}

// handleModifiedCmd handles the "modified" subcommand which can:
// - List all zettels sorted by mtime: zet list modified [-d]
// - Filter by temporal period: zet list modified day|week|month|year [-d] [date] [-N] [-N:]
func handleModifiedCmd(c *config.C, args []string) ([]storage.Zettel, error) {
	// Parse -d/--desc flag and separate from other args
	desc := false
	var remainingArgs []string
	for _, arg := range args {
		if arg == "-d" || arg == "--desc" {
			desc = true
		} else {
			remainingArgs = append(remainingArgs, arg)
		}
	}

	sortOrder := "ASC"
	if desc {
		sortOrder = "DESC"
	}

	// No subcommand - list all zettels sorted by mtime
	if len(remainingArgs) == 0 {
		return meta.List(c.ZetDir, c.DBPath, fmt.Sprintf("mtime %s", sortOrder))
	}

	// Check for temporal subcommand
	subcmd := strings.ToLower(remainingArgs[0])
	switch subcmd {
	case `day`, `week`, `month`, `year`:
		start, end, err := parseDateArgs(subcmd, remainingArgs[1:])
		if err != nil {
			return nil, fmt.Errorf("Failed to parse date arguments: %v", err)
		}
		return meta.ListByMtimeRange(c.ZetDir, c.DBPath, start, end, sortOrder)
	default:
		return nil, fmt.Errorf("unknown modified subcommand: %s", subcmd)
	}
}

// parseDateArgs parses date arguments for day/week/month/year subcommands.
// Returns start and end dates in YYYYMMDD format.
func parseDateArgs(period string, args []string) (string, string, error) {
	now := time.Now()

	// No additional args - use current period
	if len(args) == 0 {
		return dateRangeForPeriod(now, period)
	}

	arg := args[0]

	// Check for -N or -N: format (relative offset)
	if strings.HasPrefix(arg, "-") {
		isRange := strings.HasSuffix(arg, ":")
		numStr := strings.TrimPrefix(arg, "-")
		numStr = strings.TrimSuffix(numStr, ":")

		offset, err := strconv.Atoi(numStr)
		if err != nil {
			return "", "", fmt.Errorf("invalid offset: %s", arg)
		}

		targetDate := shiftDate(now, period, -offset)
		start, end, err := dateRangeForPeriod(targetDate, period)
		if err != nil {
			return "", "", err
		}

		if isRange {
			// -N: means from N periods ago to now
			_, endNow, err := dateRangeForPeriod(now, period)
			if err != nil {
				return "", "", err
			}
			end = endNow
		}
		return start, end, nil
	}

	// Parse explicit date
	targetDate, err := parseExplicitDate(arg, period)
	if err != nil {
		return "", "", err
	}
	return dateRangeForPeriod(targetDate, period)
}

// parseExplicitDate parses a date string based on the period type.
func parseExplicitDate(dateStr, period string) (time.Time, error) {
	var layout string
	switch period {
	case "day":
		layout = "2006-01-02"
	case "week":
		layout = "2006-01-02"
	case "month":
		layout = "2006-01"
	case "year":
		layout = "2006"
	}
	return time.Parse(layout, dateStr)
}

// shiftDate shifts a date by the given offset for the specified period.
func shiftDate(t time.Time, period string, offset int) time.Time {
	switch period {
	case "day":
		return t.AddDate(0, 0, offset)
	case "week":
		return t.AddDate(0, 0, offset*7)
	case "month":
		return t.AddDate(0, offset, 0)
	case "year":
		return t.AddDate(offset, 0, 0)
	}
	return t
}

// dateRangeForPeriod returns start and end dates in YYYYMMDD format for the given period.
func dateRangeForPeriod(t time.Time, period string) (string, string, error) {
	var start, end time.Time

	switch period {
	case "day":
		start = t
		end = t
	case "week":
		// Week starts on Monday
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		start = t.AddDate(0, 0, -(weekday - 1))
		end = start.AddDate(0, 0, 6)
	case "month":
		start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		end = start.AddDate(0, 1, -1)
	case "year":
		start = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
		end = time.Date(t.Year(), 12, 31, 0, 0, 0, 0, t.Location())
	default:
		return "", "", fmt.Errorf("unknown period: %s", period)
	}

	return start.Format("20060102"), end.Format("20060102"), nil
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

	s, err := storage.UpdateDB(c.ZetDir, c.DBPath)
	if err != nil {
		return fmt.Errorf("Error syncing database and flat files: %v", err)
	}
	defer s.Close()

	n := len(args)

	switch n {
	case 2: // no args, use pwd as path
		l, err = meta.CurrLink(c.ZetDir)
		if err != nil {
			return err
		}
		fmt.Println(l)
	case 3: // one arg, use c.ZetDir/arg as path
		switch strings.ToLower(args[2]) {
		case `annotate`, `a`:
			content, err := getStdin()
			if err != nil {
				return fmt.Errorf("Error getting standard input: %v", err)
			}
			links, err := zet.AnnotateLink(s, c.ZetDir, c.DBPath, content)
			if err != nil {
				return fmt.Errorf("Error getting standard input: %v", err)
			}
			for _, link := range links {
				fmt.Println(link)
			}
		case `help`:
			fmt.Printf(linkUsage)
		default:
			p := filepath.Join(c.ZetDir, args[2])
			l, err = meta.Link(p)
			if err != nil {
				return err
			}
			fmt.Println(l)
		}
	}
	return nil
}

// AddCmd parses and validates user arguments for the add command.
// If arguments are valid, it calls the desired operation.
func AddCmd(args []string) error {
	var title, body, stdin string
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	// Assign title and body based on positional arguments
	if n > 2 {
		if strings.ToLower(args[2]) == `help` {
			fmt.Printf(addUsage)
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

	// If current link cannot be found, skip auto-linking
	currLink, err := meta.CurrLink(c.ZetDir)
	if err != nil {
		currLink = ""
	}

	var openZettel bool
	// If no title and no body and no stdin, then open newly created zettel.
	if title == "" && body == "" && stdin == "" {
		openZettel = true
	}

	// Otherwise, just create the zettel without opening it.
	if err := zet.CreateAdd(c.ZetDir, c.Editor, title, body, stdin, currLink, openZettel); err != nil {
		return err
	}

	return nil
}

// IsosecCmd parses and validates user arguments for the isosec command.
// If arguments are valid, it calls the desired operation.
func IsosecCmd(args []string) {
	var iso string
	n := len(args)
	switch n {
	case 2:
		iso = zet.Isosec()
	case 3:
		if strings.ToLower(args[2]) == `help` {
			fmt.Printf(isoUsage)
			return
		}
	}
	fmt.Println(iso)
}

// CommitCmd parses and validates user arguments for the commit command.
// If arguments are valid, it calls the desired operation.
func CommitCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // no args, use pwd as path
		// Get path to zettel directory and ensure user is in a zettel.
		p, ok, err := meta.InZettel(c.ZetDir)
		if err != nil {
			return fmt.Errorf("Failed to check if user is in a zettel: %v", err)
		}
		if !ok {
			return errors.New("not in a zettel")
		}
		p = filepath.Join(p, `README.md`)

		// Get zettel title to use as commit message body.
		t, err := meta.Title(p)
		if err != nil {
			return fmt.Errorf("Failed to retrieve zettel title: %v", err)
		}

		if err := zet.Commit(".", p, t); err != nil {
			return fmt.Errorf("Failed to commit zettel: %v", err)
		}
	case 3: // one arg
		switch strings.ToLower(args[2]) {
		case `help`:
			fmt.Printf(commitUsage)
			return nil
		case `all`:
			files, err := zet.ReadmeFiles(c.ZetDir)
			if err != nil {
				return fmt.Errorf("Failed to retrieve files to commit: %v", err)
			}
			if err := zet.CommitBulk(c.ZetDir, files); err != nil {
				return fmt.Errorf("Failed to commit zettels: %v", err)
			}
		default:
			fmt.Fprintln(os.Stderr, "Error: incorrect sub-command.")
			fmt.Fprintf(os.Stderr, commitUsage)
			os.Exit(1)
		}
	}

	return nil
}

func RelatedCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	if n < 2 {
		fmt.Fprintln(os.Stderr, "Error: Not enough arguments.")
		fmt.Fprintf(os.Stderr, relatedUsage)
		os.Exit(1)
	}

	arg := ""
	if n > 2 {
		arg = args[2]
	}

	switch strings.ToLower(arg) {
	case `create`:
		bp := filepath.Join(c.ZetDir, "zet.bleve")
		index, err := zet.CreateIndex(bp)
		if err != nil {
			return err
		}

		// Get all zettels
		zettels, err := meta.List(c.ZetDir, c.DBPath, `dir_name ASC`)
		if err != nil {
			return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
		}

		if err := zet.IndexZettels(index, zettels); err != nil {
			return err
		}
	case `help`:
		fmt.Printf(relatedUsage)
	default: // user entered isosec
		// Set zettel content to stdin. If no stdin, fallback on passed in
		// argument.
		content, err := getStdin()
		if err != nil {
			return fmt.Errorf("Error getting standard input: %v", err)
		}
		if content == "" {
			if arg == "" {
				return fmt.Errorf("must provide zettel content either through stdin or an argument")
			}
			content = args[2]
		}

		bp := filepath.Join(c.ZetDir, "zet.bleve")
		index, err := bleve.Open(bp)
		if err != nil {
			return fmt.Errorf("error opening bleve index: %v\n", err)
		}
		zids, err := zet.RelatedZettels(index, content, 10)
		if err != nil {
			return err
		}

		for _, id := range zids {
			p := filepath.Join(c.ZetDir, id)
			l, err := meta.Link(p)
			if err != nil {
				return err
			}
			fmt.Println(l)
		}
	}

	return nil
}
