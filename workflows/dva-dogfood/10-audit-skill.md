<!-- v:2026-07-16 -->

<constants>
SELF = workflows/dva-dogfood/10-audit-skill.md
ROOT = workflows/dva-dogfood
METHODOLOGY = ./METHODOLOGY.md
CONTEXT = ROOT/ref-context.md
ARTIFACTS = ROOT/ref-artifacts.md
EVALUATION = ROOT/ref-evaluation.md
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
1. Read METHODOLOGY, stage-required references per SESSION, state, handoff,
   and `skills/README.md` plus `skills/_targets.yaml`.
2. Create a unique ATTEMPT_ID; prior stage-10 attempts never block rerun.
3. Select `skills/config` for configuration authoring/diagnosis or `skills/dva`
   for build/test/lifecycle execution; inspect both when the hypothesis crosses
   that boundary.
4. Read selected skills completely and only task-required references.
5. Resolve the active host's projection from `skills/_targets.yaml`; compare
   normalized frontmatter, body or pointer, resources, and expected symlink/generated form.
6. Separate catalog visibility, body loading, explicit use, and natural triggering.
7. Check whether the skill detects ownership duplication: built-in versus custom doctor checks,
   stack versus legacy application runners, provision versus mode/plan lifecycle, and repeated raw
   Compose commands. It must distinguish standalone `docker run` runners from Compose-owned services
   and prefer named plans for new/rewrite configuration.
8. Verify that it treats lifecycle `--help` and `--dry-run` as potentially
   executable until the installed DVA version is proven safe in a disposable
   fixture, including process-backed down/stop/restart state.
9. Select canonical SKILL_SOURCE and active-host SKILL_INSTALLED/projection, or
   record that the expected projection is missing.
10. Write the unique attempt report; update state latest pointer and handoff.
</steps>

<gate>PASS when source/install and trigger evidence exist. Missing skill is a
valid finding.</gate>

<constraints>
- Read-only: do not edit, install, or sync skills.
- Do not edit a projection directly or infer fresh-session natural triggering from the current session.
</constraints>

<trigger>Audit skill loading, set prompt 20, then continue or hand off per
SESSION.</trigger>
