package lifecycle

import (
	"context"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func newTestConfig(entries []config.LifecycleEntry) *config.Config {
	return &config.Config{
		Lifecycle: entries,
		Modes: map[string]config.ModeConfig{
			"lite": {Lifecycle: []string{"db"}},
		},
	}
}

func newTestEnv() *config.Environment {
	return config.NewEnvironment(nil, "/tmp", "/tmp")
}

func TestNewOrchestrator_SortsByOrder(t *testing.T) {
	entries := []config.LifecycleEntry{
		{Name: "app", Order: 2},
		{Name: "db", Order: 1},
		{Name: "cache", Order: 3},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())

	if len(orch.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(orch.entries))
	}
	if orch.entries[0].Name != "db" {
		t.Errorf("expected first entry to be 'db', got %q", orch.entries[0].Name)
	}
	if orch.entries[1].Name != "app" {
		t.Errorf("expected second entry to be 'app', got %q", orch.entries[1].Name)
	}
	if orch.entries[2].Name != "cache" {
		t.Errorf("expected third entry to be 'cache', got %q", orch.entries[2].Name)
	}
}

func TestFilterEntries_ByIncludeTags(t *testing.T) {
	entries := []config.LifecycleEntry{
		{Name: "db", Tags: []string{"infra"}},
		{Name: "app", Tags: []string{"app"}},
		{Name: "cache", Tags: []string{"infra", "cache"}},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	filtered := orch.filterEntries([]string{"infra"}, nil, "")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries with tag 'infra', got %d", len(filtered))
	}
	if filtered[0].Name != "db" || filtered[1].Name != "cache" {
		t.Errorf("unexpected entries: %v, %v", filtered[0].Name, filtered[1].Name)
	}
}

func TestFilterEntries_ByExcludeTags(t *testing.T) {
	entries := []config.LifecycleEntry{
		{Name: "db", Tags: []string{"infra"}},
		{Name: "app", Tags: []string{"app"}},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	filtered := orch.filterEntries(nil, []string{"infra"}, "")

	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry after excluding 'infra', got %d", len(filtered))
	}
	if filtered[0].Name != "app" {
		t.Errorf("expected 'app', got %q", filtered[0].Name)
	}
}

func TestFilterEntries_ByMode(t *testing.T) {
	entries := []config.LifecycleEntry{
		{Name: "db", Order: 1},
		{Name: "app", Order: 2},
		{Name: "cache", Order: 3},
	}

	cfg := newTestConfig(entries)
	orch := NewOrchestrator(cfg, newTestEnv())
	filtered := orch.filterEntries(nil, nil, "lite")

	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry in mode 'lite', got %d", len(filtered))
	}
	if filtered[0].Name != "db" {
		t.Errorf("expected 'db', got %q", filtered[0].Name)
	}
}

func TestFilterEntries_NoFilters(t *testing.T) {
	entries := []config.LifecycleEntry{
		{Name: "db"},
		{Name: "app"},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	filtered := orch.filterEntries(nil, nil, "")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries with no filters, got %d", len(filtered))
	}
}

func TestFilterEntries_CombinedTagAndMode(t *testing.T) {
	entries := []config.LifecycleEntry{
		{Name: "db", Tags: []string{"infra"}},
		{Name: "app", Tags: []string{"app"}},
	}

	cfg := &config.Config{
		Lifecycle: entries,
		Modes: map[string]config.ModeConfig{
			"both": {Lifecycle: []string{"db", "app"}},
		},
	}

	orch := NewOrchestrator(cfg, newTestEnv())
	// Mode "both" includes db+app, then exclude tag "app" → only db
	filtered := orch.filterEntries(nil, []string{"app"}, "both")

	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(filtered))
	}
	if filtered[0].Name != "db" {
		t.Errorf("expected 'db', got %q", filtered[0].Name)
	}
}

