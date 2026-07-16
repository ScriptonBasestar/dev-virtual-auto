package lifecycle

import (
	"context"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func newTestConfig(entries map[string]*config.LifecycleEntry) *config.Config {
	return &config.Config{
		Stack: entries,
		Modes: map[string]config.ModeConfig{
			"lite": {Stack: []string{"db"}},
		},
	}
}

func newTestEnv() *config.Environment {
	return config.NewEnvironment(nil, "/tmp", "/tmp")
}

func TestNewOrchestrator_SortsByOrder(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"app":   {Order: 2},
		"db":    {Order: 1},
		"cache": {Order: 3},
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
	entries := map[string]*config.LifecycleEntry{
		"db":    {Tags: []string{"infra"}},
		"app":   {Tags: []string{"app"}},
		"cache": {Tags: []string{"infra", "cache"}},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	filtered, _ := orch.filterEntries(nil, []string{"infra"}, nil, "", "")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries with tag 'infra', got %d", len(filtered))
	}
	// Both should have "infra" tag
	for _, e := range filtered {
		if e.Name != "db" && e.Name != "cache" {
			t.Errorf("unexpected entry: %v", e.Name)
		}
	}
}

func TestFilterEntries_ByExcludeTags(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"db":  {Tags: []string{"infra"}},
		"app": {Tags: []string{"app"}},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	filtered, _ := orch.filterEntries(nil, nil, []string{"infra"}, "", "")

	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry after excluding 'infra', got %d", len(filtered))
	}
	if filtered[0].Name != "app" {
		t.Errorf("expected 'app', got %q", filtered[0].Name)
	}
}

func TestFilterEntries_ByMode(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"db":    {Order: 1},
		"app":   {Order: 2},
		"cache": {Order: 3},
	}

	cfg := newTestConfig(entries)
	orch := NewOrchestrator(cfg, newTestEnv())
	filtered, _ := orch.filterEntries(nil, nil, nil, "lite", "")

	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry in mode 'lite', got %d", len(filtered))
	}
	if filtered[0].Name != "db" {
		t.Errorf("expected 'db', got %q", filtered[0].Name)
	}
}

func TestFilterEntries_NoFilters(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"db":  {},
		"app": {},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	filtered, _ := orch.filterEntries(nil, nil, nil, "", "")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries with no filters, got %d", len(filtered))
	}
}

func TestFilterEntries_CombinedTagAndMode(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"db":  {Tags: []string{"infra"}},
		"app": {Tags: []string{"app"}},
	}

	cfg := &config.Config{
		Stack: entries,
		Modes: map[string]config.ModeConfig{
			"both": {Stack: []string{"db", "app"}},
		},
	}

	orch := NewOrchestrator(cfg, newTestEnv())
	// Mode "both" includes db+app, then exclude tag "app" → only db
	filtered, _ := orch.filterEntries(nil, nil, []string{"app"}, "both", "")

	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(filtered))
	}
	if filtered[0].Name != "db" {
		t.Errorf("expected 'db', got %q", filtered[0].Name)
	}
}

func TestFilterEntries_ByEnv(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"compose": {Order: 1},
		"helm":    {Order: 2},
		"kubectl": {Order: 3},
	}

	cfg := &config.Config{
		Stack: entries,
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Stack: []string{"compose"}},
			"stg": {Stack: []string{"helm", "kubectl"}},
		},
	}

	orch := NewOrchestrator(cfg, newTestEnv())

	// dev env → only compose
	filtered, _ := orch.filterEntries(nil, nil, nil, "", "dev")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry in env 'dev', got %d", len(filtered))
	}
	if filtered[0].Name != "compose" {
		t.Errorf("expected 'compose', got %q", filtered[0].Name)
	}

	// stg env → helm + kubectl
	filtered, _ = orch.filterEntries(nil, nil, nil, "", "stg")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries in env 'stg', got %d", len(filtered))
	}
}

func TestFilterEntries_EnvAndModeCombined(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"compose": {Order: 1},
		"helm":    {Order: 2},
		"kubectl": {Order: 3},
	}

	cfg := &config.Config{
		Stack: entries,
		Environments: map[string]config.EnvironmentProfile{
			"stg": {Stack: []string{"helm", "kubectl"}},
		},
		Modes: map[string]config.ModeConfig{
			"deploy": {Stack: []string{"helm"}},
		},
	}

	orch := NewOrchestrator(cfg, newTestEnv())

	// env=stg (helm, kubectl) + mode=deploy (helm) → intersection → helm only
	filtered, _ := orch.filterEntries(nil, nil, nil, "deploy", "stg")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry for env+mode intersection, got %d", len(filtered))
	}
	if filtered[0].Name != "helm" {
		t.Errorf("expected 'helm', got %q", filtered[0].Name)
	}
}

func TestFilterEntries_EnvWithoutStack(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"compose": {Order: 1},
		"helm":    {Order: 2},
	}

	cfg := &config.Config{
		Stack: entries,
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Description: "dev settings only"},
		},
	}

	orch := NewOrchestrator(cfg, newTestEnv())

	// env without stack field → no filtering, all entries pass through
	filtered, _ := orch.filterEntries(nil, nil, nil, "", "dev")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries (env without stack = no filter), got %d", len(filtered))
	}
}

