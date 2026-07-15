# DVA Reusable Patterns

Use this reference when creating, reviewing, or migrating `dva.yml` files. Keep the config declarative: `stack` declares reusable units, `plans` names executable combinations, `interaction` covers one-shot commands, and `provision` covers setup/reset.

## Template Selection

| Task | Start from |
|---|---|
| Parent devbox owns shared infra and app orchestration | `assets/templates/root-devbox-plan.yml` |
| Subproject only needs local commands, tests, build, setup | `assets/templates/subproject-local.yml` |
| Existing config has `modes`, `applications`, `stack.*.order`, or top-level `compose` | `assets/templates/migrate-modes-to-plans.yml` |

## Standard Workflow

1. Discover current surface: `dva config show`, `dva ls`, `dva show`, existing compose files, Makefile/package scripts, and subproject directories.
2. Classify responsibilities:
   - `stack`: compose bundles, native apps, docker images, kubectl/helm targets.
   - `plans`: executable names such as `local-infra`, `local-dev`, `local-full`, `docker-full`.
   - `environments`: `dev`, `stg`, `prd` variable differences.
   - `sites`: host/runtime differences such as `local`, `office`, `remote`, `cloud`.
   - `interaction`: `db shell`, `test`, `lint`, `fmt`, `seed`, `env` helpers.
   - `provision`: initial setup, reset, secret/env bootstrap, migrations.
3. Render from the smallest matching template. Delete unused sections instead of leaving placeholders.
4. Validate: `dva config validate`, then `dva config validate --strict`.
5. Drive the user surface: `dva ls`, `dva show <plan>` when supported, and `dva --dry-run up <plan>` or `dva up <plan> --dry-run` depending on the CLI version.

## Canonical Section Order

Use this order for readability and stable review diffs:

```text
version -> vars -> environment -> env_file -> stack -> plans -> environments -> sites -> checks -> suggestion_ignore -> health_checks -> interaction -> provision -> subprojects -> endpoints
```

Use `interaction`, not `interactions`, for the current schema.

For current `0.1.44` validation, use `environments.<name>.environment` for environment-specific variables. The newer design documents describe this concept as `vars`; keep that terminology in reasoning, but emit schema-valid YAML until the config type is fully migrated.

## Naming Conventions

| Concept | Preferred names |
|---|---|
| Compose infra bundle | `infra`, `core-compose`, `infra-compose` |
| API app | `api`, `backend`, `<domain>-api` |
| Worker app | `worker`, `<domain>-worker` |
| Frontend app | `web`, `frontend`, `admin-ui`, `portal-ui` |
| Plans | `local-infra`, `local-dev`, `local-full`, `docker-full`, `observability`, `tracing` |
| Environments | `dev`, `stg`, `prd`, `ci` |
| Sites | `local`, `office`, `remote`, `cloud`, `docker-host` |

Plan names should describe executable intent. Avoid exposing implementation terms like `stack-compose-main`.

## Migration Map

| Old shape | New shape |
|---|---|
| top-level `compose` | `stack.<entry>.runners.compose` |
| `modes.<name>.compose_services` | `plans.<name>.entries[].services` |
| `modes.<name>.compose_profiles` | runner config or plan-specific compose vars until profile override is supported |
| `stack.*.order` | `plans.*.entries[].order` |
| `applications.<name>` | `stack.<name>.runners.native/docker` |
| `default_mode` | default plan naming plus docs; avoid relying on implicit all-service startup |
| `environments.*.stack` or `stack_overrides` | `plans.*.entries` and `sites.*.entry_overrides` |
| `interaction.logs/build/clean` plain commands | lifecycle hooks with `replace`, or renamed commands such as `app-logs` |

## Review Gates

Fail the review if any of these are true for a root devbox config:

- `stack` exists but `plans` is empty.
- `modes` remains as the primary execution model.
- top-level `compose` remains.
- `stack.*.order` controls execution.
- `applications` remains when the app can be a multi-runner `stack` entry.
- `subprojects.*.path` exists but the child `dva.yml` is missing.
- `interaction` uses reserved command names (`logs`, `build`, `clean`, `status`, `show`, etc.) as plain commands.
- `.sb/dva/` is not ignored when DVA writes transient state.

Acceptable exceptions:

- A subproject can omit `stack` and `plans` when it only exposes local `interaction` and `provision`.
- Legacy configs can keep `modes` temporarily if the task is validation-only, but strict validation should document the migration work.

## Validation Commands

```bash
dva version
dva config validate
dva config validate --strict
dva ls
dva show
dva manifest -f json
```

When validating many devboxes, run validation per config directory. Group failures by cause: missing subproject config, legacy compose schema, reserved command conflict, version mismatch, and strict migration warnings.
