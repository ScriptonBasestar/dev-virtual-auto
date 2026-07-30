---
id: TASK-079
title: "`--json` covers successes only, so an LLM consumer sees an empty document instead of the failure"
type: fix
priority: P3
status: done
effort: M
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — root.go error rendering, gitignore.go warning, and whichever commands own a --json envelope"
---

# Task 079: Make the failure reachable through the flag that exists for machines

## Problem

`internal/cli/CLAUDE.md` describes `--json` as being for the LLM pipeline
("`--json` 플래그 → `output.go`의 JSON 포맷터 사용 (LLM 파이프라인용)"). Errors do not honour it.
`root.go:220` renders every error the same way whatever the flag says:

```go
if err := rootCmd.Execute(); err != nil {
    errMsg := err.Error()
    fmt.Fprintf(os.Stderr, "\nERROR: %s\n", errMsg)
```

`gitignore.go:178` does the same for its warning.

## Evidence (verified 2026-07-30)

On a config with no `applications:` (the TASK-074 fixture), against `bin/dva` 0.1.44:

```
$ dva app up myapp --json    # stdout captured, stderr discarded
stdout=''  exit=1
$ dva app up --json          # the success side of the same command
stdout=''  exit=0
```

A consumer that pipes stdout to `jq` gets an empty document either way. The exit code is the
only signal, and it cannot say *why*.

## Why it matters

The flag's stated audience is a program, and a program cannot read `ERROR: …` on stderr without
parsing prose that no test pins. Every message this repo has recently made more actionable —
[TASK-073](../done/073-version-error-blames-the-config-for-a-build-defect.md)'s build-vs-config
blame, [TASK-074](../done/074-app-subcommands-answer-an-absent-section-three-ways.md)'s route —
is invisible to that audience.

Not P2: the human path works, and `--json` on the success path of commands that implement it
works. It is a gap in coverage, not a wrong answer.

## Shipped: one envelope, at both choke points, yielding to a document already there

Candidate (1) — the single choke point — with one correction the investigation forced. `root.go`
has **two** places a command dies, not one, and the second never reaches the first:
`mustLoadConfig` calls `os.Exit(1)` directly, so `rootCmd.Execute()` never returns and its error
block never runs. Fifteen commands reach a missing or unreadable config that way, which is the
most common failure a machine consumer will hit. Both now call `emitFailureJSON`.

Errors go to **stdout**, settling the open question in the old Fix-shape section. The task's own
acceptance criterion (`… | jq -e .error.message`, no redirection) already assumed stdout, and it
is the right assumption: stderr is where the human answer lives, and a consumer that wanted the
machine answer would have to demultiplex two formats from one stream to get it.

The condition worth naming is the second one. A command that already printed its document must
not get a second one appended:

| invocation | fixture | stdout | exit |
| --- | --- | --- | --- |
| `dva app up myapp --json` | config with no `applications:` | **1 doc**, `.error.message` set | 1 |
| `dva app up myapp` | same | **0 bytes** | 1 |
| `dva ls --json` | no `dva.yml` anywhere (the `mustLoadConfig` path) | **1 doc**, `.error.message` set | 1 |
| `dva ls` | same | **0 bytes** | 1 |
| `dva doctor --json` | a `checks:` entry that must fail | **1 doc**, keys `checks`, no `error` | 1 |

Rows 1 and 3 were `0 bytes` before this change; row 5 is the one that decided the design.
`dva doctor --json` exits 1 while having already written a complete 488-byte `{"checks": …}`
object whose failing check *is* the failure. An envelope appended there would make stdout two
concatenated documents — `jq` reports `invalid character '{' after top-level value` — so
`internal/output` records that a printer has written, and the envelope yields when it has.

Candidate (2), per-command envelopes, stays rejected for the reason the task gave: it repeats one
decision N times, which is how TASK-074's five duplicated literals happened.

Stderr is byte-identical to before — the suggestion list and the `dva init` hint are untouched and
still human-only, per the non-goal.

## Non-goals

- Do not change any error's text. This is about the envelope.
- Do not add `--json` to commands that do not have it. Establish the failure shape first.

## Acceptance criteria

- [x] A failed command under `--json` writes a parseable document to stdout | verify: `dva app up myapp --json 2>/dev/null | jq -e .error.message` — exits 0; was 4 (no output) before
- [x] The `mustLoadConfig` path is covered too, since it exits before `Execute` returns | verify: `cd <dir with no dva.yml> && dva ls --json 2>/dev/null | jq -e .error.message` — exits 0, one document, `exit_code: 1`
- [x] It carries the message the human path prints | verify: `human — the stdout .error.message and the stderr ERROR: line were compared byte-for-byte on the same invocation and are identical, hint line included`
- [x] Success output under `--json` is unchanged | verify: `go test ./internal/cli/ -run JSON` — passes; 17 pre-existing JSON tests selected besides the new one, so the regex is not matching an empty set
- [x] Without `--json` nothing moves to stdout | verify: `test -z "$(dva app up myapp 2>/dev/null)"` — captured value was `''`, 0 bytes; `dva ls` without the flag likewise yields `jq` exit 4
- [x] A command that already printed its document does not get a second one | verify: `dva doctor --json 2>/dev/null | jq -s 'length'` on a failing-check fixture — prints 1, `has("error")` is false, exit still 1
- [x] Not vacuous | verify: `human — three mutations, each caught: drop the already-printed guard → doctor's stdout fails to parse; drop the --json guard → the plain path emits; stop recording the write in PrintJSON → the concatenation subtest fails`
- [x] Full suite passes under -race | verify: `make test`

## Left open

- **`dva validate --json` never reads `jsonOutput`.** It prints plain text on stdout and returns
  no error, so the envelope never applies and a consumer gets prose. The flag is accepted and
  silently ignored — a wrong answer to a machine, not a gap, and larger than this task's scope.
- **The suggestion list and the `dva init` hint stay human-only.** `did_you_mean` and the init
  hint are printed after the envelope, to stderr. Keeping them out of the document follows the
  non-goal ("Do not change any error's text"), and the actionable content lives in `message`
  itself, which travels. If a consumer ever needs the alternatives as data, that is a schema
  question, not a rendering one.
- **Commands with no `--json` support still print human output on stdout when they succeed.**
  Only the failure path is now uniform. Establishing the success shape was explicitly a non-goal.
- **`internal/output` has no test file** (0.0% coverage). `StdoutHasDocument` and
  `ResetStdoutDocument` are exercised from `internal/cli`, which is where the behaviour that
  needs pinning lives, but the package's own printers remain untested.
- **`gitignore.go:178` left this task's scope.** [TASK-080](080-gitignore-warning-preempts-every-command.md)
  shipped the `if jsonOutput { return }` gate, so the warning no longer contaminates a `--json`
  stream and nothing is owed here.

## Related

- [TASK-080](080-gitignore-warning-preempts-every-command.md) — the warning that shared this path.
- [TASK-073](../done/073-version-error-blames-the-config-for-a-build-defect.md),
  [TASK-074](../done/074-app-subcommands-answer-an-absent-section-three-ways.md) — the messages
  this envelope now carries to a machine.
