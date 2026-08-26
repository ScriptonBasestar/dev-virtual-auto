package main

import (
	"os/exec"
	"strings"
	"testing"
)

func rules(t *testing.T, subject string) []string {
	t.Helper()
	var out []string
	for _, v := range checkSubject(subject) {
		out = append(out, v.rule)
	}
	return out
}

func TestCheckSubjectAcceptsConformingSubjects(t *testing.T) {
	// One per SSOT type, so a typo in ssotTypes cannot pass unnoticed.
	for _, s := range []string{
		"feat(build): gate commit subjects at 72 characters",
		"fix(flows): quote the heredoc delimiter",
		"refactor(cli): fold the plan router into one prologue",
		"docs(tasks): separate the two archive skips 202 reported as one",
		"test(config): cover the env_file precedence order",
		"chore(tasks): register the three gaps the flowcheck run left behind",
	} {
		if got := rules(t, s); got != nil {
			t.Errorf("subject %q should pass, got %v", s, got)
		}
	}
}

func TestCheckSubjectLength(t *testing.T) {
	at72 := "chore(x): " + strings.Repeat("a", 72-len("chore(x): "))
	if len(at72) != 72 {
		t.Fatalf("fixture is %d chars, want 72", len(at72))
	}
	if got := rules(t, at72); got != nil {
		t.Errorf("exactly %d chars must pass, got %v", maxSubject, got)
	}

	over := at72 + "a"
	if got := rules(t, over); len(got) != 1 || got[0] != "length" {
		t.Errorf("%d chars must fail on length alone, got %v", len(over), got)
	}
}

func TestCheckSubjectLengthCountsRunesNotBytes(t *testing.T) {
	// Korean subjects are legal here and each character is three bytes. Counting bytes
	// would reject a 24-character subject as if it were 72.
	s := "docs(가이드): " + strings.Repeat("한", 20)
	if n := len(s); n <= maxSubject {
		t.Fatalf("fixture must exceed %d *bytes* to be meaningful, got %d", maxSubject, n)
	}
	if got := rules(t, s); got != nil {
		t.Errorf("subject of %d runes should pass, got %v", len([]rune(s)), got)
	}
}

func TestCheckSubjectFormat(t *testing.T) {
	for _, tc := range []struct {
		subject string
		want    string
	}{
		{"add a thing without any prefix", "format"},
		{"chore: no scope at all", "format"},
		{"chore(): empty scope", "scope"},
		{"wip(build): unknown type", "type"},
		{"style(fmt): a type this repo used but the SSOT does not list", "type"},
		{"Feat(build): uppercase type is not the SSOT spelling", "format"},
		{"chore(build):missing space after the colon", "format"},
	} {
		got := rules(t, tc.subject)
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("subject %q: want [%s], got %v", tc.subject, tc.want, got)
		}
	}
}

func TestCheckSubjectReportsEveryBrokenRule(t *testing.T) {
	// Length and format are independent; a subject that breaks both must say so rather
	// than stopping at the first finding.
	s := strings.Repeat("no prefix and far too long ", 4)
	got := rules(t, s)
	if len(got) != 2 || got[0] != "length" || got[1] != "format" {
		t.Errorf("want [length format], got %v", got)
	}
}

func TestBaselineIsReachable(t *testing.T) {
	// The gate exits 2 rather than passing when the baseline is missing. This asserts the
	// pinned constant is a real commit in this repository, so that exit is only ever
	// reached by a truncated clone and never by a stale hash nobody re-checked.
	if err := exec.Command("git", "cat-file", "-e", baseline+"^{commit}").Run(); err != nil {
		t.Fatalf("baseline %s is not a commit in this repository: %v", baseline, err)
	}
}

func TestGrandfatheredCommitsAreTheExactHistoricalObjects(t *testing.T) {
	want := []struct {
		sha     string
		subject string
	}{
		{"d7976538a9f68dad0c7873ce8c256fb7c60212a0", "feat: add deterministic skill installer"},
		{"c6ed4eab2750ec4e6aca3e130dfcad61abc3fc6f", "fix: harden skill installation transactions"},
	}
	if len(grandfatheredCommits) != len(want) {
		t.Fatalf("grandfathered commit count = %d, want %d", len(grandfatheredCommits), len(want))
	}
	for _, tc := range want {
		if !isGrandfatheredCommit(tc.sha, tc.subject) {
			t.Errorf("historical exception %s %q is missing", tc.sha[:8], tc.subject)
		}
		out, err := git("show", "-s", "--format=%s", tc.sha)
		if err != nil {
			t.Errorf("historical exception %s is not reachable: %v", tc.sha[:8], err)
			continue
		}
		if got := strings.TrimSpace(out); got != tc.subject {
			t.Errorf("historical exception %s subject = %q, want %q", tc.sha[:8], got, tc.subject)
		}
	}
}

func TestGrandfatheringCannotWaiveAChangedOrFutureViolation(t *testing.T) {
	for _, tc := range grandfatheredCommits {
		if isGrandfatheredCommit(tc.sha, tc.subject+" changed") {
			t.Errorf("subject drift waived for %s", tc.sha[:8])
		}
		if isGrandfatheredCommit("0000000000000000000000000000000000000000", tc.subject) {
			t.Errorf("future commit with historical subject was waived")
		}
	}

	sha := "0000000000000000000000000000000000000000"
	subject := "fix: a future scope-less violation"
	if isGrandfatheredCommit(sha, subject) {
		t.Fatal("future violation was waived")
	}
	if got := checkSubject(subject); len(got) != 1 || got[0].rule != "format" {
		t.Fatalf("future violation must still fail format, got %#v", got)
	}
}
