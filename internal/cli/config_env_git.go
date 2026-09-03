package cli

import (
	"os"
	"os/exec"
	"path/filepath"
)

// gitTargetState is the verdict of the §5-4 table.
type gitTargetState int

const (
	gitOutsideRepo gitTargetState = iota
	gitTracked
	gitIgnored
	gitUntrackedNotIgnored
	gitBinaryMissing
)

// gitProbe is the seam that lets the fault matrix exercise every row of §5-4
// without constructing real repositories for each one.
type gitProbe interface {
	InsideRepo(dir string) bool
	Available() bool
	Tracked(dir, target string) bool
	Ignored(dir, target string) bool
}

type realGit struct{}

// InsideRepo answers without running git, which is the whole point: when the
// answer is "inside" and git is missing, the bridge must fail closed, and it
// cannot learn that from a tool it does not have.
//
// A `.git` entry may be a directory or a file (a worktree or submodule records a
// gitdir pointer in a regular file), so existence is the test, not directory-ness.
func (realGit) InsideRepo(dir string) bool {
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func (realGit) Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Tracked and Ignored use plumbing whose exit codes the §8-1 spike pinned: 0
// means yes, anything else means no. Output is discarded — the question is
// boolean and git's stdout could name paths a diagnostic must not echo.
func (realGit) Tracked(dir, target string) bool {
	return gitQuiet(dir, "ls-files", "--cached", "--error-unmatch", "--", target)
}

func (realGit) Ignored(dir, target string) bool {
	return gitQuiet(dir, "check-ignore", "--quiet", "--", target)
}

func gitQuiet(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// bridgeGit is replaced in tests. Package-level rather than threaded through the
// preflight signature because the preflight's argument list is already the
// frozen request, and adding probes to it would let a caller pass a different
// pair for unseal than for the checks that decide whether unseal may run.
var bridgeGit gitProbe = realGit{}

// classifyGitTarget applies §5-4 in the frozen order: tracked beats ignored, and
// "inside a repo without git" beats both because it cannot rule out tracked.
func classifyGitTarget(dir, target string) gitTargetState {
	if !bridgeGit.InsideRepo(dir) {
		return gitOutsideRepo
	}
	if !bridgeGit.Available() {
		return gitBinaryMissing
	}
	if bridgeGit.Tracked(dir, target) {
		return gitTracked
	}
	if bridgeGit.Ignored(dir, target) {
		return gitIgnored
	}
	return gitUntrackedNotIgnored
}
