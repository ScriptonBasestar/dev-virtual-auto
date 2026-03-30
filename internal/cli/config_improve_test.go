package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImproveInteractiveFlagRegistered(t *testing.T) {
	f := improveCmd.Flags().Lookup("interactive")
	if f == nil {
		t.Fatal("expected --interactive flag to be registered")
	}
	if f.Shorthand != "i" {
		t.Errorf("expected shorthand 'i', got %q", f.Shorthand)
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestImproveAllFlagsRegistered(t *testing.T) {
	flags := []string{"print", "docs-only", "verbose", "recursive", "rewrite", "interactive"}
	for _, name := range flags {
		if improveCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag --%s to be registered", name)
		}
	}
}

func TestExtractSetupDvaWorkflow(t *testing.T) {
	if setupDvaWorkflowText == "" {
		t.Fatal("setup_dva_workflow.txt not embedded")
	}

	targetDir := t.TempDir()
	if err := extractSetupDvaWorkflow(targetDir); err != nil {
		t.Fatalf("extractSetupDvaWorkflow failed: %v", err)
	}

	// Verify expected files are extracted
	expectedFiles := []string{
		"auto.md",
		"stages/00-analyze.md",
		"stages/10-verify.md",
		"stages/20-transform.md",
		"stages/30-configure-full.md",
		"stages/30-configure-adopt.md",
		"stages/40-execute.md",
		"verify/checklist.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(targetDir, f)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %s not found: %v", f, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("expected file %s to be non-empty", f)
		}
	}
}

func TestExtractSetupDvaWorkflowContent(t *testing.T) {
	targetDir := t.TempDir()
	if err := extractSetupDvaWorkflow(targetDir); err != nil {
		t.Fatalf("extractSetupDvaWorkflow failed: %v", err)
	}

	// auto.md should contain orchestrator keywords
	autoContent, err := os.ReadFile(filepath.Join(targetDir, "auto.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(autoContent), "setup-dva") {
		t.Error("auto.md should reference setup-dva")
	}
}
