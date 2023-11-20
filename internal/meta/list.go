package meta

import (
	"fmt"
	"os"
	"strings"

	"github.com/iuiq/zet/internal/config"
	"github.com/iuiq/zet/internal/storage"
)

var listUsage = `NAME

	list - lists all the zettels.

USAGE

	zet list|ls          - Prints all zettels to stdout sorted by creation date.
	zet list|ls recent   - Prints all zettels sorted by modification time.
	zet list|ls length   - Prints all zettels sorted by word count.
	zet list|ls alpha    - Prints all zettels by alphabetically sorted titles.
	zet list|ls help     - Provides command information.

DESCRIPTION

The list command serves as a tool viewing a collection of zettels. This command displays a list of all zettels stored in the system. Its main purpose is to output all zettels in an organized manner, in ascending order.`

// ListCmd parses and validates user arguments for the list command.
// If arguments are valid, it calls the desired operation.
func ListCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // no args
		l, err := List(c.ZetDir, `dir_name ASC`)
		if err != nil {
			return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
		}
		for _, z := range l {
			fmt.Println(z)
		}
	case 3: // one arg
		switch strings.ToLower(args[2]) {
		case `recent`:
			l, err := List(c.ZetDir, `mtime ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
			for _, z := range l {
				fmt.Println(z)
			}
		case `alpha`:
			l, err := List(c.ZetDir, `title ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
			for _, z := range l {
				fmt.Println(z)
			}
		case `length`:
			l, err := List(c.ZetDir, `LENGTH(body) ASC`)
			if err != nil {
				return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
			}
			for _, z := range l {
				fmt.Println(z)
			}
		case `help`:
			fmt.Println(listUsage)
			return nil
		default:
			fmt.Fprintln(os.Stderr, "Error: incorrect sub-command.")
			fmt.Fprintf(os.Stderr, listUsage)
			os.Exit(1)
		}
	}
	return nil
}

// List retrieves a list of zettels. It synchronizes the database and
// gets list of zettels.
func List(zetPath, sort string) ([]string, error) {
	var l []string
	s, err := storage.UpdateDB(zetPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to sync database: %v", err)
	}
	defer s.Close()
	files, err := s.AllZettels(sort)

	for _, f := range files {
		l = append(l, f.DirName+" "+f.Title)
	}
	return l, nil
}
