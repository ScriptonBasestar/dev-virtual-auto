package main

import (
	"strings"
	"testing"
)

// Given a valid relative link target in inventory, When links are checked, Then OK.
func TestLinks_acceptsValidRelativeTarget(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	b := "docs/b.md"
	writeFile(t, root, a, "# A\n\nSee [b](b.md).\n")
	writeFile(t, root, b, "# B\n")
	inv := mustInventory(t, root, a, b)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected OK, errors=%v broken=%v", res.Errors, res.BrokenLinks)
	}
	if res.LinksChecked < 1 {
		t.Fatal("expected at least one link checked")
	}
	if res.BrokenLinks != 0 {
		t.Fatalf("broken_links=%d want 0", res.BrokenLinks)
	}
}

// Given a missing relative target, When links are checked, Then broken.
func TestLinks_failsWhenTargetMissing(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	writeFile(t, root, a, "# A\n\nSee [missing](nope.md).\n")
	inv := mustInventory(t, root, a)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected failure for missing target")
	}
	if res.BrokenLinks != 1 {
		t.Fatalf("broken_links=%d want 1", res.BrokenLinks)
	}
}

// Given a broken relative link outside docs/workflows (e.g. tasks/), When
// Check runs, Then it is detected — link scan is repository-wide (TASK-090).
func TestLinks_detectsBrokenOutsideDocsWorkflows(t *testing.T) {
	root := t.TempDir()
	task := "tasks/todo/sample.md"
	writeFile(t, root, task, "# Sample\n\nSee [gone](../nope-missing.md).\n")
	docs := "docs/ok.md"
	writeFile(t, root, docs, "# Ok\n\n[self](ok.md).\n")
	inv := mustInventory(t, root, task, docs)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected broken link outside docs/workflows to fail the gate")
	}
	if res.BrokenLinks < 1 {
		t.Fatalf("broken_links=%d want >=1; detail=%v", res.BrokenLinks, res.BrokenDetail)
	}
	found := false
	for _, d := range res.BrokenDetail {
		if strings.Contains(d, task) || strings.Contains(d, "nope-missing") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected broken detail for %s, got %v", task, res.BrokenDetail)
	}
	if res.MarkdownChecked != res.MarkdownCandidates-res.SymlinksSkipped {
		t.Fatalf("markdown_checked=%d want candidates-symlinks=%d",
			res.MarkdownChecked, res.MarkdownCandidates-res.SymlinksSkipped)
	}
}

// Given a link to a missing heading anchor, When links are checked, Then broken.
func TestLinks_failsWhenAnchorMissing(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	b := "docs/b.md"
	writeFile(t, root, a, "# A\n\nSee [b](b.md#no-such-heading).\n")
	writeFile(t, root, b, "# B\n\n## Real Heading\n")
	inv := mustInventory(t, root, a, b)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected failure for missing anchor")
	}
	if res.BrokenLinks != 1 {
		t.Fatalf("broken_links=%d want 1", res.BrokenLinks)
	}
}

// Given a link to an existing heading anchor, When links are checked, Then OK.
func TestLinks_acceptsValidAnchor(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	b := "docs/b.md"
	writeFile(t, root, a, "# A\n\nSee [b](b.md#real-heading).\n")
	writeFile(t, root, b, "# B\n\n## Real Heading\n")
	inv := mustInventory(t, root, a, b)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected OK, errors=%v broken=%v", res.Errors, res.BrokenLinks)
	}
	if res.BrokenLinks != 0 {
		t.Fatalf("broken_links=%d want 0", res.BrokenLinks)
	}
}

// Given a broken-looking path only inside fenced or inline code, When links
// are checked, Then it is not reported broken.
func TestLinks_suppressesCodeFencedAndInline(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	content := "# A\n\n" +
		"Inline `](missing.md)` noise.\n\n" +
		"```\n" +
		"[fake](also-missing.md)\n" +
		"```\n\n" +
		"Real [ok](a.md).\n"
	writeFile(t, root, a, content)
	inv := mustInventory(t, root, a)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected OK (code suppressed), errors=%v broken=%v detail=%v",
			res.Errors, res.BrokenLinks, res.BrokenDetail)
	}
	if res.BrokenLinks != 0 {
		t.Fatalf("broken_links=%d want 0", res.BrokenLinks)
	}
	if res.LinksChecked < 1 {
		t.Fatal("expected the real link to be checked")
	}
}

