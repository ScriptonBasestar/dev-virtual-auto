package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// archivePrefix is the frozen zone this guard covers. `ce task validate` has a rule that
// archived documents are history and are not maintained, but it evaluates that rule *after*
// frontmatter parsing and canonical detection have both succeeded — so a card that fails
// detection never reaches the skip, and is audited against a card format that postdates it.
// Nothing in this repository asserted the property the archive silently depends on; this does
// (TASK-206).
const archivePrefix = "tasks/_archive/"

// canonicalFields are the frontmatter keys that satisfy ce's canonical detection. Both are
// listed because detection accepts *either*, measured on one archived card mutated three ways:
// as committed it skips as history; with the single line `id:` deleted it produces five errors
// reading as unfinished current work; with `id:` deleted and `type:` added it skips again.
// Requiring `id:` alone would reject a card that ce classifies correctly — a gate whose verdict
// disagrees with the tool it exists to protect is the same class of defect as one that cannot
// fire.
var canonicalFields = []string{"id", "type"}

// splitFrontmatter returns the YAML frontmatter block of a markdown document and whether one was
// present. The opening fence must be the very first line: a `---` further down is a horizontal
// rule, and accepting it would let a card pass on a field that appears only in its prose.
func splitFrontmatter(body string) (string, bool) {
	lines := strings.Split(strings.TrimPrefix(body, "\uFEFF"), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

// hasCanonicalField reports whether the frontmatter carries a key detection accepts. Only
// top-level keys count: an indented `type:` is a property of the mapping above it, and indenting
// one is precisely how a card would stop being detected while still containing the string — so a
// substring search here would report health for the file it is meant to catch.
func hasCanonicalField(frontmatter string) bool {
	for line := range strings.SplitSeq(frontmatter, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		key, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if slices.Contains(canonicalFields, strings.TrimSpace(key)) {
			return true
		}
	}
	return false
}

// checkArchiveFrontmatter reports every archived card that carries neither `id:` nor `type:` at
// the top level of its frontmatter, and would therefore be judged as current work.
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
		frontmatter, ok := splitFrontmatter(string(data))
		if !ok {
			msgs = append(msgs, fmt.Sprintf(
				"%s: no frontmatter block — ce fails canonical detection and audits it as unfinished current work", e.Path))
			continue
		}
		if !hasCanonicalField(frontmatter) {
			msgs = append(msgs, fmt.Sprintf(
				"%s: frontmatter carries neither `id:` nor `type:` — ce fails canonical detection here and never reaches its archive skip", e.Path))
		}
	}
	return filesSeen, checked, msgs, errs
}
