package zet

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ericstrs/zet/internal/meta"
)

// CommitBulk commits a bulk of files given a list of paths to the
// files. Each commit uses the zettel's title as message body.
func CommitBulk(zetPath string, files []string) error {
	for _, pp := range files {
		fp := filepath.Join(zetPath, pp)
		t, err := meta.Title(fp)
		if err != nil {
			log.Printf("Failed to retrieve zettel title: %v", err)
			continue
		}
		if err := Commit(zetPath, fp, t); err != nil {
			return err
		}
	}
	return nil
}

// Commit commits a zettel file at a given path with a given commit
// message.
func Commit(d, p, t string) error {
	if err := runCmd(d, "git", "add", p); err != nil {
		return err
	}
	if err := runCmd(d, "git", "commit", "-m", t); err != nil {
		return err
	}
	return nil
}

// ReadmeFiles parses the output of `git status --porcelain` to find and
// return the list of modified and new README.md files at the zet
// directory.
func ReadmeFiles(zpath string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = zpath // Set working directory to specified path
	outBytes, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	out := string(outBytes)
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()

		// Look for modified files with ' M '.
		if strings.HasPrefix(line, ` M `) {
			// Check if the modified file is a README.md
			if strings.HasSuffix(line, `README.md`) {
				// Extract the partial file path
				path := strings.TrimSpace(line[3:])
				files = append(files, path)
			}
			continue
		}

		// Look for '?? ' indicating untracked files and check if untracked
		// file is a directory that contains a README.md file.
		if strings.HasPrefix(line, `?? `) {
			if strings.HasSuffix(line, `/`) {
				path := strings.TrimSpace(line[3:])
				readmePath := filepath.Join(path, `README.md`)
				_, err := os.Stat(readmePath)
				if err == nil {
					files = append(files, readmePath)
				}
			}
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return files, nil
}
