//go:build integration

package integration

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestLoadBasicFixture(t *testing.T) {
	c := loadFixtureConfig(t, "basic")

	if c.Version != "0.1.22" {
		t.Errorf("Version = %q, want %q", c.Version, "0.1.22")
	}
	if c.ComposeProjectName() != "basic-test" {
		t.Errorf("ProjectName = %q, want %q", c.ComposeProjectName(), "basic-test")
	}
	files := c.AllComposeFiles()
	if len(files) != 1 || files[0] != "compose.yml" {
		t.Errorf("Compose.Files = %v, want [compose.yml]", files)
	}
	if len(c.Interaction) != 2 {
		t.Errorf("Interaction count = %d, want 2", len(c.Interaction))
	}
	if c.Interaction["shell"] == nil {
		t.Error("missing 'shell' interaction command")
	}
	if c.Interaction["test"] == nil {
		t.Error("missing 'test' interaction command")
	}
	if c.Environment["APP_ENV"] != "development" {
		t.Errorf("APP_ENV = %q, want %q", c.Environment["APP_ENV"], "development")
	}
}

func TestLoadFullStackFixture(t *testing.T) {
	c := loadFixtureConfig(t, "full-stack")

	// Modes
	if len(c.Modes) != 2 {
		t.Fatalf("Modes count = %d, want 2", len(c.Modes))
	}
	docker := c.Modes["docker"]
	if docker.Description != "Full Docker mode" {
		t.Errorf("docker.Description = %q", docker.Description)
	}
	if len(docker.ComposeProfiles) != 2 {
		t.Errorf("docker.ComposeProfiles = %v, want 2 items", docker.ComposeProfiles)
	}

	native := c.Modes["native"]
	if native.ComposeServices == nil || len(*native.ComposeServices) != 0 {
		t.Error("native mode should have empty compose_services (not nil)")
	}

	// Environments
	if len(c.Environments) != 2 {
		t.Fatalf("Environments count = %d, want 2", len(c.Environments))
	}
	if c.Environments["stg"].Environment["DB_HOST"] != "stg-db.internal" {
		t.Errorf("stg DB_HOST = %q", c.Environments["stg"].Environment["DB_HOST"])
	}

	// Health checks
	if len(c.HealthChecks) != 2 {
		t.Fatalf("HealthChecks count = %d, want 2", len(c.HealthChecks))
	}
	if c.HealthChecks["db"].Type != "tcp" {
		t.Errorf("db health check type = %q, want 'tcp'", c.HealthChecks["db"].Type)
	}

	// Subcommands
	rails := c.Interaction["rails"]
	if rails == nil {
		t.Fatal("missing 'rails' interaction command")
	}
	if rails.Subcommands == nil || rails.Subcommands["console"] == nil {
		t.Error("rails should have 'console' subcommand")
	}

	// Provision
	if len(c.Provision.Profiles) != 2 {
		t.Errorf("Provision profiles count = %d, want 2", len(c.Provision.Profiles))
	}
	if c.Provision.DefaultProfile != "setup" {
		t.Errorf("DefaultProfile = %q, want %q", c.Provision.DefaultProfile, "setup")
	}
}

func TestLoadProvisionProfilesFixture(t *testing.T) {
	c := loadFixtureConfig(t, "provision-profiles")

	if len(c.Provision.Profiles) != 3 {
		t.Fatalf("Provision profiles count = %d, want 3", len(c.Provision.Profiles))
	}

	setup := c.Provision.Profiles["setup"]
	if len(setup) != 2 {
		t.Errorf("setup steps = %d, want 2", len(setup))
	}

	reset := c.Provision.Profiles["reset"]
	if len(reset) != 2 {
		t.Errorf("reset steps = %d, want 2", len(reset))
	}

	// Verify parallel steps
	parallel := c.Provision.Profiles["parallel-demo"]
	if len(parallel) != 3 {
		t.Fatalf("parallel-demo steps = %d, want 3", len(parallel))
	}
	if !parallel[0].Parallel {
		t.Error("parallel-demo step 0 should have Parallel=true")
	}
	if !parallel[1].Parallel {
		t.Error("parallel-demo step 1 should have Parallel=true")
	}
	if parallel[2].Parallel {
		t.Error("parallel-demo step 2 should have Parallel=false (barrier)")
	}
}

func TestLoadInvalidFixture(t *testing.T) {
	err := loadFixtureConfigErr(t, "invalid")
	if err == nil {
		t.Fatal("expected error loading invalid fixture, got nil")
	}
}

func TestLoadNonexistentFixture(t *testing.T) {
	// Use an isolated temp dir with no dva.yml anywhere in the path
	// (config.Load walks up to find dva.yml, so we need a clean tree)
	tmpDir := t.TempDir()
	_, err := config.Load(tmpDir)
	if err == nil {
		t.Fatal("expected error loading from empty dir with no dva.yml, got nil")
	}
}
