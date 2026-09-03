package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// cardZone describes one task-tracker directory and the status values a card is permitted to
// declare while it sits there.
//
// Resolved by longest matching path prefix — never by splitting the path on "/" and indexing a
// segment. tasks/_archive/done/ exists, so an nth-segment reader classifies an archived done-card
// by the "done" segment instead of the "_archive" one, checking it against the wrong permitted
// set. A peer session made exactly that mistake on 2026-09-03 and reported two closed cards
// (superseded, valid under _archive/) as dangling because it compared them to the done/ zone's
// stricter set (TASK-287).
type cardZone struct {
	prefix    string
	permitted []string
	skip      bool // true for tasks/plan/: not a card, no status: field to check
}

// cardZones is the zone table TASK-287 froze. `superseded` is permitted only under _archive/
// because every superseded card lives there and is closed work; done/ is for cards that were
// actually completed in place.
var cardZones = []cardZone{
	{prefix: "tasks/_archive/", permitted: []string{"done", "superseded"}},
	{prefix: "tasks/done/", permitted: []string{"done"}},
	{prefix: "tasks/todo/", permitted: []string{"todo"}},
	{prefix: "tasks/issue/", permitted: []string{"todo"}},
	{prefix: "tasks/plan/", skip: true},
}

// resolveCardZone returns the zone governing path, chosen by the longest matching prefix among
// cardZones. ok is false when path is not under any declared zone (e.g. tasks/README.md).
func resolveCardZone(path string) (zone cardZone, ok bool) {
	bestLen := -1
	for _, z := range cardZones {
		if strings.HasPrefix(path, z.prefix) && len(z.prefix) > bestLen {
			zone = z
			bestLen = len(z.prefix)
			ok = true
		}
	}
	return zone, ok
}

// cardStatus reads the top-level `status:` value out of frontmatter. Only a top-level key
// counts — an indented status: belongs to the mapping above it, the same reasoning
// hasCanonicalField applies to id:/type: in archive.go.
func cardStatus(frontmatter string) (value string, found bool) {
	for line := range strings.SplitSeq(stripFencedRegions(frontmatter), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if unquoteKey(strings.TrimSpace(key)) != "status" {
			continue
		}
		v := strings.TrimSpace(val)
		return strings.Trim(v, `"'`), true
	}
	return "", false
}

// checkCardStatus reports every task card whose status: is not permitted in the zone it sits in.
//
// seen counts every non-symlink file resolved to a card zone (tasks/plan/ excluded, since it is
// not a card zone), regardless of whether it is markdown or readable; checked counts cards
// actually evaluated. The pair lets the caller distinguish "no card zone exists in this tree" —
// seen==0, a legitimate state for a fixture that builds no tasks/ directory at all, mirroring
// ArchiveFilesSeen's reasoning in archive.go — from "the zone walk found files but the sweep
// never evaluated any of them", seen>0 && checked==0, which is the walk being broken (TASK-287).
func checkCardStatus(root string, inv []InventoryEntry) (seen, checked, mismatches int, msgs, errs []string) {
	for _, e := range inv {
		if isSymlinkMode(e.Mode) {
			continue
		}
		zone, ok := resolveCardZone(e.Path)
		if !ok || zone.skip {
			continue
		}
		seen++
		if !isMarkdownPath(e.Path) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e.Path)))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: read: %v", e.Path, err))
			continue
		}
		checked++
		frontmatter, state := splitFrontmatter(string(data))
		permitted := strings.Join(zone.permitted, "|")
		if state != frontmatterOK {
			mismatches++
			msgs = append(msgs, fmt.Sprintf("%s: zone %s permits status: %s, but %s",
				e.Path, zone.prefix, permitted, malformedFrontmatterReason(state)))
			continue
		}
		status, found := cardStatus(frontmatter)
		if !found {
			mismatches++
			msgs = append(msgs, fmt.Sprintf("%s: zone %s requires a status: field (permitted: %s), but frontmatter carries none",
				e.Path, zone.prefix, permitted))
			continue
		}
		if !slices.Contains(zone.permitted, status) {
			mismatches++
			msgs = append(msgs, fmt.Sprintf("%s: zone %s permits status: %s, found %q",
				e.Path, zone.prefix, permitted, status))
		}
	}
	return seen, checked, mismatches, msgs, errs
}
