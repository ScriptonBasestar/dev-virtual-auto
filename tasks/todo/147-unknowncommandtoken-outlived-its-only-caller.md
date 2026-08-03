---
id: TASK-147
title: "unknownCommandToken has no production caller and a doc comment addressing callers that do not exist"
type: chore
priority: P3
status: todo
effort: S
created-at: 2026-08-03T13:40:00+09:00
source: "TASK-098 finalize verification — collateral of TASK-108, untracked"
depends-on: [TASK-098, TASK-108]
scope: "dva repo — internal/cli/root.go"
---

# Task 147: Retire or re-justify `unknownCommandToken`

## Problem

TASK-098 added `unknownCommandToken` (`internal/cli/root.go:376`) as the parser behind the
`suggestCommands` guard. TASK-108 then deleted `suggestCommands` and its call site. The
parser stayed.

Every reference in the repo, measured 2026-08-03:

```
internal/cli/root.go:373               (doc comment)
internal/cli/root.go:376               (definition)
internal/cli/stack_exit_code_test.go:122,123   (the only invocations)
```

No production caller. Because a test still calls it, the `unused` linter cannot see it —
the function is dead in exactly the way static analysis will not report.

Two doc comments now describe machinery that is gone:

- `root.go:375` — "Empty when the message is not in that shape, which callers must read as
  'cannot tell', not as a match." There are no callers to read it either way.
- `stack_exit_code_test.go:108` — "pins the parser behind the suggestion guard in root.go".
  There is no suggestion guard.

The neighbouring helper shows what the right outcome looks like: `levenshtein` was kept
after the same deletion, and `root.go:390` says why — "Retained after TASK-108 removed
suggestCommands: stack.go and provision.go still use it". `unknownCommandToken` got no such
sentence, because there is no such reason.

## Acceptance criteria

- [ ] Decide: delete it (with its test), or name the production caller that justifies
      keeping it. Print `grep -rn unknownCommandToken --include='*.go' .` before and after.
- [ ] If kept, its doc comment gets the same treatment `levenshtein` received — who uses it
      and why it survived TASK-108.
- [ ] If deleted, `stack_exit_code_test.go`'s remaining cases still cover the exit-code
      behaviour TASK-098 actually shipped; the token parser was a means, not the deliverable.
- [ ] `make lint` and `make test` both exit 0.

## Notes

Worth a wider pass while here: any other helper whose only caller is a test is dead in the
same undetectable way. This one was found by reading TASK-108's diff, not by tooling.
