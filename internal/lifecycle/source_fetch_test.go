package lifecycle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func runGitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=DVA Test", "GIT_AUTHOR_EMAIL=dva@example.invalid", "GIT_COMMITTER_NAME=DVA Test", "GIT_COMMITTER_EMAIL=dva@example.invalid")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func createGitSource(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	if err := os.WriteFile(filepath.Join(repo, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, repo, "add", "compose.yml")
	runGitTest(t, repo, "commit", "-m", "initial")
	return repo, runGitTest(t, repo, "rev-parse", "HEAD")
}

// TASK-051 Phase 3: ensureSource decision logic (no network).

func TestEnsureSource_NilSource(t *testing.T) {
	entry := &config.LifecycleEntry{Name: "web"}
	if err := ensureSource(entry, t.TempDir(), false, nil); err != nil {
		t.Fatalf("nil source should be a no-op, got %v", err)
	}
}

func TestEnsureSource_PathMissing(t *testing.T) {
	cfgDir := t.TempDir()
	entry := &config.LifecycleEntry{
		Name:   "shared",
		Source: &config.SourceConfig{Path: "does-not-exist"},
	}
	err := ensureSource(entry, cfgDir, false, nil)
	if err == nil {
		t.Fatal("expected error for missing path source")
	}
}

func TestEnsureSource_PathPresent(t *testing.T) {
	cfgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfgDir, "shared-infra"), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := &config.LifecycleEntry{
		Name:   "shared",
		Source: &config.SourceConfig{Path: "shared-infra"},
	}
	if err := ensureSource(entry, cfgDir, false, nil); err != nil {
		t.Fatalf("present path source should succeed, got %v", err)
	}
}

func TestEnsureSource_GitExistingCheckoutReused(t *testing.T) {
	cfgDir := t.TempDir()
	origin, _ := createGitSource(t)
	dir := filepath.Join(cfgDir, config.DotDirName, "sources", "postgres")
	runGitTest(t, cfgDir, "clone", origin, dir)
	entry := &config.LifecycleEntry{
		Name:   "postgres",
		Source: &config.SourceConfig{Git: origin},
	}
	if err := ensureSource(entry, cfgDir, false, nil); err != nil {
		t.Fatalf("existing checkout should be reused without clone, got %v", err)
	}
}

func TestEnsureSource_GitRejectsStaleCache(t *testing.T) {
	cfgDir := t.TempDir()
	dir := filepath.Join(cfgDir, config.DotDirName, "sources", "postgres")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := &config.LifecycleEntry{Name: "postgres", Source: &config.SourceConfig{Git: "https://example.com/r.git"}}
	err := ensureSource(entry, cfgDir, false, nil)
	if err == nil || !strings.Contains(err.Error(), "not a git checkout") {
		t.Fatalf("ensureSource() error = %v, want stale cache rejection", err)
	}
}

func TestEnsureSource_GitCommitSHA(t *testing.T) {
	cfgDir := t.TempDir()
	origin, commit := createGitSource(t)
	entry := &config.LifecycleEntry{Name: "postgres", Source: &config.SourceConfig{Git: origin, Ref: commit}}
	if err := ensureSource(entry, cfgDir, false, nil); err != nil {
		t.Fatalf("ensureSource() = %v", err)
	}
	dir := filepath.Join(cfgDir, config.DotDirName, "sources", "postgres")
	if head := runGitTest(t, dir, "rev-parse", "HEAD"); head != commit {
		t.Fatalf("HEAD = %s, want %s", head, commit)
	}
}

func TestRequireSource_DoesNotCloneMissingGitSource(t *testing.T) {
	cfgDir := t.TempDir()
	entry := &config.LifecycleEntry{Name: "postgres", Source: &config.SourceConfig{Git: "https://example.com/r.git"}}
	if err := requireSource(entry, cfgDir); err == nil {
		t.Fatal("requireSource() = nil, want missing source error")
	}
	dir := filepath.Join(cfgDir, config.DotDirName, "sources", "postgres")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("teardown preflight created source cache: %v", err)
	}
}

func TestEnsureSource_GitDryRunDoesNotClone(t *testing.T) {
	cfgDir := t.TempDir()
	entry := &config.LifecycleEntry{
		Name:   "postgres",
		Source: &config.SourceConfig{Git: "https://example.com/r.git", Ref: "v1"},
	}
	if err := ensureSource(entry, cfgDir, true /* dryRun */, nil); err != nil {
		t.Fatalf("dry-run should not error, got %v", err)
	}
	dir := filepath.Join(cfgDir, config.DotDirName, "sources", "postgres")
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("dry-run must not create the source directory")
	}
}
