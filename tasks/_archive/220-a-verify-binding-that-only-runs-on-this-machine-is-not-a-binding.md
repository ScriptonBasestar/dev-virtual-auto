---
id: TASK-220
title: "A verify binding that only runs on this machine is not a binding"
type: docs
priority: P2
effort: L
created-at: 2026-08-20T22:40:00+09:00
source: "found while fixing TASK-199's five named bindings. 199 measured its class on one axis (`| wc -l`, 3 cards) and explicitly did not sweep; this is the sweep, on three axes it did not name"
scope: "verify: bindings in tasks/**/*.md, and a new portability check in tools/doccheck. No Go source outside tools/doccheck, no behaviour change to dva"
status: done
completed-at: 2026-08-26T11:46:05+09:00
completion-summary: "Gate non-portable verify bindings in doccheck and repair or reclassify every live instance across the task archive."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./tools/doccheck -count=1"
    result: "passed"
  - kind: automated
    command-or-step: "make doc-check"
    result: "passed; all three portability counters are zero"
  - kind: automated
    command-or-step: "dva lint"
    result: "passed; 294 Go files formatted, 0 issues"
  - kind: manual
    command-or-step: "plant one instance of each portability axis in a scratch task card"
    result: "doccheck failed with escaped_pipe_bindings=1, abs_checkout_bindings=1, external_corpus_bindings=1 and named each file:line; scratch card removed"
quality-review: pass
quality-reviewed-at: 2026-08-26T11:48:20+09:00
quality-review-evidence:
  - "independent reviewer found no correctness, scope, regression, or portability findings"
  - "go test ./tools/doccheck -count=1, go run ./tools/doccheck, git diff --check, dva lint, and make doc-check passed"
  - "scratch fail-closed probe reported all three planted bindings with counters 1/1/1"
quality-review-receipt: tmp/task-management/direct/queue-run/task-220-review-receipt.json
archived-at: 2026-08-26T11:49:15+09:00
verified-at: 2026-08-26T11:49:15+09:00
verification-summary: "Doccheck rejects all three non-portable verify-binding classes, the archive has zero live findings, and the gate is proven fail-closed."
---

# Task 220: A verify binding that only runs on this machine is not a binding

## Summary

[TASK-199](199-three-archived-bindings-cannot-produce-the-result-recorded-beside-them.md)
fixed five bindings that could not produce the result recorded beside them, and said in its
own Open Questions that it had not swept for the general shape. Sweeping it turns up three
distinct axes, none of which `199` names, and all of which break the same promise: *a later
reader can re-run this command and get the recorded answer.*

This card was drafted as TASK-216 and renumbered on rebase, a peer session having landed
216-219 on master while the branch was open. Every count re-measures identically on the
rebased base `3ad895a`: its five new cards add no instance of any of the three axes.

