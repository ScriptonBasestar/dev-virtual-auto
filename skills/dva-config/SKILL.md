---
name: dva-config
description: >-
  Use when creating, auditing, repairing, or migrating a dva.yml configuration; diagnosing `dva
  config validate`, `dva show`, or `dva doctor` warnings; separating DVA CLI defects from project
  configuration and environment issues; or applying DVA across a devbox root and active subprojects.
allowed-tools: [Read, Edit, Bash, Grep, Glob]
user-invocable: false
---

# DVA Config Authoring & Diagnosis

Configure DVA from current local evidence. Preserve working project structure, classify each finding
by owner, and validate from static schema to authorized runtime checks. For new or rewritten
configuration, prefer `stack` declarations selected by named `plans`; treat `modes` and
`applications` as legacy migration inputs.

Read **`references/diagnosis.md`** when symptoms cross configuration, CLI, project, and environment
boundaries; when comparing installed/source/candidate DVA builds; when deciding root/subproject DVA
need; or when validating process ownership and lifecycle migration behavior.

Read **`references/schema-reference.md`** when authoring or reviewing the `dva.yml` field
structure, section shapes, critical schema rules, or the canonical section/field ordering.

For CLI execution (build/test/run/lifecycle), use the sibling `dva` skill; this skill owns
configuration authoring, migration, and defect attribution.

## When to Use

- Add DVA to an existing project or root-multi devbox.
- Repair stale Compose paths, commands, endpoints, checks, or subprojects.
- Migrate deprecated DVA sections reported by the installed CLI.
- Diagnose contradictory or surprising `validate`, `show`, or `doctor` output.
- Review root and subproject `dva.yml` responsibility boundaries.

## When Not to Use

- Compose file structure only, with no DVA configuration involved.
- General devbox directory layout only.
- Port registry allocation only.

## Ownership Model

| Owner       | Signals                                                          | Action                                      |
| ----------- | ---------------------------------------------------------------- | ------------------------------------------- |
| DVA config  | stale file/service/command; warning tied to project data         | patch `dva.yml` minimally                   |
| DVA tool    | contradictory checks; incorrect discovery; parser/runtime defect | record reproduction; do not mask in project |
| Environment | missing executable, daemon, socket, credential, or agent runner  | report prerequisite; do not rewrite config  |
| Project     | Compose/Makefile/docs/ports disagree with project behavior       | fix only with project scope and evidence    |

## Workflow

### 1. Protect Context

1. Read target and active module guidance.
2. Inspect scoped Git status; protect secrets, generated files, archives, and unrelated user
   changes.
3. At a devbox root, inspect `.gz-git.yaml` when present and inventory every child listed in its
   workspace inventory before selecting scope. Inventory is read-only evidence: do not clone or
   sync children unless the user separately authorizes it.
4. Capture the installed executable path and `dva version`. Treat that executable as the schema
   authority; do not rely on remembered fields or assume a source checkout produced it.

### 2. Capture a Read-Only Baseline

Run supported equivalents of:

```bash
dva config validate
dva config show -f yaml
dva manifest -f json
dva doctor --json
```

Do not use `--fix`, rewrite, reset, cleanup, or service-starting commands in the baseline. Treat
lifecycle help and previews as potentially executable until proven safe for the installed version;
use the sibling `dva` skill's `skills/dva/references/operation-safety.md`. A zero exit status does
not make contradictory output correct.

### 3. Select the Change Mode

| Mode     | Condition                                              | Default action                                 |
| -------- | ------------------------------------------------------ | ---------------------------------------------- |
| New      | no `dva.yml`                                           | discover project, scaffold, then refine        |
| Preserve | existing config contains useful working intent         | edit minimally; default for existing projects  |
| Migrate  | installed validator reports deprecated sections        | translate while preserving observable commands |
| Rewrite  | config is unusable or user explicitly requests rebuild | require approval and a before/after proposal   |

Never infer rewrite merely because a newer model exists.

### 4. Model Boundaries

- Standardize Compose/ports/env prerequisites before DVA references them.
- Use named plans for new/rewrite configuration. Preserve legacy modes/applications only until an
  explicit, behavior-preserving migration is proven.
- Keep shared lifecycle at the devbox root; keep module-native commands in the owning active
  subproject.
- For a devbox with `.gz-git.yaml`, a request to apply DVA to the devbox includes the root and every
  locally present repository listed in its workspace inventory by default. Evaluate and change each
  repository in its own ownership and Git context; do not treat the parent checkout as owning child
  files. Exclude an inventory entry only with explicit evidence from the manifest, repository
  guidance, or its generated/vendor/archive classification; never infer inactivity from its name.
- Keep child configuration scope separate from root import scope. A request to omit child imports
  from the root `dva.yml` does not omit creating or improving the child `dva.yml` files. Only an
  explicit instruction not to modify child repositories narrows the work to root-only; report that
  result as partial devbox coverage and list the deferred children.
