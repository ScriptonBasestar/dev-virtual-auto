---
id: TASK-170
title: "clean --dry-run demands consent for the deletion, then aborts without showing the preview"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T16:55:00+09:00
source: "TASK-166 review (suggestion) — measured while confirming the eight dry-run halt sites"
depends-on: [TASK-166]
scope: "dva repo — internal/cli/compose.go"
---

# Task 170: Let `clean --dry-run` show its preview without asking to destroy anything

## Problem

`cleanCmd`'s confirmation prompt is gated on `!force && (volumes || images)`
(`internal/cli/compose.go:562`). `dryRun` is not consulted. So the one command whose
purpose is to say what would be destroyed refuses to say it until you agree to the
destruction:

```
$ dva clean --volumes --dry-run
This will remove all containers, networks, and VOLUMES (data loss!).
Continue? [y/N] n
Aborted.
```

Answering the prompt's own documented default (`N`) returns at `:576` — before the preview
runs. The preview is reachable only by answering `y` to a prompt that says "data loss!",
which is precisely the outcome `--dry-run` was passed to avoid.

It is worse non-interactively. `fmt.Scanln` at `:572` gets EOF from a pipe or a CI runner,
leaving `answer` empty, which is not `y` — so `dva clean --volumes --dry-run` in any script
prints "Aborted." and nothing else, every time. The only way to see the preview from a
script is `--force`, and on a real run `--force` means "destroy without asking". The flag
that makes the preview reachable is the flag that makes the real thing unstoppable.

TASK-166 made the eight halt paths under this command honest about what they would do. This
is the remaining reason a user still cannot read that output.

## Acceptance criteria

- [ ] `dva clean --volumes --dry-run` prints its preview and exits 0 without prompting.
      Verify: `cd $(mktemp -d) && cp <fixture> dva.yml && dva clean --volumes --dry-run </dev/null`
      exits 0 and its output contains no `Continue?` and no `Aborted.`
- [ ] `dva clean --volumes` (no `--dry-run`, no `--force`) still prompts and still aborts on
      `n` and on EOF. The prompt is the safety property; only the preview path is exempt.
      Verify: `human — run both arms against a stand-in and paste the output`
- [ ] A test drives `cleanCmd.RunE` with `dryRun` set and asserts the preview appears with no
      prompt, alongside a case with `dryRun` false that does prompt. `TestCleanDryRunKeepsProvisionMarkers`
      (`internal/cli/dry_run_halt_test.go`) currently sets `--force` to get past this prompt;
      it should stop needing to, and that is the regression signal.
      Verify: `go test ./internal/cli/ -run Clean -count=1`
- [ ] `make test` exits 0.
      Verify: `make test`

## Notes

The narrowest fix is `if !force && !dryRun && (volumes || images)`. Worth a moment's thought
on whether `--dry-run` should imply `--force` more broadly or stay a separate exemption at
each prompt — `dva down --volumes` has no prompt today, so there is currently only one site,
and a shared helper would be premature.

Check whether `Aborted.` should keep going to stdout (`fmt.Println` at `:575`) while the
prompt goes to stderr (`:570`). Splitting a single interaction across both streams means a
`2>/dev/null` run shows "Aborted." with nothing explaining what was aborted. Out of scope
unless it is one line.
