package cli

import (
	"os"
	"path/filepath"
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

func TestGenerateConfig_Rails(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfig("rails")
	if !strings.Contains(got, "RAILS_ENV") {
		t.Error("rails config should contain RAILS_ENV")
	}
	if !strings.Contains(got, "bundle exec rspec") {
		t.Error("rails config should contain rspec command")
	}
}

func TestGenerateConfig_Go(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfig("go")
	if !strings.Contains(got, "go test ./...") {
		t.Error("go config should contain go test")
	}
}

func TestGenerateConfig_Python(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfig("python")
	if !strings.Contains(got, "PYTHONDONTWRITEBYTECODE") {
		t.Error("python config should contain PYTHONDONTWRITEBYTECODE")
	}
	if !strings.Contains(got, "pytest") {
		t.Error("python config should contain pytest")
	}
}

func TestGenerateConfig_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := generateConfig("minimal")
	if !strings.Contains(got, "/bin/bash") {
		t.Error("minimal config should contain shell command")
	}
	// Should NOT contain language-specific env
	if strings.Contains(got, "RAILS_ENV") || strings.Contains(got, "NODE_ENV") {
		t.Error("minimal config should not contain language-specific env")
	}
}

func TestGenerateConfig_NoComposeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// No compose files on disk → should fallback to docker-compose.yml
	got := generateConfig("minimal")
	if !strings.Contains(got, "docker-compose.yml") {
		t.Error("should fallback to docker-compose.yml when no compose files exist")
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

func TestBuildImprovePrompt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dva-improve-prompt-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	dvaYAML := `version: "0.1.0"

compose:
  files:
    - docker-compose.yml
  project_name: sample

interaction:
  test:
    description: "Run tests"
    service: app
    command: go test ./...
`
	if err := os.WriteFile("dva.yml", []byte(dvaYAML), 0644); err != nil {
		t.Fatalf("Failed to write dva.yml: %v", err)
	}
	if err := os.WriteFile("docker-compose.yml", []byte("services:\n  app:\n    image: golang:1.24\n"), 0644); err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}
	if err := os.WriteFile("go.mod", []byte("module example.com/sample"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	cfg = nil
	env = nil

	prompt, err := buildImprovePrompt()
	if err != nil {
		t.Fatalf("buildImprovePrompt returned error: %v", err)
	}

	if !strings.Contains(prompt, "dva manifest -f json") {
		t.Fatalf("Expected improve prompt to mention manifest inspection, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Current DVA File") {
		t.Fatalf("Expected improve prompt to include current config section, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "docker-compose.yml") {
		t.Fatalf("Expected improve prompt to include compose snapshot, got:\n%s", prompt)
	}
}

func TestFilterEnv(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/root", "PATH=/extra"}
	got := filterEnv(env, "PATH")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(got), got)
	}
	if got[0] != "HOME=/root" {
		t.Errorf("expected HOME=/root, got %s", got[0])
	}
}

func TestFilterEnv_NoMatch(t *testing.T) {
	env := []string{"HOME=/root", "USER=test"}
	got := filterEnv(env, "PATH")
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

func TestFilterEnv_Empty(t *testing.T) {
	got := filterEnv(nil, "PATH")
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

func TestExtractMakefileTargets(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	content := "build: ## Build the project\n\tgo build ./...\n\ntest: ## Run tests\n\tgo test ./...\n\n.PHONY: build test\n"
	os.WriteFile("Makefile", []byte(content), 0644)
	got := extractMakefileTargets()
	if !strings.Contains(got, "build") {
		t.Error("should contain 'build' target")
	}
	if !strings.Contains(got, "Build the project") {
		t.Error("should contain target description")
	}
}

func TestExtractMakefileTargets_NoMakefile(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := extractMakefileTargets()
	if got != "" {
		t.Errorf("expected empty for no Makefile, got %q", got)
	}
}

func TestExtractComposeServices(t *testing.T) {
	tmpDir := t.TempDir()
	compose := filepath.Join(tmpDir, "docker-compose.yml")
	content := "version: '3.8'\nservices:\n  postgres:\n    image: postgres:15\n  redis:\n    image: redis:7\nvolumes:\n  data:\n"
	os.WriteFile(compose, []byte(content), 0644)

	services := extractComposeServices(compose)
	if len(services) < 2 {
		t.Fatalf("expected at least 2 services, got %d: %v", len(services), services)
	}
	found := map[string]bool{}
	for _, s := range services {
		found[s] = true
	}
	if !found["postgres"] {
		t.Error("should find 'postgres' service")
	}
	if !found["redis"] {
		t.Error("should find 'redis' service")
	}
}

func TestExtractComposeServices_NoFile(t *testing.T) {
	services := extractComposeServices("/nonexistent/compose.yml")
	if services != nil {
		t.Errorf("expected nil for nonexistent file, got %v", services)
	}
}

func TestDetectInfraComposeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// No infra dirs
	files := detectInfraComposeFiles()
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}

	// Create infra dir with compose file
	os.MkdirAll("infra", 0755)
	os.WriteFile("infra/compose.yml", []byte("services:\n  pg:\n"), 0644)
	files = detectInfraComposeFiles()
	if len(files) != 1 || files[0] != "infra/compose.yml" {
		t.Errorf("expected [infra/compose.yml], got %v", files)
	}
}

func TestDetectInfraComposeFiles_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll("infra", 0755)
	os.MkdirAll("docker", 0755)
	os.WriteFile("infra/docker-compose.yml", []byte(""), 0644)
	os.WriteFile("docker/compose.yaml", []byte(""), 0644)

	files := detectInfraComposeFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}
