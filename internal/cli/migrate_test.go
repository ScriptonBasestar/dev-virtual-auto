package cli

import (
	"os"
	"path/filepath"
	"testing"
)



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

	if err := migrateDva(dvaPath, false); err != nil {
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

	if err := migrateDva(dvaPath, false); err != nil {
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

	if err := migrateDva(dvaPath, false); err != nil {
		t.Fatalf("migrateDva returned error: %v", err)
	}
}



func TestRunMigrate_NoConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	// No dva.yml → should not error
	if err := runMigrate(nil, nil); err != nil {
		t.Fatalf("runMigrate with no config returned error: %v", err)
	}
}
