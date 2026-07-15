package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/spf13/cobra"
)

const hookTestConfig = `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [compose.yml]
`

func TestWrapWithHooks_DryRunArgProtectsHooks(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "hook-ran")
	c := loadTestConfig(t, hookTestConfig+fmt.Sprintf(`interaction:
  up:
    before:
      - run: "touch %s"
`, marker))

	oldCfg, oldEnv, oldDryRun := cfg, env, dryRun
	cfg = c
	env = config.NewEnvironment(nil, c.FileDir(), c.FileDir())
	dryRun = false
	t.Cleanup(func() {
		cfg, env, dryRun = oldCfg, oldEnv, oldDryRun
	})

	cmd := &cobra.Command{RunE: func(_ *cobra.Command, args []string) error {
		if !dryRun {
			t.Fatal("wrapped command did not receive dry-run state")
		}
		if len(args) != 0 {
			t.Fatalf("args = %v, want --dry-run consumed", args)
		}
		return nil
	}}
	wrapWithHooks("up", cmd)

	if err := cmd.RunE(cmd, []string{"--dry-run"}); err != nil {
		t.Fatalf("wrapped command failed: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("dry-run executed hook command; marker stat error = %v", err)
	}
}

func TestRunHookSteps_DryRun(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	steps := []config.ProvisionItem{
		{Step: "Run tests", Run: "make test"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := runHookSteps(e, c, "before", "up", steps)

	w.Close()
	os.Stderr = old

	if err != nil {
		t.Fatalf("runHookSteps error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "[dry-run]") {
		t.Error("should show dry-run prefix")
	}
	if !strings.Contains(output, "make test") {
		t.Error("should show command being executed")
	}
}

func TestRunHookSteps_DryRun_ComposeUp(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	steps := []config.ProvisionItem{
		{Step: "Start DB", ComposeUp: []string{"postgres"}},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := runHookSteps(e, c, "before", "up", steps)

	w.Close()
	os.Stderr = old

	if err != nil {
		t.Fatalf("runHookSteps error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "[dry-run]") {
		t.Error("should show dry-run prefix")
	}
}

func TestRunHookSteps_DryRun_ComposeExec(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	steps := []config.ProvisionItem{
		{Step: "Run migration", ComposeExec: "web rails db:migrate"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := runHookSteps(e, c, "after", "up", steps)

	w.Close()
	os.Stderr = old

	if err != nil {
		t.Fatalf("runHookSteps error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "[dry-run]") {
		t.Error("should show dry-run prefix for compose exec")
	}
}

func TestRunHookSteps_DryRun_ComposeRun(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	steps := []config.ProvisionItem{
		{Step: "Seed DB", ComposeRun: "web rails db:seed"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := runHookSteps(e, c, "replace", "up", steps)

	w.Close()
	os.Stderr = old

	if err != nil {
		t.Fatalf("runHookSteps error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "[dry-run]") {
		t.Error("should show dry-run prefix for compose run")
	}
}

func TestRunHookSteps_WithNote(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	steps := []config.ProvisionItem{
		{Step: "Info", Note: "Remember to check logs"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	runHookSteps(e, c, "after", "up", steps)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "Remember to check logs") {
		t.Error("should display note text")
	}
}

func TestRunHookSteps_RealExecution(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	steps := []config.ProvisionItem{
		{Step: "Echo test", Run: "echo hello"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := runHookSteps(e, c, "before", "up", steps)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runHookSteps error: %v", err)
	}
	if !strings.Contains(buf.String(), "hook:before:up") {
		t.Error("should show hook phase and command name")
	}
}

func TestRunHookSteps_FailingCommand(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	steps := []config.ProvisionItem{
		{Step: "Fail", Run: "false"},
	}

	old := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	err := runHookSteps(e, c, "before", "up", steps)

	w.Close()
	os.Stderr = old

	if err == nil {
		t.Fatal("expected error from failing command")
	}
	if !strings.Contains(err.Error(), "hook before:up") {
		t.Errorf("error should mention hook phase, got: %v", err)
	}
}

func TestRunHookSteps_DefaultStepLabel(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	steps := []config.ProvisionItem{
		{Run: "echo test"},
	}

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	runHookSteps(e, c, "before", "up", steps)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "step 1") {
		t.Error("should use default 'step N' label when Step is empty")
	}
}

func TestRunHookSteps_Empty(t *testing.T) {
	c := loadTestConfig(t, "version: \"0.1.22\"\n")
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	err := runHookSteps(e, c, "before", "up", nil)
	if err != nil {
		t.Fatalf("empty steps should not error: %v", err)
	}
}
