---
id: TASK-199
title: "Three archived bindings cannot produce the result recorded beside them"
type: docs
priority: P2
effort: S
created-at: 2026-08-19T17:36:34+09:00
source: "measured 2026-08-19 — 130:141 exits 1 with NameError, 066:89 exits 1 exactly when its criterion passes, 060:164 exits 0 for every possible count"
scope: "The verify: bindings on tasks/_archive/130, 066 and 060, plus the two sibling cards whose bindings also end in `| wc -l` (115, 119). Recorded results are correct and stay; only the commands change or are annotated."
status: todo
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

- [ ] `130`'s binding runs and parses the workflow | verify: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"` exits 0
- [ ] `130`'s recorded "lint job has 6 steps" is either re-derived by the binding or moved out of the binding's suffix | verify: `python3 -c "import yaml,sys; d=yaml.safe_load(open('.github/workflows/ci.yml')); print(len(d['jobs']['lint']['steps']))"` prints the number the card claims
- [ ] `066`'s binding exits 0 on the healthy state | verify: the rewritten binding, run unchanged, exits 0 today and exits non-zero against a fixture config that does warn — both directions demonstrated, not just the passing one
- [ ] `060`'s binding can fail | verify: the rewritten binding exits non-zero when the count is 0 (test with a path that matches nothing), and exits 0 today at count 40
- [ ] The two sibling `| wc -l` cards are dispositioned, not merely counted | verify: `grep -rlE 'verify:.*\| *wc -l.?( —|$)' tasks/_archive/*.md` returns 3 files (060, 115, 119), and each is either fixed or recorded as intentionally non-gating
- [ ] Every rewritten binding keeps the original alongside it, so the archive shows the correction | verify: human — read the three cards and confirm each says what the binding first was
- [ ] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

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
  are wrong.
