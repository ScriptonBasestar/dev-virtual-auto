---
id: TASK-199
title: "Three archived bindings cannot produce the result recorded beside them"
type: docs
priority: P2
effort: S
created-at: 2026-08-19T17:36:34+09:00
source: "measured 2026-08-19 — 130:141 exits 1 with NameError, 066:89 exits 1 exactly when its criterion passes, 060:164 exits 0 for every possible count"
scope: "The verify: bindings on tasks/_archive/130, 066 and 060, plus the two sibling cards whose bindings also end in `| wc -l` (115, 119). Recorded results are correct and stay; only the commands change or are annotated."
status: done
verified-at: 2026-08-20T23:10:00+09:00
archived-at: 2026-08-20T23:10:00+09:00
verification-summary: |
  Seven bindings rewritten across five cards, each quoting the original beside it. Every
  rewrite was run in three directions and the failing direction was produced by varying the
  corpus, not the command: the byte-identical binding text was executed from a fixture tree
  under the scratchpad, so no tracked file was edited to manufacture a failure. 130 prints
  `lint steps: 6` and now gates on it (asking the same text for n == 7 exits 1). 066 gives
  configs=25 warnings=0 exit 0, configs=2 warnings=1 exit 1, and exit 2 on an empty corpus.
  060 gives canonical=40 of 81 exit 0, canonical=0 of 2 exit 1, exit 2 on an empty corpus.
  115:125 gives declarations=1 call sites=8 exit 0 and exit 1 against a tree with a second
  implementation; 115:126 and 119:89 print their denominators (257 files, 811 lines) and
  exit 1 against planted instances. Three defects outside the plan were found and fixed:
  060's pipes were written \| and the command died in find before the wc -l defect could be
  reached (060:162 carried the same and was fixed with it); 115:125 stated no target, so the
  same command printing 4 at its archival commit 2065066 and 0 today read alike; and every
  "must be 0" gate passed vacuously over a missing path. Two recorded counts had drifted
  against the card's own instruction — 066's 31 measures 25, 060's 50 measures 40 of 81 —
  which is the argument for printing a denominator rather than asserting one. Criterion 5
  measured itself wrong and is recorded as such: its pattern matches the quotations that
  criterion 6 mandates, so after the fix it still returns 3 files — its recorded count, with
  none of its recorded membership, since 060 drops out and this card enters through a fenced
  quotation at 199:78; on the binding-span axis it is 4 bindings in 3 files before and 0
  after. Review found three
  more, all of the same shape as the defects above and all fixed here: 066:89 printed its
  recorded configs=25 warnings=0 and exited 0 with dva absent from PATH, because the tool's
  error was swallowed by the pipe it was written into, so it now probes the tool as well as
  the corpus; 060:162 was given the pipe fix but no denominator and still printed nothing on
  success, and now sweeps git ls-files rather than find, whose count moves from 4395 to 423
  between two checkouts of the same commit; and four of the seven rewrites were bound by no
  criterion at all, which criteria 8-11 close one site each, running the published span and
  asserting its printed denominator — all four exit 1 against master, where those cards still
  carry the original text, and exit 2 if the named line stops being the named criterion. make doc-check OK (broken_links 0,
  oversized_docs 0, run_patterns 128, unmatched_run 0, archive_cards 206). The unswept
  general shape is filed as TASK-216 with three measured axes: 6 escaped-pipe bindings, 55
  naming this machine's checkout, 20 rooted at ~/mydevbox.
---

# Task 199: Three archived bindings cannot produce the result recorded beside them

## Summary

A `verify:` binding is a promise that a later reader can re-run the command and get the
recorded answer. Three archived bindings break that promise in three distinct ways. In all
three the *recorded number is correct* — it was obtained some other way — so the defect is
not a wrong record but an unreproducible one.

**`130:141` — the command errors before it measures.**

```
- [x] CI workflow still parses | verify: `python3 -c "yaml.safe_load(open('.github/workflows/ci.yml'))"` — **OK, lint job has 6 steps**
```

Re-run 2026-08-19: `NameError: name 'yaml' is not defined`, exit 1. There is no `import
yaml`. The command cannot parse anything, and it certainly cannot count six steps. Of the
six `python3 -c` bindings in the archive, this is the only one missing its import.

**`066:89` — the command exits 1 exactly when the criterion passes.**

```
- [x] No live config warns on section order | verify: `… | while read f; do (cd "$(dirname "$f")" && dva validate 2>&1 | grep -F 'section order: found'); done` — expect no output over 31 configs
```

Re-run 2026-08-19: no output, exit 1. The loop's status is the last `grep`'s, and `grep`
exits 1 when it finds nothing — which is the success condition. Under the `task-validator`
contract (command exit 0 = pass) this criterion reports failure whenever the repository is
healthy, and would report success only if a config started warning.

