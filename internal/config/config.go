// Package config manages application settings for the tickets daemon.
// Settings are stored as JSON in a settings.json file within the tickets directory.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	defaultEditor              = "cursor"
	defaultOpenTerminalCommand = "open -a iTerm ."
)

// Current holds the active settings for the running process. It is set once
// by Init at startup and is safe to read from any goroutine thereafter.
var Current *Settings

const settingsFileName = "settings.json"

// Settings holds configuration values for the tickets daemon.
type Settings struct {
	// StaleThresholdMinutes is how many minutes before assuming an in-progress
	// ticket is abandoned. Defaults to 30.
	StaleThresholdMinutes int `json:"stale_threshold_minutes"`

	// Editor is the name of the editor to use. Supported values: "cursor",
	// "vscode". Defaults to "cursor".
	Editor string `json:"editor"`

	// OpenTerminalCommand is the command used to open a terminal window in a
	// given directory. It is run with the working directory set to the target
	// path. Defaults to "open -a iTerm .".
	OpenTerminalCommand string `json:"open_terminal_command"`
}

// Path returns the full path to the settings file within ticketsDir.
func Path(ticketsDir string) string {
	return filepath.Join(ticketsDir, settingsFileName)
}

// Default returns a Settings struct populated with default values.
func Default() *Settings {
	return &Settings{
		StaleThresholdMinutes: 30,
		Editor:                defaultEditor,
		OpenTerminalCommand:   defaultOpenTerminalCommand,
	}
}

// Init loads settings from ticketsDir and stores them in Current. It must be
// called once before any code reads config.Current.
func Init(ticketsDir string) error {
	s, err := Load(ticketsDir)
	if err != nil {
		return err
	}
	Current = s
	return nil
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
