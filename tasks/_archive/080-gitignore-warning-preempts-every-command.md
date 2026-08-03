---
id: TASK-080
title: "The .gitignore warning prints above every command's answer, on every config load, forever"
type: fix
priority: P3
status: done
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — gitignore.go warning conditions"
verified-at: 2026-08-03T12:30:00+09:00
archived-at: 2026-08-03T12:30:00+09:00
follow-up: TASK-139
verification-summary: |
  Verified against bin/dva 0.1.44 on two throwaway fixtures built from noAppsConfig, states
  changed one fact at a time. Fresh clone (.git, no .gitignore, no markers): `dva app up myapp`
  prints ERROR as its first non-blank line, `dva app up` its answer at line 1, no warning, and
  `.sb/dva` is still absent afterwards. Add `.sb/dva`: warning returns above the answer and
  `dva ls/show/validate/status` each emit exactly 1 warning line (positive control). Add `.sb/`
  to .gitignore: all four go back to 0. Under `--json`, same directory and same state, stderr is
  0 bytes while stdout carries 113 bytes of JSON — measured by redirecting each stream to a file,
  since zsh's `2>&1 >/dev/null` in a pipeline is unreliable here. `dva doctor` keeps the
  unconditional check and still reports the finding with its --fix remedy when no markers exist.
  `go test ./internal/cli/ -run Gitignore` passes with 6 named subtests actually running; the
  table gives exactly one subtest per gate, so neither gate can be dropped silently.
---

# Task 080: Say it once, and not in front of the answer

## Problem

`root.go:281` calls `checkGitignoreForWarning` from inside `loadConfig()`, so it runs for every
command that loads a config. It has no once-per-run memo, no suppression under `--json`, and no
way to be dismissed short of editing `.gitignore`.

Because it prints first, it sits above the output the user asked for.

## Evidence (verified 2026-07-30)

The TASK-074 fixture with a `.git` directory present and no `.gitignore` — the only two
conditions the warning checks (`gitignore.go:162`, `:170-176`). Output numbered by line:

```
$ dva app up myapp
1:⚠️  [warn] .sb/dva/ is not in your .gitignore. Transient markers might be committed.
2:         Run 'dva doctor --fix' to auto-fix or add '.sb/dva/' to .gitignore manually.
5:ERROR: no applications declared, so there is no 'myapp' to start; …

$ dva app up
1:⚠️  [warn] .sb/dva/ is not in your .gitignore. Transient markers might be committed.
2:         Run 'dva doctor --fix' to auto-fix or add '.sb/dva/' to .gitignore manually.
4:no applications declared; this config declares stack (1), interaction (1) — run 'dva up' …
```

Two lines of unrelated advice, then a blank line, then the answer. On both the failure and the
success path.

## Why it matters

The warning is correct and worth saying — `.sb/dva/` holds transient markers that should not be
committed. But a warning that repeats on every invocation is read once and skipped forever, and
while it is being skipped it trains the reader to skip the first lines of output. That is the
opposite of what [TASK-074](074-app-subcommands-answer-an-absent-section-three-ways.md)
just spent effort on: the answer that routes the user somewhere is now the thing under the noise.

It also interferes with measurement. Any check that reads the first line of a dva command's
output gets the warning instead, which is why the TASK-074 evidence had to be gathered outside a
git repository to be legible.

## Shipped: the warning waits until there is something to commit

None of the four candidates was adopted as written, because measuring the warning first produced
a narrower answer than any of them. `ls`, `show`, `validate`, `status` and `manifest` each warned
(1 warning line apiece) and **none of them created `.sb/dva`**. So on a fresh clone the warning
named a hazard that had not happened yet, and for a reader who only ever inspects the config it
never would. What creates the directory is the pid, log, module-cache and source-cache writers
under `internal/lifecycle` and `internal/config` — and the invocation *after* one of those is the
first moment the warning has something to point at.

Two gates, therefore: `jsonOutput`, and `.sb/dva` existing on disk. Measured through the binary,
one fixture, states changed one at a time so each row differs from its neighbour by one fact:

| state | `dva ls/show/validate/status` | note |
| --- | --- | --- |
| `.git`, no `.gitignore`, no markers | **0** warning lines; `.sb/dva` still absent after | the fresh-clone case |
| markers created, still unignored | **1** line, each of the four | the positive control |
| markers + `.sb/` in `.gitignore` | **0** | ancestor rule, same directory as the row above |
| markers, unignored, `--json` | stderr **0 bytes**; stdout still `{}` | |

The middle row is what makes the zeros mean anything: same grep, same commands, same directory.

`dva doctor` keeps the unconditional form (`checkGitignoreStatus`) — measured in the no-markers
state, it still reports `[FAIL] .sb/dva/ is ignored in .gitignore` with its `--fix` remedy. That
is the right split: doctor is asked "is my setup right?", and is asked *before* anything has run,
so it is the surface that should speak proactively. The `loadConfig` path answers the narrower
question — something committable is on disk right now — and only that version is worth putting
ahead of another command's answer.

Candidate (1), moving it out of `loadConfig` entirely, was rejected for what it gives up: the user
who never runs `doctor` and has just written markers. Candidate (3), a once-per-day marker, would
have to store that marker inside the directory it is warning about.

## Non-goals

- Do not delete the warning. `.sb/dva/` genuinely should be ignored.
- Do not change `dva doctor --fix`, which is the remedy the message names and works today.

## Acceptance criteria

- [x] The answer is first on the path the Evidence block reproduces | verify: `human — dva app up myapp on that fixture now prints the ERROR as its first non-blank line, and dva app up its answer at line 1; both warning lines gone`
- [x] Ordering is unchanged when the warning does fire | verify: `human — with markers on disk and unignored it still prints above the answer. The gate decides whether, not where; deliberate, since by then it describes a real file`
- [x] The warning still reaches a user who has not ignored the directory | verify: `human — dva doctor reports it even with no markers present, remedy included; the per-command path reports it from the first invocation after markers appear`
- [x] Nothing is emitted on the warning path under `--json` | verify: `test -z "$(dva ls --json 2>&1 >/dev/null)"` — captured value was `''`, 0 bytes, with stdout still `{}`
- [x] The conditions that decide it are covered | verify: `go test ./internal/cli/ -run Gitignore`
- [x] Not vacuous | verify: `human — three mutations, each caught by exactly one subtest: dropping the existence gate fails "no markers written yet", dropping the json gate fails the json case, never warning at all fails "markers on disk and nothing ignores them". One subtest each means the two gates are independent, not redundant`
- [x] Full suite passes under -race | verify: `make test`

## Left open

- `dva validate --json` does not honor `--json` — `validate.go` never reads `jsonOutput`. Same
  family as [TASK-079](079-json-flag-does-not-cover-failures.md), not this task's scope.
- `dva doctor`'s line reads `[FAIL] .sb/dva/ is ignored in .gitignore` — the label states the
  check, not the finding, so a failing row reads as if it passed. Cosmetic, noticed here.
- Plain `dva doctor` prints the stderr banner *and* its own structured row for the same finding.
  Under `--json` the banner is now gone; the duplication on the human path is pre-existing.