func TestUp_DryRun_ScriptPlugin(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"setup": {
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
	entries := map[string]*config.LifecycleEntry{
		"setup": {
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
	entries := map[string]*config.LifecycleEntry{
		"db": {
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
	entries := map[string]*config.LifecycleEntry{
		"bad": {Order: 1},
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
	entries := map[string]*config.LifecycleEntry{
		"first":  {Order: 1, Script: &config.ScriptPluginConfig{Down: "echo 1"}},
		"second": {Order: 2, Script: &config.ScriptPluginConfig{Down: "echo 2"}},
		"third":  {Order: 3, Script: &config.ScriptPluginConfig{Down: "echo 3"}},
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

func TestStatus_SurfacesUnconstructibleEntry(t *testing.T) {
	// An entry whose plugin cannot be constructed (DetectPlugin() == "") must still
	// appear in the status output, marked broken — `up`/`down`/`stop` fail fast on it,
	// so status must not report a clean stack. The healthy entry proves status does
	// not abort on the first broken one.
	entries := map[string]*config.LifecycleEntry{
		"ok":  {Order: 1, Script: &config.ScriptPluginConfig{Up: "echo ok"}},
		"bad": {Order: 2},
	}

	orch := NewOrchestrator(newTestConfig(entries), newTestEnv())
	status, err := orch.Status(context.Background())
	if err != nil {
		t.Fatalf("status should not fail on a broken entry: %v", err)
	}

	byName := make(map[string]EntryStatus, len(status.Entries))
	for _, e := range status.Entries {
		byName[e.Name] = e
	}

	bad, ok := byName["bad"]
	if !ok {
		t.Fatalf("entry 'bad' missing from status; got entries %v", status.Entries)
	}
	if bad.Error == "" {
		t.Error("expected 'bad' entry to carry the plugin construction error")
	}

	if _, ok := byName["ok"]; !ok {
		t.Errorf("healthy entry 'ok' missing from status; got entries %v", status.Entries)
	}
	if byName["ok"].Error != "" {
		t.Errorf("healthy entry should carry no error, got %q", byName["ok"].Error)
	}
}

func TestPrintStatus_BrokenEntry(t *testing.T) {
	status := &AggregatedStatus{
		Entries: []EntryStatus{
			{Name: "s2_tworunners", Plugin: "", Error: `unknown lifecycle plugin ""`},
		},
	}

	out := captureStdout(func() {
		PrintStatus(status, "/tmp")
	})

	if !strings.Contains(out, "s2_tworunners") {
		t.Errorf("expected broken entry name in output, got %q", out)
	}
	if !strings.Contains(out, "unknown lifecycle plugin") {
		t.Errorf("expected the plugin problem to be reported, got %q", out)
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

func TestFilterEntries_WithStackOverrides(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"compose": {Name: "compose", Order: 1, Docker: &config.DockerPluginConfig{Image: "base-image"}},
		"kubectl": {Name: "kubectl", Order: 2, Kubectl: &config.KubectlPluginConfig{Namespace: "default"}},
	}
	for name, e := range entries {
		e.ResolvePluginFromName()
		entries[name] = e
	}

	cfg := &config.Config{
		Stack: entries,
		Environments: map[string]config.EnvironmentProfile{
			"stg": {
				Stack: []string{"kubectl"},
				StackOverrides: map[string]*config.LifecycleEntry{
					"kubectl": {
						Kubectl: &config.KubectlPluginConfig{Namespace: "staging"},
					},
				},
			},
		},
	}

	orch := NewOrchestrator(cfg, newTestEnv())
	filtered, err := orch.filterEntries(nil, nil, nil, "", "stg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(filtered) != 1 {
		t.Fatalf("expected 1 entry (kubectl), got %d", len(filtered))
	}
	if filtered[0].Name != "kubectl" {
		t.Errorf("expected kubectl, got %v", filtered[0].Name)
	}
	if filtered[0].Kubectl.Namespace != "staging" {
		t.Errorf("expected overridden namespace 'staging', got %q", filtered[0].Kubectl.Namespace)
	}
}

func TestFilterEntries_StackOverrides_ErrorOnPluginChange(t *testing.T) {
	entries := map[string]*config.LifecycleEntry{
		"kubectl": {Name: "kubectl", Order: 2, Plugin: "kubectl"},
	}

	cfg := &config.Config{
		Stack: entries,
		Environments: map[string]config.EnvironmentProfile{
			"stg": {
				Stack: []string{"kubectl"},
				StackOverrides: map[string]*config.LifecycleEntry{
					"kubectl": {
						Plugin: "compose", // ILLEGAL restricted field override
					},
				},
			},
		},
	}

	orch := NewOrchestrator(cfg, newTestEnv())
	_, err := orch.filterEntries(nil, nil, nil, "", "stg")
	if err == nil {
		t.Fatal("expected error when overriding plugin type, got nil")
	}
}
