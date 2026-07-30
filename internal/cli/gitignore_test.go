package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestIsDvaIgnored pins what "already ignored" means. This file did not exist before
// TASK-065, which is how the check shipped recognizing only a literal ".sb/dva" line: with
// no test encoding the intent, an ancestor rule like ".sb/" read as unignored and every
// command warned about a path git was already excluding.
func TestIsDvaIgnored(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		want    bool
	}{
		// The rule that was already recognized.
		{"exact path", ".sb/dva", true},
		{"exact path with trailing slash", ".sb/dva/", true},

		// The regression TASK-065 fixes: git excludes the whole subtree from an ancestor.
		{"ancestor directory", ".sb/", true},
		{"ancestor bare", ".sb", true},
		{"ancestor root-anchored", "/.sb/", true},
		{"exact path root-anchored", "/.sb/dva/", true},

		// Real .gitignore files are long and the rule is rarely first or last.
		{"ancestor among many rules", "node_modules/\n*.log\n.sb/\ndist/\n", true},
		{"indented line still counts", "  .sb/dva/  \n", true},

		// Must keep warning: nothing here excludes the directory.
		{"empty", "", false},
		{"unrelated rules only", "node_modules/\n*.log\ndist/\n", false},
		{"commented out", "# .sb/dva/\n", false},
		{"sibling directory", ".sbx/\n", false},
		{"shorter prefix that is not an ancestor", ".s/\n", false},

		// A rule *below* the dot dir ignores only that child, so the markers DVA writes
		// elsewhere in .sb/dva are still committable and the warning is correct.
		{"descendant only", ".sb/dva/cache/\n", false},

		// Negations, verified against real git with the paths on disk. Before these cases
		// the check returned on its first covering match and read every one of them as
		// ignored — suppressing the warning on the two that git does not ignore.
		{"ancestor negated after being excluded", ".sb/\n!.sb/\n", false},
		{"exact path negated after being excluded", ".sb/dva/\n!.sb/dva/\n", false},
		{"negation in another spelling still counts", "/.sb\n!.sb/\n", false},
		{"negation before the exclusion loses", "!.sb/\n.sb/\n", true},
		{"descendant negation cannot re-include", ".sb/\n!.sb/dva/\n", true},
		{"negation alone excludes nothing", "!.sb/\n", false},
		{"unrelated negation is not a match", ".sb/\n!dist/\n", true},

		// Not interpreted on purpose — documented as out of scope in TASK-065. Pinned so
		// the limitation is a decision on record rather than an accident.
		{"glob is not interpreted", ".sb/*\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDvaIgnored(tt.content); got != tt.want {
				t.Errorf("isDvaIgnored(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

// TestIgnoreRulesCoveringIsDerivedFromDotDirName guards against the set being hand-written
// for today's ".sb/dva". If DotDirName ever gains or loses a segment, the rules must follow
// it — a stale set would silently warn (or stay silent) about the wrong path.
func TestIgnoreRulesCoveringIsDerivedFromDotDirName(t *testing.T) {
	rules := ignoreRulesCovering(config.DotDirName)
	if !rules[config.DotDirName] {
		t.Errorf("rules do not cover DotDirName %q itself: %v", config.DotDirName, rules)
	}

	// Every segment boundary must be represented, so a subtree rule at any depth is honored.
	for _, want := range []string{".sb", ".sb/", ".sb/dva", ".sb/dva/"} {
		if !rules[want] {
			t.Errorf("rules missing %q (DotDirName = %q): %v", want, config.DotDirName, rules)
		}
	}

	// A single-segment dir must not produce an empty-string rule, which would match every
	// blank line in a .gitignore and report any file as ignored.
	for _, dir := range []string{"tmp", "/tmp", ""} {
		if ignoreRulesCovering(dir)[""] {
			t.Errorf("ignoreRulesCovering(%q) contains the empty rule, which matches blank lines", dir)
		}
	}
}

// TestGitignoreWarningNeedsSomethingCommittable pins the gate TASK-080 added. loadConfig calls
// this on every command that reads a config, so before the gate `dva ls` in a fresh clone printed
// two lines of hygiene advice above its own answer — about a directory that did not exist, and
// that no read-only command creates (ls, show, validate, status and manifest were each measured
// leaving the tree untouched).
//
// Every case flips exactly one condition away from the warning case, so a gate that stopped
// consulting any single condition fails here instead of passing on the strength of the others.
// The name has to contain "Gitignore": the task's acceptance criterion runs `-run Gitignore`,
// which matches neither TestIsDvaIgnored nor TestIgnoreRulesCoveringIsDerivedFromDotDirName.
func TestGitignoreWarningNeedsSomethingCommittable(t *testing.T) {
	for _, tt := range []struct {
		name      string
		git       bool
		markers   bool
		gitignore string
		json      bool
		wantWarn  bool
	}{
		{name: "markers on disk and nothing ignores them", git: true, markers: true, wantWarn: true},
		{name: "no markers written yet", git: true, markers: false, wantWarn: false},
		{name: "markers but exactly ignored", git: true, markers: true, gitignore: ".sb/dva/\n", wantWarn: false},
		{name: "markers but an ancestor is ignored", git: true, markers: true, gitignore: ".sb/\n", wantWarn: false},
		{name: "markers but not a git working tree", git: false, markers: true, wantWarn: false},
		{name: "markers but json output has no schema for it", git: true, markers: true, json: true, wantWarn: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.git {
				if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatalf("MkdirAll .git: %v", err)
				}
			}
			if tt.markers {
				if err := os.MkdirAll(filepath.Join(dir, config.DotDirName), 0o755); err != nil {
					t.Fatalf("MkdirAll %s: %v", config.DotDirName, err)
				}
			}
			if tt.gitignore != "" {
				if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(tt.gitignore), 0o644); err != nil {
					t.Fatalf("WriteFile .gitignore: %v", err)
				}
			}

			oldJSON := jsonOutput
			jsonOutput = tt.json
			defer func() { jsonOutput = oldJSON }()

			out := captureOutput(t, func() { checkGitignoreForWarning(dir) })

			if warned := strings.Contains(out, "is not in your .gitignore"); warned != tt.wantWarn {
				t.Errorf("warned = %v, want %v; output was %q", warned, tt.wantWarn, out)
			}
			// A warning that does not name the remedy is the noise this task is about. Asserted
			// only where one is expected, since elsewhere the absence is what passes.
			if tt.wantWarn && !strings.Contains(out, "dva doctor --fix") {
				t.Errorf("the warning must name the command that fixes it: %q", out)
			}
		})
	}
}
