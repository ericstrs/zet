package meta

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrPathDoesNotExist = errors.New("path does not exist")

// Title returns the title for a zettel at the given path.
//
// The prefix used to parse out title differs for each unique file type:
//
//   - README.md file title is defined as the first occurrence of a number
//     sign followed by a space: `# `
func Title(path string) (string, error) {
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

	// Open file in read-only mode
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return "", fmt.Errorf("Failed to read file: %v", err)
	}
	defer file.Close()

	// Get title prefix for the file type
	f := filepath.Base(path)
	p, err := prefix(f)
	if err != nil {
		return "", fmt.Errorf("Failed to get title prefix: %v", err)
	}

	// Find title for the specific file type
	t, err := parseTitle(file, p)
	if err != nil {
		return "", fmt.Errorf("Failed to scan file: %v", err)
	}

	return t, nil
}

// IsFile reports whether a path exists and corresponds to a file.
func IsFile(p string) (bool, error) {
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ErrPathDoesNotExist
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// prefix returns the title prefix for a given file type.
func prefix(f string) (string, error) {
	var p string
	switch strings.ToLower(f) {
	case `readme.md`:
		p = `# `
	default:
		return "", fmt.Errorf("file %q not supported", f)
	}

	return p, nil
}

// parseTitle returns the title from a file using the given prefix. If a
// title is found, the title is returned without the prefix. If the
// given file doesn't have a title, an empty string is returned.
func parseTitle(f *os.File, p string) (string, error) {
	var t string
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, p) {
			t = line
			break
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}

	return strings.TrimPrefix(t, p), nil
}
