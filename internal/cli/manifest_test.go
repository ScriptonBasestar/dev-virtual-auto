package cli

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestBuildManifest_MinimalConfig(t *testing.T) {
	c := &config.Config{
		Version: "0.1.22",
		Lifecycle: []config.LifecycleEntry{
			{
				Name: "compose", Plugin: "compose", Order: 10,
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
	if m.SchemaVersion != "1.1" {
		t.Errorf("SchemaVersion = %q, want %q", m.SchemaVersion, "1.1")
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
