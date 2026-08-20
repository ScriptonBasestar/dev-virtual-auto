---
id: TASK-219
title: "The only size limit acting on Go source is one the repository never declared"
type: chore
priority: P3
effort: S
created-at: 2026-08-20T20:02:00+09:00
source: "found by the adversarial review of TASK-210's follow-up: both of the reviewer's Edit calls came back as errors naming a 500-line limit, while the edits had in fact applied"
scope: "a ruling, recorded on this card. Optionally .golangci.yml or the Makefile if the ruling is to declare a limit here. No source file is split without that ruling first."
status: todo
---

# Task 219: The only size limit acting on Go source is one the repository never declared

## Summary

Editing `internal/cli/plan_lifecycle.go` on the TASK-210 branch returns an
error:

```
🟠 internal/cli/plan_lifecycle.go
   File length 544 lines exceeds error limit 500 lines
   💡 Split this file into smaller modules
```

Two things about that message are worth separating, and neither is visible
from the message itself.

**The edit succeeded.** This arrives from a `PostToolUse` hook, which runs
after the tool has already written the file. The hook's own header says so —
it exists to "feed the verdict back so Claude fixes it right away" — so exit 2
is deliberate feedback, not a failed write. But an agent that reads an
error-shaped response as "my edit was rejected" will retry it, and the retry
lands a second copy of the change.

**The limit is not this repository's.** dva declares no size limit on Go
source anywhere. The rule comes from a workstation policy in ce-agent-kit,
which a contributor cloning this repository does not have.

## Measured

Counted in the TASK-210 worktree at `b293242`, over `git ls-files '*.go'` —
289 files, so nothing here is a sample:

| population | count | over the limit |
|---|---|---|
| all tracked `.go` | 289 | 21 over 500 |
| non-test `.go` | 117 | **13** over the `go` error limit of 500 |
| `_test.go` | 172 | 4 over the `test` error limit of 600 |
| non-test in the 301–500 warning band | 117 | 12 warned |

The thirteen, largest first:

```
 1162  internal/config/config.go
 1142  internal/cli/compose.go
  993  internal/config/validate_warnings.go
  887  internal/config/lifecycle.go
  811  internal/cli/doctor.go
  708  internal/cli/init.go
  661  internal/cli/validate.go
  655  internal/lifecycle/orchestrator.go
  649  internal/cli/provision.go
  613  internal/config/merge.go
  551  internal/cli/plan_lifecycle.go
  517  internal/cli/root.go
  509  internal/cli/manifest.go
```

`plan_lifecycle.go` is eleventh of thirteen. The file the verdict named is
one of the smallest offenders, and `config.go` is more than twice the limit.
Whatever the message is reporting, it is not "this file is unusual".

**What this branch changed.** `plan_lifecycle.go` was 501 lines at `36adfd4`,
508 at `dc762ca` after ten intervening commits, and is 551 here. The verdict
quoted above says 544 because it was observed before this branch was rebased
onto `dc762ca`; the branch's own contribution is **+43 lines at both commits**,
which is the number that means anything. It did not create the violation —
the file was already one line over at `36adfd4` — and it did not fix it.

The 544-vs-551 gap is itself the reason this card gives a commit for every
count. A file-length number is only a fact about a commit, and three other
cards on this branch had to be rewritten after quoting counts taken at a
baseline that had since moved.

**Where the rule lives.** `~/.claude/plugins/cache/ce-agent-kit/core/0.6.1/`
`skills/validation-rules/reference/file-size.yaml`, kind `go`:
`warning_lines: 300`, `error_lines: 500`; kind `test`: 600. Applied by the
`PostToolUse` hook `~/.claude/hooks/scripts/ce-validate-filesize.sh`, which
runs `ce validate filesize --changed-only <rel>` and exits 2 when the output
carries a `File Size Validation` header.

**Where it does not live.** `git grep` for `error_lines`, `warning_lines`,
`max.?lines`, `file.?size` and `line limit` across `Makefile`,
`.golangci.yml`, `AGENTS.md`, `CLAUDE.md`, `docs/` and `tools/` returns
nothing that applies to Go source. `.golangci.yml` enables the default set
plus `modernize` and `unparam`; none of those measures file length, so
`make lint` is silent on this. The repository *does* gate size on its own
documentation — `make doc-check`, TASK-090 — which is the contrast that makes
the gap visible: docs have a declared limit in the Makefile, source has an
undeclared one on one workstation.

