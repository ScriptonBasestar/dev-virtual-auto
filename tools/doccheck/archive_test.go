package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type archiveCard struct{ path, body string }

// archiveFixture runs the whole gate over a synthetic archive. Going through Check rather than
// calling checkArchiveFrontmatter directly keeps the wiring under test too: a check that is
// never reached from Check is as vacuous as one that matches nothing.
func archiveFixture(t *testing.T, cards ...archiveCard) Result {
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

// TestArchiveFrontmatter_flagsCardsDetectionWouldReject pins the guard against the shapes that
// actually reach tasks/_archive/ and fail ce's canonical detection. Without it the check could be
// defanged — by a prefix edit, or by a frontmatter parser that accepts anything — and still print
// a clean sweep, which is the vacuous-pass shape TASK-206 was filed about.
func TestArchiveFrontmatter_flagsCardsDetectionWouldReject(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "neither field — the de3f7e9 population before it was patched",
			body: "---\ntitle: \"An imported record\"\nstatus: done\n---\n\n# Body\n",
			want: "neither `id:` nor `type:`",
		},
		{
			name: "no frontmatter block at all",
			body: "# An imported record\n\nNo fence, so nothing to detect.\n",
			want: "no frontmatter block",
		},
		{
			// A substring search for "type:" would report this file healthy. It is not: the key
			// belongs to the mapping above it, and indenting one is exactly how a card stops
			// being detected while still containing the string.
			name: "type: nested under another mapping is not a top-level key",
			body: "---\ntitle: \"An imported record\"\nmetadata:\n  type: bug\n---\n\n# Body\n",
			want: "neither `id:` nor `type:`",
		},
		{
			// `---` further down is a horizontal rule. Accepting it as an opening fence would
			// let a card pass on a field that appears only in its prose.
			name: "fence that does not start the file is a horizontal rule",
			body: "# Body\n\n---\nid: TASK-001\n---\n",
			want: "no frontmatter block",
		},
		{
			// The one-byte false pass this guard shipped with, found in review. A single
			// trailing space on the closing fence made the scan walk past it and take the
			// next horizontal rule as the close, so the fenced yaml example in the body was
			// read as frontmatter and this card passed on an `id:` that is documentation.
			// Its real frontmatter is title+status, and ce rejects it. Trimming both fences
			// is what closes the hole; this case is what keeps it closed.
			name: "closing fence with a trailing space does not leak the body",
			body: "---\ntitle: \"x\"\nstatus: done\n--- \n\n# Body\n\n```yaml\nid: TASK-999\ntype: bug\n```\n\n---\n\nMore body.\n",
			want: "neither `id:` nor `type:`",
		},
		{
			// The same leak with the bait outside a code fence, which is what makes this case
			// worth its own line: the one above is caught by stripFencedRegions even with the
			// fence trim reverted — measured, it stays green on that mutation — so it pins the
			// two defences only together. Here nothing but trimming the closing fence stands
			// between the scan and a body line that reads like a key.
			name: "trailing-space fence does not leak unfenced body prose either",
			body: "---\ntitle: \"x\"\nstatus: done\n--- \n\n# Body\n\nid: TASK-999 is the field this card is about.\n\n---\n\nMore body.\n",
			want: "neither `id:` nor `type:`",
		},
		{
			// Distinct from "no frontmatter block", and the distinction is the point: this
			// card opened one. ce fails it before the field test with exactly the message
			// quoted in the diagnosis — measured on this shape and on a `...` terminator,
			// which ce also calls unterminated rather than accepting as YAML would.
			name: "frontmatter opened and never closed is diagnosed as such",
			body: "---\nid: TASK-001\ntitle: \"x\"\n\n# Body\n",
			want: "opened and never closed",
		},
		{
			// ce parses this and accepts it; a line-based reader cannot. The guard is louder
			// than ce here by choice, but it says the true thing about why — reporting that
			// this card "carries neither field" would be a false statement about a card that
			// carries both.
			name: "flow mapping is reported as unreadable, not as missing fields",
			body: "---\n{id: TASK-001, title: \"x\"}\n---\n\n# Body\n",
			want: "flow mapping",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := archiveFixture(t, archiveCard{"tasks/_archive/001-x.md", tt.body})
			if res.ArchiveCards != 1 {
				t.Fatalf("archive_cards=%d, want 1 — the sweep did not reach the fixture", res.ArchiveCards)
			}
			if res.ArchiveMissing != 1 {
				t.Fatalf("archive_missing=%d, want 1; detail=%v", res.ArchiveMissing, res.ArchiveDetail)
			}
			if res.OK {
				t.Error("Check reported OK on a card canonical detection would reject")
			}
			if !containsAny(res.ArchiveDetail, strings.ToLower(tt.want)) {
				t.Errorf("detail %v does not mention %q", res.ArchiveDetail, tt.want)
			}
		})
	}
}

