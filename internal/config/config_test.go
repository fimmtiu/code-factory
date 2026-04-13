package config_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/fimmtiu/code-factory/internal/config"
)

func TestDefault(t *testing.T) {
	s := config.Default()
	if s.StaleThresholdMinutes != 30 {
		t.Errorf("expected StaleThresholdMinutes=30, got %d", s.StaleThresholdMinutes)
	}
	if s.TerminalTheme != "tan" {
		t.Errorf("expected TerminalTheme=tan, got %q", s.TerminalTheme)
	}
}

func TestLoadFromEmptyDir(t *testing.T) {
	dir := t.TempDir()
	s, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load from empty dir should not error, got: %v", err)
	}
	if s.StaleThresholdMinutes != 30 {
		t.Errorf("expected default StaleThresholdMinutes=30, got %d", s.StaleThresholdMinutes)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	original := &config.Settings{StaleThresholdMinutes: 45}
	if err := config.Save(dir, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.StaleThresholdMinutes != 45 {
		t.Errorf("expected StaleThresholdMinutes=45, got %d", loaded.StaleThresholdMinutes)
	}
}

func TestLoadPartialJSON(t *testing.T) {
	dir := t.TempDir()
	// Write JSON with only stale_threshold_minutes; other fields should default.
	partial := map[string]int{"stale_threshold_minutes": 99}
	data, err := json.Marshal(partial)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if err := os.WriteFile(config.Path(dir), data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	s, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s.StaleThresholdMinutes != 99 {
		t.Errorf("expected StaleThresholdMinutes=99, got %d", s.StaleThresholdMinutes)
	}
}

func TestSaveCreatesFile(t *testing.T) {
	dir := t.TempDir()
	s := config.Default()
	if err := config.Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	path := config.Path(dir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected settings.json to exist at %s", path)
	}
}

func TestJSONTags(t *testing.T) {
	dir := t.TempDir()
	s := &config.Settings{StaleThresholdMinutes: 15}
	if err := config.Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(config.Path(dir))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if _, ok := raw["stale_threshold_minutes"]; !ok {
		t.Error("expected key stale_threshold_minutes in JSON output")
	}
	if _, ok := raw["terminal_theme"]; !ok {
		t.Error("expected key terminal_theme in JSON output")
	}
	if _, ok := raw["exit_after_minutes"]; ok {
		t.Error("unexpected key exit_after_minutes in JSON output")
	}
}
