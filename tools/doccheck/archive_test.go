package main

import (
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

	// 200 archived cards on 2026-08-20, 0 carrying neither field (198 when TASK-206 was written; 8762d15
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
