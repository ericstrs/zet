package meta

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iuiq/zet"
	"github.com/iuiq/zet/internal/config"
)

var linkUsage = `link prints the link of a zettel.

Usage:

	zet link          - Prints the zettel link for the current dir.
	zet link [isosec] - Prints the zettel link for the given dir isosec.
`

// linkFormat is the format for a zettel link. It should take the form
// `[dir](../dir/) title`
var linkFormat = "* [%s](../%s/) %s"

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
		l, err = CurrLink()
		if err != nil {
			return err
		}
	case 3: // one arg, use c.ZetDir/arg as path
		if strings.ToLower(args[2]) == `help` {
			fmt.Println(linkUsage)
			return nil
		}
		p := filepath.Join(c.ZetDir, args[2])
		l, err = Link(p)
		if err != nil {
			return err
		}
	default:
		fmt.Println(linkUsage)
		return nil
	}

	fmt.Println(l)
	return nil
}

// CurrLink returns the zettel link for the current zettel.
func CurrLink() (string, error) {
	var l string
	// Get path to zettel directory and ensure user is in a zettel.
	p, ok, err := zet.InZettel()
	if err != nil {
		return "", fmt.Errorf("Failed to check if user is in a zettel: %v", err)
	}
	if !ok {
		return "", errors.New("not in a zettel")
	}
	l, err = Link(p)
	if err != nil {
		return "", err
	}

	return l, nil
}

// Link returns the zettel link for the zettel at the given path.
func Link(path string) (string, error) {
	d, err := zettelDir(path)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve zettel dir: %v", err)
	}
	t, err := Title(path)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve zettel title: %v", err)
	}

	l := fmt.Sprintf(linkFormat, d, d, t)
	return l, nil
}

// zettelDir returns the zettel directory name given the path. An empty
// string and an error is returned if the parent directory is not in the
// zet directory.
func zettelDir(path string) (string, error) {
	parentDir := filepath.Dir(path)
	parentName := filepath.Base(parentDir)
	if parentName != "zet" {
		return "", fmt.Errorf("%s does not reside in zet dir", parentName)
	}
	name := filepath.Base(path)
	return name, nil
}