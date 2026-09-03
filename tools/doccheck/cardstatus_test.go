package main

import (
	"strings"
	"testing"
)

// cardFixture runs the whole gate over a synthetic tasks/ tree. Going through Check rather than
// calling checkCardStatus directly keeps the wiring under test too, the same reasoning
// archiveFixture applies in archive_test.go.
func cardFixture(t *testing.T, cards ...archiveCard) Result {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "docs/a.md", "# A\n\nSee [self](a.md).\n")
	paths := []string{"docs/a.md"}
	for _, c := range cards {
		writeFile(t, root, c.path, c.body)
		paths = append(paths, c.path)
	}
	return Check(CheckInput{Root: root, Inventory: mustInventory(t, root, paths...)})
}

// TestZoneResolvesArchiveDoneAsArchive pins the longest-prefix rule against the exact collision
// TASK-287 names: tasks/_archive/done/ exists, and an nth-segment reader takes its "done"
// segment as the zone instead of "_archive", checking a closed card against the stricter done/
// set. A card here declares status: superseded, valid only under _archive/; if the zone were
// misresolved as done/ (permits only "done") this would wrongly fail.
func TestZoneResolvesArchiveDoneAsArchive(t *testing.T) {
	zone, ok := resolveCardZone("tasks/_archive/done/001-x.md")
	if !ok {
		t.Fatal("tasks/_archive/done/001-x.md resolved to no zone")
	}
	if zone.prefix != "tasks/_archive/" {
		t.Fatalf("zone prefix = %q, want tasks/_archive/ — an archived done-card must not be classified by its done/ segment", zone.prefix)
	}

	res := cardFixture(t, archiveCard{
		path: "tasks/_archive/done/001-x.md",
		body: "---\nid: TASK-001\nstatus: superseded\n---\n\n# Body\n",
	})
	if res.CardsChecked != 1 {
		t.Fatalf("cards_checked=%d, want 1", res.CardsChecked)
	}
	if res.StatusMismatches != 0 {
		t.Errorf("status_mismatches=%d on a card valid under its real zone (_archive/); detail=%v", res.StatusMismatches, res.CardStatusDetail)
	}
	if !res.OK {
		t.Errorf("Check reported FAIL on a card whose status is permitted in its longest-matching zone; errors=%v", res.Errors)
	}
}

// TestCardStatusMismatchFailsWithNamedFile pins that a card whose status: is not permitted in
// its zone is reported by file, found status, and the permitted set, and that Check exits
// non-OK.
func TestCardStatusMismatchFailsWithNamedFile(t *testing.T) {
	const path = "tasks/done/002-x.md"
	res := cardFixture(t, archiveCard{
		path: path,
		body: "---\nid: TASK-002\nstatus: todo\n---\n\n# Body\n",
	})
	if res.CardsChecked != 1 {
		t.Fatalf("cards_checked=%d, want 1", res.CardsChecked)
	}
	if res.StatusMismatches != 1 {
		t.Fatalf("status_mismatches=%d, want 1; detail=%v", res.StatusMismatches, res.CardStatusDetail)
	}
	if res.OK {
		t.Error("Check reported OK on a card whose status: is not permitted in its zone")
	}
	if !containsAny(res.CardStatusDetail, strings.ToLower(path)) {
		t.Errorf("detail %v does not name the file %q", res.CardStatusDetail, path)
	}
	if !containsAny(res.CardStatusDetail, `found "todo"`) {
		t.Errorf("detail %v does not name the found status", res.CardStatusDetail)
	}
	if !containsAny(res.CardStatusDetail, "permits status: done") {
		t.Errorf("detail %v does not name the permitted set for tasks/done/", res.CardStatusDetail)
	}
}

