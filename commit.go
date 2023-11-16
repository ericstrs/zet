package zet

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/iuiq/zet/internal/config"
	"github.com/iuiq/zet/internal/meta"
)

var commitUsage = `NAME

	commit - performs a git commit using zettel's title.

USAGE

	zet commit      - Commits the README.md file in current directory.
	zet commit all  - Commits all modified/new README.md files.
	zet commit help - Provides command information.`

// CommitCmd parses and validates user arguments for the commit command.
// If arguments are valid, it calls the desired operation.
func CommitCmd(args []string) error {
	c := new(config.C)
	if err := c.Init(); err != nil {
		return fmt.Errorf("Failed to initialize configuration file: %v", err)
	}
	n := len(args)

	switch n {
	case 2: // no args, use pwd as path
		// Get path to zettel directory and ensure user is in a zettel.
		p, ok, err := meta.InZettel()
		if err != nil {
			return fmt.Errorf("Failed to check if user is in a zettel: %v", err)
		}
		if !ok {
			return errors.New("not in a zettel")
		}
		p = filepath.Join(p, `README.md`)

		// Get zettel title to use as commit message body.
		t, err := meta.Title(p)
		if err != nil {
			return fmt.Errorf("Failed to retrieve zettel title: %v", err)
		}

		if err := commit(".", p, t); err != nil {
			return fmt.Errorf("Failed to commit zettel: %v", err)
		}
	case 3: // one arg
		switch strings.ToLower(args[2]) {
		case `help`:
			fmt.Println(commitUsage)
			return nil
		case `all`:
			files, err := readmeFiles(c.ZetDir)
			if err != nil {
				return fmt.Errorf("Failed to retrieve files to commit: %v", err)
			}
			if err := commitBulk(c.ZetDir, files); err != nil {
				return fmt.Errorf("Failed to commit zettels: %v", err)
			}
		default:
			fmt.Fprintln(os.Stderr, "Error: incorrect sub-command.")
			fmt.Fprintf(os.Stderr, commitUsage)
			os.Exit(1)
		}
	}

	return nil
}

// commitBulk commits a bulk of files given a list of paths to the
// files. Each commit uses the zettel's title as message body.
func commitBulk(zetPath string, files []string) error {
	for _, pp := range files {
		fp := filepath.Join(zetPath, pp)
		t, err := meta.Title(fp)
		if err != nil {
			log.Printf("Failed to retrieve zettel title: %v", err)
			continue
		}
		if err := commit(zetPath, fp, t); err != nil {
			return err
		}
	}
	return nil
}

// commit commits a zettel file at a given path with a given commit
// message.
func commit(d, p, t string) error {
	if err := runCmd(d, "git", "add", p); err != nil {
		return err
	}
	if err := runCmd(d, "git", "commit", "-m", t); err != nil {
		return err
	}
	return nil
}

// readmeFiles parses the output of `git status --porcelain` to find and
// return the list of modified and new README.md files at the zet
// directory.
func readmeFiles(zpath string) ([]string, error) {
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
