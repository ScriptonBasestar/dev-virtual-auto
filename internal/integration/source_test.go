//go:build integration

package integration

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-051: exercise stack source: and infra: migration through the full
// config.Load pipeline (parse -> migrate -> schema + semantic validation) on
// committed fixtures.

func TestLoadSourceStackFixture(t *testing.T) {
	c := loadFixtureConfig(t, "source-stack")

	pg := c.Stack["pg-remote"]
	if pg == nil || pg.Source == nil {
		t.Fatal("pg-remote missing or has no source")
	}
	if pg.Source.Git != "https://example.com/shared-infra.git" || pg.Source.Ref != "v1.2.0" {
		t.Errorf("pg-remote source = %+v, want git+ref", pg.Source)
	}
	if !pg.Source.IsGit() {
		t.Error("pg-remote IsGit() = false, want true")
	}

	// git source resolves to the source cache dir under the config directory.
	dir, err := config.SourceDir(pg.Source, "pg-remote", c.FileDir())
	if err != nil {
		t.Fatalf("SourceDir: %v", err)
	}
	want := filepath.Join(c.FileDir(), config.DotDirName, "sources", "pg-remote")
	if dir != want {
		t.Errorf("SourceDir = %q, want %q", dir, want)
	}

	local := c.Stack["pg-local"]
	if local == nil || local.Source == nil || local.Source.Path != "../external" {
		t.Errorf("pg-local source = %+v, want path ../external", local)
	}
	if local != nil && local.Source.IsGit() {
		t.Error("pg-local IsGit() = true, want false")
	}
}

func TestLoadInfraLegacyFixtureMigrates(t *testing.T) {
	c := loadFixtureConfig(t, "infra-legacy")

	for _, name := range []string{"postgres", "redis"} {
		e := c.Stack[name]
		if e == nil {
			t.Fatalf("infra service %q was not migrated into stack", name)
		}
		if !slices.Contains(e.Tags, "infra") {
			t.Errorf("%s tags = %v, want to include 'infra'", name, e.Tags)
		}
		if e.Source == nil {
			t.Errorf("%s has no source after migration", name)
		}
		if e.DetectPlugin() != "compose" {
			t.Errorf("%s plugin = %q, want compose", name, e.DetectPlugin())
		}
	}

	if git := c.Stack["postgres"].Source.Git; git != "https://example.com/pg.git" {
		t.Errorf("postgres source git = %q", git)
	}
	if p := c.Stack["redis"].Source.Path; p != "../redis-infra" {
		t.Errorf("redis source path = %q, want ../redis-infra", p)
	}
}