// TestMissingStatusIsErrorAndPlansAreSkipped covers both halves of the completion criterion in
// one test: a card zone missing status: entirely is a mismatch, and tasks/plan/ is skipped
// without being counted as a card at all.
func TestMissingStatusIsErrorAndPlansAreSkipped(t *testing.T) {
	res := cardFixture(t,
		archiveCard{path: "tasks/todo/003-x.md", body: "---\nid: TASK-003\ntitle: \"no status field\"\n---\n\n# Body\n"},
		archiveCard{path: "tasks/plan/004-x.md", body: "---\nid: PLAN-004\ntype: plan\nprogress: 3/10\n---\n\n# Plan\n"},
	)
	if res.CardsChecked != 1 {
		t.Fatalf("cards_checked=%d, want 1 — the plan card must not be counted", res.CardsChecked)
	}
	if res.StatusMismatches != 1 {
		t.Fatalf("status_mismatches=%d, want 1; detail=%v", res.StatusMismatches, res.CardStatusDetail)
	}
	if !containsAny(res.CardStatusDetail, "tasks/todo/003-x.md") {
		t.Errorf("detail %v does not name the card missing status:", res.CardStatusDetail)
	}
	if containsAny(res.CardStatusDetail, "tasks/plan/004-x.md") {
		t.Errorf("detail %v mentions the plan file, which should be skipped entirely", res.CardStatusDetail)
	}
	if res.OK {
		t.Error("Check reported OK on a card zone missing status: entirely")
	}
}

// TestZeroCardsSweptIsFailure pins the vacuous-run guard: files that resolve to a task-card zone
// but yield zero checked cards must fail the gate as a broken walk, not pass as "nothing to
// check" — the same seen-vs-checked distinction ArchiveFilesSeen/ArchiveCards already draws for
// the archive guard.
func TestZeroCardsSweptIsFailure(t *testing.T) {
	// An empty inventory resolves nothing to a zone at all — not a broken walk, since no fixture
	// here builds a tasks/ tree unrelated to this file.
	seen, checked, mismatches, msgs, errs := checkCardStatus(t.TempDir(), nil)
	if seen != 0 || checked != 0 || mismatches != 0 || len(msgs) != 0 || len(errs) != 0 {
		t.Fatalf("checkCardStatus on empty inventory = (%d, %d, %d, %v, %v), want all zero", seen, checked, mismatches, msgs, errs)
	}

	// A non-markdown file under a card zone: seen>0 (the walk reached the zone) but checked==0
	// (nothing there could be read as a card) — the broken-walk shape this guard exists to catch.
	root := t.TempDir()
	writeFile(t, root, "docs/a.md", "# A\n\nSee [self](a.md).\n")
	writeFile(t, root, "tasks/todo/notes.txt", "not a card\n")
	res := Check(CheckInput{Root: root, Inventory: mustInventory(t, root, "docs/a.md", "tasks/todo/notes.txt")})
	if res.CardsSeen == 0 {
		t.Fatal("cards_seen=0, want 1 — the zone prefix matched nothing")
	}
	if res.CardsChecked != 0 {
		t.Fatalf("cards_checked=%d, want 0", res.CardsChecked)
	}
	if res.OK {
		t.Error("Check reported OK with files seen in a card zone but zero read as cards")
	}
	if !containsAny(res.Errors, "vacuous") {
		t.Errorf("errors %v do not name the vacuous sweep", res.Errors)
	}
}

// TestCardStatus_acceptsPermittedStatusPerZone is the accepting half: every zone's permitted
// status values must pass cleanly, so the guard is not accidentally stricter than the table it
// was built from.
func TestCardStatus_acceptsPermittedStatusPerZone(t *testing.T) {
	res := cardFixture(t,
		archiveCard{path: "tasks/todo/010-a.md", body: "---\nid: TASK-010\nstatus: todo\n---\n\n# A\n"},
		archiveCard{path: "tasks/done/011-b.md", body: "---\nid: TASK-011\nstatus: done\n---\n\n# B\n"},
		archiveCard{path: "tasks/issue/012-c.md", body: "---\nid: TASK-012\nstatus: todo\n---\n\n# C\n"},
		archiveCard{path: "tasks/_archive/013-d.md", body: "---\nid: TASK-013\nstatus: done\n---\n\n# D\n"},
		archiveCard{path: "tasks/_archive/014-e.md", body: "---\nid: TASK-014\nstatus: superseded\n---\n\n# E\n"},
	)
	if res.CardsChecked != 5 {
		t.Fatalf("cards_checked=%d, want 5", res.CardsChecked)
	}
	if res.StatusMismatches != 0 {
		t.Errorf("status_mismatches=%d on all-permitted cards; detail=%v", res.StatusMismatches, res.CardStatusDetail)
	}
}
