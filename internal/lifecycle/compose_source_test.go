package lifecycle

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-051 Phase 2: sourced entries resolve compose files against, and run
// with their working directory set to, the fetched/referenced source dir.

func sourcedPctx(name string, src *config.SourceConfig, files []string) *PluginContext {
	return &PluginContext{
		Entry: &config.LifecycleEntry{
			Name:    name,
			Compose: &config.ComposePluginConfig{Files: files},
			Source:  src,
		},
		Env:       config.NewEnvironment(nil, "/tmp", "/tmp"),
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}
}

func TestComposePlugin_BuildArgs_SourcedGit(t *testing.T) {
	p := &ComposePlugin{}
	pctx := sourcedPctx("postgres", &config.SourceConfig{Git: "https://example.com/r.git", Ref: "v1"}, []string{"docker-compose.yml"})

	_, args := p.buildArgs(pctx, []string{"up", "-d"})
	joined := strings.Join(args, " ")

	wantDir := filepath.Join("/project", config.DotDirName, "sources", "postgres")
	if !strings.Contains(joined, "-f "+wantDir+"/docker-compose.yml") {
		t.Errorf("compose file not resolved against source dir.\nargs: %s\nwant -f under %s", joined, wantDir)
	}
	if got := composeWorkdir(pctx); got != wantDir {
		t.Errorf("composeWorkdir = %q, want %q", got, wantDir)
	}
}

func TestComposePlugin_BuildArgs_SourcedPath(t *testing.T) {
	p := &ComposePlugin{}
	pctx := sourcedPctx("shared", &config.SourceConfig{Path: "../shared-infra"}, []string{"docker-compose.yml"})

	_, args := p.buildArgs(pctx, []string{"up"})
	joined := strings.Join(args, " ")

	wantDir := filepath.Join("/project", "../shared-infra")
	if !strings.Contains(joined, "-f "+wantDir+"/docker-compose.yml") {
		t.Errorf("compose file not resolved against local source dir.\nargs: %s\nwant -f under %s", joined, wantDir)
	}
	if got := composeWorkdir(pctx); got != wantDir {
		t.Errorf("composeWorkdir = %q, want %q", got, wantDir)
	}
}

// A sourced entry with no explicit files relies on default file discovery in
// the source working directory (legacy `cd <dir> && docker compose` parity).
func TestComposePlugin_BuildArgs_SourcedNoFiles(t *testing.T) {
	p := &ComposePlugin{}
	pctx := sourcedPctx("postgres", &config.SourceConfig{Git: "https://example.com/r.git"}, nil)

	_, args := p.buildArgs(pctx, []string{"up"})
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "-f ") {
		t.Errorf("no explicit files should add no -f (discovery via workdir).\nargs: %s", joined)
	}
	wantDir := filepath.Join("/project", config.DotDirName, "sources", "postgres")
	if got := composeWorkdir(pctx); got != wantDir {
		t.Errorf("composeWorkdir = %q, want %q", got, wantDir)
	}
}

func TestComposePlugin_BuildArgs_NoSource_UsesConfigDir(t *testing.T) {
	p := &ComposePlugin{}
	pctx := sourcedPctx("web", nil, []string{"docker-compose.yml"})

	_, args := p.buildArgs(pctx, []string{"up"})
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "-f /project/docker-compose.yml") {
		t.Errorf("non-sourced file should resolve against ConfigDir.\nargs: %s", joined)
	}
	if got := composeWorkdir(pctx); got != "" {
		t.Errorf("composeWorkdir = %q, want empty for non-sourced entry", got)
	}
}
