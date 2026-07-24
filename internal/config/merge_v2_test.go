package config

import "testing"

func TestMergeVars(t *testing.T) {
	base := &Config{Vars: map[string]string{"A": "1", "B": "2"}}
	other := &Config{Vars: map[string]string{"B": "3", "C": "4"}}

	if err := base.mergeFrom(other); err != nil {
		t.Fatal(err)
	}

	if base.Vars["A"] != "1" {
		t.Error("A should be preserved")
	}
	if base.Vars["B"] != "3" {
		t.Error("B should be overridden")
	}
	if base.Vars["C"] != "4" {
		t.Error("C should be added")
	}
}

func TestMergePlans(t *testing.T) {
	base := &Config{Plans: map[string]*PlanConfig{
		"dev": {
			Description:  "dev plan",
			Environment:  "dev",
			EndpointTags: []string{"infra"},
			Vars:         map[string]string{"A": "1"},
			Entries:      []PlanEntry{{Name: "db", Order: 10}},
		},
	}}
	other := &Config{Plans: map[string]*PlanConfig{
		"dev": {
			EndpointTags: []string{"app"},
			Vars:         map[string]string{"B": "2"},
			Entries:      []PlanEntry{{Name: "api", Order: 20}},
		},
	}}

	if err := base.mergeFrom(other); err != nil {
		t.Fatal(err)
	}

	plan := base.Plans["dev"]
	if plan.Description != "dev plan" {
		t.Error("description should be preserved")
	}
	if plan.Environment != "dev" {
		t.Error("environment should be preserved")
	}
	if len(plan.EndpointTags) != 1 || plan.EndpointTags[0] != "app" {
		t.Errorf("endpoint_tags should be replaced, got %v", plan.EndpointTags)
	}
	if plan.Vars["A"] != "1" {
		t.Error("var A should be preserved")
	}
	if plan.Vars["B"] != "2" {
		t.Error("var B should be added")
	}
	if len(plan.Entries) != 1 || plan.Entries[0].Name != "api" {
		t.Error("entries should be replaced")
	}
}

func TestMergeSites(t *testing.T) {
	base := &Config{Sites: map[string]*SiteConfig{
		"local": {Description: "local", Vars: map[string]string{"A": "1"}},
	}}
	other := &Config{Sites: map[string]*SiteConfig{
		"local": {
			Vars: map[string]string{"B": "2"},
			EntryOverrides: map[string]*SiteEntryOverride{
				"api": {Runner: "docker"},
			},
		},
	}}

	if err := base.mergeFrom(other); err != nil {
		t.Fatal(err)
	}

	site := base.Sites["local"]
	if site.Description != "local" {
		t.Error("description should be preserved")
	}
	if site.Vars["A"] != "1" {
		t.Error("var A should be preserved")
	}
	if site.Vars["B"] != "2" {
		t.Error("var B should be added")
	}
	if len(site.EntryOverrides) != 1 {
		t.Error("entry_overrides should be added")
	}
}

func TestMergeLifecycleEntryRunners(t *testing.T) {
	base := &LifecycleEntry{
		Name:          "api",
		DefaultRunner: "native",
		Runners: map[string]any{
			"native": map[string]any{"dir": "apps/api", "run": "go run ./cmd/api"},
		},
	}
	other := &LifecycleEntry{Runners: map[string]any{
		"docker": map[string]any{"image": "myorg/api:dev"},
	}}

	merged, err := MergeLifecycleEntry(base, other)
	if err != nil {
		t.Fatal(err)
	}
	if merged.DefaultRunner != "native" {
		t.Error("default_runner should be preserved")
	}
	if len(merged.Runners) != 2 {
		t.Errorf("expected 2 runners, got %d", len(merged.Runners))
	}
}

func TestMergeLifecycleEntryComposeRunner(t *testing.T) {
	base := &LifecycleEntry{
		Name:          "infra",
		DefaultRunner: "compose",
		Runners: map[string]any{
			"compose": &ComposePluginConfig{
				Files:       []string{"compose.yml"},
				ProjectName: "myapp",
			},
		},
	}
	other := &LifecycleEntry{Runners: map[string]any{
		"compose": &ComposePluginConfig{
			Files: []string{"compose.yml", "compose.dev.yml"},
		},
	}}

	merged, err := MergeLifecycleEntry(base, other)
	if err != nil {
		t.Fatal(err)
	}
	composeCfg := merged.ComposeConfig()
	if composeCfg == nil {
		t.Fatal("expected compose runner config")
	}
	if composeCfg.ProjectName != "myapp" {
		t.Errorf("project_name = %q, want preserved", composeCfg.ProjectName)
	}
	if len(composeCfg.Files) != 2 || composeCfg.Files[1] != "compose.dev.yml" {
		t.Errorf("files = %v, want override files", composeCfg.Files)
	}
}
