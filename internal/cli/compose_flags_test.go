package cli

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestParseDvaFlags_ModeOnly(t *testing.T) {
	mode, env, _, excludeTags, filtered := parseDvaFlags([]string{"--mode", "native", "postgres"})
	if mode != "native" {
		t.Errorf("mode = %q, want %q", mode, "native")
	}
	if env != "" {
		t.Errorf("env = %q, want empty", env)
	}
	if len(excludeTags) != 0 {
		t.Errorf("excludeTags = %v, want empty", excludeTags)
	}
	if len(filtered) != 1 || filtered[0] != "postgres" {
		t.Errorf("filtered = %v, want [postgres]", filtered)
	}
}

func TestParseDvaFlags_EnvOnly(t *testing.T) {
	mode, env, _, _, filtered := parseDvaFlags([]string{"--env", "stg", "-d"})
	if mode != "" {
		t.Errorf("mode = %q, want empty", mode)
	}
	if env != "stg" {
		t.Errorf("env = %q, want %q", env, "stg")
	}
	if len(filtered) != 1 || filtered[0] != "-d" {
		t.Errorf("filtered = %v, want [-d]", filtered)
	}
}

func TestParseDvaFlags_BothFlags(t *testing.T) {
	mode, env, _, _, filtered := parseDvaFlags([]string{"-M", "docker", "-E", "prd", "--wait"})
	if mode != "docker" {
		t.Errorf("mode = %q, want %q", mode, "docker")
	}
	if env != "prd" {
		t.Errorf("env = %q, want %q", env, "prd")
	}
	if len(filtered) != 1 || filtered[0] != "--wait" {
		t.Errorf("filtered = %v, want [--wait]", filtered)
	}
}

func TestParseDvaFlags_EqualsSyntax(t *testing.T) {
	mode, env, _, _, filtered := parseDvaFlags([]string{"--mode=hybrid", "--env=stg"})
	if mode != "hybrid" {
		t.Errorf("mode = %q, want %q", mode, "hybrid")
	}
	if env != "stg" {
		t.Errorf("env = %q, want %q", env, "stg")
	}
	if len(filtered) != 0 {
		t.Errorf("filtered = %v, want empty", filtered)
	}
}

func TestParseDvaFlags_ShortEqualsSyntax(t *testing.T) {
	mode, env, _, _, _ := parseDvaFlags([]string{"-M=native", "-E=prd"})
	if mode != "native" {
		t.Errorf("mode = %q, want %q", mode, "native")
	}
	if env != "prd" {
		t.Errorf("env = %q, want %q", env, "prd")
	}
}

func TestParseDvaFlags_Empty(t *testing.T) {
	mode, env, _, excludeTags, filtered := parseDvaFlags(nil)
	if mode != "" || env != "" {
		t.Errorf("got mode=%q env=%q, want both empty", mode, env)
	}
	if excludeTags != nil {
		t.Errorf("excludeTags = %v, want nil", excludeTags)
	}
	if filtered != nil {
		t.Errorf("filtered = %v, want nil", filtered)
	}
}

func TestParseDvaFlags_MissingValue(t *testing.T) {
	// --mode at end with no value — should not panic
	mode, _, _, _, filtered := parseDvaFlags([]string{"--mode"})
	if mode != "" {
		t.Errorf("mode = %q, want empty (no value provided)", mode)
	}
	if len(filtered) != 0 {
		t.Errorf("filtered = %v, want empty", filtered)
	}
}

func TestParseDvaFlags_ExcludeTags(t *testing.T) {
	_, _, _, excludeTags, filtered := parseDvaFlags([]string{"--exclude-tags", "infra", "-d"})
	if len(excludeTags) != 1 || excludeTags[0] != "infra" {
		t.Errorf("excludeTags = %v, want [infra]", excludeTags)
	}
	if len(filtered) != 1 || filtered[0] != "-d" {
		t.Errorf("filtered = %v, want [-d]", filtered)
	}
}

func TestParseDvaFlags_ExcludeTagsCommaSeparated(t *testing.T) {
	_, _, _, excludeTags, _ := parseDvaFlags([]string{"--exclude-tags=infra,dev"})
	if len(excludeTags) != 2 {
		t.Fatalf("excludeTags = %v, want 2 items", excludeTags)
	}
	if excludeTags[0] != "infra" || excludeTags[1] != "dev" {
		t.Errorf("excludeTags = %v, want [infra dev]", excludeTags)
	}
}

