package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestDetectPlanRoute_IgnoresRootPersistentFlags(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{
		"p1": {},
		"p2": {},
	}}

	tests := [][]string{
		{"--debug", "p1", "--dry-run"},
		{"--json", "--debug", "p1", "--dry-run"},
		{"p1", "--json", "--dry-run"},
	}
	for _, args := range tests {
		name, extra, ok := detectPlanRoute(c, args)
		if !ok || name != "p1" || strings.Join(extra, " ") != "--dry-run" {
			t.Fatalf("detectPlanRoute(%v) = (%q, %v, %v), want (p1, [--dry-run], true)", args, name, extra, ok)
		}
	}
}

func TestRequirePlanSelection_IgnoresRootPersistentFlags(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{
		"p1": {},
		"p2": {},
	}}

	err := requirePlanSelection(c, "up", []string{"--debug", "--json"})
	if err == nil {
		t.Fatal("root persistent flags must not turn bare multi-plan up into a whole-stack lifecycle")
	}
	if got := err.Error(); !strings.Contains(got, "dva up <p1|p2>") {
		t.Fatalf("error = %q, want sorted explicit-plan hint", got)
	}
}
