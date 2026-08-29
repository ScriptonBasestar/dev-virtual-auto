package cli

import (
	"strings"
	"testing"
)

func TestStatusRejectsUnknownPlanName(t *testing.T) {
	useConfig(t, planStackConfig)

	err := statusCmd.RunE(statusCmd, []string{"p1-typo"})
	if err == nil {
		t.Fatal("'dva status p1-typo' returned nil; an unknown plan name must not silently report on the whole workspace")
	}
	if !strings.Contains(err.Error(), "p1-typo") {
		t.Errorf("error must name the unknown argument, got: %v", err)
	}
	if !strings.Contains(err.Error(), "p1") {
		t.Errorf("error should list the available plans, got: %v", err)
	}
}

func TestStatusAcceptsKnownPlanName(t *testing.T) {
	useConfig(t, planStackConfig)

	if err := statusCmd.RunE(statusCmd, []string{"p1"}); err != nil {
		t.Fatalf("'dva status p1' must report the real plan: %v", err)
	}
}

// The guard keys off plans being configured. Without plans a positional argument
// was never a plan name, so the whole-workspace path must stay reachable.
func TestStatusWithoutPlansKeepsWorkspacePath(t *testing.T) {
	useConfig(t, noPlanStackConfig)

	if err := statusCmd.RunE(statusCmd, []string{}); err != nil {
		t.Fatalf("'dva status' with no plans must report the workspace: %v", err)
	}
	if err := statusCmd.RunE(statusCmd, []string{"s1"}); err != nil {
		t.Fatalf("'dva status s1' with no plans defined must keep its previous behavior: %v", err)
	}
}

// status is deliberately failure-tolerant: it reports "Config: not found" and
// succeeds with no dva.yml at all. The guard sits behind the config check and
// must not disturb that.
func TestStatusWithoutConfigStaysTolerant(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg = nil
	t.Cleanup(func() { cfg = nil })

	if err := statusCmd.RunE(statusCmd, []string{}); err != nil {
		t.Fatalf("'dva status' with no config must still succeed: %v", err)
	}
}

// No args means detectPlanRoute was never given a plan name slot to read, so the
// guard must stay silent even when plans exist.
func TestStatusWithPlansAndNoArgs(t *testing.T) {
	useConfig(t, planStackConfig)

	if err := statusCmd.RunE(statusCmd, []string{}); err != nil {
		t.Fatalf("'dva status' with plans but no args must not error: %v", err)
	}
}

func TestStatusWithAmbiguousPlansFallsBackToWorkspace(t *testing.T) {
	useConfig(t, planStackConfig+`  p2:
    entries:
      - name: s2
`)

	out := captureStdout(t, func() {
		if err := statusCmd.RunE(statusCmd, []string{}); err != nil {
			t.Fatalf("'dva status' with ambiguous plans must report the workspace: %v", err)
		}
	})
	for _, entry := range []string{"[s1]", "[s2]"} {
		if !strings.Contains(out, entry) {
			t.Errorf("'dva status' output does not contain workspace entry %s:\n%s", entry, out)
		}
	}
}
