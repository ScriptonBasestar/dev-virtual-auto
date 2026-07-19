package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// TASK-051 Phase 1: source: field on stack entries.

func TestStackEntry_ParsesGitSource(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  postgres:
    default_runner: compose
    source:
      git: https://example.com/shared-infra.git
      ref: v1.2.0
    runners:
      compose:
        files:
          - docker-compose.yml
`)
	e := sortedEntry(t, cfg, "postgres")
	if e.Source == nil {
		t.Fatal("Source is nil, want parsed git source")
	}
	if e.Source.Git != "https://example.com/shared-infra.git" {
		t.Errorf("Source.Git = %q", e.Source.Git)
	}
	if e.Source.Ref != "v1.2.0" {
		t.Errorf("Source.Ref = %q, want v1.2.0", e.Source.Ref)
	}
	if !e.Source.IsGit() {
		t.Error("IsGit() = false, want true")
	}
}

func TestStackEntry_ParsesPathSource(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  postgres:
    default_runner: compose
    source:
      path: ../shared-infra
    runners:
      compose:
        files:
          - docker-compose.yml
`)
	e := sortedEntry(t, cfg, "postgres")
	if e.Source == nil {
		t.Fatal("Source is nil, want parsed path source")
	}
	if e.Source.Path != "../shared-infra" {
		t.Errorf("Source.Path = %q, want ../shared-infra", e.Source.Path)
	}
	if e.Source.IsGit() {
		t.Error("IsGit() = true, want false for path source")
	}
}

func TestStackEntry_NoSourceIsNil(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  web:
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
`)
	e := sortedEntry(t, cfg, "web")
	if e.Source != nil {
		t.Errorf("Source = %+v, want nil when source: is absent", e.Source)
	}
}

func TestSourceDir(t *testing.T) {
	cfgDir := filepath.Join(string(filepath.Separator), "home", "user", "project")

	t.Run("git uses source cache dir", func(t *testing.T) {
		got, err := SourceDir(&SourceConfig{Git: "https://x/r.git"}, "postgres", cfgDir)
		if err != nil {
			t.Fatalf("SourceDir: %v", err)
		}
		want := filepath.Join(cfgDir, DotDirName, "sources", "postgres")
		if got != want {
			t.Errorf("SourceDir = %q, want %q", got, want)
		}
	})

	t.Run("relative path joins cfgDir", func(t *testing.T) {
		got, err := SourceDir(&SourceConfig{Path: "../shared-infra"}, "pg", cfgDir)
		if err != nil {
			t.Fatalf("SourceDir: %v", err)
		}
		want := filepath.Join(cfgDir, "../shared-infra")
		if got != want {
			t.Errorf("SourceDir = %q, want %q", got, want)
		}
	})

	t.Run("absolute path passes through", func(t *testing.T) {
		got, err := SourceDir(&SourceConfig{Path: "/opt/infra/pg"}, "pg", cfgDir)
		if err != nil {
			t.Fatalf("SourceDir: %v", err)
		}
		if got != "/opt/infra/pg" {
			t.Errorf("SourceDir = %q, want /opt/infra/pg", got)
		}
	})

	t.Run("path resolving to cfgDir is refused", func(t *testing.T) {
		_, err := SourceDir(&SourceConfig{Path: "."}, "bad", cfgDir)
		if err == nil {
			t.Fatal("expected error when path resolves to cfgDir")
		}
		if !strings.Contains(err.Error(), "project directory") {
			t.Errorf("error = %q, want mention of project directory", err)
		}
	})

	t.Run("nil source errors", func(t *testing.T) {
		if _, err := SourceDir(nil, "x", cfgDir); err == nil {
			t.Fatal("expected error for nil source")
		}
	})
}

// TASK-051 Phase 4: infra: → stack: migration shim.

func TestMigrateInfraToStack_Git(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
infra:
  postgres:
    git: https://example.com/pg.git
    ref: v1.2.0
`)
	e := sortedEntry(t, cfg, "postgres")
	if e.Source == nil {
		t.Fatal("migrated entry has no source")
	}
	if e.Source.Git != "https://example.com/pg.git" || e.Source.Ref != "v1.2.0" {
		t.Errorf("source = %+v, want git+ref from infra:", e.Source)
	}
	if !hasTag(e.Tags, "infra") {
		t.Errorf("migrated entry tags = %v, want to include 'infra'", e.Tags)
	}
	if e.DetectPlugin() != "compose" {
		t.Errorf("migrated entry plugin = %q, want compose", e.DetectPlugin())
	}
	if e.Order != legacyInfraOrderBase {
		t.Errorf("migrated entry order = %d, want %d", e.Order, legacyInfraOrderBase)
	}
}

func TestLoadRejectsNonComposeSource(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.0"
stack:
  external:
    plugin: helm
    source:
      path: ../external
    helm:
      chart: ./chart
`
	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(tmpDir)
	if err == nil || !strings.Contains(err.Error(), "only the compose runner") {
		t.Fatalf("Load() error = %v, want compose-only source rejection", err)
	}
}

func TestMigrateInfraToStack_Path(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
infra:
  shared:
    path: ../shared-infra
`)
	e := sortedEntry(t, cfg, "shared")
	if e.Source == nil || e.Source.Path != "../shared-infra" {
		t.Errorf("source = %+v, want path from infra:", e.Source)
	}
}

func TestMigrateInfraToStack_PathWinsOverGit(t *testing.T) {
	// Legacy infraServiceLocation preferred path when both were set.
	cfg := loadStackConfig(t, `version: "0.1.0"
infra:
  svc:
    git: https://example.com/x.git
    path: ../local
`)
	e := sortedEntry(t, cfg, "svc")
	if e.Source.Path != "../local" || e.Source.Git != "" {
		t.Errorf("source = %+v, want path to win over git", e.Source)
	}
}

func TestMigrateInfraToStack_NameCollisionErrors(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.0"
stack:
  postgres:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
infra:
  postgres:
    git: https://example.com/pg.git
`
	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("expected error on infra/stack name collision")
	}
	if !strings.Contains(err.Error(), "conflicts") {
		t.Errorf("error = %q, want mention of conflict", err)
	}
}

func TestSourceConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		src     *SourceConfig
		wantErr string // substring; "" means no error
	}{
		{"nil", nil, ""},
		{"git only", &SourceConfig{Git: "https://x/r.git"}, ""},
		{"git with ref", &SourceConfig{Git: "https://x/r.git", Ref: "main"}, ""},
		{"path only", &SourceConfig{Path: "../infra"}, ""},
		{"neither", &SourceConfig{}, "either 'git' or 'path'"},
		{"both", &SourceConfig{Git: "https://x/r.git", Path: "../infra"}, "mutually exclusive"},
		{"path with ref", &SourceConfig{Path: "../infra", Ref: "main"}, "only valid with 'git'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.src.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
