package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Given an empty inventory, When Check runs, Then it fails as vacuous
// (zero candidates and zero links).
func TestCheck_failsWhenInventoryEmpty(t *testing.T) {
	res := Check(CheckInput{
		Root:      t.TempDir(),
		Inventory: nil,
	})
	if res.OK {
		t.Fatal("expected failure on empty inventory")
	}
	if res.MarkdownCandidates != 0 || res.MarkdownChecked != 0 || res.LinksChecked != 0 {
		t.Fatalf("expected zero counts, got candidates=%d checked=%d links=%d",
			res.MarkdownCandidates, res.MarkdownChecked, res.LinksChecked)
	}
	if !containsAny(res.Errors, "vacuous", "zero", "no markdown") {
		t.Fatalf("expected vacuous error, got %v", res.Errors)
	}
}

// Given a docs file over the line limit, When size is checked, Then it is oversized.
func TestSize_failsWhenOverLineLimit(t *testing.T) {
	root := t.TempDir()
	path := "docs/over-lines.md"
	writeFile(t, root, path, strings.Repeat("line\n", maxDocLines+1))
	inv := mustInventory(t, root, path)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected oversized failure")
	}
	if res.OversizedDocs != 1 {
		t.Fatalf("oversized_docs=%d want 1", res.OversizedDocs)
	}
}

// Given a docs file over the byte limit, When size is checked, Then it is oversized.
func TestSize_failsWhenOverByteLimit(t *testing.T) {
	root := t.TempDir()
	path := "docs/over-bytes.md"
	body := strings.Repeat("x", maxDocBytes+1) + "\n"
	writeFile(t, root, path, body)
	inv := mustInventory(t, root, path)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected oversized failure")
	}
	if res.OversizedDocs != 1 {
		t.Fatalf("oversized_docs=%d want 1", res.OversizedDocs)
	}
}

// Given METHODOLOGY.md over limits, When size is checked, Then it is exempt.
func TestSize_exemptsMethodology(t *testing.T) {
	root := t.TempDir()
	path := sizeExemptPath
	body := strings.Repeat("line\n", maxDocLines+50)
	if len(body) <= maxDocBytes {
		body += strings.Repeat("y", maxDocBytes-len(body)+1)
	}
	writeFile(t, root, path, body)
	other := "docs/ok.md"
	writeFile(t, root, other, "# Ok\n\nSee [self](ok.md).\n")
	inv := mustInventory(t, root, path, other)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected OK with exempt oversized methodology, errors=%v broken=%v", res.Errors, res.BrokenLinks)
	}
	if res.OversizedDocs != 0 {
		t.Fatalf("oversized_docs=%d want 0 (exempt)", res.OversizedDocs)
	}
}

// Given a tracked path deleted from the worktree, When LoadInventory runs,
// Then that path is excluded so links to it fail (no index-blob masking).
func TestLoadInventory_excludesTrackedDeletedFromWorktree(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=doccheck",
			"GIT_AUTHOR_EMAIL=doccheck@test",
			"GIT_COMMITTER_NAME=doccheck",
			"GIT_COMMITTER_EMAIL=doccheck@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-q")
	run("git", "config", "core.autocrlf", "false")

	alive := "docs/alive.md"
	gone := "tasks/todo/old.md"
	writeFile(t, root, alive, "# Alive\n\nSee [old](../tasks/todo/old.md).\n")
	writeFile(t, root, gone, "# Old\n")
	run("git", "add", "-A")
	run("git", "commit", "-q", "-m", "seed")
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(gone))); err != nil {
		t.Fatal(err)
	}

	inv, err := LoadInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range inv {
		if e.Path == gone {
			t.Fatalf("deleted tracked path %q must be excluded from inventory", gone)
		}
	}
	var hasAlive bool
	for _, e := range inv {
		if e.Path == alive {
			hasAlive = true
		}
	}
	if !hasAlive {
		t.Fatalf("expected %q still in inventory, got %+v", alive, inv)
	}

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected broken link to deleted worktree path")
	}
	if res.BrokenLinks < 1 {
		t.Fatalf("broken_links=%d want >=1 detail=%v", res.BrokenLinks, res.BrokenDetail)
	}
	found := false
	for _, d := range res.BrokenDetail {
		if strings.Contains(d, "old.md") || strings.Contains(d, gone) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected broken detail mentioning deleted path, got %v", res.BrokenDetail)
	}
}

// Given a git symlink alias and its target, When inventory is processed,
// Then the symlink is skipped and the target is checked once.
func TestInventory_skipsSymlinkAliasOnce(t *testing.T) {
	root := t.TempDir()
	target := "docs/canonical.md"
	alias := "docs/alias.md"
	writeFile(t, root, target, "# Title\n\nBody with [link](canonical.md#title).\n")
	writeFile(t, root, alias, target)

	inv := []InventoryEntry{
		{Path: target, Mode: modeRegular},
		{Path: alias, Mode: modeSymlink},
	}

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.SymlinksSkipped != 1 {
		t.Fatalf("symlinks_skipped=%d want 1", res.SymlinksSkipped)
	}
	if res.MarkdownCandidates != 2 {
		t.Fatalf("markdown_candidates=%d want 2", res.MarkdownCandidates)
	}
	if res.MarkdownChecked != 1 {
		t.Fatalf("markdown_checked=%d want 1 (canonical only)", res.MarkdownChecked)
	}
	if !res.OK {
		t.Fatalf("expected OK, errors=%v broken=%v", res.Errors, res.BrokenLinks)
	}
}
