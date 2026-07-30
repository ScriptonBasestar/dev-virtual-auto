---
id: TASK-099
title: "USAGE.md tells the reader a reserved-name conflict is silently ignored, 30 lines above the paragraph saying it is a hard error"
type: fix
priority: P4
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
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

- [ ] The claim is corrected | verify: `grep -c '조용히 무시' USAGE.md` must be 0; print the count
- [ ] It does not simply move | verify: `grep -n '무시' USAGE.md` — print every remaining hit and confirm each is accurate
- [ ] The two paragraphs agree | verify: human — read 610-650 as one passage
- [ ] The guard is extended, or the decision recorded | verify: `go test ./internal/config/ -run Conflict` — print the tests selected
- [ ] Doc size limits hold | verify: USAGE.md stays within the 500-line / 10KB per-document standard, or is already an accepted exception — print the current size
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-076](../done/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md) — this was found while
  verifying that task's leftovers.
