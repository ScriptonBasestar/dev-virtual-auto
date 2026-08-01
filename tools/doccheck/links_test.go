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
