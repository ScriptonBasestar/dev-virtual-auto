<!-- v:2026-08-05 -->

<constants>
ROOT = workflows/dva-dogfood
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA dogfood initializer — bind target, hypothesis, unique run, and provenance</role>

<objective>
Start or explicitly resume one reproducible run without changing skill, prompt, DVA
config, or target behavior. Existing runs are optional evidence, never blockers.
</objective>

<input>
- `TARGET_PROJECT`: user path; otherwise the current working directory.
- `HYPOTHESIS`: one observable claim; derive one from the user goal if absent.
- `MODE`: `step` (default) or `continuous`.
- Optional `RESUME_RUN_DIR`: an existing run to resume.
</input>

<steps>
1. Load CONTEXT, ARTIFACTS, EVALUATION, and SAFETY whole — once per session, reused
   while unchanged. Read the target's `AGENTS.md`/`CLAUDE.md` and module guidance.
2. Resolve paths; inspect scoped Git status for both `TARGET_PROJECT` and
   `DVA_ROOT`, and record protected, archived, and generated paths.
3. If `RESUME_RUN_DIR` is valid for `TARGET_PROJECT`, load it, run the ARTIFACTS
   resume reconciliation, preserve all state and history, and emit its recorded
   next prompt. Do not run the remaining new-run steps.
4. Otherwise prove the preferred run root is Git-ignored (`git check-ignore`) or
   choose the durable user-state fallback; generate a collision-safe `RUN_ID`.
5. Optionally inspect at most three recent sibling runs; never reuse their
   validation as a current gate.
6. Create `RUN_DIR`, `state.yaml`, and `handoff.md` per ARTIFACTS.
7. Record provenance, without assuming any two values agree:
   - target and `DVA_ROOT` HEAD plus dirty hashes (derivation: ARTIFACTS `revisions`);
   - `prompt_bundle_hash` using the single command pinned on ARTIFACTS `prompt_bundle_hash`
     (tracked files under `workflows/dva-dogfood/` only — never a recursive hash of the
     directory, which picks up untracked `.ce` telemetry);
   - the installed `dva` executable path, version, and build commit;
   - the canonical skill hash for `skills/config` and `skills/dva`, and each
     projection declared by `skills/_targets.yaml`, proven by the relation its
     shape supports per ARTIFACTS Evidence rules (path-independent content digest —
     not the dirty-hash form);
   - `sources.config_projection` as `active` or `missing`.
   A stale installed binary makes every observation of its output unusable. If the
   installed commit differs from `DVA_ROOT` HEAD, record it as a finding now — do
   not silently proceed to measure output the current source no longer produces.
   If a later stage's recomputed `prompt_bundle_hash` differs, list the tracked files
   that changed (`git status` / `git diff` under that tree) and treat it as mid-run
   prompt drift to record — not as an automatic gate fail from the hash digits alone.
8. Record catalog visibility of the skills and defer natural triggering to the
   stage-30 fresh-session gate. Do not read skill bodies deeply here; stage 20 does
   that when, and only when, the owner is `skill`.
9. Record one measurable hypothesis, stage 00 PASS, and next prompt
   `10-baseline.md`.
</steps>

<gate>
PASS when the target, an ignored unique `RUN_DIR`, the hypothesis, protected paths,
repository statuses, and DVA/skill provenance are recorded. Existing directories
must not block a new run. No source, config, or target behavior may change.
A missing or stale projection is a finding, not a failure.
</gate>

<constraints>
- Read-only. Never write artifacts into a Git-trackable path.
- Never reuse a `RUN_ID`, install or sync a skill projection, run `make generate`,
  or copy secrets into evidence.
</constraints>

<output>Emit literal `RUN_DIR=` and `NEXT_PROMPT=` lines at every session boundary
per ARTIFACTS.</output>

<trigger>Initialize or resume the run, then continue or hand off.</trigger>
