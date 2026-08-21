---
id: TASK-219
title: "The only size limit acting on Go source is one the repository never declared"
type: chore
priority: P3
effort: S
created-at: 2026-08-20T20:02:00+09:00
source: "found by the adversarial review of TASK-210's follow-up: both of the reviewer's Edit calls came back as errors naming a 500-line limit, while the edits had in fact applied"
scope: "a ruling, recorded on this card. Optionally .golangci.yml or the Makefile if the ruling is to declare a limit here. No source file is split without that ruling first."
status: done
completed-at: 2026-08-21T10:13:08+09:00
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

> *Annotated 2026-08-21, left in place rather than rewritten.* The last clause
> is true of one edit shape and asserted of all of them; see the correction
> under `## Ruling`. Nothing on this card ever observed a duplicate landing —
> the `source:` above records two error-shaped Edit responses whose edits had
> applied, which is the opportunity to misread and not a retry.

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

*Re-checked while ruling, 2026-08-21.* Two corrections to that paragraph, left
as annotations rather than a rewrite so the drift stays visible. The installed
cache is now `core/0.6.2/`; `0.6.1/` no longer exists on disk, so the path
above resolves to nothing today. The `go` numbers are unchanged (300/500), and
kind `test` is `warning_lines: 400` / `error_lines: 600` — the paragraph gave
only the error limit. The threshold is not in the hook script at all: that file
carries no number and shells out to `ce validate filesize --changed-only`,
which reads the yaml; the comparison is `lines >= lineLimit` in ce-agent-kit's
`internal/usecase/quality/filesize_validator.go`. This is the same failure the
card's own Measured section warns about one paragraph up — a path is a fact
about a moment, and a card that cites one without a date cannot be re-checked.

**The rule has already reshaped this codebase twice.** Two archived cards split
a Go file specifically to stay under the undeclared 500:
`tasks/_archive/187-*.md:117` — "`tools/flowcheck/rules.go` had grown past the
500-line limit, so it was split" — and `tasks/_archive/193-*.md:80` —
"`shell.go` had reached 468 lines against a 500-line ceiling". Neither cites a
repository rule, because there is none to cite. This is the strongest evidence
in the card and it was found only when the ruling was reviewed: the limit is
not merely advisory in practice, it has been obeyed.

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

## Ruling

**Option 3. dva does not gate Go source file length. A workstation may, and its
verdict is advisory here.** Recorded in `AGENTS.md` under "Source file length is
not gated here (TASK-219 ruling)".

Three reasons, in the order they decided it.

**The thing costing time today is the misreading, not the line counts.** The
hook is a `PostToolUse` hook: the write has already landed when the verdict
arrives, so an agent that reads exit 2 as a rejected edit is acting on a belief
about its own tree that is false. That is fixed by one paragraph — no file
moves, no CI change.

