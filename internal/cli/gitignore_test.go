package cli

import (
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
