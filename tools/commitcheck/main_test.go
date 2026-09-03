package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
		{"095f525ba1c71624b9ad4b7198b1f33013d229ad", "docs: repair drift across core documents"},
		{"6ab9c64333f54e6ef97bc81e123e75fc4173f141", "docs: align CLI surface facts with shipped commands"},
		{"a6666c1a48d9fa27411831f16da201bb9c5ba6a0", "docs: restore README quick-start role and align environments vocabulary"},
		{"6e3f581447748b465808d73270900d356aaca5b8", "docs(tasks): widen TASK-273 to the manifest that advertises the dead flags"},
		{"7ce7c1469fe9ad42248b9f52c19cd27e85a0bb6b", "docs(tasks): correct TASK-273 \u2014 the four options are path-conditional, not dead"},
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

func TestSuccessMessageNamesTheExceptionBoundary(t *testing.T) {
	const want = "commitcheck: OK -- every non-exempt subject since the baseline matches the format SSOT"
	if successMessage != want {
		t.Errorf("success message = %q, want %q", successMessage, want)
	}
}

func TestEveryWaiverRecordsWhyItCouldNotBeRepaired(t *testing.T) {
	// A waiver without a reason is indistinguishable from one added to make the build green.
	// The reason is the only part a reviewer can weigh, so its absence is a test failure
	// rather than a style note.
	for _, c := range grandfatheredCommits {
		if strings.TrimSpace(c.reason) == "" {
			t.Errorf("waiver %s has no reason", c.sha[:8])
		}
	}
}

func TestMessageFileModeRejectsWhatTheHistorySweepWouldHaveCaught(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    int
	}{
		{"conforming", "fix(cli): repair the plan flag parser\n", 0},
		{"scope missing", "docs: repair drift across core documents\n", 1},
		{"over the limit", "docs(tasks): " + strings.Repeat("x", maxSubject) + "\n", 1},
		{"unknown type", "wibble(cli): do a thing\n", 1},
		{"comments and blanks skipped", "\n# please enter the commit message\nfix(cli): repair the parser\n", 0},
		{"generated merge subject", "Merge branch 'x' into master\n", 0},
		{"generated revert subject", "Revert \"fix(cli): repair the parser\"\n", 0},
		{"empty message is git's business", "\n# only comments\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
			if err := os.WriteFile(path, []byte(tc.message), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			if got := checkMessageFile(path); got != tc.want {
				t.Errorf("checkMessageFile = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMessageFileModeFailsLoudlyWhenTheFileIsMissing(t *testing.T) {
	// Exit 2, not 0. A hook that cannot read the message has not approved it.
	if got := checkMessageFile(filepath.Join(t.TempDir(), "absent")); got != 2 {
		t.Errorf("missing file exit = %d, want 2", got)
	}
}
