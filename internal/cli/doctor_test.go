package cli

import (
	"os"
	"path/filepath"
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

func TestCondStr(t *testing.T) {
	if got := condStr(true, "yes"); got != "yes" {
		t.Errorf("condStr(true) = %q", got)
	}
	if got := condStr(false, "yes"); got != "" {
		t.Errorf("condStr(false) = %q", got)
	}
}
