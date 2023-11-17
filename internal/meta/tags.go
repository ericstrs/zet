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

// Tags returns the tags for a zettel at the given path. A tags is
// defined as any line that is not a title, link, or tags line.
func Tags(path string) (string, error) {
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
	tagLines := ParseTags(content)

	return strings.Join(tagLines, "\n"), nil
}

// ParseTags parses out and returns the body of zettel content.
// A tag line takes the form of a line containing four or more spaces
// followed by a hash tag.
func ParseTags(content string) []string {
	var tagLines []string
	tagRegex := regexp.MustCompile(`^ {4,}(#[a-zA-Z]+.*)`)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		matches := tagRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			tagLines = append(tagLines, matches[1])
			continue
		}
	}
	return tagLines
}
