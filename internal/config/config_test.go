// ABOUTME: Tests for pulse configuration loading and path expansion.
// ABOUTME: Covers YAML parsing, defaults, path expansion, and remote detection.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"tilde only", "~", home},
		{"tilde slash", "~/foo/bar", filepath.Join(home, "foo", "bar")},
		{"absolute", "/tmp/foo", "/tmp/foo"},
		{"relative", "foo/bar", "foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("ExpandPath(%q) error: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLoadDefaultConfig(t *testing.T) {
	// Set config path to a non-existent location
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Social.APIKey != "" {
		t.Error("expected empty api_key in default config")
	}
	if cfg.Social.TeamID != "" {
		t.Error("expected empty team_id in default config")
	}
	if cfg.HasRemote() {
		t.Error("expected HasRemote() to be false for default config")
	}
}

func TestLoadYAMLConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "pulse")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configData := `social:
  api_key: "test-key"
  team_id: "test-team"
  api_url: "https://api.example.com"
journal:
  project_path: "~/my-journal"
  user_path: "~/global-journal"
`
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configData), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Social.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got %q", cfg.Social.APIKey)
	}
	if cfg.Social.TeamID != "test-team" {
		t.Errorf("expected team_id 'test-team', got %q", cfg.Social.TeamID)
	}
	if cfg.Social.APIURL != "https://api.example.com" {
		t.Errorf("expected api_url 'https://api.example.com', got %q", cfg.Social.APIURL)
	}
	if !cfg.HasRemote() {
		t.Error("expected HasRemote() to be true")
	}

	home, _ := os.UserHomeDir()
	expectedProject := filepath.Join(home, "my-journal")
	if got, err := cfg.GetJournalProjectPath(); err != nil {
		t.Fatalf("GetJournalProjectPath() error: %v", err)
	} else if got != expectedProject {
		t.Errorf("GetJournalProjectPath() = %q, want %q", got, expectedProject)
	}

	expectedUser := filepath.Join(home, "global-journal")
	if got, err := cfg.GetJournalUserPath(); err != nil {
		t.Fatalf("GetJournalUserPath() error: %v", err)
	} else if got != expectedUser {
		t.Errorf("GetJournalUserPath() = %q, want %q", got, expectedUser)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Social: SocialConfig{
			APIKey: "saved-key",
			TeamID: "saved-team",
			APIURL: "https://saved.example.com",
		},
		Journal: JournalConfig{
			UserPath: "~/saved-journal",
		},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Social.APIKey != "saved-key" {
		t.Errorf("expected api_key 'saved-key', got %q", loaded.Social.APIKey)
	}
	if loaded.Social.TeamID != "saved-team" {
		t.Errorf("expected team_id 'saved-team', got %q", loaded.Social.TeamID)
	}
}

func TestHasRemotePartial(t *testing.T) {
	cfg := &Config{
		Social: SocialConfig{
			APIKey: "key",
			// missing TeamID and APIURL
		},
	}
	if cfg.HasRemote() {
		t.Error("HasRemote() should be false when team_id and api_url are empty")
	}
}

func TestDefaultPaths(t *testing.T) {
	cfg := &Config{}
	home, _ := os.UserHomeDir()

	userPath, err := cfg.GetJournalUserPath()
	if err != nil {
		t.Fatalf("GetJournalUserPath() error: %v", err)
	}
	expected := filepath.Join(home, ".private-journal")
	if userPath != expected {
		t.Errorf("GetJournalUserPath() = %q, want %q", userPath, expected)
	}

	// Project path uses cwd
	projectPath, err := cfg.GetJournalProjectPath()
	if err != nil {
		t.Fatalf("GetJournalProjectPath() error: %v", err)
	}
	cwd, _ := os.Getwd()
	expectedProject := filepath.Join(cwd, ".private-journal")
	if projectPath != expectedProject {
		t.Errorf("GetJournalProjectPath() = %q, want %q", projectPath, expectedProject)
	}
}
