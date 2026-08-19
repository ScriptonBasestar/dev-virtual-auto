---
id: TASK-190
title: "The flows assume the run directory is the target, and nothing enforces it"
type: bug
priority: P1
effort: M
created-at: 2026-08-18T15:52:00+09:00
source: "measured against am cb8b4ce while adding the 30-configure report marker (47ead71)"
scope: "dva repo — agent-mesh-flows/dva-improve.yaml, dva-improve-guided/*.yaml"
status: done
completed-at: 2026-08-18T16:31:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T14:01:00+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "AC1 reproduction — check_run_dir shell extracted verbatim from the shipped YAML, run from a directory that is not the target"
    result: exit 1, stderr names both `run directory:` and `target:`; controls `target=<abs>` and `target=.` from inside the target both exit 0 with `yes`
  - kind: automated
    command-or-step: "AC2 reproduction — same guard with target=/nonexistent"
    result: exit 1 at `cd ... || exit 1`, before any later step
  - kind: automated
    command-or-step: "AC1 structural check — position of check_run_dir in each flow"
    result: check_run_dir is the FIRST step in all six flows (dva-improve.yaml:67 with no prior step; 00-analyze:37, 10-verify:35, 20-transform:32, 30-configure:58, 40-execute:35), so it precedes every write
  - kind: automated
    command-or-step: "grep -q 'id: check_run_dir' agent-mesh-flows/dva-improve.yaml (AC3)"
    result: exit 0
  - kind: automated
    command-or-step: "grep -rq 'id: check_run_dir' agent-mesh-flows/dva-improve-guided/ (AC4)"
    result: exit 0 — 5 guided stages carry one guard each, 6 guards total
  - kind: automated
    command-or-step: "grep -q '실행 디렉토리 요구사항' USAGE.md (AC5)"
    result: exit 0 — USAGE.md:955 states the requirement with the `cd` example; docs/50 links to this anchor and doc-check resolves it
  - kind: automated
    command-or-step: "am validate on all seven improve flows (AC6, widened past the single file the binding names)"
    result: exit 0 on every one
  - kind: automated
    command-or-step: "go run ./tools/flowcheck (AC7)"
    result: exit 0 — 10 flow files, 103 shell fields, no decision-path defects
  - kind: manual
    command-or-step: "residual check — the dropped 'improve must not run when its backup did not' criterion"
    result: accepted as written. The `when:` contract allows one `{{ref}} OP 'quoted'` comparison with no boolean composition, and the fresh-project vs errored-backup cases need an OR; the guard removes the only reachable cause. Correctly attributed to am rather than left as a silent gap
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

| arrangement | mode | `backup_marker` / `backup_config` | pipeline exit |
| --- | --- | --- | --- |
| CWD == target | either | both run; `backups/dva/dva.yml.bak` written | 0 |
| CWD != target | `-y` batch | fail `escaped all approved roots`, pipeline aborts | 1 |
| CWD != target | interactive | fail, then `Continue anyway?` | blocks unattended; on EOF stdin defaults to continue and ends `Done` |

Batch mode is already fail-fast, so the danger is narrower than it first looks — but the
last row is real. A caller that forgets `-y` and feeds no tty gets the default answer, and
`improve` runs with `backup_config` in its `depends_on` having errored. The config is
rewritten, the snapshot that was supposed to make that safe was never taken, and the run
ends green. A backup whose absence is silent is worse than no backup, because the whole
point of TASK-183 is that somebody will one day reach for it.

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

- [x] Running any improve flow with a run directory other than the target fails before the first write, with a message naming both directories | verify: human — run the flow from a directory that is not the target and read the first line of output
- [x] A target that does not exist stops the flow too, rather than failing later in a confusing place | verify: human — run with `target=/nonexistent` and confirm exit 1 at the guard
- [x] The guard exists in the flow, not only in a comment | verify: `grep -q 'id: check_run_dir' agent-mesh-flows/dva-improve.yaml`
- [x] The guided pipeline carries the same guard | verify: `grep -rq 'id: check_run_dir' agent-mesh-flows/dva-improve-guided/`
- [x] The CWD-equals-target requirement is stated where a user reads it, not only in YAML comments | verify: `grep -q '실행 디렉토리 요구사항' USAGE.md`
- [x] Flows still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml`
- [x] Corpus stays clean | verify: `go run ./tools/flowcheck`

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

### Evidence

Each guard was extracted verbatim from the shipped YAML into a one-step probe flow and run
on two git fixtures, so the text under test is the text that ships:

| flow | CWD == target | CWD != target | names both dirs |
| --- | --- | --- | --- |
| dva-improve | exit 0 | exit 1 | yes |
| 00-analyze | exit 0 | exit 1 | yes |
| 10-verify | exit 0 | exit 1 | yes |
| 20-transform | exit 0 | exit 1 | yes |
| 30-configure | exit 0 | exit 1 | yes |
| 40-execute | exit 0 | exit 1 | yes |

`target=.` and `target=./` both exit 0 from inside the target — the default path is not
broken by the guard. `target=/nonexistent` exits 1 at the guard rather than later.
`flowcheck` shell-field count moved 95 → 101, one per guard, so the corpus scan is seeing
them.

### Residual, deliberately not closed here

"`improve` must not run when its backup did not" was a criterion on the first draft of this
card and was dropped, not quietly but for a reason worth writing down: the `when:` contract
allows one `{{ref}} OP 'quoted'` comparison and no boolean composition. `improve` must run
on a fresh project, where the backup is skipped by design, and must not run on an existing
project whose backup errored. Those two need an OR, so no single gate expresses it.

The guard removes the only reachable cause of that state, which is why this card closes.
A general "a failed step must stop its dependents even when continued past" belongs to am,
not to a flow, and would be the honest place to fix the remaining interactive-EOF row.
