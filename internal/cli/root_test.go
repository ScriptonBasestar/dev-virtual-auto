package cli

import (
	"os"
	"slices"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

func TestIsFlag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"long flag", "--debug", true},
		{"short flag", "-d", true},
		{"command", "run", false},
		{"empty", "", false},
		{"built-in command", "up", false},
		{"supported lone-dash name", "-", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFlag(tt.input); got != tt.want {
				t.Errorf("isFlag(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestSugarFormAgreesWithExplicitRun verifies the routing result, not a remembered argv. Both
// interactions are declared because the regression selected the wrong one at exit 0: a test
// with only greet would prove a failure message, not that the user-named interaction runs.
func TestSugarFormAgreesWithExplicitRun(t *testing.T) {
	c := &config.Config{Interaction: map[string]*config.InteractionCommand{
		"greet": {Command: "echo RAN_GREET"},
		"-":     {Command: "echo RAN_DASH"},
	}}

	shorthand := dynamicRunArgs([]string{"greet", "-"}, c)
	explicit := []string{"run", "greet", "-"}
	if !slices.Equal(shorthand, explicit) {
		t.Fatalf("shorthand routing = %q, explicit routing = %q", shorthand, explicit)
	}

	resolve := func(args []string) *runner.ResolvedCommand {
		if len(args) < 2 || args[0] != "run" {
			t.Fatalf("routed args = %q, want explicit run form", args)
		}
		return runner.NewInteractionTree(c.Interaction).Find(args[1], args[2:]...)
	}
	sugarResolved := resolve(shorthand)
	explicitResolved := resolve(explicit)
	if sugarResolved == nil || explicitResolved == nil {
		t.Fatalf("routing reached sugar=%v explicit=%v", sugarResolved, explicitResolved)
	}
	if sugarResolved.Name != explicitResolved.Name || !slices.Equal(sugarResolved.Argv, explicitResolved.Argv) {
		t.Errorf("resolved sugar=%q argv=%q, explicit=%q argv=%q", sugarResolved.Name, sugarResolved.Argv, explicitResolved.Name, explicitResolved.Argv)
	}
}

func TestDynamicRunArgsPreservesExplicitRunOrder(t *testing.T) {
	c := &config.Config{Interaction: map[string]*config.InteractionCommand{
		"greet": {Command: "echo RAN_GREET"},
	}}

	for _, input := range [][]string{
		{"greet", "-"},
		{"greet", "--debug"},
		{"greet", "-e"},
		{"greet", "--project", "api"},
		{"greet", "--", "-M", "dev"},
	} {
		want := append([]string{"run"}, input...)
		if got := dynamicRunArgs(input, c); !slices.Equal(got, want) {
			t.Errorf("dynamicRunArgs(%q) = %q, want explicit order %q", input, got, want)
		}
	}
}

func TestIsTopLevelCommand(t *testing.T) {
	// Known built-in commands should return true
	builtins := []string{"run", "up", "down", "stop", "status", "show", "init", "version", "compose", config.LogsDirName, "build", "restart", "validate", "manifest", "provision", "ssh", "console", "skill", "ls"}
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

	// The four the restructure removed. Kept apart from the unknowns above because they are
	// not typos — each was a working command, and this predicate is what decides whether an
	// interaction key of the same name is shadowed. `clean` was in the builtins list until
	// now; the other three were never checked here at all, so nothing would have caught a
	// half-finished removal that left one of them registered.
	for _, cmd := range []string{"clean", "stack", "app", "infra"} {
		if isTopLevelCommand(cmd) {
			t.Errorf("isTopLevelCommand(%q) = true, but the command was removed; an "+
				"interaction key named %q would be shadowed by a built-in that no longer exists", cmd, cmd)
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
