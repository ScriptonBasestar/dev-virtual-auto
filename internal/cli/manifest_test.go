package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestBuildManifest_MinimalConfig(t *testing.T) {
	c := &config.Config{
		Version: "0.1.22",
		Stack: map[string]*config.LifecycleEntry{
			"compose": {
				Order: 10,
				Compose: &config.ComposePluginConfig{
					Files: []string{"compose.yml"},
				},
			},
		},
	}

	m := buildManifest(c)

	if m.DvaVersion != config.Version {
		t.Errorf("DvaVersion = %q, want %q", m.DvaVersion, config.Version)
	}
	if m.SchemaVersion != "1.2" {
		t.Errorf("SchemaVersion = %q, want %q", m.SchemaVersion, "1.2")
	}
	if len(m.ComposeFiles) != 1 || m.ComposeFiles[0] != "compose.yml" {
		t.Errorf("ComposeFiles = %v, want [compose.yml]", m.ComposeFiles)
	}
	if len(m.StaticCommands) == 0 {
		t.Error("StaticCommands should not be empty")
	}
	if _, ok := m.StaticCommands["up"]; !ok {
		t.Error("StaticCommands should contain 'up'")
	}
	if len(m.Runners) != 3 {
		t.Errorf("Runners = %d, want 3", len(m.Runners))
	}
}

func TestBuildManifest_WithInteraction(t *testing.T) {
	c := &config.Config{
		Interaction: map[string]*config.InteractionCommand{
			"test": {
				Description: "Run tests",
				Command:     "make test",
			},
			"lint": {
				Description: "Run linter",
				Command:     "make lint",
				Service:     "app",
				Compose:     &config.ComposeOptions{Method: "exec"},
			},
		},
	}

	m := buildManifest(c)

	if len(m.DynamicCommands) != 2 {
		t.Fatalf("DynamicCommands = %d, want 2", len(m.DynamicCommands))
	}

	testCmd, ok := m.DynamicCommands["test"]
	if !ok {
		t.Fatal("missing 'test' in DynamicCommands")
	}
	if testCmd.Description != "Run tests" {
		t.Errorf("test.Description = %q", testCmd.Description)
	}
	if testCmd.Runner != "Local" {
		t.Errorf("test.Runner = %q, want 'local'", testCmd.Runner)
	}

	lintCmd := m.DynamicCommands["lint"]
	if lintCmd.Service != "app" {
		t.Errorf("lint.Service = %q, want 'app'", lintCmd.Service)
	}
	if lintCmd.ComposeMethod != "exec" {
		t.Errorf("lint.ComposeMethod = %q, want 'exec'", lintCmd.ComposeMethod)
	}
}

func TestBuildManifest_WithHealthChecks(t *testing.T) {
	c := &config.Config{
		HealthChecks: map[string]config.HealthCheckConfig{
			"db": {
				Type:    "tcp",
				Address: "localhost:5432",
			},
			"api": {
				Type: "http",
				URL:  "http://localhost:8080/health",
			},
		},
	}

	m := buildManifest(c)

	if len(m.HealthChecks) != 2 {
		t.Fatalf("HealthChecks = %d, want 2", len(m.HealthChecks))
	}
	db := m.HealthChecks["db"]
	if db.Type != "tcp" || db.Address != "localhost:5432" {
		t.Errorf("db health check = %+v", db)
	}
}

func TestBuildManifest_WithPodCommand(t *testing.T) {
	c := &config.Config{
		Interaction: map[string]*config.InteractionCommand{
			"deploy": {
				Description: "Deploy app",
				Command:     "kubectl apply -f .",
				Pod:         "api-server",
			},
		},
	}

	m := buildManifest(c)

	deploy, ok := m.DynamicCommands["deploy"]
	if !ok {
		t.Fatal("missing 'deploy' in DynamicCommands")
	}
	if deploy.Pod != "api-server" {
		t.Errorf("deploy.Pod = %q, want 'api-server'", deploy.Pod)
	}
	if deploy.Runner != "Kubectl" {
		t.Errorf("deploy.Runner = %q, want 'Kubectl'", deploy.Runner)
	}
}

func TestBuildManifest_WithEnvironment(t *testing.T) {
	c := &config.Config{
		Environment: map[string]string{
			"DB_HOST": "localhost",
			"APP_ENV": "development",
		},
	}

	m := buildManifest(c)

	if len(m.EnvKeys) != 2 {
		t.Fatalf("EnvKeys = %d, want 2", len(m.EnvKeys))
	}
	// Should be sorted
	if m.EnvKeys[0] != "APP_ENV" || m.EnvKeys[1] != "DB_HOST" {
		t.Errorf("EnvKeys = %v, want [APP_ENV, DB_HOST]", m.EnvKeys)
	}
}

func TestBuildManifest_WithMissingPlaceholderSubproject(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "ready")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subproject dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(subDir, config.FileName), []byte(`
version: "0.1.0"
interaction:
  shell:
    description: "Open shell"
    command: "bash"
`), 0644); err != nil {
		t.Fatalf("write subproject config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`
version: "0.1.0"
subprojects:
  pending:
    path: pending
  ready:
    path: ready
`), 0644); err != nil {
		t.Fatalf("write parent config: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	m := buildManifest(c)

	if len(m.Subprojects) != 2 {
		t.Fatalf("Subprojects = %d, want 2", len(m.Subprojects))
	}
	if _, ok := m.Subprojects["pending"]; !ok {
		t.Fatal("missing placeholder subproject")
	}
	if len(m.Subprojects["pending"].Commands) != 0 {
		t.Errorf("pending commands = %v, want none", m.Subprojects["pending"].Commands)
	}
	if _, ok := m.Subprojects["ready"].Commands["shell"]; !ok {
		t.Fatalf("ready shell command missing: %v", m.Subprojects["ready"].Commands)
	}
}
