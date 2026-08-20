package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// archivePrefix is the frozen zone this guard covers. The property asserted is that every card
// under it carries `id:` or `type:` in its frontmatter. `ce task validate` does not assert that,
// under either build of it — and the two builds fail to assert it for opposite reasons, which is
// why this comment describes both rather than whichever one happens to be installed.
//
// ce has a rule that archived documents are history and are not maintained. Before ce-agent-kit
// 7391ac64 it evaluated that rule last, which left the rule mostly unreachable. Read from source
// at 01e4dc52 (internal/usecase/task/canonical_validator.go, validateCanonicalTask), the order
// was:
//
//  1. parse; "detected" means only that line 1 trimmed is `---`
//  2. YAML error      -> hard error, returns          <- before the skip
//  3. no id, no type  -> falls through to the legacy validator
//  4. canonicalFrozenZone(path) -> "Skipped: archived..."
//
// so a card reaching step 3 was judged as unfinished current work, in a directory named
// `_archive`, against a card format that postdates it. 7391ac64 hoists the zone decision into
// Validator.Validate ahead of both the decision-doc route and validateCanonicalTask, so an
// archived card is exempt before anything reads it. Measured across both binaries: 13 cards the
// old one reported red — 10 with neither field, 3 with malformed YAML — every one of them skips
// under the new one.
//
// Fixed ce therefore does not mis-judge these cards. It also reports nothing whatsoever about
// them, which is the same fact seen from the other side: the property above was only ever
// enforced by ce's incidental red, so once the fix lands it is enforced by nothing but this file
// (TASK-206). "Current ce handles the archive correctly" is not a reason to delete this guard —
// correct-and-silent is the exact condition under which it becomes the only check left.
//
// Three boundaries, each measured, so this guard is not read as more than it is:
//
//   - It asserts field presence only. A card with `id:` and malformed YAML passes here —
//     doccheck is stdlib-only and cannot parse YAML to see it. Pre-fix ce caught that as a
//     step-2 hard error; fixed ce skips it with everything else. So it is an open gap, not a
//     division of labour, and it widens rather than closes when ce is upgraded.
//   - Archived *decision* docs (a path component `decision`/`decisions`/`adr`/`adrs`) are
//     uncovered under both builds: pre-fix, validator.go routed them away before any of this;
//     post-fix, the hoisted zone check exempts them first. That is a path rule, not a
//     frontmatter one, so a frontmatter guard cannot express it — the reason it is absent
//     here, rather than that dva has none.
//   - `ce task validate --all` excludes the archive during its tree walk, so none of this ever
//     fired there — measured, 7 files validated against 200 archived cards on disk. The old
//     misjudgement was reachable only by explicit path, which is how an audit sweep reaches it.
const archivePrefix = "tasks/_archive/"

// canonicalFields are the frontmatter keys that satisfy step 3 above. Both are listed because
// it accepts *either* — confirmed against source, where the whole test is a presence lookup for
// "id" then "type" in the parsed frontmatter map, with no third key and no inspection of the
// value. Requiring `id:` alone would reject a card ce classifies correctly, and a gate whose
// verdict disagrees with the tool it exists to protect is the same class of defect as one that
// cannot fire.
var canonicalFields = []string{"id", "type"}

// frontmatterState distinguishes the two ways a document yields no frontmatter block. The
// verdict is the same — no card can be read — but the diagnosis is the part someone acts on,
// and "no frontmatter block" is simply false about a file that opened one and never closed it.
// ce separates them too: an unopened block falls through to the legacy validator, while an
// unterminated one is a hard `Invalid YAML frontmatter: unterminated frontmatter`.
type frontmatterState int

const (
	frontmatterOK frontmatterState = iota
	frontmatterAbsent
	frontmatterUnterminated
)

// splitFrontmatter returns the YAML frontmatter block of a markdown document.
//
// The opening fence must be the very first line: a `---` further down is a horizontal rule,
// and accepting it would let a card pass on a field that appears only in its prose.
//
// Both fences are compared after trimming surrounding whitespace, which matters in both
// directions and was measured in both. ce detects on `strings.TrimSpace(lines[0]) == "---"`,
// so an *indented opening fence* is a card it skips as history and this guard used to reject.
// And a *closing fence with one trailing space* used to go unrecognised, sending this scan on
// into the body to take the next horizontal rule as the close — after which everything between
// was walked as frontmatter, and a card carrying neither field passed on an `id:` quoted
// inside a ```yaml example. That was a silent false pass one byte wide, found in review.
func splitFrontmatter(body string) (string, frontmatterState) {
	lines := strings.Split(strings.TrimPrefix(body, "\uFEFF"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", frontmatterAbsent
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), frontmatterOK
		}
	}
	return "", frontmatterUnterminated
}

