package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// portConventionRule matches the rule-7/rule-5 statement wherever the corpus makes it.
// The list marker differs by file (the guardrails number it 7, the schema reference 5),
// so it is captured separately from the sentence that has to agree.
var portConventionRule = regexp.MustCompile(`^\s*\d+\.\s+(\*\*Port conventions\*\*.*)$`)

// TestPortConventionRuleStatedOnce guards a fact the generator deliberately does not own.
//
// TASK-134 decided rules 7 (forbidden ports) and 23 (naming presets) stay hand-authored:
// nothing in internal/config enforces them, so there is no behaviour for the markdown to
// drift from and a Go const would only be a copy of the prose. But hand-authored does not
// mean unowned. Rule 7 was written twice with different content — 12 ports in
// shared-guardrails.md, "5432, 6379, 8080, 3000, etc." in schema-reference.md — and
// `make generate` concatenated both into library_reference.txt, so a single flow prompt
// carried two answers to "which ports are forbidden" at lines 31 and 181.
//
// This is a consistency check, not a source of truth. It names no port numbers: it groups
// the statements by wording and fails when there is more than one group. Changing the list
// stays a one-file edit — changing it in only one file does not.
func TestPortConventionRuleStatedOnce(t *testing.T) {
	type site struct {
		path string
		line int
		text string
	}
	var found []site

	for _, root := range generatorCorpus() {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(path) {
			case ".md", ".txt":
			default:
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(content), "\n") {
				if m := portConventionRule.FindStringSubmatch(line); m != nil {
					found = append(found, site{path: path, line: i + 1, text: m[1]})
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	// Five today: shared-guardrails.md, schema-reference.md, the dva-schema.md symlink to
	// the latter, and both again in library_reference.txt. The floor is one lower so that
	// dropping the symlink or moving a corpus path is not a failure, while losing a whole
	// authored copy — which costs at least two sites — is. Without any floor, a reworded
	// heading would leave this test walking the corpus and matching nothing, forever green.
	const wantAtLeast = 4
	if len(found) < wantAtLeast {
		t.Fatalf("found %d port-convention statements, want at least %d — the rule moved or was reworded, and this test stopped guarding it",
			len(found), wantAtLeast)
	}
	t.Logf("checked %d statements of the port-convention rule", len(found))

	// Grouped by wording rather than compared against found[0]: with one file reverted,
	// pairwise comparison reports the same single divergence once per remaining copy and
	// treats whichever was walked first as correct, which it has no way to know.
	variants := map[string][]string{}
	var order []string
	for _, s := range found {
		if _, seen := variants[s.text]; !seen {
			order = append(order, s.text)
		}
		variants[s.text] = append(variants[s.text], fmt.Sprintf("%s:%d", repoPath(s.path), s.line))
	}
	if len(order) == 1 {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "the port-convention rule is stated %d different ways. All of these ship in library_reference.txt, so a flow reads them together and gets more than one answer:\n", len(order))
	for _, text := range order {
		fmt.Fprintf(&b, "\n  %s\n    %s\n", strings.Join(variants[text], ", "), text)
	}
	b.WriteString("\nPick one wording and use it in every copy — see agent-mesh-flows/shared/library/README.md.")
	t.Error(b.String())
}

// repoPath trims the absolute prefix so failures read as repo-relative paths.
func repoPath(path string) string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}