Every count below is measured at `dc762ca` plus this branch's edits, under one rule stated
in full because the numbers move when any part of it changes: a **criterion line** (`- [ ]`,
`- [x]` or `- [~]`), fenced regions removed exactly where `tools/doccheck` removes them, the
**first inline code span after `verify:`** and nothing else on the line, and `human —`
bindings dropped because they promise nothing mechanical. Each clause is load-bearing.
Reading every span on the line instead of the first counts `199`'s and this card's
*quotations* of the defect as instances of it. Keeping fenced regions admits `199:46`, which
is a quotation inside a ``` block. Dropping `[~]` loses `063:162`, a real instance. An
earlier draft of this card mixed those rules between the two halves of one pair and recorded
`22 | 11` for axis 3, a line count from one population and a card count from another.

| axis | lines | cards | what a later reader gets |
|---|---|---|---|
| shell pipe written `\|` | 6 | 5 | `find: \|: unknown primary or operator` — the command never runs |
| absolute path to this checkout | 55 | 25 | `cd: no such file or directory` in any other clone |
| `~/mydevbox` external corpus | 20 | 10 | nothing to point at; the corpus is one laptop's |

### Axis 1 — the pipe is escaped, so it is not a pipe

`tools/doccheck/verifyrun.go:85-91` already states the rule in a comment: GFM processes a
backslash escape inside a code span **only in a table row**, so outside one, `\|` reaches
the shell as a literal backslash and pipe. Every one of the 6 is a list item, not a table
row. `doccheck` applies that knowledge to `go test … -run` patterns and to nothing else.

The 6, all actionable: `057:119`, `059:156`, `063:162`, `065:101`, `065:102`, `104:163`.
`060:162` and `060:164` were the same defect and are fixed on this branch —
`find . … -print0 \| xargs -0 …` was measured dying with
`find: |: unknown primary or operator` before it reached `xargs`.

An earlier draft listed `199:46` as a seventh and called the actionable set 6 in 5 cards.
`199:46` is a quotation inside a fenced block, so it is not in the population at all under
the rule above; the number was right by cancellation, because this card's own criterion 3
was written as `` verify: `\|` `` and *was* a live seventh instance. A card whose sweep
counts its own criterion is the trap it exists to catch, so criteria 3 and 4 are now
`human —` prose and the count is 6.

**Not in this axis:** nine further bindings contain `\|` inside a quoted `grep` pattern,
where it is BRE alternation and entirely correct — `grep -rn "환경 변수 우선순위\|Priority:"`
(`012:85`), plus `022:63`, `075:92`, `113:168`, `196:73`, `199:89`, `201:43`, `201:44` and
`208:80`. A sweep on "the binding contains `\|`" flags all nine too and reports 15. The discriminator is whether
the backslash sits at shell top level, outside quotes — but "outside quotes" has to be
evaluated with a nesting stack, not a boolean. `063:162` is the counterexample:
`git tag \| /usr/bin/grep -qx "v$(./bin/dva version \| /usr/bin/awk '{print $NF}')"` puts
its second `\|` inside double quotes *and* inside `$( )`, where the shell reads a fresh
command. It is a broken pipe, not alternation. The first extractor written for this card got
that wrong in both directions — it classified `063:162`'s inner pipe as quoted and
`075:92`'s genuinely single-quoted alternation as shell — which is the reason the rule is
written out here rather than left as "outside quotes".

One of them is a third defect and is *not* claimed here: `057:119` writes
`grep -vE '/tmp/\|/\.omo/evidence/'`, using BRE alternation syntax under `-E`, where `\|`
matches a literal pipe character instead. Recorded so the next reader does not "fix" the
escape and leave the wrong regex.

### Axis 2 — the binding names one laptop's checkout

55 bindings open with `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && …`, almost
all of them wrapping `make test` or `go test ./internal/…`. The command itself is perfectly
portable; the `cd` is the entire problem, and it fails *loudly* in another clone, which is
the best of the three cases. Cards `027`–`048` are a contiguous era, suggesting one habit
rather than 25 decisions.

### Axis 3 — the corpus does not exist for anyone else

20 bindings read `~/mydevbox`, a directory of live personal configs. There is no portable
replacement: the value of these criteria was that they ran DVA against configs nobody wrote
for DVA's tests. `060:164` and `066:89` are two of them and are fixed on this branch,
leaving **18 lines in 8 cards**: `056:92`, `057:119`, `058:87`, `058:88`, `058:89`,
`058:90`, `058:92`, `058:93`, `059:156`, `062:122`, `062:123`, `062:124`, `064:100`,
`064:102`, `065:101`, `065:102`, `071:74`, `071:76`. The enumeration is here rather than a
bare count because the earlier `22 | 11 → 19 in 8` arithmetic subtracted `199:46` from a
population that never contained it, and no reader could have seen that from the numbers.

The fix used for `060` and `066` is the pattern to copy: measure the corpus size first and
`exit 2` with `nothing was measured` when it is empty, so a reader without the corpus gets a
third state instead of a false pass. `066` also showed why the denominator must be *printed*
rather than asserted in prose — its recorded `31 configs` measures `25` today, with nothing
about section order having changed.

## Why this is worth a card rather than a cleanup pass

`task-validator` executes these bindings and scores `exit 0 = pass`. An earlier draft said
all three axes produce a non-zero exit and read as failing criteria on closed tasks — the
harmless direction. Half of axis 1 does the opposite. Run from the primary checkout:

| binding | rc | what it printed |
|---|---|---|
| `057:119` | **0** | `find: \|: unknown primary or operator` |
| `059:156` | 1 | `ERROR: unknown shorthand flag: 'q' in -q` |
| `063:162` | 129 | git: `unknown switch 'q'` |
| `065:101` | **0** | `ERROR: unknown shorthand flag: 'c' in -c` |
| `065:102` | 1 | `ERROR: unknown shorthand flag: 'q' in -q` |
| `104:163` | **0** | the full `dva manifest` JSON |

`057:119` and `065:101` end in `; test $? -ne 0`, which inverts the `find`/`dva` failure into
a pass; `104:163` passes because `dva` ignores the stray `\|` argv and prints the manifest
the `jq` filter never saw. Three of six are **green while measuring nothing**, which is the
class TASK-199 was filed for, on cards already closed. Axis 2 does fail loudly (`cd` to a
missing path), and axis 3 mostly returns an empty corpus. Axis 1 additionally hides a
`grep`-inverted binding (`057`, `065:101`) behind a command that never got as far as
running, so fixing the escape may expose a second defect underneath. Expect that.

## Completion Criteria

- [x] `doccheck` gains a portability check for verify bindings | verify: `n=$(/usr/bin/grep -rl 'func checkBindingPortability' tools/doccheck/ | wc -l | tr -d ' '); echo "declarations=$n"; [ "$n" -eq 1 ]` — prints `declarations=0` and exits 1 today, so this criterion can fail
- [x] The check is tested against a planted instance of each axis | verify: `n=$(/usr/bin/grep -rho 'func TestBindingPortability[A-Za-z]*' tools/doccheck/ | sort -u | wc -l | tr -d ' '); echo "test funcs=$n"; [ "$n" -ge 3 ]` — prints `test funcs=0` and exits 1 today. Bound on the test *source* rather than on a `go test` run, because a run naming a test that does not exist yet prints "no tests to run" and exits 0 — and because `doccheck`'s own TASK-136 guard rejects such a binding, which is how this line got written twice
- [x] The check reads the binding span, not the line | verify: human — a `doccheck` test case whose criterion line carries an escaped pipe in the *annotation* after a correct binding must yield 0 findings. Written as prose deliberately: an earlier draft put the two-character defect in the binding span itself, which made this card's own criterion the seventh member of the axis it was counting, and handed `task-validator` a command that exits 127
- [x] The check does not flag BRE alternation | verify: human — a test case holding a quoted `grep` alternation must yield 0 findings, and one holding the same two characters inside `$( )` *within* double quotes must yield 1. `063:162` is the live instance of the second, and the first extractor written for this card misclassified it, so a check that only asks "inside quotes?" reproduces the bug it is meant to catch
- [x] Axis 1 is closed | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && n=$(make doc-check 2>/dev/null | /usr/bin/grep -c '^escaped_pipe_bindings: *0$'); echo "escaped_pipe_bindings=0 lines in doc-check output: $n"; [ "$n" -eq 1 ]` — prints `0` and exits 1 today, because `doc-check` reports no such counter. The count is bound to the checker's own output rather than to a `grep` over `tasks/`, so the number a reader sees is the one the gate enforces
- [x] Axis 2 is closed | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && n=$(make doc-check 2>/dev/null | /usr/bin/grep -c '^abs_checkout_bindings: *0$'); echo "abs_checkout_bindings=0 lines in doc-check output: $n"; [ "$n" -eq 1 ]` — prints `0` and exits 1 today, down from the 55 recorded above once the check exists
- [x] Axis 3 is dispositioned per card, not swept | verify: human — each of the 8 remaining cards either carries the `exit 2` guard `060:164` uses, or is reclassified to a `human —` binding with the count in its prose. A mechanical rewrite of all 18 satisfies the letter and loses the intent. The guard has to cover the **tool** as well as the corpus: `066:89` was rewritten with a corpus probe and still printed its recorded `configs=25 warnings=0` and exited 0 with `dva` absent from `PATH`, because the tool's error was swallowed by the `2>&1 |` it was piped into
- [x] The gate can fail | verify: human — plant one instance of each axis in a scratch card, confirm `make doc-check` goes red naming the file and line, and remove it line-scoped
- [x] `make doc-check` and `make lint` pass | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check && make lint`

