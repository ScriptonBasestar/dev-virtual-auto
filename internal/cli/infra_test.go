package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-051 Phase 5: `dva infra` delegates to the stack lifecycle over
// infra-tagged entries. Location/clone logic now lives in config.SourceDir
// (covered by internal/config/source_test.go).

func infraTestConfig() *config.Config {
	return &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"redis":    {Name: "redis", Tags: []string{"infra"}},
			"postgres": {Name: "postgres", Tags: []string{"infra"}},
			"web":      {Name: "web", Tags: []string{"app"}},
		},
	}
}

func TestInfraServiceNames_SortedInfraTaggedOnly(t *testing.T) {
	got := infraServiceNames(infraTestConfig())
	want := []string{"postgres", "redis"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("infraServiceNames = %v, want %v", got, want)
	}
}

func TestResolveInfraTargets_KnownService(t *testing.T) {
	got, err := resolveInfraTargets(infraTestConfig(), []string{"postgres"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"postgres"}) {
		t.Errorf("targets = %v, want [postgres]", got)
	}
}

func TestResolveInfraTargets_EmptyMeansAll(t *testing.T) {
	got, err := resolveInfraTargets(infraTestConfig(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("targets = %v, want empty (all infra)", got)
	}
}

func TestResolveInfraTargets_RejectsUnsupportedFlags(t *testing.T) {
	_, err := resolveInfraTargets(infraTestConfig(), []string{"--force", "redis"})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v, want unsupported flag error", err)
	}
}

func TestResolveInfraTargets_UnknownServiceErrorsWithList(t *testing.T) {
	_, err := resolveInfraTargets(infraTestConfig(), []string{"nope"})
	if err == nil {
		t.Fatal("expected error for unknown infra service")
	}
	if !strings.Contains(err.Error(), "not found") || !strings.Contains(err.Error(), "postgres") {
		t.Errorf("error = %q, want 'not found' and available list", err)
	}
}

func TestResolveInfraTargets_NoInfraDefined(t *testing.T) {
	c := &config.Config{Stack: map[string]*config.LifecycleEntry{
		"web": {Name: "web", Tags: []string{"app"}},
	}}
	_, err := resolveInfraTargets(c, []string{"anything"})
	if err == nil {
		t.Fatal("expected error when no infra services are defined")
	}
}
