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

	zet ls|list          - Prints all zettels to stdout.
	zet ls|list recent   - Prints all zettels sorted by modification time.
	zet ls|list length   - Prints all zettels sorted by word count.
	zet ls|list alpha    - Prints all zettels sorted by alphabetical titles.
	zet ls|list help     - Provides command information.`

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
		l, err := List(c.ZetDir)
		if err != nil {
			return fmt.Errorf("Failed to retrieve list of zettels: %v", err)
		}
		for _, z := range l {
			fmt.Println(z)
		}
	case 3: // one arg
		switch strings.ToLower(args[2]) {
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
func List(zetPath string) ([]string, error) {
	var l []string
	s, err := storage.UpdateDB(zetPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to sync database: %v", err)
	}
	defer s.Close()
	files, err := s.AllZettels()

	for _, f := range files {
		l = append(l, f.DirName+" "+f.Name)
	}
	return l, nil
}