func TestParseDvaFlags_IncludeTags(t *testing.T) {
	_, _, includeTags, _, filtered := parseDvaFlags([]string{"--tags", "backend", "-d"})
	if len(includeTags) != 1 || includeTags[0] != "backend" {
		t.Errorf("includeTags = %v, want [backend]", includeTags)
	}
	if len(filtered) != 1 || filtered[0] != "-d" {
		t.Errorf("filtered = %v, want [-d]", filtered)
	}
}

func TestParseDvaFlags_IncludeTagsCommaSeparated(t *testing.T) {
	_, _, includeTags, _, _ := parseDvaFlags([]string{"-T=backend,ui"})
	if len(includeTags) != 2 {
		t.Fatalf("includeTags = %v, want 2 items", includeTags)
	}
	if includeTags[0] != "backend" || includeTags[1] != "ui" {
		t.Errorf("includeTags = %v, want [backend ui]", includeTags)
	}
}

func TestResolveMode_Empty(t *testing.T) {
	c := &config.Config{
		HealthChecks: map[string]config.HealthCheckConfig{
			"pg": {Type: "tcp"},
		},
	}
	rm, err := resolveMode(c, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rm.Mode != nil {
		t.Error("Mode should be nil for empty mode")
	}
	if rm.SkipCompose {
		t.Error("SkipCompose should be false for empty mode")
	}
	if len(rm.HealthChecks) != 1 {
		t.Errorf("HealthChecks = %d, want 1", len(rm.HealthChecks))
	}
}

func TestResolveMode_NotFound(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"docker": {Description: "Docker mode"},
		},
	}
	_, err := resolveMode(c, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent mode")
	}
}

func TestResolveMode_NoModesDefined(t *testing.T) {
	c := &config.Config{}
	_, err := resolveMode(c, "native")
	if err == nil {
		t.Fatal("expected error when no modes defined")
	}
}

func TestResolveMode_SkipCompose(t *testing.T) {
	empty := []string{}
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"native": {
				Description:     "Native",
				ComposeServices: &empty,
			},
		},
		HealthChecks: map[string]config.HealthCheckConfig{
			"pg": {Type: "tcp"},
		},
	}
	rm, err := resolveMode(c, "native")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rm.SkipCompose {
		t.Error("SkipCompose should be true for empty compose_services")
	}
}

func TestResolveMode_ComposeProfiles(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"docker": {
				ComposeProfiles: []string{"web", "dev"},
			},
		},
	}
	rm, err := resolveMode(c, "docker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"--profile", "web", "--profile", "dev"}
	if len(rm.ComposeArgs) != len(want) {
		t.Fatalf("ComposeArgs = %v, want %v", rm.ComposeArgs, want)
	}
	for i, v := range want {
		if rm.ComposeArgs[i] != v {
			t.Errorf("ComposeArgs[%d] = %q, want %q", i, rm.ComposeArgs[i], v)
		}
	}
}

func TestResolveMode_HealthCheckFilter(t *testing.T) {
	c := &config.Config{
		Modes: map[string]config.ModeConfig{
			"native": {
				HealthChecks: []string{"pg"},
			},
		},
		HealthChecks: map[string]config.HealthCheckConfig{
			"pg":    {Type: "tcp"},
			"redis": {Type: "tcp"},
		},
	}
	rm, err := resolveMode(c, "native")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rm.HealthChecks) != 1 {
		t.Errorf("HealthChecks count = %d, want 1", len(rm.HealthChecks))
	}
	if _, ok := rm.HealthChecks["pg"]; !ok {
		t.Error("HealthChecks should contain 'pg'")
	}
}

func TestApplyEnv_Empty(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{}
	if err := applyEnv(e, c, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyEnv_NotFound(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Description: "Dev"},
		},
	}
	if err := applyEnv(e, c, "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent env")
	}
}

func TestApplyEnv_NoEnvironmentsDefined(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{}
	if err := applyEnv(e, c, "stg"); err == nil {
		t.Fatal("expected error when no environments defined")
	}
}

func TestApplyEnv_MergesVars(t *testing.T) {
	e := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := &config.Config{
		Environments: map[string]config.EnvironmentProfile{
			"stg": {
				Description: "Staging",
				Environment: map[string]string{
					"RAILS_ENV": "staging",
					"DB_HOST":   "stg-db",
				},
			},
		},
	}
	if err := applyEnv(e, c, "stg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Vars["RAILS_ENV"] != "staging" {
		t.Errorf("RAILS_ENV = %q, want %q", e.Vars["RAILS_ENV"], "staging")
	}
	if e.Vars["DB_HOST"] != "stg-db" {
		t.Errorf("DB_HOST = %q, want %q", e.Vars["DB_HOST"], "stg-db")
	}
}