*Corrected while reviewing this ruling, 2026-08-21.* The sentence above
originally said the retry "lands the change twice" and that this "has already
happened once (this card's `source:`)". Both overreached, and the second is the
sharper error: the `source:` records two Edit calls that came back error-shaped
while having applied — the *opportunity* to misread, not a retry, and not a
duplicated hunk. Nothing on this card ever showed a duplicate landing. The first
overreached in a different direction: what a retry does depends on the edit's
shape. A `Write` re-lands the same bytes harmlessly; an `Edit` whose `old_string`
the first pass consumed fails loudly; only an `Edit` that wrapped or extended a
still-present `old_string` matches again and lands the addition twice. So
duplication is one outcome of the misreading, not its definition, and the reason
this is the deciding cost stands without it: the agent's next decision is made
against a tree it believes is in a state it is not. `AGENTS.md` states it that
way.

**Option 1 would go red on thirteen files on day one.** Re-counted here at
`3ad895a`, the census is *identical* to the one taken at `b293242` in the
Measured section: 289 tracked `.go`, 117 non-test with 13 over 500, 172 test
with 4 over 600. Declaring 500 today therefore does not describe this
repository; it describes an aspiration, and a gate that thirteen files violate
on the day it lands teaches contributors to route around gates. Option 1 stays
available, but it needs a threshold above 1162 or a grandfathered list, and that
is a separate argument this card does not need to win.

**Option 2 is the one that can hurt another session.** Two of the thirteen —
`internal/cli/compose.go` (1142) and `internal/lifecycle/orchestrator.go` (655)
— are named in the `scope:` of cards still in the queue. Splitting a file while
another branch holds it reproduces, in code, the collision this card's own
branch already hit in card numbers.

**One word in the ruling is descriptively wrong, and it is kept deliberately.**
Calling the workstation verdict "advisory" describes its intended status here,
not its observed effect. The review of this ruling turned up two archived cards
— `187:117` and `193:80` — where a Go file was split *specifically* to stay
under 500, neither citing a repository rule, because there is none to cite. So
the undeclared number has already moved code twice. That does not flip the
ruling: option 1's cost is thirteen red files on day one whether or not the
number has been obeyed before. It changes what the ruling is *for*. The point
is not to make contributors stop caring about file length; it is that a
constraint which has already reshaped the codebase twice should be arguable in
the repository that carries the code, and today it is not. Option 1 is the fix
for that, and this card leaves it on the table rather than closing it.

**What the ruling does not say.** It does not say the thirteen files are fine,
and it does not close the question. Splitting any one of them remains a task
that can be filed and argued on its own merits; what is now settled is that it
does not happen as a side effect of an editing verdict.

**There was a fourth option, and the card never listed it.** `file-size.yaml`
carries an `exemption:` block — `exempt_marker` matched on the bare token
`size-limit: exempt`, so `// size-limit: exempt` in a Go file's first 1KB works,
and `custom_marker_pattern` `<!-- size-limit: (\d+) -->` for a per-file budget.
Marking the thirteen exempt would have ended the misreading by deleting the
message, needs no ruling from this repository, and would have been the cheapest
of the four. It is rejected on what it costs rather than on what it takes: the
verdict is the only running signal that a Go file is growing, and suppressing it
first on the thirteen largest files removes it exactly where it carries
information. That the option was invisible until the review is itself the
finding — three options were enumerated as though they were the whole space.

**The first paragraph of the `AGENTS.md` section was wrong about its own
evidence, and the conclusion survived it.** As first written it said `make lint`
"reads `.golangci.yml`, which enables the default set plus `modernize` and
`unparam`". `make lint` is `lint: vet fmt-check` (`Makefile:44`) *plus*
`golangci-lint run ./...` *plus* `gopls check -severity=hint` over `cmd internal
tools`, and CI runs the four as separate steps (`ci.yml:34`, `:48`, `:53`,
`:72`); the config also sets `govet: enable-all: true`, disabling only
`fieldalignment` and `shadow`. So the paragraph inventoried one of four gates and
described that one incompletely. The claim it was supporting — that nothing here
measures file length — holds for all four, and the sweep it rests on
(`git grep -cE 'error_lines|max-lines|funlen|lll' -- Makefile .golangci.yml`,
no output, exit 1) was already the right binding. Corrected in the same pass as
this paragraph. The lesson generalises past this card: a true conclusion is not
evidence that the sentence written under it was ever checked, and a supporting
inventory is exactly where nobody looks, because only the claim under argument
gets read for truth.

**The commit that recorded this ruling defers to a card that does not exist.**
`4fba05d`'s closing paragraph concedes that criterion 2's binding
(`grep -ci 'file length\|파일 길이\|file size' AGENTS.md`) certifies that a
statement exists and not that it is the right one, then says "TASK-221 is where
that class gets a rule". At the time that paragraph was written there was no
TASK-221 — not on that branch, not on `master`, not anywhere; 219 was the highest
id. A commit message cannot be amended after it is pushed, so the correction
lives here: the concession stood on its own and the mitigation named beside it
did not. Naming an unfiled id as though it were a plan is the same move the
concession was apologising for, one paragraph later.

**Resolved as to the id, not as to the class.**
[TASK-221](../todo/221-the-grep-that-measured-a-binding-is-not-the-grep-that-re-runs-it.md)
was filed afterwards, and 220 and 222 landed on `master` in between, so the
reference is no longer dangling. But 221's subject is *bare wrapped tools in
bindings* — whether a binding's recorded number is the reader's number. That is
not *bindings that certify existence instead of correctness*. 221 is a card that
happens to carry the number, not the plan that was promised, and the class named
above is still unowned. This paragraph is left standing rather than deleted for
the reason the paragraph two above it gives.

**Scope check.** The ruling touched `AGENTS.md` only. `.golangci.yml`, the
`Makefile` and every `.go` file are unchanged — which is what a ruling of 3
should look like in a diff, and is why criteria 3 and 4 below are marked N/A
rather than left open.

## Completion Criteria

- [x] The ruling is recorded on this card, with the reason | verify: human — see `## Ruling`: option 3, with the three reasons that decided it and an explicit statement of what it does not settle
- [x] Whatever the ruling, `AGENTS.md` states whether Go file length is gated here and by what | verify: `grep -ci 'file length\|파일 길이\|file size' AGENTS.md` returns ≥ 1 (today: 0 — measured, not assumed) — now **4**
- [~] N/A under ruling 3. If the ruling is 1 (declare it here): the limit is in a file this repository tracks, and `make lint` or a new target reports it | verify: `git grep -cE 'error_lines|max-lines|funlen|lll' -- Makefile .golangci.yml` returns ≥ 1 (today: 0). Skip, marking N/A, under rulings 2 or 3 — re-run after the ruling: **no output, exit 1**. Recorded that way rather than as "still 0" because `git grep -c` prints nothing at all when no file matches; "0" was a value I supplied for a command that never printed one. The reading is the same — the diff did not quietly declare a limit — but the two are different observations and only one of them happened
- [~] N/A under ruling 3. If the ruling is 2 (split): the census shrinks, and the number is stated rather than implied | verify: `bash -c 'n=0; for f in $(git ls-files "*.go" | grep -v "_test.go"); do [ "$(wc -l < "$f")" -gt 500 ] && n=$((n+1)); done; test "$n" -lt 13'` exits 0 (today it exits 1, n=13). Not bound on the listing command in ## Technical Notes: that pipeline ends in `sort`, which exits 0 whatever it printed, so it would mark this criterion passed the day the card was filed. Skip, marking N/A, under rulings 1 or 3 — re-run after the ruling: exits **1** at n=**13**, unchanged. That is a null result, not a positive control, and the earlier wording here called it one. An unchanged value is consistent with "no file was split" and equally consistent with a probe that could not have seen a split; what makes it readable at all is the separate observation that `git diff --stat master..HEAD -- '*.go'` is empty, and criterion 5 below records why *that* one is weaker than it looks
- [x] No source file is split as a side effect of an unrelated card — **on this branch**, which is the only span the evidence covers | verify: human — the diff of this branch touches `AGENTS.md` and this card only; `git diff --stat master..HEAD -- '*.go'` is empty. **The qualifier is a correction, not a hedge.** As written the criterion made an unqualified claim and was marked done against `git diff master..HEAD`, a probe scoped to this branch alone: it cannot see any commit reachable from `master`, so it could only ever have returned the answer it returned, and the review of this card found two commits it structurally could not see. `tasks/_archive/187-*.md:117` splits `tools/flowcheck/rules.go` because it "had grown past the 500-line limit", and `tasks/_archive/193-*.md:80` splits `shell.go` at 468 "against a 500-line ceiling" — both on cards about something else, which is exactly the side effect this criterion names. The unqualified version of it is **false**. The binding that would have caught it is repository-wide and must exclude its own writeup: `git grep -nE '500-line (limit|ceiling)' -- 'tasks/' ':!tasks/*219-*'` returns exactly those **2** lines, where the same sweep without the exclusion returns **3** files and 4 extra matches, all of them this card arguing about them. What is true, and all that is claimed here, is that this branch did not add a third

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
