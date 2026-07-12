package config

import (
	"sort"
	"testing"
)

func TestConfigHasTag(t *testing.T) {
	cfg := &Config{
		Stack: map[string]*LifecycleEntry{
			"compose": {
				Order:   10,
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
		Stack: map[string]*LifecycleEntry{
			"compose": {
				Order: 10,
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
		Stack: map[string]*LifecycleEntry{
			"compose": {
				Order: 10,
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
		Stack: map[string]*LifecycleEntry{
			"compose": {
				Order: 10,
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