## Cause

Not a defect in the hook. The hook does what its header says. The gap is that
a real constraint on how work gets done in this repository is declared
somewhere the repository cannot see, so:

- it cannot be reviewed, versioned or argued with here;
- it fires per-edit rather than per-commit, so it names whichever file was
  touched and never the census above;
- CI does not enforce it, so the thirteen files stay as they are and every
  edit to any of them produces the same error-shaped response.

TASK-211 hit the same distinction from the other side and stated it: a
workstation policy "is a real constraint on how work gets done here and not a
fact about this codebase, so a commit message should not cite it as 'the
limit' without saying whose". This card is that sentence applied to the gate
rather than to a commit message.

## What to change

A ruling, and only then code. The three answers, with what each costs:

1. **Declare it here.** Add the limit to the repository — a `make` target or
   a linter setting — so it is versioned, reviewable, and identical for every
   contributor. Cost: CI goes red on thirteen files on day one, so this
   answer needs a starting threshold above the current worst file, or a
   grandfathered list, or both.
2. **Split the offenders.** Cost: thirteen files, several of them the ones
   most edited by concurrent sessions, and splitting `compose.go` while other
   branches are in flight is how the collision this branch already hit gets
   repeated in code instead of in card numbers.
3. **Record it as accepted.** Say in `AGENTS.md` that dva does not gate Go
   file length, that a workstation may, and that its verdict is advisory.
   Cost: nothing changes; the error-shaped feedback keeps arriving, and the
   only gain is that the next agent reads one line and stops treating it as a
   failed write.

The recommendation is 3 now and 1 later, because 3 is the only one that can
be done without touching a file another session may be holding, and because
the thing actually hurting today is the misreading, not the line counts.

## Completion Criteria

- [ ] The ruling is recorded on this card, with the reason | verify: human
- [ ] Whatever the ruling, `AGENTS.md` states whether Go file length is gated here and by what | verify: `grep -ci 'file length\|파일 길이\|file size' AGENTS.md` returns ≥ 1 (today: 0 — measured, not assumed)
- [ ] If the ruling is 1 (declare it here): the limit is in a file this repository tracks, and `make lint` or a new target reports it | verify: `git grep -cE 'error_lines|max-lines|funlen|lll' -- Makefile .golangci.yml` returns ≥ 1 (today: 0). Skip, marking N/A, under rulings 2 or 3
- [ ] If the ruling is 2 (split): the census shrinks, and the number is stated rather than implied | verify: re-run the census command in ## Technical Notes; the non-test count is below 13 (today: 13). Skip, marking N/A, under rulings 1 or 3
- [ ] No source file is split as a side effect of an unrelated card | verify: human — the point of this card is that the ruling comes first

## References

- `tasks/_archive/211-a-stack-flag-missing-its-value-is-dropped-and-the-command-runs-as-if-unwritten.md` — the same workstation-vs-repository distinction, stated for the `test` kind at 600 lines, and a commit message corrected for citing it as "the limit"
- `tasks/_archive/090-seven-documents-exceed-the-doc-standard-nothing-enforces-it.md` — the doc-side version of this card, and the one that ended with a real gate in the Makefile
- `AGENTS.md` — where a ruling of 3 would be written
- `.golangci.yml` — where a ruling of 1 would most naturally live
- `tasks/todo/217-a-lone-terminator-disarms-build-s-plan-selection-guard-and-builds-the-whole-stack.md` — the branch whose review surfaced this

## Technical Notes

The census is one command, and it is written out because a card that reports
a count without the command that produced it cannot be re-checked:

```bash
for f in $(git ls-files '*.go' | grep -v '_test.go'); do
  n=$(wc -l < "$f"); [ "$n" -gt 500 ] && printf '%5d  %s\n' "$n" "$f"
done | sort -rn
```

Two traps worth naming for whoever re-runs it. First, run it in a checkout,
not a bare comparison against `origin/master`: two of the counts above differ
by a handful of lines between commits, and a census that does not say which
commit it counted is the same defect this branch spent a day correcting on
three other cards. Second, `ce validate filesize` takes one path per
invocation in the form used here and examines only the last argument when
given several, so a loop that passes a list will silently report on one file
and read as a clean sweep.
