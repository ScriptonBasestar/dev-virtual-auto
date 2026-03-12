package cli

import (
	"os"
	"strings"
	"testing"
)

func TestDetectTemplate(t *testing.T) {
	// Setup a temporary directory
	tempDir, err := os.MkdirTemp("", "dva-init-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// Test default (minimal)
	if tmpl := detectTemplate(); tmpl != "minimal" {
		t.Errorf("Expected minimal, got %s", tmpl)
	}

	// Test node detection
	os.WriteFile("package.json", []byte("{}"), 0644)
	if tmpl := detectTemplate(); tmpl != "node" {
		t.Errorf("Expected node, got %s", tmpl)
	}
	os.Remove("package.json")

	// Test go detection
	os.WriteFile("go.mod", []byte("module foo"), 0644)
	if tmpl := detectTemplate(); tmpl != "go" {
		t.Errorf("Expected go, got %s", tmpl)
	}
	os.Remove("go.mod")
}

func TestGenerateConfig(t *testing.T) {
	// Setup a temporary directory
	tempDir, err := os.MkdirTemp("", "dva-init-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// Create a dummy compose file so it gets detected
	os.WriteFile("docker-compose.yml", []byte(""), 0644)

	config := generateConfig("node")
	if !strings.Contains(config, `npm run dev`) {
		t.Errorf("Expected config to contain 'npm run dev', got:\n%s", config)
	}
	if !strings.Contains(config, `docker-compose.yml`) {
		t.Errorf("Expected config to contain 'docker-compose.yml', got:\n%s", config)
	}
}

func TestDetectComposeFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dva-compose-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// No compose files initially
	files := detectComposeFiles()
	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}

	// Create some compose files
	os.WriteFile("docker-compose.yml", []byte(""), 0644)
	os.WriteFile("docker-compose.override.yml", []byte(""), 0644)
	os.Mkdir("db", 0755) // should be ignored

	files = detectComposeFiles()
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	if files[0] != "docker-compose.yml" {
		t.Errorf("Expected primary file first, got %s", files[0])
	}
	if files[1] != "docker-compose.override.yml" {
		t.Errorf("Expected override file second, got %s", files[1])
	}
}
