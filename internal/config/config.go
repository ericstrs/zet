// Package config provides functionality related to zet configurations.
package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"
)

type C struct {
	Id      string `yaml:"id"`       // application name
	ConfDir string `yaml:"conf_dir"` // os.UserConfigDir
	File    string `yaml:"file"`     // config.yaml

	ZetDir string `yaml:"zet_dir"` // directory where zet resides
}

// Init initializes a new configuration.
func (c *C) Init() {
	// Find path to zet directory.
	p, err := zetPath()
	if err != nil {
		log.Printf("Failed to initialize configuration file: %v.\n", err)
		return
	}

	// Find path to configuration directory.
	d, err := dir()
	if err != nil {
		log.Printf("Failed to initialize configuration file: %v.\n", err)
		return
	}

	c.ZetDir = p
	c.ConfDir = d
	c.Id = `zet`
	c.File = `config.yaml`
}

// Dir returns the user defined configuration directory. An error is
// returned if the location cannot be determined.
func dir() (string, error) {
	dir, err := os.UserConfigDir()
	return dir, err
}

// confPath returns the path to the configuration file.
func (c C) confPath() string {
	return filepath.Join(c.ConfDir, c.Id, c.File)
}

// ZetPath returns the path to where the zet resides. It first checks
// for the ZET_PATH environment variable. If the environment variable
// is not set, it falls back to reading from a configuration file.
func zetPath() (string, error) {
	path := os.Getenv("ZET_PATH")
	if path != "" {
		// Return the path if it's found in the environment variable
		return path, nil
	}
	return path, errors.New("Zet path not found")
}
