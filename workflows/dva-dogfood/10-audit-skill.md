<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/10-audit-skill.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
SAFETY = ROOT/ref-safety.md
SESSION = ROOT/ref-session.md
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA skill projection auditor — verify canonical source, platform projection, and
triggers</role>

<objective>
Prove whether the canonical DVA skill is projected to the active AI host and current before proposing changes.
</objective>

<input>`RUN_DIR` from handoff. If absent, resolve it per SESSION. Stage 00 must
PASS.</input>

<steps>
1. Load METHODOLOGY and this stage's references per SESSION reference reuse —
   once per session, reused while unchanged. Read state, handoff,
   and `skills/README.md` plus `skills/_targets.yaml`.
2. Create a unique ATTEMPT_ID; prior stage-10 attempts never block rerun.
3. Select `skills/config` for configuration authoring/diagnosis or `skills/dva`
   for build/test/lifecycle execution; inspect both when the hypothesis crosses
   that boundary.
4. Read selected skills completely and only task-required references.
5. Resolve the active host's projection from `skills/_targets.yaml` and prove the
   relation its declared shape supports, per ARTIFACTS Evidence rules: identity
   for a copy/symlink target, currency for a conversion target.
6. Record catalog visibility, body loading, and explicit use. Natural triggering
   is fresh-session evidence by definition; record it as deferred to the stage-40
   gate rather than claiming or inferring it here.
7. Check whether the skill detects ownership duplication: built-in versus custom doctor checks,
   stack versus legacy application runners, provision versus mode/plan lifecycle, and repeated raw
   Compose commands. It must distinguish standalone `docker run` runners from Compose-owned services
   and prefer named plans for new/rewrite configuration.
8. Verify that it treats lifecycle `--help` and `--dry-run` as potentially
   executable until the installed DVA version is proven safe in a disposable
   fixture, including process-backed down/stop/restart state.
9. Select canonical SKILL_SOURCE and active-host SKILL_INSTALLED/projection, or
   record that the expected `config` projection is missing as an `environment`
   finding. Do not install, sync, or synthesize it.
10. Write the unique attempt report; update state latest pointer and handoff.
</steps>

<gate>PASS when source/install evidence is complete and catalog visibility is
proven. Deferred natural triggering does not fail this stage. Missing skill is a
valid finding.</gate>

<constraints>
- Read-only: do not edit, install, or sync skills. Do not run `make generate`; a
  stale projection is a finding, and regenerating it destroys that evidence.
- Do not edit a projection directly or infer natural triggering from this session.
</constraints>

<trigger>Audit skill loading, set prompt 20, then continue or hand off per
SESSION.</trigger>
