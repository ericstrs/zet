package meta

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// linkFormat is the format for a zettel link. It should take the form
	// `* [dir](../dir/) title`
	linkFormat = "* [%s](../%s) %s"
)

// CurrLink returns the zettel link for the current zettel.
func CurrLink(zetDir string) (string, error) {
	var l string
	// Get path to zettel directory and ensure user is in a zettel.
	p, ok, err := InZettel(zetDir)
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

// InZettel checks if the user is in a zettel. This is done by checking
// if the current working directory's parent directory path is equal to
// saved zettel directory path. It returns path to current working
// directory, reports whether user is in a zettel directory, and an error
// indicating if something went wrong with retrieving the current
// working directory.
func InZettel(zetDir string) (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("failed to get current working directory: %w", err)
	}
	parentDir := filepath.Dir(cwd)
	isZettel := parentDir == zetDir
	return cwd, isZettel, nil
}

// Link returns the zettel link for the zettel at the given path.
func Link(path string) (string, error) {
	t, err := Title(path)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve zettel title: %v", err)
	}

	d := filepath.Base(path)

	l := fmt.Sprintf(linkFormat, d, d, t)
	return l, nil
}

// Links returns links from a zettel at the given path.
func Links(path string) (string, error) {
	// This essentially locks support to just readme files.
	if !strings.HasSuffix(path, `README.md`) {
		path = filepath.Join(path, `README.md`)
	}

	// Does the file exist?
	ok, err := IsFile(path)
	if err != nil {
		if err == ErrPathDoesNotExist {
			return "", err
		}
		return "", fmt.Errorf("Failed to ensure file exists: %v", err)
	}
	if !ok {
		return "", errors.New("path corresponds to a directory")
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(contentBytes)
	linkLines := ParseLinks(content)

	return strings.Join(linkLines, "\n"), nil
}

// ParseLinks parses out and returns the links from zettel content.
// A link is takes the form of a line containing the substring
// "[dir](../dir) title".
func ParseLinks(content string) []string {
	var linkLines []string
	linkRegex := regexp.MustCompile(`^.*(\[(.+)\]\(\.\./(.*?)/?\) (.+))`)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		matches := linkRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			linkLines = append(linkLines, line)
			continue
		}
	}
	return linkLines
}
