---
id: TASK-154
title: "The (CI) suffix marks one of the five targets CI actually runs"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T14:30:00+09:00
source: "TASK-112 finalize verification — the closing claim's second direction"
depends-on: [TASK-112]
scope: "dva repo — Makefile help labels, .github/workflows/ci.yml"
---

# Task 154: Make the (CI) label mean something in both directions

## Problem

TASK-112 removed a `(CI)` suffix from `check-generate` because CI did not run it, and closed
with: *"The vocabulary now reads correctly in both directions: a labelled target is in CI, an
unlabelled one is not."*

The second direction is false. Measured 2026-08-03:

```
$ grep -n 'run: make' .github/workflows/ci.yml
23:        run: make doc-check
26:        run: make build
29:        run: make test
32:        run: make test-integration
49:        run: make fmt-check

$ grep -c '(CI)' Makefile
1                       # Makefile:86, on fmt-check
```

Four of the five targets CI runs carry no label. So the suffix is sufficient but not necessary,
and its absence tells a reader nothing — which is the same shape of false assurance TASK-112 was
filed to remove, just inverted. It was already false when written: TASK-112's own Problem table
lists `build`, `test` and `test-integration` as CI steps three lines above the sentence.

## Acceptance criteria

- [ ] Pick a direction and record why:
      (A) label all five, so absence is a real signal; or
      (B) drop the convention entirely, on the grounds that a Makefile is the wrong place to
      mirror a workflow file that can change without it.
- [ ] Under A, the labels are derived or gated, not hand-kept — a target added to `ci.yml`
      without a label, or a label on a target CI does not run, must fail something. A comment
      asking the next person to remember is what produced this task.
- [ ] Print the label count and the `ci.yml` target count side by side in the Resolution. A bare
      "they match now" is not a result when the denominator is unstated.
- [ ] The corrected sentence lands in `tasks/_archive/112-…md` too, or that file points here.
- [ ] `make test` exits 0.

## Notes

TASK-112's second finding is adjacent and still untracked: `ci.yml` duplicates three Makefile
recipes rather than calling them — `go vet ./...` inline at `:34` while `Makefile:79` owns `vet`;
golangci-lint pinned at `ci.yml:53` while `Makefile:44` runs the mise-pinned binary; the gopls
steps at `:63-79` mirror the Makefile and say so in a comment. Versions agree today
(`.mise.toml:8` golangci-lint 2.12.2, `:9` gopls 0.22.0), so the drift is held off by comments
rather than by structure. Option B above makes that duplication worse, which is an argument for A
— or for a separate task that makes `ci.yml` call the Makefile throughout.
