package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestRunSingleCheck_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")
	os.WriteFile(existingFile, []byte("hello"), 0644)

	t.Run("file exists", func(t *testing.T) {
		check := config.DoctorCheck{
			Name:    "Test file",
			Type:    "file_exists",
			Path:    existingFile,
			FixHint: "create the file",
		}
		result := runSingleCheck(check, tmpDir)
		if !result.Passed {
			t.Error("expected pass for existing file")
		}
		if result.FixHint != "" {
			t.Error("fix_hint should be cleared on pass")
		}
	})

	t.Run("file missing", func(t *testing.T) {
		check := config.DoctorCheck{
			Name:    "Missing file",
			Type:    "file_exists",
			Path:    filepath.Join(tmpDir, "missing.txt"),
			FixHint: "create it",
		}
		result := runSingleCheck(check, tmpDir)
		if result.Passed {
			t.Error("expected fail for missing file")
		}
		if result.FixHint != "create it" {
			t.Errorf("fix_hint = %q, want 'create it'", result.FixHint)
		}
	})
}

func TestRunSingleCheck_FileExists_Relative(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("KEY=val"), 0644)

	check := config.DoctorCheck{
		Name: ".env exists",
		Type: "file_exists",
		Path: ".env",
	}
	result := runSingleCheck(check, tmpDir)
	if !result.Passed {
		t.Error("expected pass for relative path resolved against configDir")
	}
}

func TestRunSingleCheck_Command(t *testing.T) {
	t.Run("command succeeds", func(t *testing.T) {
		check := config.DoctorCheck{
			Name:    "true command",
			Type:    "command",
			Command: "true",
		}
		result := runSingleCheck(check, ".")
		if !result.Passed {
			t.Error("expected pass for 'true' command")
		}
	})

	t.Run("command fails", func(t *testing.T) {
		check := config.DoctorCheck{
			Name:    "false command",
			Type:    "command",
			Command: "false",
			FixHint: "fix it",
		}
		result := runSingleCheck(check, ".")
		if result.Passed {
			t.Error("expected fail for 'false' command")
		}
	})
}

func TestRunSingleCheck_UnknownType(t *testing.T) {
	check := config.DoctorCheck{
		Name: "Unknown",
		Type: "bogus",
	}
	result := runSingleCheck(check, ".")
	if result.Passed {
		t.Error("expected fail for unknown type")
	}
}

func TestPrintDoctorResults_AllPass(t *testing.T) {
	results := []DoctorResult{
		{Name: "Docker installed", Passed: true},
		{Name: "Config found", Passed: true},
	}
	output := captureStdout(t, func() {
		printDoctorResults(results)
	})
	if !strings.Contains(output, "[pass]") {
		t.Error("should contain [pass]")
	}
	if !strings.Contains(output, "2 passed, 0 failed") {
		t.Error("should show 2 passed, 0 failed")
	}
}

func TestPrintDoctorResults_WithFailures(t *testing.T) {
	results := []DoctorResult{
		{Name: "Docker installed", Passed: true},
		{Name: "Docker running", Passed: false, FixHint: "Start Docker Desktop"},
	}
	output := captureStdout(t, func() {
		printDoctorResults(results)
	})
	if !strings.Contains(output, "[FAIL]") {
		t.Error("should contain [FAIL]")
	}
	if !strings.Contains(output, "Start Docker Desktop") {
		t.Error("should contain fix hint")
	}
	if !strings.Contains(output, "1 passed, 1 failed") {
		t.Error("should show 1 passed, 1 failed")
	}
}

func TestCondStr(t *testing.T) {
	if got := condStr(true, "yes"); got != "yes" {
		t.Errorf("condStr(true) = %q", got)
	}
	if got := condStr(false, "yes"); got != "" {
		t.Errorf("condStr(false) = %q", got)
	}
}

