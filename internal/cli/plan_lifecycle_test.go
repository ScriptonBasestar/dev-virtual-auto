package cli

import (
	"os"
	"path/filepath"
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

func TestRunPlanRestartDryRunPreservesNativeProcessState(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  api:
    default_runner: native
    runners:
      native:
        run: sleep 999
plans:
  local-dev:
    entries:
      - name: api
        runner: native
`)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())
	pidDir := filepath.Join(c.FileDir(), config.DotDirName, config.PidsDirName)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(pidDir, "api.pid")
	if err := os.WriteFile(pidFile, []byte("invalid-pid"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runPlanRestart(c, e, "local-dev", []string{"--dry-run"}); err != nil {
		t.Fatalf("plan restart dry-run failed: %v", err)
	}
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("plan restart dry-run removed PID state: %v", err)
	}
}
