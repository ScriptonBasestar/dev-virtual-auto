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

func TestExtractGuidedWorkflow(t *testing.T) {
	if improveGuidedWorkflowText == "" {
		t.Fatal("improve_guided_workflow.txt not embedded")
	}

	targetDir := t.TempDir()
	if err := extractGuidedWorkflow(targetDir); err != nil {
		t.Fatalf("extractGuidedWorkflow failed: %v", err)
	}

	expectedFiles := []string{
		"orchestrator.md",
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

func TestExtractGuidedWorkflowContent(t *testing.T) {
	targetDir := t.TempDir()
	if err := extractGuidedWorkflow(targetDir); err != nil {
		t.Fatalf("extractGuidedWorkflow failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(targetDir, "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "improve-guided") {
		t.Error("orchestrator.md should reference improve-guided")
	}
}
