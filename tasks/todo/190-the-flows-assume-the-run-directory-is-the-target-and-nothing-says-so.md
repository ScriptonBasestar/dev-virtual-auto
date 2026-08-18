---
id: TASK-190
title: "The flows assume the run directory is the target, and nothing enforces it"
type: bug
priority: P1
effort: M
created-at: 2026-08-18T15:52:00+09:00
source: "measured against am cb8b4ce while adding the 30-configure report marker (47ead71)"
scope: "dva repo — agent-mesh-flows/dva-improve.yaml, dva-improve-guided/*.yaml"
status: todo
---

# Task 190: Make the CWD-equals-target assumption explicit, or stop depending on it

## Summary

Every `dva-improve` flow takes a `target` parameter, which reads as a promise that it can
be pointed at an arbitrary project. It cannot. The flows only behave when the run directory
*is* the target, and nothing states that or checks it.

Two subsystems disagree about what a path means, and the flows straddle both:

| step kind | path resolution | outside the run root |
| --- | --- | --- |
| `type: file`, `read_file` | against the run directory, path-sandboxed | refused |
| `context`, `shell` | whatever the shell does after `cd` | allowed |

Measured against am cb8b4ce with a throwaway flow carrying the same steps as the shipped
ones, on two git fixtures:

| arrangement | `backup_marker` / `backup_config` | pipeline exit |
| --- | --- | --- |
| CWD == target | both run; `backups/dva/dva.yml.bak` written | `Done` |
| CWD != target | both fail `escaped all approved roots` | `Continue anyway?` then `Done` |

The second row is the defect, and the interesting part is the third column. The failure is
not fatal — am prompts, and a run that continues reports success. `improve` declares
`backup_config` in its `depends_on`, but a dependency that errored and was continued past
does not stop it. So the reachable outcome is: the config is rewritten, the snapshot that
was supposed to make that safe was never taken, and the run ends green. A backup whose
absence is silent is worse than no backup, because the whole point of TASK-183 is that
somebody will one day reach for it.

The same split shows up harmlessly elsewhere, which is how it was found. `30-configure`'s
`validate` writes its report after `cd '{{param.target}}'`, so the report follows the
target while the four other stages' reports follow the run directory. Its `.gitignore`
marker is a `file` step and therefore cannot follow the report out. When the two
directories differ, that project gets an untracked `tmp/` after every run — measured, `??
tmp/`. Cosmetic on its own; the same root cause as the row above.

`dva-improve-guided.yaml:39` already bakes the assumption in: it gates on
`[ -f 'tmp/improve-guided/00-analysis-report.json' ]`, a relative path, so pointing the
pipeline elsewhere makes the gate read the wrong directory. Every `read_file` `src` in the
stages is relative for the same reason.

Two honest directions, and the choice is the work:

1. **Enforce the assumption.** A preflight step that fails loudly when the run directory is
   not the target, before anything writes. Small, and it converts a silent wrong answer
   into a clear one. `target` stops pretending to be free.
2. **Drop the assumption.** Make every path target-anchored, which means the `file` steps
   need the target inside an approved root — an operator-side `allowed_roots_add`, i.e. a
   setup step outside the repo that the flow cannot guarantee. See
   `agent-mesh-flows/shared/` in the sandbox-override note; the same wall was hit there.

Direction 1 is recommended and is what the criteria below assume. Direction 2 is a larger
change that should not be started without deciding whether a flow may require operator
configuration to be correct at all.

Do not close this by removing the `target` parameter. Callers pass it, and the shell steps
genuinely use it.

## Completion Criteria

- [ ] Running any improve flow with a run directory other than the target fails before the first write, with a message naming both directories | verify: human — run the flow from a directory that is not the target and read the first line of output
- [ ] The `improve` step cannot run when its backup did not | verify: human — force the backup to fail, continue past the prompt, and confirm the config is untouched
- [ ] The guard exists in the flow, not only in a comment | verify: `grep -q 'id: check_run_dir' agent-mesh-flows/dva-improve.yaml`
- [ ] The guided pipeline carries the same guard | verify: `grep -rq 'id: check_run_dir' agent-mesh-flows/dva-improve-guided/`
- [ ] The CWD-equals-target requirement is stated where a user reads it, not only in YAML comments | verify: human — check the improve flow docs named in TASK-185
- [ ] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml`
- [ ] Corpus stays clean | verify: `go run ./tools/flowcheck`

## Technical Notes

- Reproduction, both rows of the table: build two git fixtures with a `dva.yml`, run the
  flow once with CWD set to the fixture and once from a sibling directory.
- The exact refusal is
  `security: path resolution "<target>/backups/dva/.gitignore" (resolved to "<target>") escaped all approved roots`.
- The 30-configure marker was deliberately left relative in 47ead71 rather than anchored to
  the target — anchoring it is the change the sandbox refuses. Its comment points here.
- Related: TASK-183 (restore path) depends on the snapshot actually existing, so this
  should land first or alongside it.
- `flowcheck` cannot see any of this today; it reads decision paths, not path resolution.
  Whether it should grow a rule is a question for TASK-186's rule work, not this card.
