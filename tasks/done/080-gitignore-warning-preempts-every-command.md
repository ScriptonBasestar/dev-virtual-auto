---
id: TASK-080
title: "The .gitignore warning prints above every command's answer, on every config load, forever"
type: fix
priority: P3
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/cli — root.go loadConfig call site + gitignore.go warning conditions"
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
opposite of what [TASK-074](../done/074-app-subcommands-answer-an-absent-section-three-ways.md)
just spent effort on: the answer that routes the user somewhere is now the thing under the noise.

It also interferes with measurement. Any check that reads the first line of a dva command's
output gets the warning instead, which is why the TASK-074 evidence had to be gathered outside a
git repository to be legible.

## Fix shape (not decided)

Four candidates, not mutually exclusive:

1. **Move it out of `loadConfig`.** The warning is about repository hygiene, not about loading a
   config. `dva doctor` already owns hygiene checks and already offers `--fix`.
2. **Print it after the command's own output** rather than before.
3. **Once per process**, which it already effectively is, and once per *day* or per *config* via
   a marker under `.sb/dva/` — which is itself the thing being warned about.
4. **Suppress under `--json`**, unconditionally. A warning on stderr that no schema describes is
   noise to the flag's stated audience — see
   [TASK-079](079-json-flag-does-not-cover-failures.md).

(1) is the largest behaviour change and the most defensible: it puts the check where the fix
lives. (4) should happen regardless of which of the others is chosen.

## Non-goals

- Do not delete the warning. `.sb/dva/` genuinely should be ignored.
- Do not change `dva doctor --fix`, which is the remedy the message names and works today.

## Acceptance criteria

- [ ] The answer a command was asked for is the first thing printed | verify: `human — dva app up in a repo with no .gitignore entry; line 1 is the command's own output`
- [ ] The warning still reaches a user who has not ignored the directory | verify: `human — whichever surface it moves to still says it`
- [ ] Nothing is emitted on the warning path under `--json` | verify: `test -z "$(dva ls --json 2>&1 >/dev/null)"` — print the captured value
- [ ] The conditions that decide it are covered | verify: `go test ./internal/cli/ -run Gitignore`
- [ ] Full suite passes under -race | verify: `make test`
