---
id: TASK-200
title: "The flowcheck rules page overstates its scope in six places"
type: docs
priority: P3
effort: S
created-at: 2026-08-19T17:36:34+09:00
source: "measured 2026-08-19 against tools/flowcheck source — a stated non-goal the page itself contradicts 69 lines later, an exit-0 claim the source denies, an exact-match map written as a wildcard, a closed set the next sentence reopens, one rule with no stated trigger, and a sample output reflowed away from what the tool prints"
scope: "docs/51-flowcheck-rules.md prose only. No rule is added, removed or renamed; tools/flowcheck is not touched."
status: done
completed-at: 2026-08-26T12:23:20+09:00
completion-summary: "Correct six scope, trigger, and output claims in the canonical flowcheck rule reference without changing rule behavior."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "make doc-check"
    result: "passed; 279 Markdown files, 552 links, 16 flowcheck rules and all decision-path gates valid"
  - kind: automated
    command-or-step: "compare go run ./tools/flowcheck first line with the documented sample"
    result: "byte-for-byte identical"
  - kind: manual
    command-or-step: "cross-check corrected prose against tools/flowcheck gate.go, shell.go, and rules.go"
    result: "scope, seven exact field names, bare-word trigger set, jq/tmp guard, and batch behavior match source"
quality-review: pass
quality-reviewed-at: 2026-08-26T12:24:39+09:00
quality-review-evidence:
  - "independent reviewer confirmed all six prose corrections match the canonical flowcheck implementation without changing rule behavior"
  - "make doc-check and go test ./tools/flowcheck -count=1 passed; all sixteen source rule ids remain documented"
  - "the documented summary line matches flowcheck byte-for-byte and the page remains within the enforced size caps"
quality-review-receipt: tmp/task-management/direct/queue-run/task-200-review-receipt.json
archived-at: 2026-08-26T12:26:18+09:00
verified-at: 2026-08-26T12:26:18+09:00
verification-summary: "The canonical flowcheck reference now states each rule's actual scope, trigger, and literal output while retaining all sixteen rule ids."
---

# Task 200: The flowcheck rules page overstates its scope in six places

## Summary

`docs/51-flowcheck-rules.md` is the page a contributor reads to predict whether their flow
edit will pass `make doc-check`. Six statements make it predict wrong. None of the sixteen
rule ids is incorrect — the ids were re-derived from source two independent ways and all
sixteen match — so this is precision work on the prose around a correct list.

**1. `:14` — a stated non-goal the same page contradicts.** The scope section says the tool
does not look at YAML schema ("그건 `am validate`"). `param-type` does exactly that:
`tools/flowcheck/rules.go:210` — "checkParameters enforces flow.schema.json's Parameter
contract". The page documents it 69 lines later under a heading literally named `## 스키마`
(`:83`). The scope line at `:13` has the same gap: `parameters:` and the `exit_if_empty:`
key are neither shell fields nor `when:` operands.

**2. `:4` — "all sixteen exit 0" is overstated.** The opening claims every rule here covers
a case where "am이 exit 0으로 끝내면서 틀린 답을 낸다". `tools/flowcheck/shell.go:43` says a
blocked step "fails a batch run and, interactively, prompts and defaults to continue". A
failing batch run is not exit 0, and the page's own `## 셸 텍스트` section describes these as
blocked at run rather than silently wrong.

**3. `:33` — `file.*` is not a wildcard.** `gate-skip-prompt`'s row writes the field set as
`instruction`·`prompt`·`file.*`·`src`. `tools/flowcheck/gate.go:96` is an exact-match map of
seven keys and the lookup at `gate.go:140` is `skipPromptFields[r.field]` — exact, not
prefix. A field named `file.dest` classifies as `gate-skip-leak`, which is a different rule
with different advice.

**4. `:51` — a closed set the next sentence reopens.** "위 네 명령만 인자를 명령 이름으로
읽는다" states the four `bareWordTriggers` as exhaustive; `shell.go:139` says "`eval` and
`exec` block the same way and are deliberately absent". The four are the rule's triggers, not
the complete set of commands am misreads, and the page's own following clause says so.

**5. `:74` — `unguarded-report` is the only row with no stated trigger.** The rule fires only
when a field matches both `\bjq\b` (`shell.go:19`) and a `tmp/` path and does not already use
`jq -e -s` (`shell.go:21`). The row gives the reason but never the condition, so a reader
cannot tell whether their own `jq` field is in scope. Every other row in the document names
its trigger.

