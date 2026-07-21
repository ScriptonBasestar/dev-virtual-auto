---
name: config
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
3. Capture `dva version`. Treat the installed CLI as the schema authority; do not rely on
   remembered fields. Before running lifecycle `--help` in a live workspace, prove in a disposable
   fixture that the installed manual flag parser returns help without entering command execution.

### 2. Capture a Read-Only Baseline

Run supported equivalents of:

```bash
dva config validate
dva config show -f yaml
dva manifest -f json
dva doctor --json
```

Read help only after the help path is proven non-executing. Do not use `--fix`, rewrite, reset,
cleanup, or service-starting commands in the baseline. Do not assume a global `--dry-run` prevents
lifecycle execution: verify every selected runner's up/down/stop behavior in a disposable fixture,
including process-backed PID state, before using it with `up`, `down`, `restart`, `provision`, or
similar commands in a live workspace. A zero exit status does not make contradictory output correct.

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
- When a service has distinct run variants (e.g. hot-reload `dev` vs built `preview`/prod), model
  each variant as its own stack entry and select it with a named plan (`dev`, `preview`) rather than
  overloading one entry or relying on `--dev`. Declare `default_plan` to set the local default (e.g.
  `default_plan: dev` for a devbox); it selects among plans and is validated against them. This keeps
  "which mode is default" a project config choice, not a DVA built-in.
- `dva stack up` starts every stack entry with no plan/`default_plan` awareness — for the Compose
  runner it is a bare `docker compose up`, so only profile-less services start. Make the minimal
  default the Compose layer's job: keep core data (postgres, redis) with no Compose `profiles:` so
  they always start, and gate optional tiers behind Compose-native `profiles: [workflow|monitoring|
  dev-tools|apps]`. Plans still select explicit subsets via `dva up <plan>`; naming a profiled
  service in a plan starts it regardless of profile. This is Compose's own `profiles:` field, not a
  DVA profiles layer.
- Keep shared lifecycle at the devbox root; keep module-native commands in the owning active
  subproject.
- Declare a DVA subproject only when its child `dva.yml` exists. If it is missing, choose explicitly
  between removing the root declaration and adding an owned child configuration; never leave a
  broken declaration as a placeholder.
- Exclude archived, legacy, generated, and guidance-prohibited modules.
- Use recursive improvement only when each detected subproject is in scope.
- Keep docs and advertised commands aligned with `dva show`/`dva ls` output.
- Per-service `run`/`dev`/`build` commands are config-defined: `dva app up` runs the `run` command
  (production/preview) by default, and `--dev` opts into the `dev` command (hot-reload). A run-vs-dev
  default difference is DVA config, not a tool defect. Confirm the actual dev-mode surface against the
  installed CLI (`dva app up <name> --dev`); do not assume a bare `dva dev` command exists.
- `up` is idempotent: it will not switch an already-running app between `run` and `dev`. With the app
  live it prints `already running` and skips the entry, so `up --dev` on a preview-mode app is a
  silent no-op. Flip a live app's mode with `restart --dev` (or `down` then `up --dev`). For a built
  frontend (`preview`), the bundle's `MODE` is baked at build time, so restarting `preview` never
  exposes dev-only, `MODE`-gated UI — only the `dev` command rebuilds in development mode.
- A native `run` command must make its process bind the port its `health` check targets. App servers
  often default to a shared standalone port (`:8080`, `:3000`); set it per app via `environment`, do
  not assume the health URL configures anything, and confirm the value survives three seams: an
  `env_file` exporting it under a name the app never reads (`API_PORT` vs the app's `PORT`); a Makefile
  `PORT=$(PORT) ./bin` where a makefile `PORT = 8080` shadows the process env; a `--port` flag whose
  default overrides the env inside the app. Two apps sharing one default collide — first wins, rest
  crash on bind. The last two seams are project defects to report, not `dva.yml` fixes.

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
   plan. Verify `up`, `down`, and `stop` semantics separately. Use lifecycle `--dry-run` in the
   target only after proving the installed version does not execute it.
6. Every configured Compose combination passes `docker compose ... config --quiet` — also surfaced
   continuously by the `dva doctor` "Compose config resolves" built-in, so this is a durable check,
   not only an author-time one.
7. Target lint/tests pass.
8. Start services and run health checks only when locally safe and authorized. When you do start a
   native app, confirm it listens on the port its `health` check targets (e.g. `lsof -iTCP:<port>`),
   not merely that its pidfile is live: a wrapper process (`make`, `sh`, `node`) stays alive while the
   real server bound the wrong port or crashed, so a green liveness check can mask a wrong-port bind.

## Common Mistakes

- Treating an unavailable optional AI runner as a broken `dva.yml`.
- Editing project config to silence a contradictory DVA diagnostic.
- Rewriting a working config before measuring its commands and modes/plans.
- Trusting `--dry-run` without checking that the installed lifecycle command is non-mutating.
- Keeping the same Compose service or prerequisite check under multiple DVA ownership models.
- Treating `docker` and `compose` runners as interchangeable names for the same service.
- Migrating to plans because validation passes while runtime silently ignores resolved fields.
- Declaring root subprojects before their child `dva.yml` files exist.
- Applying root rules recursively to archived or independently owned modules.
- Copying a schema example without checking the installed DVA version.
- Expecting `dva stack up` to honor `default_plan` or start a minimal set — it runs a bare Compose
  `up` over all profile-less services. A minimal default comes from Compose `profiles:` gating
  optional tiers, not from DVA plan selection.
- Assuming `dva app up` starts dev mode — it runs the `run`/preview command by default; `--dev` opts
  into the `dev` command. Misreading this as a bug instead of a config choice.
- Running `up --dev` against an app already started in `run`/preview and expecting it to switch — `up`
  is idempotent and skips a live app, so use `restart --dev` or `down`+`up --dev`.
- Reading a live pidfile (or a `health` URL) as proof the app is on the expected port. It binds
  whatever its command/env/flags resolve to; a wrapper (`make`/`node`) stays alive on the wrong port.

## Output

Report baseline, selected mode, protected paths, owner-classified findings, changed files,
validation commands/results, deferred tool/environment issues, and the next safest action.
