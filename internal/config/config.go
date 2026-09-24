// Package config stores user settings in a small JSON file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const DefaultAutoKeep = 10

type Config struct {
	// BackupDir is the root folder that holds one subfolder of archives per game.
	BackupDir string `json:"backupDir"`
	// AutoKeep is how many automatic safety backups are kept per game.
	AutoKeep int `json:"autoKeep"`
	// SavePaths overrides the auto-detected save folder, keyed by game ID.
	SavePaths map[string]string `json:"savePaths,omitempty"`
	// LastGame is the game that was selected when the app was closed.
	LastGame string `json:"lastGame,omitempty"`

	path string
}

// Load reads the config at path, filling in defaults for anything missing.
// A missing file is not an error.
func Load(path, defaultBackupDir string) (*Config, error) {
	c := &Config{path: path}

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("config file %s is corrupted: %w", path, err)
		}
	}

	if c.BackupDir == "" {
		c.BackupDir = defaultBackupDir
	}
	if c.AutoKeep <= 0 {
		c.AutoKeep = DefaultAutoKeep
	}
	if c.SavePaths == nil {
		c.SavePaths = map[string]string{}
	}
	return c, nil
}

// Save writes the config atomically: a crash mid-write never leaves a truncated file.
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), c.path)
}
