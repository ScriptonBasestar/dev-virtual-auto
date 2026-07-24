package lifecycle

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestResolvePlanBasic(t *testing.T) {
	cfg := &config.Config{
		Vars: map[string]string{"GLOBAL": "val"},
		Stack: map[string]*config.LifecycleEntry{
			"db": {
				Name:          "db",
				Plugin:        "compose",
				DefaultRunner: "compose",
				Runners: map[string]any{
					"compose": &config.ComposePluginConfig{Files: []string{"docker-compose.yml"}},
				},
				Compose: &config.ComposePluginConfig{Files: []string{"docker-compose.yml"}},
			},
			"api": {
				Name:          "api",
				Plugin:        "process",
				DefaultRunner: "native",
				Runners: map[string]any{
					"native": &config.NativeRunnerConfig{Run: "go run ./cmd/api"},
				},
				Process: &config.ProcessPluginConfig{Command: "go run ./cmd/api"},
			},
		},
		Plans: map[string]*config.PlanConfig{
			"local-dev": {
				Description:  "local dev",
				Environment:  "dev",
				Site:         "local",
				EndpointTags: []string{"app", "ui"},
				Vars:         map[string]string{"LOG_LEVEL": "debug"},
				Entries: []config.PlanEntry{
					{Name: "db", Runner: "compose", Order: 10},
					{Name: "api", Runner: "native", Order: 20, DependsOn: []string{"db"}},
				},
			},
		},
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Environment: map[string]string{"APP_ENV": "dev"}},
		},
		Sites: map[string]*config.SiteConfig{
			"local": {Vars: map[string]string{"DVA_SITE": "local"}},
		},
	}

	plan, err := ResolvePlan(cfg, "local-dev", nil)
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	if plan.Name != "local-dev" {
		t.Errorf("plan name: got %s", plan.Name)
	}
	if plan.EnvironmentName != "dev" {
		t.Errorf("env name: got %s", plan.EnvironmentName)
	}
	if plan.SiteName != "local" {
		t.Errorf("site name: got %s", plan.SiteName)
	}
	if len(plan.EndpointTags) != 2 || plan.EndpointTags[0] != "app" || plan.EndpointTags[1] != "ui" {
		t.Errorf("endpoint tags: got %v", plan.EndpointTags)
	}
	if len(plan.Entries) != 2 {
		t.Fatalf("entries: got %d", len(plan.Entries))
	}

	if plan.Entries[0].Wave != 0 {
		t.Errorf("db should be wave 0, got %d", plan.Entries[0].Wave)
	}
	if plan.Entries[1].Wave != 1 {
		t.Errorf("api should be wave 1, got %d", plan.Entries[1].Wave)
	}

	if plan.EnvVars["GLOBAL"] != "val" {
		t.Error("global vars missing")
	}
	if plan.EnvVars["APP_ENV"] != "dev" {
		t.Error("environment vars missing")
	}
	if plan.EnvVars["DVA_SITE"] != "local" {
		t.Error("site vars missing")
	}
	if plan.EnvVars["LOG_LEVEL"] != "debug" {
		t.Error("plan vars missing")
	}
}

func TestResolvePlanNotFound(t *testing.T) {
	cfg := &config.Config{Plans: map[string]*config.PlanConfig{}}
	_, err := ResolvePlan(cfg, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestResolvePlanMissingStackRef(t *testing.T) {
	cfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{},
		Plans: map[string]*config.PlanConfig{
			"dev": {Entries: []config.PlanEntry{{Name: "nonexistent"}}},
		},
	}
	_, err := ResolvePlan(cfg, "dev", nil)
	if err == nil {
		t.Fatal("expected error for missing stack ref")
	}
}

func TestResolvePlanCycleDetection(t *testing.T) {
	entries := []ResolvedEntry{
		{Name: "a", DependsOn: []string{"b"}},
		{Name: "b", DependsOn: []string{"a"}},
	}

	err := CalculateWaves(entries)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestResolvePlanDefaultRunner(t *testing.T) {
	cfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"db": {
				Name:          "db",
				DefaultRunner: "compose",
				Runners: map[string]any{
					"compose": &config.ComposePluginConfig{Files: []string{"dc.yml"}},
				},
				Compose: &config.ComposePluginConfig{Files: []string{"dc.yml"}},
			},
		},
		Plans: map[string]*config.PlanConfig{
			"dev": {Entries: []config.PlanEntry{{Name: "db"}}},
		},
	}

	plan, err := ResolvePlan(cfg, "dev", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Entries[0].Runner != "compose" {
		t.Errorf("runner should be compose (default), got %s", plan.Entries[0].Runner)
	}
}
