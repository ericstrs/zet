// Package config provides functionality related to zet configurations.
package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var ErrPathDoesNotExist = errors.New("path does not exist")

type C struct {
	Id      string `yaml:"id"`       // application name
	ConfDir string `yaml:"conf_dir"` // os.UserConfigDir
	File    string `yaml:"file"`     // config.yaml
	Editor  string `yaml:"editor"`   // user's preferred editor

	ZetDir string `yaml:"zet_dir"` // directory where zet resides
}

// Init initializes a new configuration.
func (c *C) Init() error {
	e, err := preferredEditor()
	if err != nil {
		return fmt.Errorf(
			"%v. Please install a text editor or set the 'VISUAL' or 'EDITOR' "+
				"environment variable to your preferred editor.",
			err,
		)
	}

	// Find path to zet directory.
	p, err := zetPath()
	if err != nil {
		return fmt.Errorf("Couldn't resolve zet directory path: %v", err)
	}

	// Find path to configuration directory.
	d, err := dir()
	if err != nil {
		return fmt.Errorf("Couldn't resolve user config directory: %v", err)
	}

	c.ZetDir = p
	c.ConfDir = d
	c.Id = `zet`
	c.File = `config.yaml`
	c.Editor = e

	return nil
}

// Dir returns the user defined configuration directory. An error is
// returned if the location cannot be determined.
func dir() (string, error) {
	dir, err := os.UserConfigDir()
	return dir, err
}

// ConfPath returns the path to the configuration file.
func (c C) confPath() string {
	return filepath.Join(c.ConfDir, c.Id, c.File)
}

// ZetPath returns and validates the path to where the zet resides. It
// first checks for the ZET_PATH environment variable. If the
// environment variable is not set, it falls back to reading from a
// configuration file.
func zetPath() (string, error) {
	path, ok := os.LookupEnv("ZET_PATH")
	if ok {
		e, err := isDir(path)
		if err != nil {
			return "", fmt.Errorf("Failed to validate the zet directory: %v", err)
		}
		if err == ErrPathDoesNotExist {
			return "", fmt.Errorf("Specified path does not exist: %s", path)
		}
		if !e {
			return "", fmt.Errorf("Path exists but is not a directory: %s", path)
		}

		// Return the path if it's found in the environment variable
		return path, nil
	}

	return path, errors.New("Config file and $ZET_PATH not found")
}

// IsDir checks if a given path exists and is a directory.
func isDir(path string) (bool, error) {
	// Use os.Stat to get information about the path
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ErrPathDoesNotExist
		}
		// Propagate any other error
		return false, err
	}
	// Use FileInfo.IsDir method to check if the path is a directory
	return info.IsDir(), nil
}

// PreferredEditor returns the user's preferred editor based the priority:
func preferredEditor() (string, error) {
	if visual := os.Getenv("VISUAL"); visual != "" {
		return visual, nil
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor, nil
	}

	// List of fallback editors
	fallbacks := []string{"code", "vim", "vi", "emacs", "nano"}

	// Check if any of the fallback editors are available
	for _, editor := range fallbacks {
		if hasCmd(editor) {
			return editor, nil
		}
	}

	// If none are available, return an error
	return "", errors.New("No text editor found")
}

// HasCmd checks if a command is available on the system.
func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