func TestApplyDoctorFixes(t *testing.T) {
	t.Run("fixable success", func(t *testing.T) {
		results := []DoctorResult{
			{Name: "fix me", Passed: false, Fixable: true, fixFunc: func() error { return nil }},
		}
		applyDoctorFixes(results)
		if !results[0].Fixed || !results[0].Passed {
			t.Error("expected Fixed=true and Passed=true after successful fix")
		}
	})

	t.Run("fixable failure", func(t *testing.T) {
		results := []DoctorResult{
			{Name: "broken", Passed: false, Fixable: true, fixFunc: func() error { return fmt.Errorf("cannot fix") }},
		}
		applyDoctorFixes(results)
		if results[0].Fixed || results[0].Passed {
			t.Error("expected Fixed=false and Passed=false when fix fails")
		}
	})

	t.Run("not fixable skipped", func(t *testing.T) {
		results := []DoctorResult{
			{Name: "manual", Passed: false, Fixable: false},
		}
		applyDoctorFixes(results)
		if results[0].Fixed {
			t.Error("non-fixable check should not be fixed")
		}
	})

	t.Run("already passed skipped", func(t *testing.T) {
		called := false
		results := []DoctorResult{
			{Name: "ok", Passed: true, Fixable: true, fixFunc: func() error { called = true; return nil }},
		}
		applyDoctorFixes(results)
		if called {
			t.Error("fixFunc should not be called for passing checks")
		}
	})
}

func TestRunSingleCheck_WithFixCommand(t *testing.T) {
	tmpDir := t.TempDir()
	check := config.DoctorCheck{
		Name:    "missing file with fix",
		Type:    "file_exists",
		Path:    filepath.Join(tmpDir, "missing.txt"),
		FixHint: "create it",
		Fix:     "touch " + filepath.Join(tmpDir, "missing.txt"),
	}
	result := runSingleCheck(check, tmpDir)
	if result.Passed {
		t.Error("expected fail before fix")
	}
	if !result.Fixable {
		t.Error("expected Fixable=true when Fix command is set")
	}
	if result.fixFunc == nil {
		t.Error("expected fixFunc to be set")
	}
}

func TestPrintDoctorResults_WithFixed(t *testing.T) {
	results := []DoctorResult{
		{Name: "auto-fixed item", Passed: true, Fixed: true},
		{Name: "still failing", Passed: false, Fixable: true, FixHint: "run something"},
	}
	output := captureStdout(t, func() {
		printDoctorResults(results)
	})
	if !strings.Contains(output, "[fixed]") {
		t.Error("should contain [fixed]")
	}
	if !strings.Contains(output, "auto-fix") {
		t.Error("should suggest --fix for fixable failures")
	}
	if !strings.Contains(output, "1 auto-fixed") {
		t.Error("should show auto-fixed count")
	}
}

func TestCheckStackFilesReadsComposeRunner(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	results := checkStackFiles(c)
	if len(results) != 1 {
		t.Fatalf("expected 1 stack file result, got %d: %+v", len(results), results)
	}
	if !results[0].Passed {
		t.Fatalf("expected compose runner file check to pass: %+v", results[0])
	}
	if !strings.Contains(results[0].Name, "compose.yml") {
		t.Fatalf("expected result to mention compose.yml: %+v", results[0])
	}
}

func TestCheckGitignoreStatus_Fix(t *testing.T) {
	tmpDir := t.TempDir()
	// No .gitignore → should be fixable
	result := checkGitignoreStatus(tmpDir)
	if result.Passed {
		t.Error("expected fail when .gitignore missing")
	}
	if !result.Fixable {
		t.Error("expected Fixable=true")
	}
	if result.fixFunc == nil {
		t.Fatal("expected fixFunc to be set")
	}

	// Apply fix
	if err := result.fixFunc(); err != nil {
		t.Fatalf("fixFunc failed: %v", err)
	}

	// Re-check
	result2 := checkGitignoreStatus(tmpDir)
	if !result2.Passed {
		t.Error("expected pass after fix")
	}
}