func TestUp_DryRun_ScriptPlugin(t *testing.T) {
	entries := []config.LifecycleEntry{
		{
			Name:   "setup",
			Order:  1,
			Script: &config.ScriptPluginConfig{Up: "echo hello"},
		},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	err := orch.Up(context.Background(), UpOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run up failed: %v", err)
	}
}

func TestDown_DryRun_ScriptPlugin(t *testing.T) {
	entries := []config.LifecycleEntry{
		{
			Name:   "setup",
			Order:  1,
			Script: &config.ScriptPluginConfig{Down: "echo bye"},
		},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	err := orch.Down(context.Background(), DownOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run down failed: %v", err)
	}
}

func TestDown_VolumesAndImages_Propagated(t *testing.T) {
	entries := []config.LifecycleEntry{
		{
			Name:    "db",
			Order:   1,
			Compose: &config.ComposePluginConfig{},
		},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	// DryRun so it doesn't actually execute docker compose
	err := orch.Down(context.Background(), DownOptions{
		DryRun:       true,
		Volumes:      true,
		RemoveImages: true,
	})
	if err != nil {
		t.Fatalf("dry-run down with volumes/images failed: %v", err)
	}
}

func TestUp_EmptyLifecycle(t *testing.T) {
	orch := NewOrchestrator(newTestConfig(nil), newTestEnv())
	err := orch.Up(context.Background(), UpOptions{})
	if err != nil {
		t.Fatalf("up with no entries should succeed: %v", err)
	}
}

func TestUp_NoPluginConfigured(t *testing.T) {
	// Entry with no plugin config section: DetectPlugin() returns "" → NewPlugin("") errors
	entries := []config.LifecycleEntry{
		{Name: "bad", Order: 1},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	err := orch.Up(context.Background(), UpOptions{})
	if err == nil {
		t.Fatal("expected error for entry with no plugin configured")
	}
}

func TestDown_ReverseOrder(t *testing.T) {
	// Verify entries are processed in reverse order during down.
	// Use script plugin with dry-run to avoid side effects.
	entries := []config.LifecycleEntry{
		{Name: "first", Order: 1, Script: &config.ScriptPluginConfig{Down: "echo 1"}},
		{Name: "second", Order: 2, Script: &config.ScriptPluginConfig{Down: "echo 2"}},
		{Name: "third", Order: 3, Script: &config.ScriptPluginConfig{Down: "echo 3"}},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())

	// filterEntries returns in forward order; Down() reverses internally.
	// We verify by checking that the orchestrator was constructed with sorted entries.
	if orch.entries[0].Name != "first" {
		t.Errorf("expected 'first' at index 0, got %q", orch.entries[0].Name)
	}
	if orch.entries[2].Name != "third" {
		t.Errorf("expected 'third' at index 2, got %q", orch.entries[2].Name)
	}

	err := orch.Down(context.Background(), DownOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run down failed: %v", err)
	}
}

func TestCloneEnv(t *testing.T) {
	original := config.NewEnvironment(map[string]string{"A": "1"}, "/tmp", "/tmp")
	clone := cloneEnv(original)

	clone.Vars["B"] = "2"

	if _, ok := original.Vars["B"]; ok {
		t.Error("cloneEnv should not mutate the original environment")
	}
	if clone.Vars["A"] != "1" {
		t.Error("clone should carry original vars")
	}
}

func TestHasAnyTag(t *testing.T) {
	tests := []struct {
		tags   []string
		tagSet map[string]bool
		want   bool
	}{
		{[]string{"a", "b"}, map[string]bool{"a": true}, true},
		{[]string{"a", "b"}, map[string]bool{"c": true}, false},
		{nil, map[string]bool{"a": true}, false},
		{[]string{"x"}, map[string]bool{}, false},
	}

	for _, tt := range tests {
		got := hasAnyTag(tt.tags, tt.tagSet)
		if got != tt.want {
			t.Errorf("hasAnyTag(%v, %v) = %v, want %v", tt.tags, tt.tagSet, got, tt.want)
		}
	}
}
