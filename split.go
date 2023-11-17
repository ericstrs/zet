package zet

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iuiq/zet/internal/meta"
	"github.com/iuiq/zet/internal/storage"
)

// SplitZettel splits zettel content from stdin into sub-zettels.
func SplitZettel(zetDir, zettelDir, content string) error {
	if content == "" {
		return errors.New("zettel content is empty")
	}

	currLink, err := meta.Link(zettelDir)
	if err != nil {
		return fmt.Errorf("Error getting current link: %v", err)
	}

	b := meta.ParseBody(content)
	zettels := makeZettels(b)

	for _, z := range zettels {
		if err := Add(zetDir, "", z.Title, z.Body, "", currLink, false); err != nil {
			return fmt.Errorf("Error adding sub-zettels: %v", err)
		}
		// Set sleep timer to prevent isosec stomping
		time.Sleep(1000 * time.Millisecond)
	}

	return nil
}

// makeZettels construct sub-zettels from a list of strings.
func makeZettels(bodyLines []string) []storage.Zettel {
	var zettels []storage.Zettel
	var currZettel storage.Zettel
	var isInsideZettel bool

	for i, line := range bodyLines {
		if strings.HasPrefix(line, `## `) || i == len(bodyLines)-1 {
			if isInsideZettel {
				zettels = append(zettels, currZettel)
				currZettel = storage.Zettel{}
			}
			currZettel.Title = strings.TrimPrefix(line, `## `)
			isInsideZettel = true
			continue
		}

		currZettel.Body += line + "\n"
	}

	return zettels
}
