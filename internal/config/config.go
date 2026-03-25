// Package config manages application settings for the tickets daemon.
// Settings are stored as JSON in a settings.json file within the tickets directory.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const settingsFileName = "settings.json"

// Settings holds configuration values for the tickets daemon.
type Settings struct {
	// StaleThresholdMinutes is how many minutes before assuming an in-progress
	// ticket is abandoned. Defaults to 30.
	StaleThresholdMinutes int `json:"stale_threshold_minutes"`

	// ExitAfterMinutes is how many minutes a daemon will sit idle before
	// exiting. Defaults to 60.
	ExitAfterMinutes int `json:"exit_after_minutes"`
}

// Path returns the full path to the settings file within ticketsDir.
func Path(ticketsDir string) string {
	return filepath.Join(ticketsDir, settingsFileName)
}

// Default returns a Settings struct populated with default values.
func Default() *Settings {
	return &Settings{
		StaleThresholdMinutes: 30,
		ExitAfterMinutes:      60,
	}
}

// Load reads settings from a settings.json file in ticketsDir. If the file
// does not exist, defaults are returned. Fields absent from the file take their
// default values.
func Load(ticketsDir string) (*Settings, error) {
	path := Path(ticketsDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, err
	}

	// Start with defaults so that absent fields keep their default values.
	s := Default()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Save writes the settings to a settings.json file in ticketsDir,
// creating the directory if it does not exist.
func Save(ticketsDir string, s *Settings) error {
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(Path(ticketsDir), data, 0644)
}
