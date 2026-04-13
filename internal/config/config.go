// Package config manages application settings for the tickets daemon.
// Settings are stored as JSON in a settings.json file within the tickets directory.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
)

// Current holds the active settings for the running process. It is set once
// by Init at startup and is safe to read from any goroutine thereafter.
var Current *Settings

const settingsFileName = "settings.json"

// Settings holds configuration values for the tickets daemon. Each field's
// default value is declared in its `default` struct tag; Default() populates
// all fields automatically via reflection.
type Settings struct {
	// StaleThresholdMinutes is how many minutes before assuming an in-progress
	// ticket is abandoned. Defaults to 30.
	StaleThresholdMinutes int `json:"stale_threshold_minutes" default:"30"`

	// Editor is the name of the editor to use. Supported values: "cursor",
	// "vscode". Defaults to "cursor".
	Editor string `json:"editor" default:"cursor"`

	// OpenTerminalCommand is the command used to open a terminal window in a
	// given directory. It is run with the working directory set to the target
	// path. Defaults to "open -a iTerm .".
	OpenTerminalCommand string `json:"open_terminal_command" default:"open -a iTerm ."`

	// ModelImplement, ModelRefactor, ModelReview, and ModelRespond set the
	// Claude model used for each ticket phase independently. Empty string uses
	// Claude's default model for that phase.
	ModelImplement string `json:"model_implement" default:"sonnet"`
	ModelRefactor  string `json:"model_refactor" default:"opus"`
	ModelReview    string `json:"model_review" default:"opus"`
	ModelRespond   string `json:"model_respond" default:"opus"`

	// Effort is the effort level passed to Claude (e.g. "low", "normal",
	// "high"). Empty string uses Claude's default.
	Effort string `json:"effort" default:"high"`

	// TerminalTheme selects the colour scheme. See theme.Init for valid
	// values. Defaults to "tan".
	TerminalTheme string `json:"terminal_theme" default:"tan"`
}

// ModelForPhase returns the configured model for the given ticket phase,
// or an empty string if none is set (meaning use Claude's default).
func (s *Settings) ModelForPhase(phase string) string {
	switch phase {
	case "implement":
		return s.ModelImplement
	case "refactor":
		return s.ModelRefactor
	case "review":
		return s.ModelReview
	case "respond":
		return s.ModelRespond
	}
	return ""
}

// Path returns the full path to the settings file within ticketsDir.
func Path(ticketsDir string) string {
	return filepath.Join(ticketsDir, settingsFileName)
}

// Default returns a Settings struct populated with default values read from
// `default` struct tags. Adding a new field with a `default` tag is sufficient
// to make it appear in the generated settings.json — no other code changes
// are needed.
func Default() *Settings {
	s := &Settings{}
	v := reflect.ValueOf(s).Elem()
	t := v.Type()
	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("default")
		if tag == "" {
			continue
		}
		f := v.Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString(tag)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(tag, 10, 64)
			if err != nil {
				panic(fmt.Sprintf("config: bad default %q for int field %s: %v", tag, t.Field(i).Name, err))
			}
			f.SetInt(n)
		case reflect.Bool:
			b, err := strconv.ParseBool(tag)
			if err != nil {
				panic(fmt.Sprintf("config: bad default %q for bool field %s: %v", tag, t.Field(i).Name, err))
			}
			f.SetBool(b)
		}
	}
	return s
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
