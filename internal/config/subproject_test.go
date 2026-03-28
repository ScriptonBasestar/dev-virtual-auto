package config

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestLoadSubprojects(t *testing.T) {
	parentDir := t.TempDir()

	subDir := filepath.Join(parentDir, "engine")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "dva.yml"), []byte(`
version: "0.1.0"
lifecycle:
  - name: compose
    plugin: compose
    order: 10
    compose:
      files:
        - docker-compose.yml
      project_name: engine
      tags: [app]
      services:
        postgres:
          tags: [infra]
        django-engine:
          tags: [app]
interaction:
  test:
    description: "Run tests"
    service: django-engine
    command: "pytest"
    tags: [test]
  shell:
    description: "Open shell"
    service: django-engine
    command: "bash"
`), 0644)

	subs := map[string]SubprojectConfig{
		"engine": {Path: "engine", ExcludeTags: []string{"infra"}},
	}

	result, err := LoadSubprojects(parentDir, subs)
	if err != nil {
		t.Fatalf("LoadSubprojects error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 subproject, got %d", len(result))
	}

	eng, ok := result["engine"]
	if !ok {
		t.Fatal("subproject 'engine' not found")
	}
	if eng.ComposeProjectName() != "engine" {
		t.Errorf("project_name = %s, want engine", eng.ComposeProjectName())
	}
	if len(eng.Interaction) != 2 {
		t.Errorf("interaction count = %d, want 2", len(eng.Interaction))
	}
}

func TestLoadSubprojectsMissing(t *testing.T) {
	parentDir := t.TempDir()

	subs := map[string]SubprojectConfig{
		"missing": {Path: "nonexistent"},
	}

	_, err := LoadSubprojects(parentDir, subs)
	if err == nil {
		t.Error("expected error for missing subproject, got nil")
	}
}

func TestConfigHasTag(t *testing.T) {
	cfg := &Config{
		Lifecycle: []LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &ComposePluginConfig{Tags: []string{"infra", "shared"}},
			},
		},
	}
	if !cfg.HasTag("infra") {
		t.Error("expected HasTag('infra') to be true")
	}
	if cfg.HasTag("app") {
		t.Error("expected HasTag('app') to be false")
	}
}

func TestFilterInteractions(t *testing.T) {
	cfg := &Config{
		Interaction: map[string]*InteractionCommand{
			"test":  {Description: "Run tests", Tags: []string{"test"}},
			"lint":  {Description: "Run lint", Tags: []string{"quality"}},
			"shell": {Description: "Shell", Tags: []string{}},
			"db":    {Description: "DB shell", Tags: []string{"infra"}},
		},
	}

	filtered := cfg.FilterInteractions([]string{"infra"})
	if len(filtered) != 3 {
		t.Errorf("expected 3 commands after infra exclusion, got %d", len(filtered))
	}
	if _, ok := filtered["db"]; ok {
		t.Error("'db' should be excluded with infra tag")
	}

	all := cfg.FilterInteractions(nil)
	if len(all) != 4 {
		t.Errorf("expected 4 commands with no exclusion, got %d", len(all))
	}

	multi := cfg.FilterInteractions([]string{"test", "quality"})
	if len(multi) != 2 {
		t.Errorf("expected 2 commands after multi-tag exclusion, got %d", len(multi))
	}
}

func TestGetComposeServicesExcluding(t *testing.T) {
	cfg := &Config{
		Lifecycle: []LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &ComposePluginConfig{
					Tags: []string{"app"},
					Services: map[string]ServiceTagConfig{
						"postgres":      {Tags: []string{"infra"}},
						"redis":         {Tags: []string{"infra"}},
						"django-engine": {Tags: []string{"app"}},
						"worker":        {},
					},
				},
			},
		},
	}

	included := cfg.GetComposeServicesExcluding([]string{"infra"})
	sort.Strings(included)
	if len(included) != 2 {
		t.Fatalf("expected 2 included services, got %d: %v", len(included), included)
	}
	if included[0] != "django-engine" || included[1] != "worker" {
		t.Errorf("included = %v, want [django-engine, worker]", included)
	}
}

func TestGetExcludedComposeServices(t *testing.T) {
	cfg := &Config{
		Lifecycle: []LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &ComposePluginConfig{
					Tags: []string{"app"},
					Services: map[string]ServiceTagConfig{
						"postgres":      {Tags: []string{"infra"}},
						"redis":         {Tags: []string{"infra"}},
						"django-engine": {Tags: []string{"app"}},
					},
				},
			},
		},
	}

	excluded := cfg.GetExcludedComposeServices([]string{"infra"})
	sort.Strings(excluded)
	if len(excluded) != 2 {
		t.Fatalf("expected 2 excluded services, got %d: %v", len(excluded), excluded)
	}
	if excluded[0] != "postgres" || excluded[1] != "redis" {
		t.Errorf("excluded = %v, want [postgres, redis]", excluded)
	}
}

func TestGetComposeServicesExcludingEmpty(t *testing.T) {
	cfg := &Config{}

	result := cfg.GetComposeServicesExcluding([]string{"infra"})
	if len(result) != 0 {
		t.Errorf("expected 0 results for empty services, got %d", len(result))
	}

	cfg2 := &Config{
		Lifecycle: []LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
				Compose: &ComposePluginConfig{
					Services: map[string]ServiceTagConfig{
						"app": {Tags: []string{"app"}},
					},
				},
			},
		},
	}
	result2 := cfg2.GetComposeServicesExcluding(nil)
	if len(result2) != 0 {
		t.Errorf("expected 0 results for nil exclude tags, got %d", len(result2))
	}
}

func TestLoadConfigWithSubprojects(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "sub-app")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "dva.yml"), []byte(`
version: "0.1.0"
lifecycle:
  - name: compose
    plugin: compose
    order: 10
    compose:
      files: [docker-compose.yml]
interaction:
  test:
    description: "Sub test"
    service: app
    command: "npm test"
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.yml"), []byte(`
version: "0.1.0"
lifecycle:
  - name: compose
    plugin: compose
    order: 10
    compose:
      files: [docker-compose.yml]
      tags: [infra]
subprojects:
  sub-app:
    path: sub-app
    exclude_tags: [infra]
interaction:
  shell:
    service: db
    command: "psql"
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Subprojects) != 1 {
		t.Fatalf("expected 1 subproject, got %d", len(cfg.Subprojects))
	}

	sub, ok := cfg.Subprojects["sub-app"]
	if !ok {
		t.Fatal("subproject 'sub-app' not found")
	}
	if sub.Path != "sub-app" {
		t.Errorf("path = %s, want sub-app", sub.Path)
	}
	if len(sub.ExcludeTags) != 1 || sub.ExcludeTags[0] != "infra" {
		t.Errorf("exclude_tags = %v, want [infra]", sub.ExcludeTags)
	}

	subs, err := LoadSubprojects(cfg.FileDir(), cfg.Subprojects)
	if err != nil {
		t.Fatalf("LoadSubprojects error: %v", err)
	}
	if _, ok := subs["sub-app"]; !ok {
		t.Error("sub-app not loaded")
	}
}