## Resolution

`tools/doccheck` now inspects only the first mechanical binding span on task criterion
lines, ignores fenced examples and later annotation spans, and reports all three portability
classes with the exact task file and line. Four test functions cover the three positive
classes plus annotation, quoted-BRE, nested-command-substitution, human, table, and fenced
controls.

The live archive is clean: shell-level escaped pipes were repaired, checkout-specific
prefixes were removed or made repository-relative, and the eight actionable personal-corpus
cards were reclassified as human evidence while preserving their measured counts in prose.
The two already guarded corpus bindings in TASK-060 and TASK-066 remain mechanical. A
temporary scratch card proved that one instance of each class makes the gate fail with
counter values `1/1/1`; it was removed before the passing run.

## Open Questions

- Axis 2's 55 bindings mostly wrap `make test`. Deleting the `cd` is correct, but it changes
  55 lines across 25 archived cards in one diff. Whether that is one commit or one per era is
  a review-load question, not a correctness one.
- Rewriting an archived binding rewrites the record. `130`, `060`, `066`, `115` and `119` on
  this branch each keep the original quoted in the annotation for that reason. At 55 lines
  that convention costs more than it returns, and axis 2 may deserve a single note in each
  card instead of a per-line one.
- Whether `task-validator` should skip `tasks/_archive/` entirely is a real alternative to
  fixing axes 2 and 3. It would make the archive un-re-validatable by design, which is
  honest but forecloses ever noticing drift like `066`'s `31 → 25`.

## References

- [TASK-199](199-three-archived-bindings-cannot-produce-the-result-recorded-beside-them.md)
  — the five bindings this sweep generalises, and the `exit 2` pattern to copy
- `tools/doccheck/verifyrun.go:85-91` — the rule, already written down and applied to one
  binding shape only
- `tools/doccheck/verifyrun.go:111` — `inTableRow`, the discriminator axis 1 needs