// TestArchiveFrontmatter_acceptsEitherFieldAlone is the half that keeps the guard honest in the
// other direction. Detection is satisfied by `id:` OR `type:` — measured on a real archived card
// mutated three ways — so a guard demanding `id:` would reject the eight legacy cards that ce
// itself classifies correctly, and disagree with the tool it exists to protect.
func TestArchiveFrontmatter_acceptsEitherFieldAlone(t *testing.T) {
	for _, tt := range []struct{ name, body string }{
		{"id: alone, as de3f7e9 wrote it", "---\nid: TASK-001\ntitle: \"x\"\n---\n\n# Body\n"},
		{"type: alone, which ce also accepts", "---\ntype: bug\ntitle: \"x\"\n---\n\n# Body\n"},
		{"both", "---\nid: TASK-001\ntype: bug\n---\n\n# Body\n"},
		{"id: not first in the block", "---\ntitle: \"x\"\nstatus: done\nid: TASK-001\n---\n\n# Body\n"},
		// ce reads these fields off a real YAML parse, so a quoted key arrives as the bare key
		// and is accepted — confirmed against ce-agent-kit source at 01e4dc52, where the test is
		// a map lookup on `fields["id"]`. A line-based reader that did not unquote would be
		// stricter than the tool it protects, on the imported-card case TASK-206 names.
		{"double-quoted key", "---\n\"id\": TASK-001\ntitle: \"x\"\n---\n\n# Body\n"},
		{"single-quoted key", "---\n'type': bug\ntitle: \"x\"\n---\n\n# Body\n"},
		// ce detects on `strings.TrimSpace(lines[0]) == "---"`, so it reads this card and skips
		// it as history — measured. A guard requiring the fence at column 0 would fail a card ce
		// is entirely happy with: the false-alarm direction, noisy rather than dangerous, but it
		// invites someone to "fix" a card that was never broken.
		{"indented opening fence, which ce trims too", "  ---\nid: TASK-001\ntitle: \"x\"\n---\n\n# Body\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := archiveFixture(t, archiveCard{"tasks/_archive/001-x.md", tt.body})
			if res.ArchiveCards != 1 {
				t.Fatalf("archive_cards=%d, want 1", res.ArchiveCards)
			}
			if res.ArchiveMissing != 0 {
				t.Errorf("archive_missing=%d on a card ce accepts; detail=%v", res.ArchiveMissing, res.ArchiveDetail)
			}
		})
	}
}

// TestArchiveFrontmatter_failsWhenArchiveYieldsNoCards pins the vacuity guard itself. Files under
// the prefix with none read as a card means the sweep stopped reaching the archive, which would
// otherwise print exactly what a clean archive prints.
func TestArchiveFrontmatter_failsWhenArchiveYieldsNoCards(t *testing.T) {
	res := archiveFixture(t, archiveCard{"tasks/_archive/notes.txt", "not a card\n"})
	if res.ArchiveFilesSeen == 0 {
		t.Fatal("archive_files_seen=0, want 1 — the prefix matched nothing")
	}
	if res.ArchiveCards != 0 {
		t.Fatalf("archive_cards=%d, want 0", res.ArchiveCards)
	}
	if res.OK {
		t.Error("Check reported OK on an archive that yielded no cards")
	}
	if !containsAny(res.Errors, "zero read as cards") {
		t.Errorf("errors %v do not name the vacuous sweep", res.Errors)
	}
}

// TestArchiveFrontmatter_sweepsTheRealCorpus is the binding that would catch a prefix which
// drifted away from the real archive: every fixture above builds its own tree, so all of them
// would still pass with archivePrefix pointing at a directory this repository does not have.
func TestArchiveFrontmatter_sweepsTheRealCorpus(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")

	inv, err := LoadInventory(root)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	res := Check(CheckInput{Root: root, Inventory: inv})

	// 201 archived cards on 2026-08-20, 0 carrying neither field (198 when TASK-206 was written; 8762d15
	// archived two more). The floor is well under that:
	// this test owns "the sweep still reaches the archive", not "the archive stopped shrinking".
	if res.ArchiveCards < 150 {
		t.Errorf("archive_cards=%d (from %d file(s) under %s), want at least 150 — the archive sweep no longer reaches the corpus",
			res.ArchiveCards, res.ArchiveFilesSeen, archivePrefix)
	}
	if res.ArchiveMissing > 0 {
		t.Errorf("%d archived card(s) carry neither `id:` nor `type:`:\n  %s",
			res.ArchiveMissing, strings.Join(res.ArchiveDetail, "\n  "))
	}
	t.Logf("swept %d archived card(s) from %d file(s) under %s", res.ArchiveCards, res.ArchiveFilesSeen, archivePrefix)
}

