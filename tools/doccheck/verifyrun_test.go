package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixtureTests is the package the fixture markdown below binds against. Two names sharing a
// prefix are deliberate: prefix patterns are the common form in this repo's corpus, and a
// checker that compared for equality instead of matching would reject them.
const fixtureTests = `package thing

import "testing"

func TestMigrateNestedCompose(t *testing.T)  {}
func TestMigrateDuplicatesTags(t *testing.T) {}
func TestSameStringSet(t *testing.T)         {}
func TestMain(m *testing.M)                  {}
func TestingHelperNotATest(t *testing.T)     {}
`

// checkRunFixture runs the whole gate over one markdown body plus fixtureTests, and returns the
// -run findings. Going through Check rather than calling checkRunPatterns keeps the wiring under
// test too: a check that is never reached from Check is as vacuous as one that matches nothing.
func checkRunFixture(t *testing.T, body string) Result {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "docs/a.md", "# A\n\nSee [self](a.md).\n\n"+body+"\n")
	writeFile(t, root, "pkg/thing_test.go", fixtureTests)
	inv := mustInventory(t, root, "docs/a.md", "pkg/thing_test.go")
	return Check(CheckInput{Root: root, Inventory: inv})
}

// TestRunPatterns_flagsPlantedDefects pins the detector itself. Without it the guard could be
// defanged by a regex edit — or by the unquoting rules quietly agreeing with everything — and
// still report a clean sweep, which is the exact vacuous-pass shape TASK-136 was filed about.
func TestRunPatterns_flagsPlantedDefects(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "names a test that never existed — the TASK-069 defect",
			body: "verify: `go test ./pkg/ -run TestMigrateLegacyCompose`",
			want: "selects no test",
		},
		{
			name: "prefix that matches nothing",
			body: "verify: `go test ./pkg/ -run TestNothingStartsWithThis`",
			want: "selects no test",
		},
		{
			name: "single-quoted backslash-pipe outside a table reaches Go as a literal pipe",
			body: "verify: `go test ./pkg/ -run 'TestSameStringSet\\|TestMigrateNestedCompose'`",
			want: "selects no test",
		},
		{
			name: "-run=pattern form is parsed too",
			body: "verify: `go test ./pkg/ -run=TestAbsent`",
			want: "selects no test",
		},
		{
			name: "TestMain is not selectable by -run",
			body: "verify: `go test ./pkg/ -run TestMain`",
			want: "selects no test",
		},
		{
			name: "lower case after Test is a helper, not a test",
			body: "verify: `go test ./pkg/ -run TestingHelperNotATest`",
			want: "selects no test",
		},
		{
			name: "pattern that is not a regexp at all",
			body: "verify: `go test ./pkg/ -run 'TestMigrate('`",
			want: "not a valid regexp",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := checkRunFixture(t, tt.body)
			if res.UnmatchedRunFlags != 1 {
				t.Fatalf("planted defect went undetected: unmatched_run=%d, detail=%v", res.UnmatchedRunFlags, res.UnmatchedRunDetail)
			}
			if !strings.Contains(res.UnmatchedRunDetail[0], tt.want) {
				t.Errorf("detail %q does not say %q", res.UnmatchedRunDetail[0], tt.want)
			}
			if res.OK {
				t.Error("Check reported OK with an unmatched -run pattern")
			}
		})
	}
}

