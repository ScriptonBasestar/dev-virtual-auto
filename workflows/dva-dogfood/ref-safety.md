# DVA Dogfood Safety and Validation Reference

## Safety invariants

These hold for every numbered stage, in every mode.

- Evidence is append-only; `state.yaml` and `handoff.md` are indexes only.
- One owner per run. No source change after an accepted evaluation.
- No stage commits, pushes, or performs an irreversible target action.
- **No numbered stage ever invokes `provision`, `up`, `down`, `stop`, or `restart`
  (or an equivalent target lifecycle command) against a real target.** Runtime
  startup is a distinct post-cycle QA surface.
- Forward-test children use disposable, seeded fixtures and keep real targets
  read-only. Only stage 30, and only under a `target_project` owner, may edit a
  real target — as a reversible, baseline-justified patch with the exact pre-edit
  patch and its inverse recorded first.
- Before editing any skill, prompt, or DVA source, record scoped Git status,
  revision, protected paths, and the exact paths this run may change. Each edit
  path is clean or has a recorded pre-edit patch and inverse; unrelated dirty paths
  stay untouched.
- On failed validation, reverse only the run-owned captured patch and preserve the
  failure evidence. Never use a destructive Git command.
- Record a secret by name and pattern, never its value. Never place literal
  credentials, private file contents, or unrelated dirty-file content in evidence.

## Protected operations

Require explicit user authority before:

- touching production or staging infrastructure;
- using `dva doctor --fix`, destructive cleanup, volume removal, or reset flows;
- rewriting an existing `dva.yml` instead of preserving its structure;
- overwriting `.env`, or decrypting or editing secret stores;
- reverting unrelated user changes;
- installing or replacing global executables.

## Runtime authority boundary

A post-cycle authority record must name every command and its **concrete** side
effect for that command — the named plan, and that it starts a service. An empty
command list or an empty side-effect list is not a record. A generic runtime
approval is not authority, and valid post-cycle authority never unlocks a numbered
stage.

## Validation ladder

Run the lowest-risk checks first. Flags may change by DVA version. Treat lifecycle
`--help` as executable until the installed manual flag parser is proven to return
help safely in a disposable fixture.

1. Static files and references exist.
2. Capture `dva version`; capture relevant `--help` only after its path is proven
   non-executing.
3. `dva config validate` succeeds; classify every warning.
4. `dva config show -f yaml` and `dva manifest -f json` match the expected merged
   config and command surface.
5. `dva doctor --json` runs without fixes; classify environment/tool/project
   failures.
6. A printed or rendered execution plan is inspected without running target
   lifecycle. Treat lifecycle `--dry-run` as unsafe until each selected runner's
   up/down/stop/restart path is proven non-mutating in a disposable fixture,
   including process-backed PID and log state. An entry declares one command, so
   verify which entry a plan selects rather than assuming a mode toggle picks a
   variant — `dva app up --dev` was removed with `dva app`.
7. `docker compose ... config --quiet` succeeds for every configured combination.
   This check needs no daemon — passing it does not prove a daemon is reachable.
8. Target-specific lint and tests pass.
9. Runtime startup and health checks run only when authorized and locally safe.

**A zero exit code does not override contradictory or materially incorrect
output.** When behavior is contradictory, reproduce it first, then inspect the
relevant CLI/schema/doctor implementation in `DVA_ROOT` and record exact source
references.

## Skill validation

Generation is a mutation. It belongs to the stage that owns the skill change, never
to a read-only stage.

Read-only stages:

- Read the complete skill-creator instructions when available.
- Validate source skill structure with the repository's official validator **when
  one exists**. This repository ships none: `tools/skillgen` is a generator, and
  `make check-generate` runs it — which a read-only stage must not do. Absent a
  validator, check structure directly (frontmatter fields, `SKILL.md` present,
  declared reference paths resolve) and record the missing validator as a finding.
  Never substitute the generator for the validator.
- Verify that every projection declared by `skills/_targets.yaml` exists and is
  current, using the relation its shape supports (`ref-artifacts.md`, Evidence
  rules). Do not run `make generate` to find out — a projection that needs
  regenerating is a finding, and running it destroys the evidence that it was
  stale.

Skill-change stages:

- Run `make generate` from `DVA_ROOT` after editing a canonical skill; do not
  assume copies auto-update.
- Compare the canonical source with each generated or installed projection
  afterwards.

Both:

- **Natural triggering is fresh-session evidence by definition.** A session that has
  already read or edited the skill cannot produce it; only the fresh-session gate
  can. A read-only stage records catalog visibility and explicit use, and defers
  natural triggering rather than claiming it.
- Forward tests receive the raw task and the skill, never the expected answer or a
  prior diagnosis.

## Worktree and fixture safety

- Avoid archived, generated, vendor, dependency, and legacy directories defined by
  target guidance.
- Make minimal patches; do not perform broad formatting alongside behavioral
  changes.
- Use a cycle-owned ignored fixture, copy, or worktree for prompt and lifecycle
  experiments; never reset or rewrite an original target to recreate old state.
- Record the source path and seed revision before mutation.
- Remove only disposable files created by the current cycle, and only after their
  reproduction evidence is recorded.
