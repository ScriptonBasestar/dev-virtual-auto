---
id: TASK-176
title: "`--explain` prints a blank Command for a script-only interaction and never mentions the script"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T22:40:00+09:00
source: "TASK-175 — found while checking what the plan says about a script that now runs in the pod"
scope: "dva repo — internal/runner/runner.go Explain"
---

# Task 176: The plan for a script interaction describes nothing that will run

## Problem

[TASK-146](../done/146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md) fixed the
blank `Command:` line for a **steps-only** interaction. The switch it added
(`internal/runner/runner.go:149`) has three arms — a command, steps, and a `default:` that still
prints a bare `Command:`. `script:` and `script_file:` land in the default.

Measured on `b59ab6d`, an interaction declaring `pod:` and `script:` and nothing else:

```
$ dva run podscript --explain
=== Command Execution Plan ===
Command:
Runner: Kubectl
Pod: web
Shell Mode: true
```

Two separate omissions:

- `Command:` is blank, which is exactly the reading TASK-146 called out — "a blank `Command:`
  line invites the reading that nothing will run". Something will.
- The script is never mentioned. `grep -n 'Script' internal/runner/runner.go` returns only import
  lines: the plan has no equivalent of `explainSteps`, so there is no line anywhere in the output
  that names the work. The one tool for checking what is about to happen shows a plan with no
  content in it.

This is runner-independent — the same output appears for a local `script:` interaction. It became
worth filing while measuring [TASK-175](175-kubectl-runner-drops-script-and-script-file-and-runs-the-inherited-command.md),
which made such a script actually run in the pod: the plan now understates a real execution rather
than an ignored one.

## Acceptance criteria

- [ ] `Command:` says what a script-only interaction is, the way the steps arm does. Match
      TASK-146's wording style rather than inventing a second vocabulary for the same idea.
- [ ] The plan names the script. Decide how much of it to show — a `script_file:` path is one
      line, an inline block is not — and say why that cut, since `explainSteps` prints every step
      and an inconsistent depth needs a reason.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] `script_file:` and `script:` are both covered, and `script_file:` shows the path as declared
      rather than the resolved absolute one, or says which and why.
- [ ] The `--json` branch agrees with the text branch. TASK-146 changed only the text branch and
      [TASK-174](174-explain-names-the-parents-command-for-a-child-that-runs-a-script.md) is the
      standing bill for that split; do not widen it.
- [ ] `make test` exits 0.

## Related

- [TASK-174](174-explain-names-the-parents-command-for-a-child-that-runs-a-script.md) — the same
  plan, but the `Command:` line carrying a *wrong* value rather than an empty one. If 174 stops a
  script child inheriting the parent's command, that child starts landing in this default arm too,
  so 174 makes this defect more visible rather than less.