// TestRunPatterns_acceptsWorkingBindings is the other half: a detector that flags everything
// catches the planted defects above and is still useless. Each of these runs tests today.
func TestRunPatterns_acceptsWorkingBindings(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"exact name", "verify: `go test ./pkg/ -run TestSameStringSet`"},
		{"prefix matching several", "verify: `go test ./pkg/ -run TestMigrate`"},
		{"unanchored substring", "verify: `go test ./pkg/ -run Duplicates`"},
		{"alternation", "verify: `go test ./pkg/ -run 'TestSameStringSet|TestAbsent'`"},
		{"anchored", "verify: `go test ./pkg/ -run '^TestSameStringSet$' -count=1`"},
		{"double quoted with a group", "verify: `go test ./pkg/ -run \"TestMigrate(Nested|Duplicates)\"`"},
		{"subtest path judges only the first element", "verify: `go test ./pkg/ -run TestMigrateNestedCompose/case-1`"},
		{"unquoted backslash-pipe: the shell eats it, Go sees an alternation",
			"verify: `go test ./pkg/ -run TestSameStringSet\\|TestAbsent`"},
		{"GFM table escape is unescaped before judging",
			"| check | outcome |\n| --- | --- |\n| `go test ./pkg/ -run 'TestSameStringSet\\|TestAbsent' -v` | PASS |"},
		{"trailing flags are not part of the pattern", "verify: `go test ./pkg/ -run TestSameStringSet -v | grep -c PASS`"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := checkRunFixture(t, tt.body)
			if res.UnmatchedRunFlags != 0 {
				t.Fatalf("false positive: %v", res.UnmatchedRunDetail)
			}
			if res.RunPatternsChecked != 1 {
				t.Fatalf("run_patterns=%d, want 1 — the binding was not seen at all", res.RunPatternsChecked)
			}
		})
	}
}

// TestRunPatterns_ignoresNonBindings keeps the denominator honest. Everything here would inflate
// run_patterns without being a command anyone runs, and the first two would be flagged outright.
func TestRunPatterns_ignoresNonBindings(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"a flag that merely ends in -run", "verify: `dva up --dry-run` and `human — re-run the sweep`"},
		{"prose about -run without a command", "The `-run TestAbsent` selector was wrong."},
		{"a fenced illustration is not a binding", "```\ngo test ./pkg/ -run TestAbsent\n```"},
		{"no -run flag at all", "verify: `go test ./pkg/ -count=1`"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := checkRunFixture(t, tt.body)
			if res.RunPatternsChecked != 0 {
				t.Fatalf("run_patterns=%d, want 0: %v", res.RunPatternsChecked, res.UnmatchedRunDetail)
			}
			if res.UnmatchedRunFlags != 0 {
				t.Fatalf("false positive: %v", res.UnmatchedRunDetail)
			}
		})
	}
}

// TestRunPatterns_failsWhenTestNamesVanish covers the one way this check can go green for the
// wrong reason: if the declaration regex stops matching, every pattern in the repo would be
// compared against an empty set and pass. Files swept with zero names is that state.
func TestRunPatterns_failsWhenTestNamesVanish(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/a.md", "# A\n\nSee [self](a.md).\n")
	writeFile(t, root, "pkg/thing_test.go", "package thing\n\nfunc helper() {}\n")
	inv := mustInventory(t, root, "docs/a.md", "pkg/thing_test.go")

	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.OK {
		t.Fatal("expected failure when _test.go files yield no test names")
	}
	if !containsAny(res.Errors, "vacuous") {
		t.Fatalf("errors=%v, want a vacuity error", res.Errors)
	}
}

// TestRunPatterns_sweepsTheRealCorpus is the floor. The fixture tests above prove the detector
// works on a fixture; only this one proves it is pointed at this repository. A rename of the
// tasks tree, or a regex edit that stops recognising the binding shape, would leave every test
// above green while the gate silently examined nothing.
func TestRunPatterns_sweepsTheRealCorpus(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")

	inv, err := LoadInventory(root)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	res := Check(CheckInput{Root: root, Inventory: inv})

	// 125 bindings and 1051 tests on 2026-08-03. The floors are well under both: this test owns
	// "the sweep still reaches the corpus", not "the corpus stopped shrinking" — tasks get
	// archived and bindings get consolidated, and neither should turn the suite red.
	if res.RunPatternsChecked < 50 {
		t.Errorf("run_patterns=%d, want at least 50 — the -run sweep no longer reaches the task corpus", res.RunPatternsChecked)
	}
	if res.TestFuncsFound < 500 {
		t.Errorf("test_funcs_found=%d (from %d files), want at least 500 — the declaration sweep broke", res.TestFuncsFound, res.TestFilesSwept)
	}
	if res.UnmatchedRunFlags > 0 {
		t.Errorf("%d binding(s) select no test:\n  %s", res.UnmatchedRunFlags, strings.Join(res.UnmatchedRunDetail, "\n  "))
	}
	t.Logf("swept %d -run patterns against %d test functions from %d files",
		res.RunPatternsChecked, res.TestFuncsFound, res.TestFilesSwept)
}
