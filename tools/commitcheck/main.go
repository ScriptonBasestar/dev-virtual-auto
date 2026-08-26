// Command commitcheck: hold this repository's commit subjects to the format SSOT.
//
// The format is declared once, outside this repository, in the ce-agent-kit git skill
// (skills/git/SKILL.md, <commit-format>): `{type}({scope}): {subject}`, six types, scope
// required, subject 50 chars max. Nothing enforced it here, and the corpus drifted in
// exactly the way an unenforced rule drifts -- silently, and only in the dimension that
// is hardest to notice while writing.
//
// Measured over the 679 commits reachable at the point this gate landed:
//
//	<=50 chars    85   (12.5%)
//	51-60        102
//	61-72        249
//	>72          243   (35.8%)
//
// and over the 60 most recent: 3 at <=50, 12 at 51-60, 27 at 61-72, 18 over 72.
//
// That distribution is why the enforced limit here is 72 and not the SSOT's 50. A gate at
// 50 would reject roughly seven of every eight commits this repository has ever written,
// including nearly all of the recent ones written carefully; it would be bypassed or
// deleted within a day, and a deleted gate enforces nothing. 72 is the Conventional
// Commits hard limit and the width at which `git log --oneline` stops wrapping in an
// 80-column terminal, and it rejects 30% of recent subjects -- a real tightening that the
// existing writing habit can actually reach. The SSOT's 50 remains the target to aim at;
// this is the floor below which the build stops. The deviation is deliberate, was chosen
// by the repository owner, and is recorded here rather than left for a reader to discover
// as an inconsistency.
//
// History before the gate is history. baseline pins the commit this gate landed on top
// of, so the 243 subjects already over 72 are not retroactively failures -- the same
// logic the archived task cards get. The range only ever grows from there, so every
// commit written after the gate is checked on every run, forever.
//
// Two SSOT rules are deliberately not enforced, because a gate that fires on correct
// input teaches people to route around it:
//
//   - Imperative mood is not mechanically decidable. "update the parser" and "updates the
//     parser" differ by a suffix that also appears in correct subjects.
//   - Lowercase is decidable but not safely. Measured 0 violations in the last 200
//     subjects, while legitimate subjects begin with an identifier that must keep its
//     case (`DVA`, `TASK-203`, a backticked command). Enforcing it would produce only
//     false positives against this corpus.
//
// The Co-Authored-By trailer is reported but does not fail the build. 10 of the last 60
// commits lack it and all 60 are authored by the same human git identity, so the trailer
// is an agent-attribution rule rather than a repository convention, and this gate cannot
// tell which kind of author it is looking at. Counting it makes a silent omission visible
// without blocking a human's own commit.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

// baseline is the commit this gate landed on top of. Commits at or before it predate the
// rule and are not checked; everything after it is. Moving this forward would retire
// findings rather than fix them, so it moves only when history is rewritten under it.
const baseline = "c100ba06de0e64ebe6079908b8681b993e674a58"

// grandfatheredCommits are the two scope-less installer commits created before
// commit-check was added to the integration workflow. The baseline must not move to
// retire them: every other commit after it remains checked. A waiver matches both the
// immutable object ID and its intended subject, so copying either half into a future
// commit cannot bypass the gate.
var grandfatheredCommits = []struct {
	sha     string
	subject string
}{
	{
		sha:     "d7976538a9f68dad0c7873ce8c256fb7c60212a0",
		subject: "feat: add deterministic skill installer",
	},
	{
		sha:     "c6ed4eab2750ec4e6aca3e130dfcad61abc3fc6f",
		subject: "fix: harden skill installation transactions",
	},
}

// maxSubject is the enforced ceiling. See the package comment for why it is 72 and not
// the SSOT's 50.
const maxSubject = 72

// ssotTypes is the type list from ce-agent-kit skills/git/SKILL.md <commit-format>. It is
// copied rather than widened on purpose: this repository has used `style` twice, and
// quietly accepting a seventh type here would fork the SSOT instead of amending it. A
// rejected type is a prompt to change one of the two, not to edit this list in passing.
var ssotTypes = []string{"feat", "fix", "refactor", "docs", "test", "chore"}

// conventional matches `type(scope): subject` with a non-empty type, scope and subject.
// The type is checked against ssotTypes separately so an unknown type and a malformed
// subject produce different messages.
var conventional = regexp.MustCompile(`^([a-z]+)\(([^)]*)\): (.+)$`)

type violation struct {
	sha     string
	subject string
	rule    string
	msg     string
}

