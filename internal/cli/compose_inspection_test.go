package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestDetectConfigDriftWarnings_LiteralTopLevelServiceMatches(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	writeComposeFixture(t, filepath.Join(tmpDir, "compose.yaml"), "services:\n  app:\n    image: nginx\n")
	c := loadComposeInteractionFixture(t, composeInteractionFixture{dir: tmpDir, service: "app", composeFile: "compose.yaml"})

	// When
	warnings := detectConfigDriftWarnings(c)

	// Then
	if len(warnings) != 0 {
		t.Fatalf("expected literal top-level service to match, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_SymlinkAliasRepresentsOneComposeFile(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	writeComposeFixture(t, filepath.Join(tmpDir, "compose.yaml"), "services:\n  app:\n    image: nginx\n")
	if err := os.Symlink("compose.yaml", filepath.Join(tmpDir, "docker-compose.yml")); err != nil {
		t.Fatalf("symlink docker-compose.yml: %v", err)
	}
	c := loadComposeInteractionFixture(t, composeInteractionFixture{dir: tmpDir, service: "app", composeFile: "docker-compose.yml"})

	// When
	warnings := detectConfigDriftWarnings(c)

	// Then
	for _, warning := range warnings {
		if strings.Contains(warning, "compose.files") {
			t.Fatalf("expected symlink alias to represent one compose file, got %v", warnings)
		}
	}
}

func TestDetectConfigDriftWarnings_InteractionServiceFromIncludedComposeMatches(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	childDir := filepath.Join(tmpDir, "child")
	if err := os.Mkdir(childDir, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	writeComposeFixture(t, filepath.Join(tmpDir, "compose.yaml"), "include:\n  - ./child/compose.yaml\nservices:\n  root:\n    image: nginx\n")
	writeComposeFixture(t, filepath.Join(childDir, "compose.yaml"), "services:\n  shell-worker:\n    image: alpine\n")
	c := loadComposeInteractionFixture(t, composeInteractionFixture{dir: tmpDir, service: "shell-worker", composeFile: "compose.yaml"})

	// When
	warnings := detectConfigDriftWarnings(c)

	// Then
	if warningsContainMissingService(warnings, "shell-worker") {
		t.Fatalf("expected included compose service to match interaction, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_MissingIncludeWarnsForInteractionService(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	writeComposeFixture(t, filepath.Join(tmpDir, "compose.yaml"), "include:\n  - ./missing.yaml\nservices:\n  root:\n    image: nginx\n")
	c := loadComposeInteractionFixture(t, composeInteractionFixture{dir: tmpDir, service: "missing-service", composeFile: "compose.yaml"})

	// When
	warnings := detectConfigDriftWarnings(c)

	// Then
	if !warningsContainMissingService(warnings, "missing-service") {
		t.Fatalf("expected missing included service warning, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_MalformedIncludeWarnsForInteractionService(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	writeComposeFixture(t, filepath.Join(tmpDir, "compose.yaml"), "include: 42\nservices:\n  root:\n    image: nginx\n")
	c := loadComposeInteractionFixture(t, composeInteractionFixture{dir: tmpDir, service: "missing-service", composeFile: "compose.yaml"})

	// When
	warnings := detectConfigDriftWarnings(c)

	// Then
	if !warningsContainMissingService(warnings, "missing-service") {
		t.Fatalf("expected malformed include to preserve missing-service warning, got %v", warnings)
	}
}

func TestDetectConfigDriftWarnings_IncludeCycleTerminatesAndFindsService(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	childDir := filepath.Join(tmpDir, "child")
	if err := os.Mkdir(childDir, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	writeComposeFixture(t, filepath.Join(tmpDir, "compose.yaml"), "include:\n  - ./child/compose.yaml\nservices:\n  root:\n    image: nginx\n")
	writeComposeFixture(t, filepath.Join(childDir, "compose.yaml"), "include:\n  - ../compose.yaml\nservices:\n  worker:\n    image: alpine\n")
	c := loadComposeInteractionFixture(t, composeInteractionFixture{dir: tmpDir, service: "worker", composeFile: "compose.yaml"})

	// When
	warnings := detectConfigDriftWarnings(c)

	// Then
	if warningsContainMissingService(warnings, "worker") {
		t.Fatalf("expected service through include cycle to match, got %v", warnings)
	}
}

func writeComposeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

type composeInteractionFixture struct {
	dir         string
	service     string
	composeFile string
}

func loadComposeInteractionFixture(t *testing.T, fixture composeInteractionFixture) *config.Config {
	t.Helper()
	dvaYAML := `version: "0.1.44"
stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files: [` + fixture.composeFile + `]
interaction:
  shell:
    service: ` + fixture.service + `
    command: /bin/sh
`
	if err := os.WriteFile(filepath.Join(fixture.dir, config.FileName), []byte(dvaYAML), 0o644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}
	c, err := config.Load(fixture.dir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return c
}

func warningsContainMissingService(warnings []string, service string) bool {
	needle := `references compose service "` + service + `"`
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}
