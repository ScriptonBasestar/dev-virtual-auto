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

// grandfatheredCommits are the post-baseline commits explicitly waived by this gate. The
// baseline must not move to retire them: every other commit after it remains checked. A
// waiver matches both the immutable object ID and its intended subject, so copying either
// half into a future commit cannot bypass the gate.
//
// A waiver is only ever the right answer for a subject that is already published. Rewriting
// pushed history is forbidden here, so a bad subject that reaches an integration branch can
// no longer be fixed -- it can only be recorded. Every entry therefore carries the reason it
// could not be repaired, and adding one is an admission, not a routine step.
//
// Note what the five 2026-09-03 entries have in common: all five landed on the same day,
// from separate agent sessions, while this gate existed and passed for none of them. That is
// the shape of a check nothing runs before the thing it checks. The commit-msg hook installed
// by `make install-hooks` is the answer to that, and it is why this table should stop growing.
var grandfatheredCommits = []struct {
	sha     string
	subject string
	reason  string
}{
	{
		sha:     "d7976538a9f68dad0c7873ce8c256fb7c60212a0",
		subject: "feat: add deterministic skill installer",
		reason:  "scope-less installer commit predating the scope rule's enforcement",
	},
	{
		sha:     "c6ed4eab2750ec4e6aca3e130dfcad61abc3fc6f",
		subject: "fix: harden skill installation transactions",
		reason:  "scope-less installer commit predating the scope rule's enforcement",
	},
	{
		sha:     "095f525ba1c71624b9ad4b7198b1f33013d229ad",
		subject: "docs: repair drift across core documents",
		reason:  "scope omitted; published to master 2026-09-03 before any pre-commit gate existed",
	},
	{
		sha:     "6ab9c64333f54e6ef97bc81e123e75fc4173f141",
		subject: "docs: align CLI surface facts with shipped commands",
		reason:  "scope omitted; published to master 2026-09-03 before any pre-commit gate existed",
	},
	{
		sha:     "a6666c1a48d9fa27411831f16da201bb9c5ba6a0",
		subject: "docs: restore README quick-start role and align environments vocabulary",
		reason:  "scope omitted; published to master 2026-09-03 before any pre-commit gate existed",
	},
	{
		sha:     "6e3f581447748b465808d73270900d356aaca5b8",
		subject: "docs(tasks): widen TASK-273 to the manifest that advertises the dead flags",
		reason:  "74-char subject; published to master 2026-09-03 before any pre-commit gate existed",
	},
	{
		sha:     "7ce7c1469fe9ad42248b9f52c19cd27e85a0bb6b",
		subject: "docs(tasks): correct TASK-273 \u2014 the four options are path-conditional, not dead",
		reason:  "79-char subject; published to master 2026-09-03 before any pre-commit gate existed",
	},
}

// maxSubject is the enforced ceiling. See the package comment for why it is 72 and not
// the SSOT's 50.
const maxSubject = 72

const successMessage = "commitcheck: OK -- every non-exempt subject since the baseline matches the format SSOT"

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

// checkMessageFile validates a single commit message being written, and is what the
// commit-msg hook calls. It exists because the history sweep below can only ever report a
// violation that already happened: by the time `make commit-check` sees a subject, the
// commit object exists, and once that object is pushed the subject is unfixable. Checking
// the message before the object is created is the only point at which the answer is still
// "rewrite this line" rather than "add a waiver".
//
// Git passes the path of the message file. Comment lines are stripped the way git itself
// strips them, and the first non-comment, non-blank line is the subject. An empty message
// is not this gate's business -- git aborts that commit on its own.
func checkMessageFile(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "commitcheck: cannot read commit message file %s: %v\n", path, err)
		return 2
	}

	subject := ""
	for line := range strings.SplitSeq(string(raw), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		subject = strings.TrimRight(line, "\r")
		break
	}
	if subject == "" {
		return 0
	}

	// A merge or revert subject is generated by git, not authored, so holding it to the
	// format would fail commits nobody wrote a subject for.
	if strings.HasPrefix(subject, "Merge ") || strings.HasPrefix(subject, "Revert ") {
		return 0
	}

	violations := checkSubject(subject)
	if len(violations) == 0 {
		return 0
	}
	fmt.Fprintf(os.Stderr, "commitcheck: this subject cannot be committed as written\n    %s\n", subject)
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  [%s] %s\n", v.rule, v.msg)
	}
	fmt.Fprintf(os.Stderr, "\nFix the subject line and commit again. The format is `type(scope): summary`,\n"+
		"%d types (%s), scope required, %d chars max (SSOT target is 50).\n",
		len(ssotTypes), strings.Join(ssotTypes, ", "), maxSubject)
	return 1
}

func main() {
	// The hook path takes precedence and never touches git history, so it works in a
	// shallow clone, in a worktree, and during a rebase.
	if len(os.Args) == 3 && os.Args[1] == "-message-file" {
		os.Exit(checkMessageFile(os.Args[2]))
	}
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "usage: commitcheck [-message-file PATH]\n")
		os.Exit(2)
	}

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
	fmt.Println(successMessage)
}
