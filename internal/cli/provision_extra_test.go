package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestListProvisionProfiles_Empty(t *testing.T) {
	c := &config.Config{}
	output := captureStdout(t, func() {
		listProvisionProfiles(c)
	})
	if !strings.Contains(output, "No provision profiles") {
		t.Errorf("expected 'No provision profiles' message, got: %s", output)
	}
}

func TestListProvisionProfiles_Table(t *testing.T) {
	c := &config.Config{
		Provision: config.ProvisionConfig{
			Profiles: map[string][]config.ProvisionItem{
				"setup": {{Step: "Create DB", Run: "createdb test"}},
				"reset": {{Step: "Drop DB", Run: "dropdb test"}},
			},
			DefaultProfile: "setup",
		},
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	output := captureStdout(t, func() {
		listProvisionProfiles(c)
	})
	if !strings.Contains(output, "setup") {
		t.Error("should list 'setup' profile")
	}
	if !strings.Contains(output, "reset") {
		t.Error("should list 'reset' profile")
	}
	if !strings.Contains(output, "PROFILE") {
		t.Error("should contain table header")
	}
}

func TestListProvisionProfiles_JSON(t *testing.T) {
	c := &config.Config{
		Provision: config.ProvisionConfig{
			Profiles: map[string][]config.ProvisionItem{
				"setup": {{Step: "Init", Run: "echo ok"}},
			},
		},
	}

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	output := captureStdout(t, func() {
		listProvisionProfiles(c)
	})
	if !strings.Contains(output, "setup") {
		t.Error("JSON should contain 'setup' profile")
	}
}

func TestWriteProvisionMarker(t *testing.T) {
	dir := t.TempDir()
	writeProvisionMarker(dir, "setup")

	markerFile := filepath.Join(dir, config.DotDirName, "provisioned-setup")
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Error("expected marker file to be created")
	}
}

func TestClearProvisionMarkers(t *testing.T) {
	dir := t.TempDir()

	// Create markers
	writeProvisionMarker(dir, "setup")
	writeProvisionMarker(dir, "reset")

	// Should have 2 marker files
	markerDir := filepath.Join(dir, config.DotDirName)
	entries, _ := os.ReadDir(markerDir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 markers, got %d", len(entries))
	}

	clearProvisionMarkers(dir)

	entries, _ = os.ReadDir(markerDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "provisioned-") {
			t.Errorf("marker %q should have been removed", e.Name())
		}
	}
}

func TestClearProvisionMarkers_NoDir(t *testing.T) {
	// Should not panic or error on missing directory
	clearProvisionMarkers(filepath.Join(t.TempDir(), "nonexistent"))
}

func TestFirstStepDescription_Empty(t *testing.T) {
	if got := firstStepDescription(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFirstStepDescription_Step(t *testing.T) {
	steps := []config.ProvisionItem{{Step: "Create DB"}}
	if got := firstStepDescription(steps); got != "Create DB" {
		t.Errorf("got %q", got)
	}
}

func TestFirstStepDescription_Raw(t *testing.T) {
	steps := []config.ProvisionItem{{Raw: "make setup"}}
	if got := firstStepDescription(steps); got != "make setup" {
		t.Errorf("got %q", got)
	}
}

func TestFirstStepDescription_Echo(t *testing.T) {
	steps := []config.ProvisionItem{{Echo: "Setting up..."}}
	if got := firstStepDescription(steps); got != "Setting up..." {
		t.Errorf("got %q", got)
	}
}

func TestFirstStepDescription_FallbackEmpty(t *testing.T) {
	steps := []config.ProvisionItem{{Run: "echo hi"}}
	if got := firstStepDescription(steps); got != "" {
		t.Errorf("expected empty fallback, got %q", got)
	}
}

func TestRunShellCommand_Success(t *testing.T) {
	if err := runShellCommand(nil, "true"); err != nil {
		t.Errorf("expected success, got: %v", err)
	}
}

func TestRunShellCommand_Failure(t *testing.T) {
	if err := runShellCommand(nil, "false"); err == nil {
		t.Error("expected error from 'false'")
	}
}
