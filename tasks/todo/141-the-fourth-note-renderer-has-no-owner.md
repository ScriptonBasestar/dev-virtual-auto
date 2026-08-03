---
id: TASK-141
title: "hooks.go still renders note: inline, on a different stream and indent than writeNote"
type: chore
priority: P3
status: todo
effort: S
created-at: 2026-08-03T13:00:00+09:00
source: "TASK-086 finalize verification — scoped out by 086, handed to 093, scoped out by 093, now unowned"
depends-on: [TASK-086, TASK-093]
scope: "dva repo — internal/cli/hooks.go, internal/cli/provision.go"
---

# Task 141: Route the last inline note renderer through `writeNote`

## Problem

TASK-086 introduced `writeNote` (`internal/cli/provision.go:124`) so the `note:` key would
render identically everywhere. Two call sites use it — `:152` (sequential, `os.Stdout`)
and `:271` (parallel, into the per-step `bytes.Buffer`). A third copy lived in
`compose.go` and was deleted by TASK-093 (commit `8ae8da5`).

The fourth copy is still inline, at `internal/cli/hooks.go:181-188`:

```go
if step.Note != "" {
    fmt.Fprintln(os.Stderr)
    for line := range strings.SplitSeq(step.Note, "\n") {
        fmt.Fprintf(os.Stderr, "  %s\n", line)
    }
    fmt.Fprintln(os.Stderr)
}
```

It differs from `writeNote` on two axes at once — **stderr** where `provision.go` writes
stdout, and a **two-space** indent where `writeNote` uses four. So the same `note:` string
renders differently depending on which command reached it, which is precisely the drift
`writeNote` was created to end.

## Why it has no owner

TASK-086 scoped it out and handed it to TASK-093. TASK-093 scoped it out too, recording it
as "TASK-088's and TASK-086's shared observation" and noting the stream question was
"not settled, only reduced from four independent decisions to three". All three tasks are
now archived. A sweep of `tasks/todo/`, `tasks/blocked/` and `tasks/plan/` on 2026-08-03
found 0 files mentioning `writeNote` or `hooks.go` — the thread exists only inside closed
task files, so it will not resurface on its own.

## Acceptance criteria

- [ ] `hooks.go`'s note block calls `writeNote`; `grep -c 'step.Note' internal/cli/hooks.go`
      shows the inline loop is gone — print the count.
- [ ] The stream choice is decided rather than inherited: either `writeNote` takes the
      writer (it already does) and hooks passes `os.Stderr` deliberately, or hooks moves to
      stdout. Whichever it is, the reason is written down — hook output is a progress
      channel, provision output is a result channel, and that may justify the split.
- [ ] The indent is one value, not two. If two-space is right for hooks, it is right for
      `writeNote` and the provision tests get updated with it.
- [ ] `TestNativeBuildLoopPrintsNote` and the three tests in
      `internal/cli/provision_note_test.go` still pass, and one of them now pins the hooks
      rendering against `writeNote`'s so the two cannot diverge again.
- [ ] `make test` exits 0.

## Notes

Related but distinct: TASK-142 covers `validate`'s warning/verdict stream split. This task
is only about the `note:` renderer. Fixing both with one convention would be better than
fixing them with two.