// Given same-file anchor only, When checked, Then resolves against own headings.
func TestLinks_sameFileAnchor(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	writeFile(t, root, a, "# Top\n\n## Section One\n\nJump [here](#section-one).\n")
	inv := mustInventory(t, root, a)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected OK, errors=%v broken=%v", res.Errors, res.BrokenLinks)
	}
}

// Given external and mailto links, When checked, Then they are ignored.
func TestLinks_ignoresExternalAndMailto(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	writeFile(t, root, a, "# A\n\n[web](https://example.com) [mail](mailto:a@b.c) [ok](a.md).\n")
	inv := mustInventory(t, root, a)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected OK, errors=%v broken=%v", res.Errors, res.BrokenLinks)
	}
	if res.LinksChecked != 1 {
		t.Fatalf("links_checked=%d want 1 (only relative)", res.LinksChecked)
	}
}

// Given the only matching heading is inside a fenced block, When a link
// targets that anchor, Then the link is broken (fenced headings are not anchors).
func TestLinks_failsWhenAnchorOnlyInsideFence(t *testing.T) {
	root := t.TempDir()
	a := "docs/a.md"
	body := "# Top\n\n" +
		"```\n" +
		"## Secret Heading\n" +
		"```\n\n" +
		"Jump [here](#secret-heading).\n"
	writeFile(t, root, a, body)
	inv := mustInventory(t, root, a)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected failure: fenced heading must not create an anchor")
	}
	if res.BrokenLinks != 1 {
		t.Fatalf("broken_links=%d want 1 detail=%v", res.BrokenLinks, res.BrokenDetail)
	}
}

// Given a task link whose target moved to another state directory, When links are checked, Then
// it resolves to the file's actual location — a task's identity is its number, its directory is
// its (changeable) state (TASK-143).
func TestLinks_resolvesMovedTaskLink(t *testing.T) {
	root := t.TempDir()
	referrer := "tasks/_archive/113-old.md"
	moved := "tasks/done/153-app-up.md"
	writeFile(t, root, referrer, "# 113\n\nSee [153](../todo/153-app-up.md).\n")
	writeFile(t, root, moved, "# 153\n")
	inv := mustInventory(t, root, referrer, moved)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected the moved task link to resolve; errors=%v detail=%v", res.Errors, res.BrokenDetail)
	}
}

// Given a task link whose basename matches nothing under tasks/, When links are checked, Then it
// stays broken — resolution is not a free pass, a genuinely missing task still fails (TASK-143).
func TestLinks_taskLinkGenuinelyMissingStaysBroken(t *testing.T) {
	root := t.TempDir()
	referrer := "tasks/todo/100-ref.md"
	writeFile(t, root, referrer, "# 100\n\nSee [gone](../done/999-not-anywhere.md).\n")
	inv := mustInventory(t, root, referrer)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected a genuinely missing task link to stay broken")
	}
	if res.BrokenLinks != 1 {
		t.Fatalf("broken_links=%d want 1; detail=%v", res.BrokenLinks, res.BrokenDetail)
	}
}

// Given a task link whose basename matches more than one file under tasks/, When links are
// checked, Then it fails as ambiguous — the checker refuses to guess which one the link meant
// (TASK-143).
func TestLinks_ambiguousTaskLinkIsAnError(t *testing.T) {
	root := t.TempDir()
	referrer := "tasks/todo/100-ref.md"
	// The link points into _archive/ (absent); the basename lives in BOTH todo/ and done/.
	writeFile(t, root, referrer, "# 100\n\nSee [dup](../_archive/200-dup.md).\n")
	writeFile(t, root, "tasks/todo/200-dup.md", "# 200 todo\n")
	writeFile(t, root, "tasks/done/200-dup.md", "# 200 done\n")
	inv := mustInventory(t, root, referrer, "tasks/todo/200-dup.md", "tasks/done/200-dup.md")

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected an ambiguous task link to fail the gate")
	}
}

