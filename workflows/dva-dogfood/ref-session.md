# DVA Dogfood Session and Rerun Reference

Domain deltas only; invariants live in
[METHODOLOGY.md](./METHODOLOGY.md).

## New Run

- `RUN_DIR` is `<TARGET_PROJECT>/tmp/dogfood-dva/<RUN_ID>`.
- Fallback:
  `${XDG_STATE_HOME:-$HOME/.local/state}/dogfood-dva/<project-slug>-<path-hash>/<RUN_ID>`.

## Resume Resolution

- `RESUME_RUN_DIR` is an alias accepted by the orchestrator.
- Existing unrelated or completed runs never block execution.
- If multiple active runs are equally plausible, list them and start a new
  run unless the user explicitly selects one.

## Separate-Session Protocol

A session boundary occurs after every accepted `PASS` or accepted `SKIPPED`
stage in `MODE=step`, on BLOCKED/FAIL, when a stage requires user authority,
after stage 30 when `fresh_session_required` is true, and when the run ends.
Every boundary regenerates `handoff.md` and emits literal `RUN_DIR=` and
`NEXT_PROMPT=` lines. A completion boundary names the distinct post-cycle QA
prompt (or `none`); it never starts runtime work from a numbered stage.

When a skill's installed metadata or body changes, stop after stage 30 and
require a fresh session before stage 40. Stage 40 clears
`fresh_session_required` only after recording a successful natural-trigger
result; failure keeps the flag and blocks the run.

## Reference Reuse

Load each required reference completely once per session and reuse it while its
path and Git revision are unchanged. A stage that already has METHODOLOGY or a
`ref-*` loaded does not re-read it. Do not load unrelated reference sections or
older attempt reports preemptively.

`ref-context.md`, `ref-artifacts.md`, and this file are loaded whole. They are
small, stage 20 consumes all of each, and their definitions are cross-cutting:
scoping them by section saves nothing over a run and routes a stage past a name
it uses.

`ref-evaluation.md` and `ref-safety.md` are stage-specific. Load the sections the
stage consumes, plus the file's preamble above the first `##`:

<!-- markdownlint-disable MD013 -->

| Stage | ref-evaluation                                                       | ref-safety                                      |
| ----- | -------------------------------------------------------------------- | ----------------------------------------------- |
| 00    | —                                                                    | Protected operations                            |
| 10    | Finding ownership                                                    | Skill validation, Disposable prompt experiments |
| 20    | Evaluation surfaces, Deriving the run's cases, Freezing the contract | Validation ladder, Runtime authority boundary   |
| 30    | Finding ownership                                                    | Skill validation, Worktree safety               |
| 40    | Finding ownership                                                    | Disposable prompt experiments, Worktree safety  |
| 45    | Finding ownership                                                    | Validation ladder, Worktree safety              |
| 50    | Freezing the contract, Forward test                                  | all                                             |
| 60    | Finding ownership, Regression severity                               | Runtime authority boundary                      |
| 70    | Finding ownership, Regression severity                               | Protected operations                            |

<!-- markdownlint-enable MD013 -->

Each cell names an exact `##` heading; `—` means the stage does not load that
file at all. Loading a section that turns out to be insufficient is not a
violation — loading less than the stage's own steps require is.

Stage-60 scoring, the cycle gate, and cross-run promotion live in
`60-evaluate.md`; prompt-validation rules live in `40-improve-prompts.md`.
Neither is a `ref-*` load for any other stage.

## Historical Reference

Initially inspect at most the three newest sibling runs. Load older runs
only for a named finding or regression comparison.
