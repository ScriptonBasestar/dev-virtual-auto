package cli

import (
	"os"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestIsFlag(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"--debug", true},
		{"-d", true},
		{"run", false},
		{"", false},
		{"up", false},
		{"-", true},
	}
	for _, tt := range tests {
		if got := isFlag(tt.input); got != tt.want {
			t.Errorf("isFlag(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsTopLevelCommand(t *testing.T) {
	// Known built-in commands should return true
	builtins := []string{"run", "up", "down", "stop", "status", "show", "init", "version", "compose", config.LogsDirName, "build", "restart", "clean", "validate", "manifest", "provision", "ssh", "console", "ls"}
	for _, cmd := range builtins {
		if !isTopLevelCommand(cmd) {
			t.Errorf("isTopLevelCommand(%q) = false, want true", cmd)
		}
	}

	// Unknown commands should return false
	unknowns := []string{"mycommand", "test", "deploy", "foo", "migrate", "cmd"}
	for _, cmd := range unknowns {
		if isTopLevelCommand(cmd) {
			t.Errorf("isTopLevelCommand(%q) = true, want false", cmd)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"up", "up", 0},
		{"up", "pu", 2},
		{"run", "rn", 1},
		{"status", "stats", 1},
		{"abc", "def", 3},
	}
	for _, tt := range tests {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggestCommands_KnownSimilar(t *testing.T) {
	// "urn" is 2 edits from "run"
	suggestions := suggestCommands("urn")
	found := false
	for _, s := range suggestions {
		if s == "run" {
			found = true
		}
	}
	if !found {
		t.Errorf("suggestCommands(%q) = %v, want to contain 'run'", "urn", suggestions)
	}
}

func TestSuggestCommands_NoMatch(t *testing.T) {
	suggestions := suggestCommands("zzzzzzzzz")
	if len(suggestions) != 0 {
		t.Errorf("suggestCommands(%q) = %v, want empty", "zzzzzzzzz", suggestions)
	}
}

func TestLoadConfig_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Reset global
	oldCfg := cfg
	cfg = nil
	defer func() { cfg = oldCfg }()

	os.WriteFile(config.FileName, []byte("version: \"0.1.22\"\n"), 0644)

	c, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil config")
	}

	// Second call should return cached config
	c2, err := loadConfig()
	if err != nil {
		t.Fatalf("second loadConfig error: %v", err)
	}
	if c2 != c {
		t.Error("second call should return same cached config")
	}
}

func TestLoadConfig_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	oldCfg := cfg
	cfg = nil
	defer func() { cfg = oldCfg }()

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error when no dva.yml")
	}
}

func TestLoadEnv(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	oldCfg := cfg
	oldEnv := env
	cfg = nil
	env = nil
	defer func() { cfg = oldCfg; env = oldEnv }()

	os.WriteFile(config.FileName, []byte("version: \"0.1.22\"\nenvironment:\n  APP_ENV: dev\n"), 0644)

	c, _ := loadConfig()
	e := loadEnv(c)
	if e == nil {
		t.Fatal("expected non-nil environment")
	}
	if e.Vars["APP_ENV"] != "dev" {
		t.Errorf("APP_ENV = %q, want 'dev'", e.Vars["APP_ENV"])
	}

	// Second call should return cached env
	e2 := loadEnv(c)
	if e2 != e {
		t.Error("second call should return same cached env")
	}
}

func TestIsTerminal(t *testing.T) {
	// A pipe is not a terminal
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()
	if isTerminal(r) {
		t.Error("pipe should not be a terminal")
	}
}

func TestSuggestCommands_ExactMatch(t *testing.T) {
	suggestions := suggestCommands("up")
	found := false
	for _, s := range suggestions {
		if s == "up" {
			found = true
		}
	}
	if !found {
		t.Errorf("suggestCommands(%q) should include 'up'", "up")
	}
}