// Given a task path inside inline code (a verify: binding) whose target moved to another state
// directory, When Check runs, Then it resolves — the link scan cannot see code, so without this
// the binding would rot silently when the task is archived (TASK-143).
func TestLinks_resolvesMovedInlineCodeTaskPath(t *testing.T) {
	root := t.TempDir()
	referrer := "tasks/todo/100-ref.md"
	moved := "tasks/done/153-app-up.md"
	writeFile(t, root, referrer, "# 100\n\n[self](100-ref.md)\n\nverify: `grep -c x tasks/todo/153-app-up.md`\n")
	writeFile(t, root, moved, "# 153\n")
	inv := mustInventory(t, root, referrer, moved)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if !res.OK {
		t.Fatalf("expected the moved inline-code task path to resolve; detail=%v", res.BrokenDetail)
	}
}

// Given a task path inside inline code that matches nothing under tasks/, When Check runs, Then
// it fails — inline-code coverage is not a free pass, a genuinely missing binding still breaks
// the gate (TASK-143).
func TestLinks_inlineCodeTaskPathGenuinelyMissingFails(t *testing.T) {
	root := t.TempDir()
	referrer := "tasks/todo/100-ref.md"
	writeFile(t, root, referrer, "# 100\n\n[self](100-ref.md)\n\nverify: `grep -c x tasks/done/999-gone.md`\n")
	inv := mustInventory(t, root, referrer)

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected a genuinely-missing inline-code task path to fail the gate")
	}
	var names string
	for _, d := range res.BrokenDetail {
		names += d + "\n"
	}
	if !strings.Contains(names, "999-gone") {
		t.Fatalf("expected the broken detail to name the missing inline path; detail=%v", res.BrokenDetail)
	}
}

// Given a task path inside inline code whose basename matches more than one file under tasks/,
// When Check runs, Then it fails as ambiguous — the inline-code path has the same one/zero/many
// rule as a markdown link (TASK-143 review M2).
func TestLinks_ambiguousInlineCodeTaskPathIsAnError(t *testing.T) {
	root := t.TempDir()
	referrer := "tasks/todo/100-ref.md"
	writeFile(t, root, referrer, "# 100\n\n[self](100-ref.md)\n\nverify: `tasks/_archive/200-dup.md`\n")
	writeFile(t, root, "tasks/todo/200-dup.md", "# 200 todo\n")
	writeFile(t, root, "tasks/done/200-dup.md", "# 200 done\n")
	inv := mustInventory(t, root, referrer, "tasks/todo/200-dup.md", "tasks/done/200-dup.md")

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected an ambiguous inline-code task path to fail the gate")
	}
}

// Given a moved task link that carries an anchor, When Check runs, Then the anchor is validated
// against the RESOLVED file (the one the link now points at by basename), not the stale literal
// path (TASK-143 review M3).
func TestLinks_movedTaskLinkAnchorCheckedAgainstResolved(t *testing.T) {
	root := t.TempDir()
	referrer := "tasks/_archive/113-old.md"
	moved := "tasks/done/153-app-up.md"
	writeFile(t, root, referrer, "# 113\n\nSee [153 section](../todo/153-app-up.md#the-heading).\n")
	writeFile(t, root, moved, "# 153\n\n## The Heading\n")
	inv := mustInventory(t, root, referrer, moved)

	if res := Check(CheckInput{Root: root, Inventory: inv}); !res.OK {
		t.Fatalf("expected the moved link's anchor to resolve against the moved file; detail=%v", res.BrokenDetail)
	}

	// Same link, anchor absent from the resolved file → broken.
	writeFile(t, root, moved, "# 153\n\n## Some Other Heading\n")
	inv2 := mustInventory(t, root, referrer, moved)
	res2 := Check(CheckInput{Root: root, Inventory: inv2})
	if res2.OK {
		t.Fatal("expected a moved link with an absent anchor to fail")
	}
}
