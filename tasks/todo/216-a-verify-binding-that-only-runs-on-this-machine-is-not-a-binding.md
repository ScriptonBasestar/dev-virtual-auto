---
id: TASK-216
title: "A verify binding that only runs on this machine is not a binding"
type: docs
priority: P2
effort: L
created-at: 2026-08-20T22:40:00+09:00
source: "found while fixing TASK-199's five named bindings. 199 measured its class on one axis (`| wc -l`, 3 cards) and explicitly did not sweep; this is the sweep, on three axes it did not name"
scope: "verify: bindings in tasks/**/*.md, and a new portability check in tools/doccheck. No Go source outside tools/doccheck, no behaviour change to dva"
status: todo
---

# Task 216: A verify binding that only runs on this machine is not a binding

## Summary

[TASK-199](199-three-archived-bindings-cannot-produce-the-result-recorded-beside-them.md)
fixed five bindings that could not produce the result recorded beside them, and said in its
own Open Questions that it had not swept for the general shape. Sweeping it turns up three
distinct axes, none of which `199` names, and all of which break the same promise: *a later
reader can re-run this command and get the recorded answer.*

Every count below is measured at `dc762ca` plus this branch's edits, on the **binding span
only** — the first inline code span after `verify:` — and excluding `human —` bindings,
which promise nothing mechanical. That axis matters: a looser sweep that reads every code
span on the line counts `199`'s and this card's *quotations* of the defect as instances of
it, and reports 18 where the real number is 7.

| axis | lines | cards | what a later reader gets |
|---|---|---|---|
| shell pipe written `\|` | 7 | 6 | `find: \|: unknown primary or operator` — the command never runs |
| absolute path to this checkout | 55 | 25 | `cd: no such file or directory` in any other clone |
| `~/mydevbox` external corpus | 22 | 11 | nothing to point at; the corpus is one laptop's |

### Axis 1 — the pipe is escaped, so it is not a pipe

`tools/doccheck/verifyrun.go:85-91` already states the rule in a comment: GFM processes a
backslash escape inside a code span **only in a table row**, so outside one, `\|` reaches
the shell as a literal backslash and pipe. Every one of the 7 is a list item, not a table
row. `doccheck` applies that knowledge to `go test … -run` patterns and to nothing else.

The 7: `057:119`, `059:156`, `063:162`, `065:101`, `065:102`, `104:163`, and `199:46`.
The last is `199` quoting `060`'s broken binding as evidence and archives with it, so the
actionable set is **6 lines in 5 cards**. `060:162` and `060:164` were the same defect and
are fixed on this branch — `find . … -print0 \| xargs -0 …` was measured dying with
`find: |: unknown primary or operator` before it reached `xargs`.

**Not in this axis:** nine further bindings contain `\|` inside a quoted `grep` pattern,
where it is BRE alternation and entirely correct — `grep -rn "환경 변수 우선순위\|Priority:"`
(`012:85`) and eight others. A sweep on "the binding contains `\|`" flags all nine and
reports 16. The discriminator is whether the backslash sits at shell top level, outside
quotes.

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

22 bindings read `~/mydevbox`, a directory of live personal configs. There is no portable
replacement: the value of these criteria was that they ran DVA against configs nobody wrote
for DVA's tests. `060:164` and `066:89` were two of them and are fixed on this branch, and
`199:46` archives with `199`, leaving **19 lines in 8 cards** (`056`, `057`, `058`, `059`,
`062`, `064`, `065`, `071`).

The fix used for `060` and `066` is the pattern to copy: measure the corpus size first and
`exit 2` with `nothing was measured` when it is empty, so a reader without the corpus gets a
third state instead of a false pass. `066` also showed why the denominator must be *printed*
rather than asserted in prose — its recorded `31 configs` measures `25` today, with nothing
about section order having changed.

## Why this is worth a card rather than a cleanup pass

`task-validator` executes these bindings and scores `exit 0 = pass`. All three axes produce
a non-zero exit, so today they read as **failing criteria on closed tasks** — which is the
harmless direction, but it means the archive cannot be re-validated as a whole, and nobody
will notice when a criterion starts failing for a real reason. Axis 1 additionally hides a
`grep`-inverted binding (`057`, `065:101`) behind a command that never got as far as
running, so fixing the escape may expose a second defect underneath. Expect that.

## Completion Criteria

- [ ] `doccheck` gains a portability check for verify bindings | verify: `n=$(/usr/bin/grep -rl 'func checkBindingPortability' tools/doccheck/ | wc -l | tr -d ' '); echo "declarations=$n"; [ "$n" -eq 1 ]` — prints `declarations=0` and exits 1 today, so this criterion can fail
- [ ] The check is tested against a planted instance of each axis | verify: `n=$(/usr/bin/grep -rho 'func TestBindingPortability[A-Za-z]*' tools/doccheck/ | sort -u | wc -l | tr -d ' '); echo "test funcs=$n"; [ "$n" -ge 3 ]` — prints `test funcs=0` and exits 1 today. Bound on the test *source* rather than on a `go test` run, because a run naming a test that does not exist yet prints "no tests to run" and exits 0 — and because `doccheck`'s own TASK-136 guard rejects such a binding, which is how this line got written twice
- [ ] The check reads the binding span, not the line | verify: a test case whose line carries `\|` in prose *after* a correct binding must not be flagged — plant it and assert 0 findings, since a sweep on the whole line reports 18 where the truth is 7
- [ ] The check does not flag BRE alternation | verify: a test case with `grep "a\|b"` inside quotes must not be flagged; without this the check reports 16 instead of 7
- [ ] Axis 1 is closed | verify: `make doc-check` exits 0 with the new check active, and the check's own count for axis 1 is `0` over `tasks/` excluding `tasks/_archive/199-*.md`
- [ ] Axis 2 is closed | verify: the check's count for absolute-checkout bindings is `0`, down from the 55 recorded above
- [ ] Axis 3 is dispositioned per card, not swept | verify: human — each of the 8 remaining cards either carries the `exit 2` guard `060:164` uses, or is reclassified to a `human —` binding with the count in its prose. A mechanical rewrite of all 19 satisfies the letter and loses the intent
- [ ] The gate can fail | verify: human — plant one instance of each axis in a scratch card, confirm `make doc-check` goes red naming the file and line, and remove it line-scoped
- [ ] `make doc-check` and `make lint` pass | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check && make lint`

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
