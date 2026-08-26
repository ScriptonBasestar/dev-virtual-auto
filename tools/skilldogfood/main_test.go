package main

import (
	"os"
	"path/filepath"
	"reflect"
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
	result := commandResult{Results: []destinationResult{
		{Destination: "/fixture/.agents/skills", Status: "installed"},
		{Destination: "/fixture/.claude/skills", Status: "installed"},
		{Destination: "/fixture/.grok/skills", Status: "installed"},
		{Destination: "/fixture/.opencode/skills", Status: "installed"},
	}}
	if err := requireDestinations(result, "installed", nil); err != nil {
		t.Fatalf("valid destination set rejected: %v", err)
	}
	result.Results[0].Status = "absent"
	if err := requireDestinations(result, "installed", nil); err == nil {
		t.Fatal("wrong destination status unexpectedly accepted")
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
