package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateHip_LegacyKeys(t *testing.T) {
	dir := t.TempDir()
	hipPath := filepath.Join(dir, ".hip.yml")

	content := `scripts:
  test:
    description: Run tests
    service: app
    command: bundle exec rspec
env:
  RAILS_ENV: development
`
	if err := os.WriteFile(hipPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Should not error; output goes to stdout
	if err := migrateHip(hipPath); err != nil {
		t.Fatalf("migrateHip returned error: %v", err)
	}
}

func TestMigrateHip_NoLegacyKeys(t *testing.T) {
	dir := t.TempDir()
	hipPath := filepath.Join(dir, ".hip.yml")

	content := `interaction:
  test:
    description: Run tests
    service: app
    command: bundle exec rspec
`
	if err := os.WriteFile(hipPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateHip(hipPath); err != nil {
		t.Fatalf("migrateHip returned error: %v", err)
	}
}

func TestMigrateDva_LegacyKeys(t *testing.T) {
	dir := t.TempDir()
	dvaPath := filepath.Join(dir, "dva.yml")

	content := `scripts:
  test:
    service: app
commands:
  build:
    service: app
env:
  KEY: value
`
	if err := os.WriteFile(dvaPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateDva(dvaPath); err != nil {
		t.Fatalf("migrateDva returned error: %v", err)
	}
}

func TestMigrateDva_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	dvaPath := filepath.Join(dir, "dva.yml")

	// interaction command without description → should flag it
	content := `interaction:
  test:
    service: app
    command: rspec
`
	if err := os.WriteFile(dvaPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateDva(dvaPath); err != nil {
		t.Fatalf("migrateDva returned error: %v", err)
	}
}

func TestMigrateDva_UpToDate(t *testing.T) {
	dir := t.TempDir()
	dvaPath := filepath.Join(dir, "dva.yml")

	content := `interaction:
  test:
    description: Run test suite
    service: app
    command: rspec
environment:
  RAILS_ENV: development
`
	if err := os.WriteFile(dvaPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := migrateDva(dvaPath); err != nil {
		t.Fatalf("migrateDva returned error: %v", err)
	}
}

func TestRunMigrate_NoConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	// No .hip.yml and no dva.yml → should not error
	if err := runMigrate(nil, nil); err != nil {
		t.Fatalf("runMigrate with no config returned error: %v", err)
	}
}