**`060:164` — the command cannot fail.**

```
- [x] Live user configs carry the canonical URL | verify: `find ~/mydevbox -name dva.yml -print0 \| xargs -0 grep -l '…/schema.json' \| wc -l`
```

Re-run 2026-08-19: prints `40`, exit 0. A pipeline's status is its last command's, and
`wc -l` succeeds on empty input. This binding exits 0 for a count of 40 and for a count of
0 alike; it records a number but gates on nothing. Two sibling cards share the shape —
`115` and `119` also end a binding in `| wc -l` (3 cards total).

The three failure modes are worth separating because only one is a typo. The missing import
is a slip; the inverted exit and the swallowed status are both cases of a shell contract
(pipeline status, `grep`'s "not found" convention) quietly overriding the author's intent.

## Completion Criteria

- [x] `130`'s binding runs and parses the workflow | verify: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"` exits 0 — **exits 0. Without the import it is `NameError: name 'yaml' is not defined`, exit 1**
- [x] `130`'s recorded "lint job has 6 steps" is either re-derived by the binding or moved out of the binding's suffix | verify: `python3 -c "import yaml,sys; d=yaml.safe_load(open('.github/workflows/ci.yml')); print(len(d['jobs']['lint']['steps']))"` prints the number the card claims — **prints `6`.** The shipped binding goes further and gates on it: asking the same text for `n == 7` against the byte-identical file exits 1, so a seventh step reads as a red criterion rather than a changed number nobody looks at
- [x] `066`'s binding exits 0 on the healthy state | verify: the rewritten binding, run unchanged, exits 0 today and exits non-zero against a fixture config that does warn — both directions demonstrated, not just the passing one — **`configs=25 warnings=0` exit 0; against a two-config fixture ordering `stack:` before `vars:`, `configs=2 warnings=1` exit 1. A third state was added that the criterion did not ask for: a corpus with no `dva.yml` prints `nothing was measured` and exits 2, which is what any reader without this machine's `~/mydevbox` now gets instead of a silent pass over zero files**
- [x] `060`'s binding can fail | verify: the rewritten binding exits non-zero when the count is 0 (test with a path that matches nothing), and exits 0 today at count 40 — **`canonical=40 of 81` exit 0; `canonical=0 of 2` exit 1 against a fixture holding configs with no canonical ref; exit 2 for a path with no `dva.yml`.** The criterion's failing direction had to be split in two: a path that matches nothing produces the count 0 *and* an empty corpus at once, and a binding that cannot tell those apart is the defect this card is about
- [x] The two sibling `| wc -l` cards are dispositioned, not merely counted | verify: `/usr/bin/grep -rlE 'verify:.*\| *wc -l.?( —|$)' tasks/_archive/*.md` returns 3 files (060, 115, 119), and each is either fixed or recorded as intentionally non-gating — **all three fixed, and this criterion measured itself wrong twice over.** Run as written *after* the fix it returns 3 files — the recorded count, and not one of the recorded three. `060` has dropped out; `115` and `119` still match because criterion 6 requires each rewrite to quote the binding it replaces and the quotation sits on the same line; and the third file is **this card**, matched at `199:78`, where the defect is quoted inside a fenced block. The count survived while its membership turned over completely, which no reader could have seen from the number — the failure this card is about, committed by the card's own criterion. Criteria 5 and 6 are also in direct tension: as long as the archive keeps what it replaced, a line-wide pattern can never reach 0. Re-measured on the axis that separates them — the *binding span*, the first inline code span after `verify:` — it returns **4 bindings in 3 files at `dc762ca`** (`060:164`, `115:125`, `115:126`, `119:89`; one more than "3 cards" implied, since `115` has two) **and 0 after**
- [x] Every rewritten binding keeps the original alongside it, so the archive shows the correction | verify: human — read the three cards and confirm each says what the binding first was — **all seven rewritten bindings quote the original: `130:141`, `060:162`, `060:164`, `066:89`, `115:125`, `115:126`, `119:89`**
- [x] `060:162`'s rewrite is exercised by a criterion, not only described in its own annotation | verify: `f=tasks/_archive/060-go-module-path-does-not-resolve.md; l=$(sed -n '162p' "$f"); printf '%s' "$l" | /usr/bin/grep -q 'No tracked source names the old repo' || { echo '060:162 is not that criterion any more — nothing was measured'; exit 2; }; b=$(printf '%s' "$l" | sed 's/^.*verify: *//' | tr '\140' '\n' | sed -n '2p'); out=$(eval "$b"); echo "$out"; printf '%s' "$out" | /usr/bin/grep -qE 'offenders=0 of [1-9][0-9]* tracked files swept'` — **`offenders=0 of 420 tracked files swept`, exit 0.** Four of the seven rewrites — this one, `115:125`, `115:126` and `119:89` — were made by this card and asserted by nothing in it. Criteria 8-11 close that one site per criterion: a single criterion covering all four would stay green with three of them reverted. Each extracts the **published** span rather than a copy (`tr` turns the backticks into newlines, so what runs is whatever the card now says) and asserts the *printed denominator*, not the exit code, because every pre-fix binding here exited 0 while measuring nothing. Failing directions, both produced by varying the artifact and not the command: run against `master`, where these four cards still carry their original text at the same line numbers, all four exit 1 — `find: |: unknown primary or operator` for this one, a bare `0` for the other three; and with the line number moved off its criterion, all four exit 2
- [x] `115:125`'s rewrite is exercised by a criterion | verify: `f=tasks/_archive/115-four-compose-argv-builders-share-two-bugs.md; l=$(sed -n '125p' "$f"); printf '%s' "$l" | /usr/bin/grep -q 'The four copies are one' || { echo '115:125 is not that criterion any more — nothing was measured'; exit 2; }; b=$(printf '%s' "$l" | sed 's/^.*verify: *//' | tr '\140' '\n' | sed -n '2p'); out=$(eval "$b"); echo "$out"; printf '%s' "$out" | /usr/bin/grep -qE 'declarations=1 call sites=[2-9][0-9]*'` — **`declarations=1 call sites=8`, exit 0; exit 1 on `master`, where the published binding prints `0` and exits 0 — the case that made 4 and 0 read alike**
- [x] `115:126`'s rewrite is exercised by a criterion | verify: `f=tasks/_archive/115-four-compose-argv-builders-share-two-bugs.md; l=$(sed -n '126p' "$f"); printf '%s' "$l" | /usr/bin/grep -q 'Path joining is uniform' || { echo '115:126 is not that criterion any more — nothing was measured'; exit 2; }; b=$(printf '%s' "$l" | sed 's/^.*verify: *//' | tr '\140' '\n' | sed -n '2p'); out=$(eval "$b"); echo "$out"; printf '%s' "$out" | /usr/bin/grep -qE 'occurrences=0 over [1-9][0-9]* files'` — **`occurrences=0 over 257 files`, exit 0; exit 1 on `master`, whose binding prints `0` for a clean tree and for a missing path alike**
- [x] `119:89`'s rewrite is exercised by a criterion | verify: `f=tasks/_archive/119-doctor-compose-check-ignores-the-configured-command.md; l=$(sed -n '89p' "$f"); printf '%s' "$l" | /usr/bin/grep -q 'No sixth copy' || { echo '119:89 is not that criterion any more — nothing was measured'; exit 2; }; b=$(printf '%s' "$l" | sed 's/^.*verify: *//' | tr '\140' '\n' | sed -n '2p'); out=$(eval "$b"); echo "$out"; printf '%s' "$out" | /usr/bin/grep -qE 'occurrences=0 in internal/cli/doctor.go \([1-9][0-9]* lines\)'` — **`occurrences=0 in internal/cli/doctor.go (811 lines)`, exit 0; exit 1 on `master`, whose binding prints `0` whether the file holds no literal or has been deleted**
- [x] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check` — **OK. `broken_links` 0, `oversized_docs` 0, `run_patterns` 128, `unmatched_run` 0, `archive_cards` 206**

## Resolution

Seven bindings rewritten across five cards, each keeping the original quoted beside it.
Three of the seven were not in the card's plan.

**`060` was broken twice over, and the second defect hid the first.** Both its pipes were
written `\|`. GFM processes a backslash escape inside a code span *only in a table row*, and
these are list items, so the backslash was literal: the command handed `|` to `find` as an
argument and died with `find: |: unknown primary or operator`. The swallowed `wc -l` status
this card was filed for was therefore never even reached. `060:162` — one line above, not in
scope, same defect — was fixed with it rather than left to a sweep that might not come.

**`115:125` had no target, so it could not drift.** Its binding printed a count and the card
never wrote the count down. Run at `115`'s own archival commit `2065066` it prints **4**; run
today it prints **0**. All three files it names still exist — none was renamed — they simply
no longer call `SplitCommand`. The 8 call sites are in 6 other files: `internal/exec/exec.go`
(beside the one declaration), `internal/exec/compose_argv.go`, `internal/runner/runner.go`,
`internal/cli/validate.go`, and `internal/runner/docker_compose.go` and
`internal/runner/kubectl.go` with two each. With no target stated, 4 and 0 read alike. It was also measuring the wrong thing: three named files cannot show that *one*
implementation exists. Rebound on `declarations=1 call sites=8`.

**Every "must be 0" gate needed a denominator, not just a comparison.** `grep` over a path
that does not exist prints `0` exactly as a clean tree does, so `[ "$n" -eq 0 ]` alone still
passes by measuring nothing. `115:126` and `119:89` now print the corpus size (`257 files`,
`811 lines`) and exit 2 when it is empty.

All five rewrites were sabotaged, and the sabotage varied the **corpus, not the command**:
the byte-identical binding text was run from a fixture tree under the scratchpad carrying a
second `func SplitCommand`, a `cfgDir + "/"` join, and a `doctor.go` holding `"compose"`.
Each returned exit 1; each returned exit 2 against an empty tree. Nothing in a tracked file
was edited to produce a failure, so there is no cleanup to have got wrong.

Two recorded numbers had drifted, against this card's own instruction not to change any:
`066`'s "31 configs" measures **25**, and `060`'s summary line records 50 canonical configs
where the criterion now measures **40 of 81**. Neither is a wrong record — `~/mydevbox` is a
live personal corpus and simply changed. That is the argument for printing a denominator
rather than asserting one in prose, and it is why the rewrites report `configs=$n` and
`canonical=$n of $all` instead of a bare verdict.

The Open Questions asked for a per-card decision rather than a mechanical `-gt 0`. `060` got
one: `40` is not the whole population and never was — 81 configs exist, 54 carry some
`schema.json` reference and 14 carry a non-canonical one. What the criterion can honestly
assert is that the canonical URL is *in use*, not that the migration is complete, and the
binding now says so.

The second Open Question — whether the corpus holds more of this — is answered by
[TASK-216](216-a-verify-binding-that-only-runs-on-this-machine-is-not-a-binding.md), filed
with three measured axes and their denominators, each stated with the extraction rule that
produces it: 6 bindings in 5 cards whose shell pipes are escaped, 55 in 25 cards that `cd`
into this machine's checkout, and 20 in 10 cards rooted at `~/mydevbox`.
Its first axis needed the discriminator this card learned the hard way — nine further
bindings contain `\|` inside a quoted `grep` pattern, where it is correct BRE alternation,
so the sweep has to ask whether the backslash sits at shell top level, not whether the line
contains one.

## References

- `tasks/_archive/130-…:141` — missing `import yaml`
- `tasks/_archive/066-…:89` — `grep`'s not-found exit inverts the criterion
- `tasks/_archive/060-…:164`, `115`, `119` — pipeline status swallowed by `wc -l`
- `task-validator` agent contract — "EXECUTING each acceptance criterion's `verify:` binding
  (command exit 0 = pass)". That contract is what makes an inverted exit a defect rather than
  a style choice.

## Open Questions

- `060`'s intent may have been to record a count rather than to gate. If so the honest fix is
  not a rewritten command but a reclassification — a `verify: human —` binding with the count
  in its prose. Deciding that per card is the actual work here; mechanically appending
  `&& [ "$(…)" -gt 0 ]` to all three would satisfy the letter and lose the intent.
- Whether the corpus has more inverted-exit bindings than `066` is unmeasured. A `grep` inside
  a loop is the general shape; this card does not claim to have swept for it, and a follow-up
  should state its axis and denominator when it does.

## Technical Notes

- Measure exit codes without a trailing pipe. Writing `cmd | tail -2; echo $?` reports
  `tail`'s status and will silently confirm whichever answer you expected — the same defect
  this card is about.
- Under zsh, `${PIPESTATUS[0]}` is empty; use `${pipestatus[1]}`.
- The sweep pattern's `.?( —|$)` is what makes the denominator 3 rather than 5: it requires
  `wc -l` to be the binding's last pipeline stage, allowing only the closing backtick and an
  optional ` — result` suffix after it. Dropping that tail matches `063:162` and `196:74`,
  where `wc -l` feeds a further stage and the exit status is not swallowed. Both counts were
  measured; only one of them is the class this card is about.
- Do not change any recorded count. All three numbers re-measure correct; only the commands
  are wrong. ⚠️ **Half wrong, corrected at completion.** `130`'s `6` re-measures correct.
  `066`'s "31 configs" measures **25**, and `060`'s summary records 50 canonical configs
  where the criterion measures **40 of 81** — both were correct when written and drifted,
  because `~/mydevbox` is a live personal corpus. The instruction still holds in its intent:
  the historical numbers stay as recorded. What changed is that the two drifting ones are no
  longer *asserted in prose* beside a command that cannot derive them — the binding prints
  its own denominator, so the next drift is visible rather than silent.
