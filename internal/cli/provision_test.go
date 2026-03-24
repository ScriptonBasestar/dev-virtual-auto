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
