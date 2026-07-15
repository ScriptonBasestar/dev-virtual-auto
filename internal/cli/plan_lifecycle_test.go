package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestRequirePlanSelection_MultiplePlansRequireName(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{
		"full-stack": {},
		"infra":      {},
	}}

	err := requirePlanSelection(c, "up", nil)
	if err == nil {
		t.Fatal("expected multiple plans without a name to fail")
	}
	if got := err.Error(); !strings.Contains(got, "dva up <full-stack|infra>") {
		t.Fatalf("error = %q, want sorted plan hint", got)
	}
}

func TestRequirePlanSelection_AllowsSingleDefaultPlan(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"infra": {}}}
	if err := requirePlanSelection(c, "up", nil); err != nil {
		t.Fatalf("single default plan failed: %v", err)
	}
}

func TestRequirePlanSelection_AllowsLegacyOrNamedArgs(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{
		"full-stack": {},
		"infra":      {},
	}}
	for _, args := range [][]string{{"infra"}, {"--mode", "legacy"}} {
		if err := requirePlanSelection(c, "up", args); err != nil {
			t.Fatalf("args %v failed: %v", args, err)
		}
	}
}