// checkSubject returns every rule a subject breaks. It is pure so the rules can be tested
// without a repository standing behind them.
func checkSubject(subject string) []violation {
	var out []violation

	if n := len([]rune(subject)); n > maxSubject {
		out = append(out, violation{
			rule: "length",
			msg:  fmt.Sprintf("subject is %d chars, limit is %d (SSOT target is 50)", n, maxSubject),
		})
	}

	m := conventional.FindStringSubmatch(subject)
	switch {
	case m == nil:
		out = append(out, violation{
			rule: "format",
			msg:  "subject is not `type(scope): summary` -- scope is required by the SSOT",
		})
	case m[2] == "":
		out = append(out, violation{
			rule: "scope",
			msg:  "scope is empty; the SSOT requires one",
		})
	default:
		if !slices.Contains(ssotTypes, m[1]) {
			out = append(out, violation{
				rule: "type",
				msg: fmt.Sprintf("type %q is not one of %s (ce-agent-kit skills/git/SKILL.md)",
					m[1], strings.Join(ssotTypes, ", ")),
			})
		}
	}
	return out
}

// isGrandfatheredCommit is intentionally exact. The fixed SHA keeps this narrow to the
// historical object, while the fixed subject makes the policy readable and detects an
// accidental change to the exception table in tests.
func isGrandfatheredCommit(sha, subject string) bool {
	for _, c := range grandfatheredCommits {
		if sha == c.sha && subject == c.subject {
			return true
		}
	}
	return false
}

// git runs a git command and returns its stdout, failing loudly. Every caller here is
// asking a question whose wrong answer is "zero commits", so nothing is tolerated.
func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func main() {
	// A gate whose range cannot be resolved must not report a clean run. In a shallow
	// clone the baseline is simply absent and `baseline..HEAD` would quietly yield
	// nothing, which reads exactly like a repository with no violations.
	if _, err := git("cat-file", "-e", baseline+"^{commit}"); err != nil {
		fmt.Fprintf(os.Stderr, "commitcheck: baseline %s is not reachable -- "+
			"a shallow or truncated clone cannot run this gate (fetch with full history)\n", baseline[:8])
		os.Exit(2)
	}

	rng := baseline + "..HEAD"

	// Merge commits carry no authored subject, so they are excluded -- but the count is
	// printed rather than dropped, so the denominator below stays honest.
	allOut, err := git("rev-list", "--count", rng)
	if err != nil {
		fmt.Fprintf(os.Stderr, "commitcheck: %v\n", err)
		os.Exit(2)
	}
	logOut, err := git("log", "--no-merges", "--format=%H%x1f%s%x1f%b%x1e", rng)
	if err != nil {
		fmt.Fprintf(os.Stderr, "commitcheck: %v\n", err)
		os.Exit(2)
	}

	var violations []violation
	checked, grandfathered, missingTrailer := 0, 0, 0
	for rec := range strings.SplitSeq(logOut, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, "\x1f", 3)
		if len(f) != 3 {
			fmt.Fprintf(os.Stderr, "commitcheck: unparsable git log record %q\n", rec)
			os.Exit(2)
		}
		sha, subject, body := f[0], f[1], f[2]
		checked++

		if isGrandfatheredCommit(sha, subject) {
			grandfathered++
		} else {
			for _, v := range checkSubject(subject) {
				v.sha, v.subject = sha, subject
				violations = append(violations, v)
			}
		}
		if !strings.Contains(strings.ToLower(body), "co-authored-by:") {
			missingTrailer++
		}
	}

	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "%s [%s] %s\n    %s\n", v.sha[:8], v.rule, v.msg, v.subject)
	}

	// Print the denominator on every run. A gate that matched nothing reads exactly like
	// a gate that passed, and that is how a check rots into decoration.
	merges := 0
	if n := strings.TrimSpace(allOut); n != "" {
		var total int
		if _, err := fmt.Sscanf(n, "%d", &total); err == nil {
			merges = total - checked
		}
	}
	fmt.Printf("commitcheck: checked %d commit(s) since %s (%d merge(s) skipped), limit %d chars\n",
		checked, baseline[:8], merges, maxSubject)
	if grandfathered > 0 {
		fmt.Printf("commitcheck: %d exact historical exception(s) skipped; all other commits remain enforced\n", grandfathered)
	}
	if missingTrailer > 0 {
		fmt.Printf("commitcheck: %d of %d lack a Co-Authored-By trailer (reported, not enforced -- see package comment)\n",
			missingTrailer, checked)
	}

	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "commitcheck: %d violation(s)\n", len(violations))
		os.Exit(1)
	}
	fmt.Println("commitcheck: OK -- every subject since the baseline matches the format SSOT")
}
