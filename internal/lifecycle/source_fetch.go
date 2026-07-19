package lifecycle

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var fullGitCommitRE = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

// ensureSource makes a stack entry's source available before its runner runs.
//
//   - git: clone into the source cache when missing. It never pulls an existing
//     checkout — an already-fetched source is reused as-is so repeated `up`
//     runs are reproducible (TASK-051 D4). Updates are an explicit action.
//   - path: verify the referenced local directory exists.
//
// Fetching is non-interactive: it never prompts, so `dva up` stays usable by
// agents and CI (TASK-051 D5).
func ensureSource(entry *config.LifecycleEntry, cfgDir string, dryRun bool, logger *slog.Logger) error {
	src := entry.Source
	if src == nil {
		return nil
	}

	dir, err := config.SourceDir(src, entry.Name, cfgDir)
	if err != nil {
		return err
	}

	if !src.IsGit() {
		return requireSource(entry, cfgDir)
	}

	// git source: reuse an existing checkout, never auto-pull.
	if _, statErr := os.Stat(dir); statErr == nil {
		if err := validateGitCheckout(dir, src); err != nil {
			return fmt.Errorf("source cache for %q: %w", entry.Name, err)
		}
		if logger != nil {
			logger.Debug("source present, reusing checkout", "entry", entry.Name, "dir", dir)
		}
		return nil
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("checking source cache for %q: %w", entry.Name, statErr)
	}

	if dryRun {
		if logger != nil {
			logger.Info("dry-run: would clone source", "entry", entry.Name, "git", src.Git, "ref", src.Ref, "dir", dir)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("preparing source dir for %q: %w", entry.Name, err)
	}

	cloneArgs := []string{"clone"}
	if fullGitCommitRE.MatchString(src.Ref) {
		cloneArgs = append(cloneArgs, "--no-checkout")
	} else {
		cloneArgs = append(cloneArgs, "--single-branch", "--depth", "1")
		if src.Ref != "" {
			cloneArgs = append(cloneArgs, "--branch", src.Ref)
		}
	}
	cloneArgs = append(cloneArgs, src.Git, dir)

	if logger != nil {
		logger.Info("cloning source", "entry", entry.Name, "git", src.Git, "ref", src.Ref, "dir", dir)
	}
	if err := runSourceGit("", cloneArgs...); err != nil {
		return fmt.Errorf("cloning source for %q: %w", entry.Name, err)
	}
	if fullGitCommitRE.MatchString(src.Ref) {
		if err := runSourceGit(dir, "checkout", "--detach", src.Ref); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("checking out source ref for %q: %w", entry.Name, err)
		}
	}
	return nil
}

// requireSource verifies that an already-resolved source is usable without
// fetching it. Teardown paths use this so down/stop never clone implicitly.
func requireSource(entry *config.LifecycleEntry, cfgDir string) error {
	if entry.Source == nil {
		return nil
	}
	dir, err := config.SourceDir(entry.Source, entry.Name, cfgDir)
	if err != nil {
		return err
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("source for %q is unavailable at %s: %w", entry.Name, dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("source for %q is not a directory: %s", entry.Name, dir)
	}
	if entry.Source.IsGit() {
		return validateGitCheckout(dir, entry.Source)
	}
	return nil
}

func validateGitCheckout(dir string, src *config.SourceConfig) error {
	fi, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil || (!fi.IsDir() && !fi.Mode().IsRegular()) {
		return fmt.Errorf("%s is not a git checkout; remove the stale cache and retry", dir)
	}
	origin, err := sourceGitOutput(dir, "config", "--get", "remote.origin.url")
	if err != nil {
		return fmt.Errorf("reading git origin in %s: %w", dir, err)
	}
	if strings.TrimSpace(origin) != strings.TrimSpace(src.Git) {
		return fmt.Errorf("git origin mismatch in %s: have %q, want %q; remove the stale cache and retry", dir, origin, src.Git)
	}
	if strings.TrimSpace(src.Ref) == "" {
		return nil
	}
	head, err := sourceGitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("reading git HEAD in %s: %w", dir, err)
	}
	want, err := sourceGitOutput(dir, "rev-parse", src.Ref+"^{commit}")
	if err != nil {
		return fmt.Errorf("configured ref %q is not present in %s; run the explicit update or remove the stale cache", src.Ref, dir)
	}
	if head != want {
		return fmt.Errorf("git ref mismatch in %s: HEAD is %s, configured %q resolves to %s; run the explicit update or remove the stale cache", dir, head, src.Ref, want)
	}
	return nil
}

func runSourceGit(dir string, args ...string) error {
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	return c.Run()
}

func sourceGitOutput(dir string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}
