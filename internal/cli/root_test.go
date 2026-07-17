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
	unknowns := []string{"mycommand", "test", "deploy", "foo"}
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

	// TestLoadEnv_IncludesGlobalVars locks Option A (TASK-019): top-level vars:
	// are injected on the run path as the lowest config layer, beneath
	// environment: and env_file.
	//
	// Given: vars, environment, and env_file all set overlapping keys
	// When:  loadEnv builds the run-path environment
	// Then:  vars are present; environment overrides vars; env_file overrides both
	func TestLoadEnv_IncludesGlobalVars(t *testing.T) {
		tmpDir := t.TempDir()
		oldDir, _ := os.Getwd()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		defer os.Chdir(oldDir)

		oldCfg := cfg
		oldEnv := env
		cfg = nil
		env = nil
		defer func() { cfg = oldCfg; env = oldEnv }()

		// Ensure OS does not pin these keys (MergeVars prefers OS when set).
		for _, k := range []string{"P_GLOBAL", "P_SHARED", "P_FROM_ENVFILE"} {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}

		if err := os.WriteFile(".env", []byte("P_SHARED=from-env-file\nP_FROM_ENVFILE=from-env-file\n"), 0o644); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		yml := "" +
			"version: \"0.1.22\"\n" +
			"vars:\n" +
			"  P_GLOBAL: from-global-vars\n" +
			"  P_SHARED: from-global-vars\n" +
			"environment:\n" +
			"  P_SHARED: from-environment\n" +
			"  P_ENV_ONLY: from-environment\n" +
			"env_file: .env\n"
		if err := os.WriteFile(config.FileName, []byte(yml), 0o644); err != nil {
			t.Fatalf("write dva.yml: %v", err)
		}

		c, err := loadConfig()
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		e := loadEnv(c)
		if e == nil {
			t.Fatal("expected non-nil environment")
		}

		if got := e.Vars["P_GLOBAL"]; got != "from-global-vars" {
			t.Errorf("P_GLOBAL = %q, want from-global-vars (vars must reach run path)", got)
		}
		if got := e.Vars["P_ENV_ONLY"]; got != "from-environment" {
			t.Errorf("P_ENV_ONLY = %q, want from-environment", got)
		}
		// environment: overrides vars for the same key
		if got := e.Vars["P_SHARED"]; got != "from-env-file" {
			t.Errorf("P_SHARED = %q, want from-env-file (env_file > environment > vars)", got)
		}
		if got := e.Vars["P_FROM_ENVFILE"]; got != "from-env-file" {
			t.Errorf("P_FROM_ENVFILE = %q, want from-env-file", got)
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
