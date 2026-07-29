<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/00-start-cycle.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA dogfood initializer — bind target, hypothesis, unique run, and
handoff</role>

<objective>
Start or explicitly resume one reproducible run without changing skill, prompt,
DVA config, or target behavior. Existing runs are optional evidence and never blockers.
</objective>

<input>
- `TARGET_PROJECT`: user path; otherwise current working directory.
- `HYPOTHESIS`: one observable claim; derive one from the user goal if absent.
- Optional `RESUME_RUN_DIR`: validated existing run to resume.
</input>

<steps>
1. Read METHODOLOGY, CONTEXT, ARTIFACTS, SAFETY, SESSION, and target guidance
   completely.
2. Resolve paths; inspect scoped git status and protected/archive/generated paths.
3. If RESUME_RUN_DIR is valid for TARGET_PROJECT, load it, run SESSION recovery,
   preserve all state/history, and emit its recorded next prompt. Do not execute
   new-run initialization steps.
4. Otherwise prove the preferred root is Git-ignored or choose the durable
   user-state fallback; generate a collision-safe RUN_ID.
5. Optionally inspect at most three recent sibling summaries; do not reuse their validation.
6. For a new run only, create RUN_DIR, `state.yaml`, and `handoff.md` using ARTIFACTS.
7. Record target and DVA source revisions/dirty hashes plus canonical
   skill/projection hashes; one measurable hypothesis, stage 00 PASS, and next
   prompt 10. Initialize owner and candidate DVA fields per ARTIFACTS.
</steps>

<gate>
PASS when target, ignored unique RUN_DIR, hypothesis, protected paths, and repository statuses are
recorded. Existing directories must not block a new run. No source/config behavior may change.
</gate>

<output>In MODE=step, stop after acceptance and emit literal `RUN_DIR=` and
`NEXT_PROMPT=` lines according to SESSION.</output>

<constraints>
- Never write artifacts into a Git-trackable path.
- Never reuse a RUN_ID for a new run or copy secrets into evidence.
</constraints>

<trigger>Initialize or resume the run, then continue or hand off per
SESSION.</trigger>
