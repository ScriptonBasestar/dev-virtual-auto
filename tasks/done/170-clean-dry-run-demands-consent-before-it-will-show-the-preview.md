---
id: TASK-170
title: "clean --dry-run demands consent for the deletion, then aborts without showing the preview"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-08-03T16:55:00+09:00
completed-at: 2026-08-03T17:25:00+09:00
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

- [x] `dva clean --volumes --dry-run` prints its preview and exits 0 without prompting.
      Verify: `cd $(mktemp -d) && cp <fixture> dva.yml && dva clean --volumes --dry-run </dev/null`
      exits 0 and its output contains no `Continue?` and no `Aborted.`
- [x] `dva clean --volumes` (no `--dry-run`, no `--force`) still prompts and still aborts on
      `n` and on EOF. The prompt is the safety property; only the preview path is exempt.
      Verify: `human — run both arms against a stand-in and paste the output`
- [x] A test drives `cleanCmd.RunE` with `dryRun` set and asserts the preview appears with no
      prompt, alongside a case with `dryRun` false that does prompt. `TestCleanDryRunKeepsProvisionMarkers`
      (`internal/cli/dry_run_halt_test.go`) currently sets `--force` to get past this prompt;
      it should stop needing to, and that is the regression signal.
      Verify: `go test ./internal/cli/ -run Clean -count=1`
- [x] `make test` exits 0.
      Verify: `make test`

## Result

`if !force && !dryRun && (volumes || images)` — the narrowest fix, as the notes proposed.
Exempted at the site rather than behind a shared helper: this is still the only prompt in
the codebase (`dva down --volumes` has none), so a helper would abstract over one caller.

`--dry-run` was not made to imply `--force` more broadly. They mean different things —
"don't do it" versus "do it without asking" — and conflating them would mean a future
prompt on a genuinely destructive path inherits an exemption nobody wrote for it.

### Both arms, measured

Against a compose fixture, `bin/dva` built from this change:

```
$ dva clean --volumes --dry-run </dev/null
[lifecycle] stopping core (compose)
level=INFO msg=dry-run entry=core plugin=compose command=docker \
  args="[compose -f .../compose.yml down --remove-orphans --volumes]"
rc=0                      # no Continue?, no Aborted.

$ dva clean --volumes </dev/null          # no --dry-run, no --force
This will remove all containers, networks, and VOLUMES (data loss!).
Continue? [y/N] Aborted.
rc=0                      # prompt intact, EOF still declines

$ printf 'n\n' | dva clean --volumes 2>/dev/null
                          # stdout entirely empty
```

The preview names the args it would hand to docker, `--volumes` among them, which is the
thing a person runs this command to read.

### The stream split, fixed in one line

The notes marked this out of scope unless it was one line. It was: `fmt.Println("Aborted.")`
→ `fmt.Fprintln(os.Stderr, "Aborted.")`. The prompt was already on stderr, so the two halves
of one interaction were on different streams — `2>/dev/null` showed a verdict with nothing
saying what had been aborted, and a script reading stdout for the preview got the verdict
mixed into its data. The third arm above is the check: stdout is now empty on abort.

## Tests

`internal/cli/clean_prompt_test.go` — `TestCleanDryRunSkipsTheDestructionPrompt` and
`TestCleanWithoutDryRunStillPrompts`, the second being the safety control. Both drive
`cleanCmd.RunE` and point `os.Stdin` at `/dev/null`, because `fmt.Scanln` resolves `os.Stdin`
at call time: without that the tests would depend on how `go test` was invoked.

`TestCleanDryRunKeepsProvisionMarkers` no longer sets `--force`, per criterion 3.

Falsified twice, once per half of the fix.

Reverting `!dryRun` from the gate:

```
--- FAIL: TestCleanDryRunSkipsTheDestructionPrompt
    clean_prompt_test.go:114: ... still emits "Continue?"
    clean_prompt_test.go:114: ... still emits "Aborted."
    clean_prompt_test.go:122: the preview never ran
--- FAIL: TestCleanDryRunKeepsProvisionMarkers
    dry_run_halt_test.go:220: clean --dry-run did not name the marker it would delete
```

Reverting `Aborted.` to stdout:

```
--- FAIL: TestCleanWithoutDryRunStillPrompts
    clean_prompt_test.go:150: ... stderr is missing "Aborted."
    clean_prompt_test.go:158: Aborted. is on stdout, split from the prompt it answers
```

Each falsification fires only its own half's assertions, so neither is carrying the other.

The marker `os.Stat` assertion deliberately stayed green under the first falsification, and
that is the point of the criterion. The old behaviour aborted, so the marker survived by
accident — a test that checked only "the marker still exists" would have been green
throughout the entire defect. That is why the old test needed `--force` to reach a state
worth asserting on at all.

## Notes

Found while measuring, out of scope here: **the abort exits 0.** `dva clean --volumes` piped
anything that is not `y` prints "Aborted." and returns `nil`, so a script sees success from a
command that did nothing. That is the same shape of silence this task fixed on the preview
side, on the arm that still declines. Filed as TASK-171.
