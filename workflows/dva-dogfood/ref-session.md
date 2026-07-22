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

A session boundary occurs in step mode, on BLOCKED/FAIL, when a stage
requires user authority, after stage 30 when `fresh_session_required` is
true, and when the run ends.

When a skill's installed metadata or body changes, stop after stage 30 and
require a fresh session before stage 40. Stage 40 clears
`fresh_session_required` only after recording a successful natural-trigger
result; failure keeps the flag and blocks the run.

## Reference Reuse

Do not load unrelated reference sections or older attempt reports
preemptively.

## Historical Reference

Initially inspect at most the three newest sibling runs. Load older runs
only for a named finding or regression comparison.
