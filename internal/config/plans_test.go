package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPlanConfigParsing(t *testing.T) {
	yamlStr := `
description: local dev
environment: dev
site: local
endpoint_tags: [app, ui]
vars:
  LOG_LEVEL: debug
entries:
  - name: core-compose
    runner: compose
    order: 10
    services: [postgres, redis]
  - name: api
    runner: native
    order: 20
    depends_on: [core-compose]
`

	var plan PlanConfig
	if err := yaml.Unmarshal([]byte(yamlStr), &plan); err != nil {
		t.Fatalf("failed to parse PlanConfig: %v", err)
	}

	if plan.Description != "local dev" {
		t.Errorf("description mismatch")
	}
	if plan.Environment != "dev" {
		t.Errorf("environment mismatch")
	}
	if plan.Site != "local" {
		t.Errorf("site mismatch")
	}
	if len(plan.EndpointTags) != 2 || plan.EndpointTags[0] != "app" || plan.EndpointTags[1] != "ui" {
		t.Errorf("endpoint_tags mismatch: %v", plan.EndpointTags)
	}
	if plan.Vars["LOG_LEVEL"] != "debug" {
		t.Errorf("vars mismatch")
	}
	if len(plan.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(plan.Entries))
	}
	if plan.Entries[0].Name != "core-compose" {
		t.Errorf("entry 0 name mismatch")
	}
	if plan.Entries[0].Runner != "compose" {
		t.Errorf("entry 0 runner mismatch")
	}
	if plan.Entries[0].Order != 10 {
		t.Errorf("entry 0 order mismatch")
	}
	if len(plan.Entries[0].Services) != 2 {
		t.Errorf("entry 0 services count mismatch")
	}
	if plan.Entries[1].Name != "api" {
		t.Errorf("entry 1 name mismatch")
	}
	if len(plan.Entries[1].DependsOn) != 1 || plan.Entries[1].DependsOn[0] != "core-compose" {
		t.Errorf("entry 1 depends_on mismatch")
	}
}

func TestSiteConfigParsing(t *testing.T) {
	yamlStr := `
description: local dev site
vars:
  DVA_SITE: local
entry_overrides:
  api:
    runner: docker
    vars:
      API_URL: localhost
`

	var site SiteConfig
	if err := yaml.Unmarshal([]byte(yamlStr), &site); err != nil {
		t.Fatalf("failed to parse SiteConfig: %v", err)
	}

	if site.Description != "local dev site" {
		t.Errorf("description mismatch")
	}
	if site.Vars["DVA_SITE"] != "local" {
		t.Errorf("vars mismatch")
	}
	if len(site.EntryOverrides) != 1 {
		t.Errorf("entry_overrides count mismatch")
	}
	override := site.EntryOverrides["api"]
	if override.Runner != "docker" {
		t.Errorf("override runner mismatch")
	}
	if override.Vars["API_URL"] != "localhost" {
		t.Errorf("override vars mismatch")
	}
}

func TestLifecycleEntryWithRunners(t *testing.T) {
	yamlStr := `
description: backend api
default_runner: native
runners:
  native:
    dir: apps/api
    run: go run ./cmd/api
  docker:
    image: myorg/api:dev
`

	var entry LifecycleEntry
	if err := yaml.Unmarshal([]byte(yamlStr), &entry); err != nil {
		t.Fatalf("failed to parse LifecycleEntry: %v", err)
	}

	if entry.Description != "backend api" {
		t.Errorf("description mismatch")
	}
	if entry.DefaultRunner != "native" {
		t.Errorf("default_runner mismatch")
	}
	if len(entry.Runners) != 2 {
		t.Errorf("expected 2 runners, got %d", len(entry.Runners))
	}
}

func TestSubprojectImportEntryParsing(t *testing.T) {
	var entry SubprojectImportEntry

	if err := yaml.Unmarshal([]byte(`"local-dev"`), &entry); err != nil {
		t.Fatalf("string form parse error: %v", err)
	}
	if entry.Name != "local-dev" || entry.As != "" {
		t.Errorf("string form: name=%s as=%s", entry.Name, entry.As)
	}

	obj := `
name: local-dev
as: dev
`
	if err := yaml.Unmarshal([]byte(obj), &entry); err != nil {
		t.Fatalf("object form parse error: %v", err)
	}
	if entry.Name != "local-dev" || entry.As != "dev" {
		t.Errorf("object form: name=%s as=%s", entry.Name, entry.As)
	}
}

func TestConfigPlanHelpers(t *testing.T) {
	cfg := &Config{}
	if cfg.HasPlans() {
		t.Error("empty config should not have plans")
	}
	if cfg.DefaultPlan() != "" {
		t.Error("empty config should have no default plan")
	}
	if source := cfg.DefaultPlanSource(); source != "none" {
		t.Errorf("empty config default plan source = %q, want none", source)
	}

	cfg.Plans = map[string]*PlanConfig{
		"local-dev": {Description: "local dev"},
	}
	if !cfg.HasPlans() {
		t.Error("config with 1 plan should have plans")
	}
	if dp := cfg.DefaultPlan(); dp != "local-dev" {
		t.Errorf("default plan should be 'local-dev', got '%s'", dp)
	}
	if source := cfg.DefaultPlanSource(); source != "implicit-single" {
		t.Errorf("single plan source = %q, want implicit-single", source)
	}

	cfg.Plans["stg"] = &PlanConfig{Description: "staging"}
	if dp := cfg.DefaultPlan(); dp != "" {
		t.Errorf("2 plans should return empty default, got '%s'", dp)
	}
	if source := cfg.DefaultPlanSource(); source != "none" {
		t.Errorf("multiple-plan source = %q, want none", source)
	}

	// Explicit default_plan selects among multiple plans.
	cfg.DefaultPlanName = "stg"
	if dp := cfg.DefaultPlan(); dp != "stg" {
		t.Errorf("explicit default_plan should win, got '%s'", dp)
	}
	if source := cfg.DefaultPlanSource(); source != "explicit" {
		t.Errorf("explicit default plan source = %q, want explicit", source)
	}

	// An explicit default_plan that names no existing plan resolves to empty.
	cfg.DefaultPlanName = "ghost"
	if dp := cfg.DefaultPlan(); dp != "" {
		t.Errorf("unknown default_plan should resolve to empty, got '%s'", dp)
	}
	if source := cfg.DefaultPlanSource(); source != "none" {
		t.Errorf("invalid explicit default plan source = %q, want none", source)
	}
}
