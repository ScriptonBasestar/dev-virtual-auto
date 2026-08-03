---
id: TASK-151
title: "The manifest documents every per-command flag except the three global ones, including --json"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T13:55:00+09:00
source: "TASK-105 finalize verification — 105's own Residual, untracked"
depends-on: [TASK-105]
scope: "dva repo — internal/cli/manifest.go"
---

# Task 151: Publish the global flags in the manifest

## Problem

`dva manifest` exists so an agent can discover what DVA can do without reading help text.
It documents each command's own options — and deliberately skips the root persistent flags.
`fillStaticCommandOptions` (`internal/cli/manifest.go:151`) filters out `--debug`,
`--dry-run` and `--json` per command, and nothing else in the manifest carries them.

So the manifest's stated audience cannot discover `--json` — the one flag that turns DVA's
output into something that audience can parse. An agent reading the manifest learns the
shape of every command and not the flag that makes them machine-readable.

Skipping them per command is right: repeating three flags on every command is noise. The
missing half is a place to say them once. TASK-105 recorded this under "Residual,
deliberately not closed" and named the fix — a top-level `global_flags` field.

`grep -rn global_flags --include='*.go' --include='*.md' .` returns **1** hit, which is that
task file. Nothing in `tasks/todo/`, `tasks/blocked/`, `tasks/decision/` or `tasks/plan/`
(including TASK-134..150) covers it. TASK-137 is a different manifest gap — the unroutable
namespaced form.

## Acceptance criteria

- [ ] The manifest carries the root persistent flags once, at the top level, with the same
      name/type/description shape used for per-command options.
- [ ] Per-command option lists still exclude them — this adds a section, it does not undo
      TASK-105's filter.
- [ ] `dva manifest | jq '.global_flags'` (or whatever the key is named) lists all three.
      Print the output, and print the per-command count too so the filter is shown intact.
- [ ] The list is derived from cobra's registered persistent flags, not hand-written. A
      fourth persistent flag added later must appear without anyone editing the manifest.
- [ ] A test fails if a persistent flag exists that the manifest does not publish.

## Notes

The second half of TASK-105's residual is a stated limitation rather than a defect, but it
belongs in the same fix: `TestHandParsedOptionsAreDocumented`'s `want` map is
hand-maintained (`manifest_static_commands_test.go:195-202`), so a flag newly added to
`parseDvaFlags` or `parsePlanFlags` goes undocumented with no test failure. Deriving from
cobra rather than from a literal closes both at once — that is the reason to prefer
derivation over a hand-written `global_flags` block.
