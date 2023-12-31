package zet

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/justericg/zet/internal/meta"
	"github.com/justericg/zet/internal/storage"
)

// SplitZettel splits zettel content from stdin into sub-zettels.
func SplitZettel(zetDir, zettelDir, b string) error {
	if b == "" {
		return errors.New("zettel content is empty")
	}

	currLink, err := meta.Link(zettelDir)
	if err != nil {
		return fmt.Errorf("Error getting current link: %v", err)
	}

	zettels := makeZettels(strings.Split(b, "\n"))
	i := Isosec()
	iso, err := strconv.Atoi(i)
	if err != nil {
		return fmt.Errorf("Error converting isosec string to int: %v", err)
	}

	for _, z := range zettels {
		iso++
		newDirPath := filepath.Join(zetDir, fmt.Sprintf("%d", iso))
		if err := dir(newDirPath); err != nil {
			return fmt.Errorf("Error creating new zettel directory: %v", err)
		}

		if err := Add(newDirPath, "", z.Title, z.Body, "", currLink, false); err != nil {
			return fmt.Errorf("Error adding sub-zettels: %v", err)
		}
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

		// Check if the line starts with more than two hash symbols
		if strings.HasPrefix(line, "###") {
			// Remove one hash symbol
			line = "#" + strings.TrimPrefix(line, "##")
		}

		// If line starts with more than two hash tags, then remove one.
		currZettel.Body += line + "\n"
	}

	return zettels
}
