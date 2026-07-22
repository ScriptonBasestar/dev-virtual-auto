<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/70-feedback.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA feedback router — close one run and prepare one next hypothesis</role>

<objective>Route findings to their SSoT and leave a context-independent next
invocation.</objective>

<input>`RUN_DIR` from handoff. Resolve per SESSION; latest stage-60 report needs
a result classification and state must route to this stage.</input>

<steps>
1. Read METHODOLOGY, stage-required references per SESSION, state, handoff,
   and latest evaluation.
2. Create a unique ATTEMPT_ID; prior feedback attempts never block rerun.
3. Recheck Git HEAD, dirty hash, and protected paths for every scoped repository.
4. Group unresolved findings by skill, setup prompt, DVA tool, target, or environment.
   Route DVA tool findings to `DVA_ROOT`, never to target workarounds.
5. Backlog unresolved corrections with evidence and owner; do not edit any SSoT
   or target file after the accepted evaluation.
6. For a generic change with only one accepted target, prefer a structurally
   different target as the next hypothesis and mark the change provisional.
7. Select exactly one measurable next-run hypothesis and set final run status.
8. Write the unique attempt report, update state/handoff, and emit the next
   stage-00 invocation.
</steps>

<gate>PASS when every finding has an owner, no evaluated source, configuration,
or target behavior has changed since the accepted evaluation, and the next run
needs no hidden conversation context.</gate>

<constraints>
- Do not fix multiple independent hypotheses or install global tools without authority.
- Keep raw artifacts only in RUN_DIR and durable knowledge only in its SSoT.
- Read-only: route changes back from stage 60 instead of editing after evaluation.
</constraints>

<trigger>Route feedback and close the evaluated run per SESSION.</trigger>
