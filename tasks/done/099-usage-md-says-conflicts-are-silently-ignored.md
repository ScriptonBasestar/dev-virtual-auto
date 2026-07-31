---
id: TASK-099
title: "USAGE.md tells the reader a reserved-name conflict is silently ignored, 30 lines above the paragraph saying it is a hard error"
type: fix
priority: P4
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
completed-at: 2026-07-31T11:35:00+09:00
scope: "USAGE.md:614-615 — the sentence contradicted by USAGE.md:645-649 and by internal/config/reserved.go"
---

# Task 099: one page, two answers, and the wrong one comes first

## Problem

USAGE.md:614-615:

> `interaction:` 키는 `dva run <name>`으로 실행할 커맨드를 정의합니다. 이름이 내장
> 커맨드와 겹치면 **조용히 무시될 수 있으므로** 아래 규칙을 따릅니다.

USAGE.md:645-646, in the same section:

> 충돌은 **경고가 아니라 에러**입니다 — `dva validate`(= `dva config validate`)가 exit 1로
> 실패합니다.

and USAGE.md:649:

> 선언이 버려지는 것은 아닙니다. 짧은 형식(`dva build`)만 내장 커맨드에게 넘어가고, 선언한
> 커맨드 자체는 `dva run build`로 그대로 실행됩니다.

The later text matches the code; the earlier one does not:

- `internal/config/validate.go:120-131` returns a hard error for any conflict.
- `internal/config/config.go:879` calls `WarnReservedCommandConflicts` on **every** config load,
  which `slog.Warn`s — visible, not silent.
- `internal/config/reserved.go:158-160` carries a comment saying exactly this: *"The interaction is
  not discarded: measured, `dva run status` still executes it… Telling the reader it was ignored
  sends them looking for a command that never ran."*

So "조용히 무시될 수 있으므로" is stale wording that the code comments were already written to
refute, and `reserved_test.go` already greps for the word "ignored" to keep it out of the Go
strings — the doc simply was not covered by that guard.

## Fix

Correct the sentence at 614-615 to match 645-649 and the code: the conflict is an error that fails
`dva validate` with exit 1, and the declaration is not discarded — only the short form is taken by
the built-in.

Worth checking in the same pass whether the `reserved_test.go` "ignored" guard can be extended to
cover USAGE.md, which is what would have caught this.

## Acceptance criteria

- [x] The claim is corrected | verify: `grep -c '조용히 무시' USAGE.md` must be 0; print the count
- [x] It does not simply move | verify: `grep -n '무시' USAGE.md` — print every remaining hit and confirm each is accurate
- [x] The two paragraphs agree | verify: human — read 610-650 as one passage
- [x] The guard is extended, or the decision recorded | verify: `go test ./internal/config/ -run Conflict` — print the tests selected
- [x] Doc size limits hold | verify: USAGE.md stays within the 500-line / 10KB per-document standard, or is already an accepted exception — print the current size
- [x] Full suite passes | verify: `make test`

## Resolution

The lead sentence now says what the code does — validate fails with exit 1, a warning is printed on
every config load, and the declaration is not discarded — which is what the paragraph 30 lines below
already said. `grep -c '조용히 무시' USAGE.md` = **0**, and `grep -n '무시' USAGE.md` returns **no hits
at all**, so the claim did not move somewhere else in the document.

### The guard was extended

`TestUsageDocDoesNotSayReservedConflictsAreIgnored` in `internal/config/reserved_test.go`, next to
the `mustNotHav: "ignored"` guard it extends. That guard covered the Go advice strings; the document
was the copy nothing checked, which is why the claim survived.

It has three parts, because the first two alone are each satisfiable by a broken document:

| assertion | what it stops |
| --- | --- |
| the section must not contain `조용히 무시` / `silently ignored` | the false claim returning |
| the section must contain `exit 1` and `에러` | the sentence being deleted instead of corrected |
| a missing heading is `t.Fatalf`, not a skip | the guard scanning an empty string and passing |

`go test ./internal/config/ -run Conflict` selects **16** entries, and the new test is among them —
`ReservedConflictsAreIgnored` contains `Conflict`, so the criterion's own selector reaches it. Full
list printed during verification.

### Non-vacuity

Each part reverted separately in its own copy of the tree:

| probe | result |
| --- | --- |
| A — the false claim restored | FAIL: `USAGE.md still says "조용히 무시"` |
| B — the `exit 1`/`에러` sentence deleted | FAIL: `no longer states "에러"` |
| C — the heading renamed | FAIL: `has no section titled "### interaction (예약어와 훅)"` |
| control — as committed | PASS, `section scanned: 65 lines` |

Probe B fired on `에러` but not on `exit 1`: the deleted sentence carried both, and `exit 1` still
appears elsewhere in the section. The assertion held; only one of its two terms was load-bearing for
that particular deletion.

### Doc size — over the standard, pre-existing

USAGE.md is **730 lines / 27KB** against the 500-line / 10KB standard in
`skill:docs:doc-standards`. It was **729 lines / 27KB at HEAD**; this change added one line. Every
other root document is inside the limit (README 203, AGENTS 172, ARCHITECTURE 171, PRODUCT 71,
SOUL 68 lines).

Not treated as an accepted exception and not fixed here — splitting the user manual is a structural
change with its own blast radius, not something to fold into a one-sentence correction. Filed as
[TASK-106](../done/106-usage-md-is-46-percent-over-the-doc-size-standard.md).

## Related

- [TASK-076](076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md) — this was found while
  verifying that task's leftovers.