// malformedFrontmatterReason names what went wrong and what ce does about it, for each way a
// document fails to yield a readable frontmatter block. The two are kept apart because the
// diagnosis is what someone acts on: an unopened block is a card ce never detects, while an
// unterminated one never even reaches detection's field test.
func malformedFrontmatterReason(state frontmatterState) string {
	if state == frontmatterUnterminated {
		return "frontmatter opened and never closed — ce fails this earlier still, as `Invalid YAML frontmatter: unterminated frontmatter`"
	}
	return "no frontmatter block — ce never detects it as a card and the legacy validator audits it as unfinished current work"
}

// flowMappingFrontmatter reports frontmatter written as a YAML flow mapping
// (`{id: TASK-1, title: ...}`). ce parses it and accepts it — measured, rc=0 with the archive
// skip — and a line-based reader cannot evaluate it. Reporting "carries neither field" about
// such a card would state something false, so it gets its own message. It still fails the
// gate: every archived card here is a block mapping, so one that is not is worth a look.
func flowMappingFrontmatter(frontmatter string) bool {
	for line := range strings.SplitSeq(frontmatter, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return strings.HasPrefix(t, "{")
		}
	}
	return false
}

// hasCanonicalField reports whether the frontmatter carries a key detection accepts. Only
// top-level keys count: an indented `type:` is a property of the mapping above it, and indenting
// one is precisely how a card would stop being detected while still containing the string — so a
// substring search here would report health for the file it is meant to catch.
func hasCanonicalField(frontmatter string) bool {
	// Blank fenced code blocks first. Well-formed frontmatter has none, so this only matters
	// once a malformed fence has already carried the scan into the body — it keeps an `id:`
	// quoted in a ```yaml example from being read as a real key. Belt to the fence fix's
	// braces, using the helper the link and anchor passes already share.
	for line := range strings.SplitSeq(stripFencedRegions(frontmatter), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		key, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if slices.Contains(canonicalFields, unquoteKey(strings.TrimSpace(key))) {
			return true
		}
	}
	return false
}

// unquoteKey strips one matching pair of surrounding quotes. YAML allows a quoted key, and ce
// reads these fields off a real YAML parse, so `"id":` reaches it as the bare key `id` and is
// accepted — measured. A line-based reader that did not unquote would reject a card ce
// classifies correctly, which is the failure this guard was written to avoid rather than
// commit. An import from another tracker, the very case TASK-206 names, is where a quoted key
// actually arrives.
func unquoteKey(key string) string {
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'') && key[len(key)-1] == key[0] {
		return key[1 : len(key)-1]
	}
	return key
}

// checkArchiveFrontmatter reports every archived card this guard cannot confirm carries `id:`
// or `type:` at the top level of its frontmatter, and which ce would therefore judge as current
// work.
//
// filesSeen counts everything under the prefix and checked counts what was read as a card, so
// the two together can distinguish "the archive is clean" from "the sweep stopped reaching it".
// A count is the only thing separating those two outcomes: both otherwise print nothing and
// exit 0.
func checkArchiveFrontmatter(root string, inv []InventoryEntry) (filesSeen, checked int, msgs, errs []string) {
	for _, e := range inv {
		if !strings.HasPrefix(e.Path, archivePrefix) || isSymlinkMode(e.Mode) {
			continue
		}
		filesSeen++
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
		if state != frontmatterOK {
			msgs = append(msgs, fmt.Sprintf("%s: %s", e.Path, malformedFrontmatterReason(state)))
			continue
		}
		if flowMappingFrontmatter(frontmatter) {
			msgs = append(msgs, fmt.Sprintf(
				"%s: frontmatter is a YAML flow mapping — ce parses it, this guard cannot; rewrite it as a block mapping", e.Path))
			continue
		}
		if !hasCanonicalField(frontmatter) {
			msgs = append(msgs, fmt.Sprintf(
				"%s: frontmatter carries neither `id:` nor `type:` — ce falls through to the legacy validator here and never reaches its archive skip", e.Path))
		}
	}
	return filesSeen, checked, msgs, errs
}
