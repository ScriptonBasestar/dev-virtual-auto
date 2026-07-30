---
id: TASK-079
title: "`--json` covers successes only, so an LLM consumer sees an empty document instead of the failure"
type: fix
priority: P3
status: todo
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

## Fix shape (not decided)

The unresolved question is scope, and it should be settled before any code moves.

1. **A failure envelope at the single choke point.** `root.go:220` sees every error. Emitting
   `{"error": {"message": …, "exit_code": …}}` on stdout when `jsonOutput` is set covers all
   commands at once, including the ones that have no `--json` support today.
2. **Per-command envelopes.** Truer to each command's shape but repeats the decision N times,
   which is how the five duplicated literals in TASK-074 happened.

(1) is the smaller change and the one that matches the flag's audience. It needs a decision on
whether errors go to stdout (parseable next to the success document) or stay on stderr as JSON.

The warning at `gitignore.go:178` is a separate question — see
[TASK-080](080-gitignore-warning-preempts-every-command.md), which may remove it from this path
entirely.

## Non-goals

- Do not change any error's text. This is about the envelope.
- Do not add `--json` to commands that do not have it. Establish the failure shape first.

## Acceptance criteria

- [ ] A failed command under `--json` writes a parseable document to stdout | verify: `dva app up myapp --json 2>/dev/null | jq -e .error.message`
- [ ] It carries the message the human path prints | verify: `human — compare stdout JSON against stderr text for the same invocation`
- [ ] Success output under `--json` is unchanged | verify: `go test ./internal/cli/ -run JSON`
- [ ] Without `--json` nothing moves to stdout | verify: `test -z "$(dva app up myapp 2>/dev/null)"` — print the captured value
- [ ] Full suite passes under -race | verify: `make test`
