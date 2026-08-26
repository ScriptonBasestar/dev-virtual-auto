package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExecutableFileRequiresAbsoluteExecutableRegularFile(t *testing.T) {
	if _, err := executableFile("dva"); err == nil {
		t.Fatal("relative binary path unexpectedly accepted")
	}
	directory := t.TempDir()
	if _, err := executableFile(directory); err == nil {
		t.Fatal("directory unexpectedly accepted as executable")
	}
	file := filepath.Join(directory, "dva")
	if err := os.WriteFile(file, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := executableFile(file); err != nil || got != want {
		t.Fatalf("executableFile() = %q, %v; want %q, nil", got, err, want)
	}
}

func TestRequireDestinationStatusSet(t *testing.T) {
	project := "/fixture"
	result := commandResult{Results: []destinationResult{
		{Destination: "/fixture/.agents/skills", Status: "installed", Runtimes: []string{"antigravity", "codex"}, RuntimeStatuses: []runtimeStatus{{Runtime: "antigravity", Status: "installed"}, {Runtime: "codex", Status: "installed"}}},
		{Destination: "/fixture/.claude/skills", Status: "installed", Runtimes: []string{"claude-code"}, RuntimeStatuses: []runtimeStatus{{Runtime: "claude-code", Status: "installed"}}},
		{Destination: "/fixture/.grok/skills", Status: "installed", Runtimes: []string{"grok"}, RuntimeStatuses: []runtimeStatus{{Runtime: "grok", Status: "installed"}}},
		{Destination: "/fixture/.opencode/skills", Status: "installed", Runtimes: []string{"opencode"}, RuntimeStatuses: []runtimeStatus{{Runtime: "opencode", Status: "installed"}}},
	}}
	if err := requireDestinations(project, result, "installed"); err != nil {
		t.Fatalf("valid destination set rejected: %v", err)
	}
	result.Results[0].Status = "absent"
	if err := requireDestinations(project, result, "installed"); err == nil {
		t.Fatal("wrong destination status unexpectedly accepted")
	}
	result.Results[0].Status = "installed"
	result.Results[0].RuntimeStatuses[0].Status = "absent"
	if err := requireDestinations(project, result, "installed"); err == nil {
		t.Fatal("wrong runtime status unexpectedly accepted")
	}
}

func TestWithEnvironmentReplacesExistingValuesOnce(t *testing.T) {
	got := withEnvironment([]string{"HOME=/old", "PATH=/bin", "HOME=/older"}, map[string]string{"HOME": "/new", "XDG_STATE_HOME": "/state"})
	want := map[string]bool{"HOME=/new": true, "PATH=/bin": true, "XDG_STATE_HOME=/state": true}
	if len(got) != len(want) {
		t.Fatalf("withEnvironment() = %v, want %d entries", got, len(want))
	}
	for _, entry := range got {
		if !want[entry] {
			t.Fatalf("unexpected environment entry %q in %v", entry, got)
		}
	}
}

func TestSnapshotRuntimePathsDetectsIgnoredSkillMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".agents", "skills", "dva", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, after) {
		t.Fatal("runtime snapshot did not detect file content mutation")
	}
}

func TestRequireEmptyDirectory(t *testing.T) {
	directory := t.TempDir()
	if err := requireEmptyDirectory(directory); err != nil {
		t.Fatalf("empty directory rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "receipt.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requireEmptyDirectory(directory); err == nil {
		t.Fatal("non-empty directory unexpectedly accepted")
	}
}

func TestRequireEnvelopeRejectsFakeOutput(t *testing.T) {
	result := commandResult{Operation: "status", DryRun: false, Scope: "project"}
	if err := requireEnvelope(result, "install", false); err == nil {
		t.Fatal("fake operation unexpectedly accepted")
	}
}

func TestImmutableExecutableCopyRejectsWrongHash(t *testing.T) {
	file := filepath.Join(t.TempDir(), "dva")
	if err := os.WriteFile(file, []byte("fake executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, cleanup, err := immutableExecutableCopy(file, strings.Repeat("0", 64))
	if cleanup != nil {
		t.Fatal("failed immutable copy unexpectedly returned cleanup")
	}
	if err == nil {
		t.Fatal("wrong binary hash unexpectedly accepted")
	}
}

func TestGitRootRejectsNestedDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := commandOutput(nil, "git", "-C", root, "init"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := gitRoot(nested); err == nil {
		t.Fatal("nested Git directory unexpectedly accepted as FLOW_ROOT")
	}
}

func TestGitRootAcceptsStableDirtyRepository(t *testing.T) {
	root := t.TempDir()
	if _, err := commandOutput(nil, "git", "-C", root, "init"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "pre-existing.txt"), []byte("user work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := gitRoot(root)
	if err != nil {
		t.Fatalf("stable dirty Git root rejected: %v", err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("gitRoot() = %q, want %q", got, want)
	}
}

func TestBuiltExecutableDogfood(t *testing.T) {
	binary := os.Getenv("DVA_DOGFOOD_BIN")
	if binary == "" {
		t.Skip("DVA_DOGFOOD_BIN is set only by the hermetic Make/CI gate")
	}
	sha, err := fileSHA256(binary)
	if err != nil {
		t.Fatalf("hash built executable: %v", err)
	}
	flowRoot := t.TempDir()
	if _, err := commandOutput(nil, "git", "-C", flowRoot, "init"); err != nil {
		t.Fatalf("initialize temporary flow repository: %v", err)
	}
	var out strings.Builder
	if err := run(binary, sha, flowRoot, &out); err != nil {
		t.Fatalf("built executable dogfood failed: %v\n%s", err, out.String())
	}
	for _, marker := range []string{
		"real_target_dry_run: passed",
		"fixture_round_trip: passed",
		"shared_runtime_unlink: passed",
	} {
		if !strings.Contains(out.String(), marker) {
			t.Errorf("dogfood output missing %q:\n%s", marker, out.String())
		}
	}
}

func TestSnapshotRuntimePathsRejectsAncestorSymlink(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, ".agents")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := snapshotRuntimePaths(root); err == nil {
		t.Fatal("ancestor symlink unexpectedly accepted")
	}
}