// TestArchiveFrontmatter_reportsAnUnreadableCard pins that a card under tasks/_archive/ which
// cannot be read fails this gate, and that the archive guard is one of the things failing it.
//
// It matters because ce no longer says anything here. Fixed ce decides the archive exemption
// from the path before it opens the file, so an unreadable archived card answers
// `rc=0 Skipped: archived` — true, and the only reason it gives; `--all` does not walk the
// archive either.
//
// The assertions are split because inside dva the failure is reported three times, and a
// Check-level assertion cannot tell which pass produced it. The link scan, the archive guard, and
// the card zone/status guard (TASK-287) each read every non-symlink markdown candidate under
// tasks/_archive/ independently and format their read failures identically (check.go, archive.go,
// cardstatus.go), so `res.OK` stays false even with the archive guard's error deleted — measured,
// by deleting it. The direct call is therefore the assertion that can fail; the Check-level count
// pins the duplication as a measured fact rather than an assumption, and will fail loudly if any
// source stops reporting.
//
// A readable card sits beside the broken one deliberately. Alone, the unreadable card would also
// trip the `zero read as cards` vacuity guard, and this test would pass on that error while the
// read error went unnoticed. With ArchiveCards at 1 that guard cannot fire, which the final
// assertion checks.
func TestArchiveFrontmatter_reportsAnUnreadableCard(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not gate reads on windows")
	}
	// Root bypasses DAC permission checks, so chmod 000 leaves the file readable and the test
	// would assert against a condition it never established — green, and measuring nothing.
	// Containerised CI runs as root by default, which is exactly where this would go unnoticed.
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 000 cannot make the fixture unreadable")
	}

	const (
		broken = "tasks/_archive/206-unreadable.md"
		intact = "tasks/_archive/206-readable.md"
	)
	root := t.TempDir()
	writeFile(t, root, "docs/a.md", "# A\n\nSee [self](a.md).\n")
	writeFile(t, root, intact, "---\nid: TASK-206\nstatus: done\n---\n\n# Readable\n")
	writeFile(t, root, broken, "---\nid: TASK-206\nstatus: done\n---\n\n# Unreadable\n")

	full := filepath.Join(root, filepath.FromSlash(broken))
	if err := os.Chmod(full, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// t.TempDir removes the file via its parent directory, which is still writable, but restore
	// the mode anyway so cleanup cannot depend on that detail.
	t.Cleanup(func() { _ = os.Chmod(full, 0o644) })

	// Verify the precondition instead of trusting the chmod: some filesystems ignore permission
	// bits, and a fixture that did not take must not read as a pass.
	if _, err := os.ReadFile(full); err == nil {
		t.Skip("filesystem ignores permission bits here; the fixture is still readable")
	}

	inv := mustInventory(t, root, "docs/a.md", intact, broken)

	// The guard on its own. This is the only assertion here that stops holding if the read-error
	// branch is softened to a bare `continue`.
	filesSeen, checked, msgs, errs := checkArchiveFrontmatter(root, inv)
	if filesSeen != 2 || checked != 1 {
		t.Fatalf("files_seen=%d checked=%d, want 2 and 1 — the unreadable file is seen but cannot be read as a card", filesSeen, checked)
	}
	if len(errs) != 1 || !strings.Contains(errs[0], broken+": read:") {
		t.Fatalf("archive guard errors %v, want exactly one naming the read failure on %s", errs, broken)
	}
	if len(msgs) != 0 {
		t.Errorf("archive guard messages %v — an unreadable card must not be judged on frontmatter it never yielded", msgs)
	}

	// The wiring, and the duplication that makes the split above necessary.
	res := Check(CheckInput{Root: root, Inventory: inv})
	n := 0
	for _, e := range res.Errors {
		if strings.Contains(e, broken+": read:") {
			n++
		}
	}
	if n != 3 {
		t.Errorf("%d error(s) name the read failure on %s, want 3 — the link scan, the archive guard, and the card zone/status guard each report it; if one stopped, this gate has less coverage than it reads as having", n, broken)
	}
	if res.OK {
		t.Error("Check reported OK over an archive holding an unreadable card")
	}
	if res.ArchiveFilesSeen != 2 || res.ArchiveCards != 1 {
		t.Errorf("archive_files_seen=%d archive_cards=%d, want 2 and 1 — Check must carry the guard's own counts, not just its errors", res.ArchiveFilesSeen, res.ArchiveCards)
	}
	if containsAny(res.Errors, "zero read as cards") {
		t.Errorf("errors %v name the vacuous sweep; this test must fail on the read error alone", res.Errors)
	}
}