**6. `:113-116` — the sample output does not match what the tool prints.** flowcheck emits
its summary on one line; the page reflows it onto three. The content is identical when
joined, but a reader grepping the literal string will not match, which is precisely how this
page is used.

Size, for the record rather than as a defect: 120 lines against the 100-line soft guide, and
8747 bytes against the 10240-byte cap that `tools/doccheck` enforces under `docs/` and
`workflows/` (`tools/doccheck/policy.go:6-7,31`). It is inside the gate with roughly 1.5 KB
of headroom, so corrections must be made without growing it much.

## Completion Criteria

- [x] The scope section names the two structural YAML checks it currently disclaims | verify: `grep -n 'param-type' docs/51-flowcheck-rules.md` returns a hit in the `## 검사 대상과 비목표` section (lines 11-20), not only in the schema section
- [x] The opening no longer claims all sixteen rules are exit-0 cases | verify: human — read `:4` and confirm it distinguishes the silently-wrong rules from the ones that block a batch run, citing `tools/flowcheck/shell.go:43`
- [x] `gate-skip-prompt`'s row lists the seven exact field names instead of a wildcard | verify: `grep -c 'file\.path' docs/51-flowcheck-rules.md` returns ≥ 1 (today: 0), and the row's key count matches `sed -n '/skipPromptFields = map/,/^}/p' tools/flowcheck/gate.go | grep -c '":'` (today: 7)
- [x] The bare-word section states that the four are triggers, not the closed set | verify: human — read `:51` and confirm it and the `eval`/`exec` sentence no longer disagree
- [x] `unguarded-report`'s row states its trigger | verify: `grep -c 'jq -e -s' docs/51-flowcheck-rules.md` returns ≥ 1 and the row names both the `jq` and `tmp/` conditions
- [x] The sample output block matches the tool byte-for-byte on the summary line | verify: `go run ./tools/flowcheck | head -1` and the corresponding line in the document are identical when compared with `diff`
- [x] The page stays inside the enforced caps | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check` exits 0, and `wc -lc docs/51-flowcheck-rules.md` is under 500 lines / 10240 bytes
- [x] The sixteen ids still match source after the edit | verify: `grep -rhoE '(s\.add\(|rule :?= |rule: *)"[a-z-]+"' tools/flowcheck/*.go | sed 's/.*"\(.*\)"/\1/' | sort -u | wc -l` returns 16 and each appears in the document

## Resolution

The opening keeps silent-wrong answers as the main defect class but qualifies the shell-policy
rules that stop a batch run or prompt interactively. The scope now names the two structural YAML
checks rather than disclaiming all schema inspection. The remaining four edits use the exact
source trigger sets and the tool's literal one-line summary. No rule id or implementation changed;
the page remains 120 lines and 9,027 bytes.

## References

- `docs/51-flowcheck-rules.md:4,14,33,51,74,113-116` — the six statements
- `tools/flowcheck/rules.go:210` — `checkParameters`, the schema check the page disclaims
- `tools/flowcheck/gate.go:96,140` — the exact-match map and its lookup
- `tools/flowcheck/shell.go:19,21,43,139` — jq trigger, guard, blocked-step behaviour, eval/exec
- `tools/doccheck/policy.go:6-7,31` — the caps, and the fact they apply only under `docs/` and `workflows/`
- `6f3df72` — the seventh statement on this page, already corrected: the reserved-command list's provenance

## Open Questions

- Item 2 may be a scoping decision rather than an error: if the page means "the defect class
  is silent-wrong answers" and blocked steps are a secondary effect, then the sentence needs a
  qualifier, not a rewrite. Whoever takes this should decide which, and say which they chose.
- Item 6 could be solved by generating the sample from `go run ./tools/flowcheck` at
  `make generate` time instead of pasting it. That is a larger change than this card, and it
  would put a second file under `tools/libgen`-style ownership — see the ownership rule at
  `agent-mesh-flows/shared/library/README.md`, which currently covers exactly one file.

## Technical Notes

- Line numbers here are as of `6f3df72`; the provenance correction in that commit shifted
  everything after `:78` by +2. Re-locate by string, not by number.
- Do not fix these by adding length. The page has ~1.5 KB of headroom against a gate that is
  enforced, and three of the six fixes are replacements rather than additions.
