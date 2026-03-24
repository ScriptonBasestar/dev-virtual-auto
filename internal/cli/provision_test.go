package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func makeProvision(profiles ...string) map[string][]config.ProvisionItem {
	m := make(map[string][]config.ProvisionItem, len(profiles))
	for _, p := range profiles {
		m[p] = []config.ProvisionItem{{Step: p + " step 1"}}
	}
	return m
}

func TestResolveProvisionProfile_DefaultExists(t *testing.T) {
	prov := makeProvision("default", "reset")
	name, steps, err := resolveProvisionProfile(prov, "", "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "default" {
		t.Errorf("name = %q, want %q", name, "default")
	}
	if len(steps) != 1 {
		t.Errorf("steps len = %d, want 1", len(steps))
	}
}

func TestResolveProvisionProfile_ExplicitProfile(t *testing.T) {
	prov := makeProvision("setup", "reset")
	name, steps, err := resolveProvisionProfile(prov, "", "setup", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "setup" {
		t.Errorf("name = %q, want %q", name, "setup")
	}
	if len(steps) != 1 {
		t.Errorf("steps len = %d, want 1", len(steps))
	}
}

func TestResolveProvisionProfile_SingleFallback(t *testing.T) {
	prov := makeProvision("setup")
	name, steps, err := resolveProvisionProfile(prov, "", "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "setup" {
		t.Errorf("name = %q, want %q", name, "setup")
	}
	if len(steps) != 1 {
		t.Errorf("steps len = %d, want 1", len(steps))
	}
}

func TestResolveProvisionProfile_ExplicitDefaultNotFound(t *testing.T) {
	prov := makeProvision("setup")
	_, _, err := resolveProvisionProfile(prov, "", "default", true)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestResolveProvisionProfile_MultipleNoDefault(t *testing.T) {
	prov := makeProvision("setup", "reset")
	_, _, err := resolveProvisionProfile(prov, "", "default", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
	if !strings.Contains(err.Error(), "setup") || !strings.Contains(err.Error(), "reset") {
		t.Errorf("error = %q, want to list available profiles", err.Error())
	}
}

func TestResolveProvisionProfile_TypoWithSuggestion(t *testing.T) {
	prov := makeProvision("setup", "reset")
	_, _, err := resolveProvisionProfile(prov, "", "seutp", true)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Did you mean") {
		t.Errorf("error = %q, want to contain 'Did you mean'", err.Error())
	}
	if !strings.Contains(err.Error(), "dva provision setup") {
		t.Errorf("error = %q, want to suggest 'dva provision setup'", err.Error())
	}
}

func TestResolveProvisionProfile_TypoNoMatch(t *testing.T) {
	prov := makeProvision("setup")
	_, _, err := resolveProvisionProfile(prov, "", "zzzzz", true)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "Did you mean") {
		t.Errorf("error = %q, should not contain 'Did you mean'", err.Error())
	}
}

func TestResolveProvisionProfile_EmptyProvision(t *testing.T) {
	prov := map[string][]config.ProvisionItem{}
	_, _, err := resolveProvisionProfile(prov, "", "default", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no provision commands defined") {
		t.Errorf("error = %q, want 'no provision commands defined'", err.Error())
	}
}

// --- default_profile tests ---

func TestResolveProvisionProfile_DefaultProfileAlias(t *testing.T) {
	prov := makeProvision("setup", "reset")
	name, steps, err := resolveProvisionProfile(prov, "setup", "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "setup" {
		t.Errorf("name = %q, want %q", name, "setup")
	}
	if len(steps) != 1 {
		t.Errorf("steps len = %d, want 1", len(steps))
	}
}

func TestResolveProvisionProfile_DefaultProfileOverridesSingleFallback(t *testing.T) {
	// With 2 profiles + default_profile set, should use default_profile (not error)
	prov := makeProvision("setup", "reset")
	name, _, err := resolveProvisionProfile(prov, "reset", "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "reset" {
		t.Errorf("name = %q, want %q", name, "reset")
	}
}

func TestResolveProvisionProfile_DefaultProfileInvalidName(t *testing.T) {
	// default_profile points to a non-existent profile → falls through to error
	// but 2 profiles exist so single-profile fallback doesn't trigger
	prov := makeProvision("setup", "reset")
	_, _, err := resolveProvisionProfile(prov, "nonexistent", "default", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestResolveProvisionProfile_DefaultProfileInvalidFallsToSingle(t *testing.T) {
	// default_profile invalid + only 1 real profile → single-profile auto kicks in
	prov := makeProvision("setup")
	name, _, err := resolveProvisionProfile(prov, "nonexistent", "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "setup" {
		t.Errorf("name = %q, want %q — single-profile fallback should apply", name, "setup")
	}
}

func TestResolveProvisionProfile_DefaultProfileIgnoredWhenExplicit(t *testing.T) {
	// User explicitly typed "dva provision default" → default_profile should NOT apply
	prov := makeProvision("setup")
	_, _, err := resolveProvisionProfile(prov, "setup", "default", true)
	if err == nil {
		t.Fatal("expected error for explicit 'default' when profile doesn't exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestResolveProvisionProfile_ExactMatchBeatsDefaultProfile(t *testing.T) {
	// "default" profile exists AND default_profile is set → exact match wins
	prov := makeProvision("default", "setup")
	name, _, err := resolveProvisionProfile(prov, "setup", "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "default" {
		t.Errorf("name = %q, want %q — exact match should win", name, "default")
	}
}

// --- parallel batch grouping tests ---

func TestGroupParallelBatches_AllSequential(t *testing.T) {
	steps := []config.ProvisionItem{
		{Step: "a"},
		{Step: "b"},
		{Step: "c"},
	}
	batches := groupParallelBatches(steps)
	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(batches))
	}
	for i, b := range batches {
		if len(b) != 1 {
			t.Errorf("batch[%d] len = %d, want 1", i, len(b))
		}
	}
}

func TestGroupParallelBatches_AllParallel(t *testing.T) {
	steps := []config.ProvisionItem{
		{Step: "a", Parallel: true},
		{Step: "b", Parallel: true},
		{Step: "c", Parallel: true},
	}
	batches := groupParallelBatches(steps)
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("batch[0] len = %d, want 3", len(batches[0]))
	}
}

func TestGroupParallelBatches_Mixed(t *testing.T) {
	steps := []config.ProvisionItem{
		{Step: "seq1"},
		{Step: "par1", Parallel: true},
		{Step: "par2", Parallel: true},
		{Step: "seq2"},
		{Step: "par3", Parallel: true},
	}
	batches := groupParallelBatches(steps)
	// Expected: [seq1], [par1, par2], [seq2], [par3]
	if len(batches) != 4 {
		t.Fatalf("batches = %d, want 4", len(batches))
	}
	if len(batches[0]) != 1 || batches[0][0].Step != "seq1" {
		t.Errorf("batch[0] = %v, want [seq1]", batches[0])
	}
	if len(batches[1]) != 2 {
		t.Errorf("batch[1] len = %d, want 2 (parallel group)", len(batches[1]))
	}
	if len(batches[2]) != 1 || batches[2][0].Step != "seq2" {
		t.Errorf("batch[2] = %v, want [seq2]", batches[2])
	}
	if len(batches[3]) != 1 || !batches[3][0].Parallel {
		t.Errorf("batch[3] should be a single parallel step")
	}
}

func TestGroupParallelBatches_Empty(t *testing.T) {
	batches := groupParallelBatches(nil)
	if len(batches) != 0 {
		t.Fatalf("batches = %d, want 0", len(batches))
	}
}

func TestGroupParallelBatches_BarrierAfterParallel(t *testing.T) {
	// Sequential step after parallel group acts as barrier
	steps := []config.ProvisionItem{
		{Step: "p1", Parallel: true},
		{Step: "p2", Parallel: true},
		{Step: "barrier"},
	}
	batches := groupParallelBatches(steps)
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("batch[0] len = %d, want 2 (parallel)", len(batches[0]))
	}
	if len(batches[1]) != 1 || batches[1][0].Step != "barrier" {
		t.Errorf("batch[1] should be barrier step")
	}
}

func TestFirstStepDescription(t *testing.T) {
	tests := []struct {
		name  string
		steps []config.ProvisionItem
		want  string
	}{
		{"empty", nil, ""},
		{"with step", []config.ProvisionItem{{Step: "Install deps"}}, "Install deps"},
		{"with echo only", []config.ProvisionItem{{Echo: "hello"}}, "hello"},
		{"no description", []config.ProvisionItem{{Cmd: "ls"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstStepDescription(tt.steps)
			if got != tt.want {
				t.Errorf("firstStepDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}
