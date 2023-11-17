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

// Body returns the body for a zettel at the given path. A body is
// defined as any line that is not a title, link, or tags line.
func Body(path string) (string, error) {
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
	bodyLines := ParseBody(content)

	return strings.Join(bodyLines, "\n"), nil
}

// ParseBody parses out and returns the body of zettel content.
func ParseBody(content string) []string {
	var bodyLines []string
	var title string
	isBody := false
	// Match lines that contain a link. E.g., `* [dir][../dir] title`
	linkRegex := regexp.MustCompile(`\[(.+)\]\(\.\./(.*?)/?\) (.+)`)
	tagRegex := regexp.MustCompile(`^ {4,}#[a-zA-Z]+`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// Is line the title?
		if title == "" && strings.HasPrefix(line, `# `) {
			title = strings.TrimPrefix(line, `# `)
			isBody = true
			continue
		}

		// Is line a markdown link?
		matches := linkRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			continue
		}

		// Is line the tag line?
		if tagRegex.MatchString(line) {
			continue
		}

		// Everything else is considered as body.
		if isBody {
			bodyLines = append(bodyLines, line)
		}
	}

	return bodyLines
}
