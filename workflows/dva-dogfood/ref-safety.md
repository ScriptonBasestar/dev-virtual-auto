# DVA Dogfood Safety and Validation Reference

Domain deltas only; invariants live in
[METHODOLOGY.md](./METHODOLOGY.md).

## Protected operations

Require explicit user authority before:

- touching production/staging infrastructure;
- using `dva doctor --fix`, destructive cleanup, volume removal, or reset flows;
- rewriting an existing `dva.yml` instead of preserving its structure;
- overwriting `.env`, decrypting or editing secret stores;
- reverting unrelated user changes;
- installing or replacing global executables.

## Worktree safety

- Avoid archived, generated, vendor, dependency, and legacy directories defined
  by target guidance.
- Make minimal patches; do not perform broad formatting alongside behavioral
  changes.

## Validation ladder

Run the lowest-risk checks first. Flags may change by DVA version. Treat
lifecycle `--help` as executable until the installed manual flag parser is
proven to return help safely in a disposable fixture.

1. Static files and references exist.
2. Capture `dva version`; capture relevant `--help` only after its path is
   proven non-executing.
3. `dva config validate` succeeds; classify every warning.
4. `dva config show -f yaml` and `dva manifest -f json` match the expected
   merged config and command surface.
5. `dva doctor --json` runs without fixes; classify environment/tool/project
   failures.
6. A printed/rendered execution plan is inspected without running target
   lifecycle. Treat lifecycle `--dry-run` as unsafe until each selected runner's
   up/down/stop/restart path is proven non-mutating in a disposable fixture,
   including process-backed PID and log state. Note that `dva app up` and
   `dva app up --dev` select different service commands (`run` vs `dev`); verify
   the intended mode rather than assuming `up` implies dev.
7. `docker compose ... config --quiet` succeeds for every configured
   combination.
8. Target-specific lint/tests pass.
9. Runtime startup/health checks run only when authorized and locally safe.

A zero exit code does not override contradictory or materially incorrect output.

## Skill validation

- Read the complete skill-creator instructions when available.
- Validate source skill structure with the repository's official validator.
- Use the plugin's supported Codex install/sync mechanism; do not assume copies
  auto-update.
- Compare source and installed metadata/body after installation.
- Use a fresh Codex session to test metadata-based discovery and natural
  triggering.
- Forward tests receive the raw task and skill, not the expected answer or prior
  diagnosis.

## Prompt validation

- Follow the prmpt framework's prompt conventions (external).
- Keep executable prompts in English and concise.
- Keep shared rules in `ref-*`, not duplicated in every prompt.
- Verify all relative links and referenced filenames.
- Run the devenv Markdown formatter/linter scoped as narrowly as supported.

## Disposable prompt experiments

- Use a cycle-owned ignored fixture, copy, or worktree for prompt variants;
  never reset or rewrite an original target to recreate old state.
- Record the source path and seed revision before mutation.
- Keep one target and one primary hypothesis per run. Cross-target comparison
  uses separate runs and their accepted reports.
- Remove only disposable files created by the current cycle, and only after
  their reproduction evidence is recorded.
