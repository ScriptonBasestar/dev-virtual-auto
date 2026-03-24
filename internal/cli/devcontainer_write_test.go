package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDevcontainerFiles(t *testing.T) {
	tmpDir := t.TempDir()
	dc := map[string]any{
		"enabled":         true,
		"name":            "Dev",
		"service":         "web",
		"workspaceFolder": "/workspace",
	}

	err := writeDevcontainerFiles(dc, []string{"compose.yml"}, tmpDir)
	if err != nil {
		t.Fatalf("writeDevcontainerFiles error: %v", err)
	}

	jsonPath := filepath.Join(tmpDir, ".devcontainer", "devcontainer.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("devcontainer.json not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "web") {
		t.Error("should contain service name")
	}
	if !strings.Contains(content, "/workspace") {
		t.Error("should contain workspaceFolder")
	}
	if strings.Contains(content, "enabled") {
		t.Error("should NOT contain 'enabled' field (DVA-only)")
	}
}

func TestWriteDevcontainerFiles_WithComposeLink(t *testing.T) {
	tmpDir := t.TempDir()
	dc := map[string]any{
		"name":    "Dev",
		"service": "app",
	}

	err := writeDevcontainerFiles(dc, []string{"docker-compose.yml"}, tmpDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, ".devcontainer", "devcontainer.json"))
	content := string(data)
	if !strings.Contains(content, "../docker-compose.yml") {
		t.Error("should link to compose file relative to .devcontainer/")
	}
}