- Declare a DVA subproject only when its child `dva.yml` exists. If it is missing, choose explicitly
  between removing the root declaration and adding an owned child configuration; never leave a
  broken declaration as a placeholder.
- Exclude archived, legacy, generated, and guidance-prohibited modules.
- Keep docs and advertised commands aligned with `dva show`/`dva ls` output.
- Decide DVA need independently for the root and every active subproject; do not generate a child
  config merely because a parent or sibling uses DVA. A `.gz-git.yaml` entry establishes discovery
  and default task scope, not proof that the child has a useful DVA command surface.
- For run/dev variants, Compose profiles, native port binding, and runtime ownership diagnostics,
  follow `references/diagnosis.md` instead of inferring behavior from configuration alone.

### 5. Deduplicate Orchestration Ownership

1. Let Compose declare services and dependencies; let DVA stack/plans select lifecycle; let
   interactions expose one-shot developer commands.
2. Compare custom `checks` with `dva doctor` built-ins. Remove equivalent Docker, Compose-file, and
   environment-file checks instead of accepting duplicate or contradictory results. `dva doctor`
   natively validates that the Compose config resolves — its "Compose config resolves" built-in runs
   `docker compose config` (including `include:` targets) — so do not hand-write a `checks:` entry
   duplicating it; rely on the built-in.
3. Flag a Compose service owned both by a stack runner and legacy `applications.*.run.docker`.
4. Flag repeated `docker compose -p/-f` commands when the same project/files are already declared in
   DVA. Use `service` + `command` for container exec interactions. Use the DVA Compose adapter only
   for an operation that cannot be expressed declaratively and preserves directory/profile semantics.
5. Keep provision focused on one-time setup; lifecycle up/down belongs to plans. Never make
   provision recursively call DVA or duplicate a plan's service startup.
6. Treat a `docker` runner as standalone `docker run`, not as a synonym for a Compose service. Do
   not generate it merely because an app is containerized.
7. Treat plan schema acceptance and plan resolution as insufficient runtime evidence. Before
   migrating from modes, prove that `up`, `down`, and `stop` consume each plan entry's selected
   runner, service subset, order, and dependencies. Reject the migration if the printed command or
   observed plan drops any of them.

### 6. Propose and Apply

1. Map every proposed edit to one evidence-backed finding and owner.
2. Separate config fixes from DVA tool/environment defects.
3. Obtain authority before rewrite, secret edits, destructive actions, service startup, global tool
   installation, or production/staging access.
4. Apply the smallest coherent change and revalidate after each logical group.

### 7. Validate in Risk Order

1. Referenced files, services, commands, endpoints, and subprojects exist — including that every
   Compose file referenced by `include:` or `-f` resolves. File existence alone is insufficient: a
   `compose.yaml` that `include:`s a renamed/removed file passes an existence check but fails at `up`.
2. `dva config validate` succeeds; classify every remaining warning.
3. `dva config show` and `dva manifest` match intended effective configuration and command surface.
4. `dva doctor --json` failures are assigned to config, tool, or environment.
5. A supported printed plan preserves runner, services, order, and dependencies for every named
   plan. Verify `up`, `down`, `stop`, and `restart` semantics separately using
   `references/diagnosis.md`; use lifecycle previews only after proving they do not execute.
6. Every configured Compose combination passes `docker compose ... config --quiet` — also surfaced
   continuously by the `dva doctor` "Compose config resolves" built-in, so this is a durable check,
   not only an author-time one.
7. Target lint/tests pass.
8. Start services and run health checks only when locally safe and authorized. Verify that the
   process group DVA controls owns the expected port; a pidfile, wrapper, or unrelated listener is
   insufficient.

## Common Mistakes

- Treating an unavailable optional AI runner as a broken `dva.yml`.
- Editing project config to silence a contradictory DVA diagnostic.
- Rewriting a working config before measuring its commands and modes/plans.
- Trusting `--dry-run` without checking that the installed lifecycle command is non-mutating.
- Keeping the same Compose service or prerequisite check under multiple DVA ownership models.
- Treating `docker` and `compose` runners as interchangeable names for the same service.
- Migrating to plans because validation passes while runtime silently ignores resolved fields.
- Declaring root subprojects before their child `dva.yml` files exist.
- Reading “no root child imports” as “do not configure child repositories.”
- Reporting a root-only change as complete devbox coverage when `.gz-git.yaml` children were
  explicitly deferred.
- Applying root rules recursively to archived or independently owned modules.
- Copying a schema example without checking the installed DVA version.
- Treating a pidfile, wrapper, or responding port as proof that DVA controls the healthy process.

## Output

Report baseline, selected mode, protected paths, owner-classified findings, changed files,
validation commands/results, deferred tool/environment issues, and the next safest action.
