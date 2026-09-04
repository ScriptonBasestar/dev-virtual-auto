# DVA Reusable Patterns

Use this reference when creating, reviewing, or migrating `dva.yml` files. Keep the config declarative: `stack` declares reusable units, `plans` names executable combinations, `interaction` covers one-shot commands, and `provision` covers setup/reset.

## Template Selection

| Task | Start from |
|---|---|
| Parent devbox owns shared infra and app orchestration | `assets/templates/root-devbox-plan.yml` |
| Subproject only needs local commands, tests, build, setup | `assets/templates/subproject-local.yml` |
| Existing config has `modes`, `applications`, `stack.*.order`, or top-level `compose` | `assets/templates/migrate-modes-to-plans.yml` |

## Standard Workflow

1. Discover current surface: `dva config show`, `dva ls`, `dva show`, `.gz-git.yaml`,
   compose files (root, `compose/`, `ops/*/compose.y*ml`), Makefile/package scripts, and
   subproject directories. Command migration, gz-git child connection, infra vs app plans,
   and Compose scaffold: `skills/dva-config/references/devbox-apply.md`.
2. Classify responsibilities:
   - `stack`: compose bundles, native apps, docker images, kubectl/helm targets.
   - `plans`: executable names such as `local-infra`, `local-dev`, `full-stack`, `observability`, `tools`.
   - `environments`: `dev`, `stg`, `prd` variable differences.
   - `sites`: host/runtime differences such as `local`, `office`, `remote`, `cloud`.
   - `interaction`: `db shell`, `test`, `lint`, `fmt`, `seed`, `env` helpers.
   - `provision`: initial setup, reset, secret/env bootstrap, migrations.
3. Resolve capability closure before rendering plans. Use one verified provider for each
   required database, cache, identity, queue, storage, and search capability.
4. Render from the smallest matching template. Delete unused sections instead of leaving placeholders.
5. Validate: `dva config validate`, then `dva config validate --strict`.
6. Drive the user surface: `dva ls`, `dva show`, and `dva up <plan> --dry-run`.

## Canonical Section Order

Use this order for readability and stable review diffs:

```text
version -> vars -> environment -> env_file -> stack -> plans -> default_plan -> environments -> sites -> checks -> default_mode -> suggestion_ignore -> modes -> health_checks -> interaction -> provision -> modules -> subprojects -> endpoints -> infra -> ssh -> devcontainer
```

This mirrors `canonicalSectionOrder` (`internal/config/validate_warnings.go`) — that variable is the
source of truth if the two ever drift.

Use `interaction`, not `interactions`, for the current schema.

For current validation, use `environments.<name>.environment` for environment-specific variables (the
current schema version lives in `internal/config/version.go`, not hardcoded here). The newer design
documents describe this concept as `vars`; keep that terminology in reasoning, but emit schema-valid
YAML until the config type is fully migrated.

## Naming Conventions

| Concept | Preferred names |
|---|---|
| Compose infra bundle | `infra`, `core-compose`, `infra-compose` |
| API app | `api`, `backend`, `<domain>-api` |
| Worker app | `worker`, `<domain>-worker` |
| Frontend app | `web`, `frontend`, `admin-ui`, `portal-ui` |
| Plans | `local-infra`, `local-dev`, `full-stack`, `observability`, `tools` |
| Environments | `dev`, `stg`, `prd`, `ci` |
| Sites | `local`, `office`, `remote`, `cloud`, `docker-host` |

Plan names should describe executable intent. Avoid exposing implementation terms like `stack-compose-main`.

## Plan Selection Contract

Plans are self-contained; they do not inherit from one another.

| Plan | Contents |
|---|---|
| `local-infra` | Core providers required by the normal native workflow |
| `local-dev` | The same explicit provider closure plus verified native app entries |
| `full-stack` | The same explicit provider closure plus verified Compose app services |
| `observability` | Monitoring services plus every provider/target they require |
| `tools` | Verified development tools plus their dependencies |

Use `default_plan: local-infra` only when it is local and non-destructive. Never make
`full-stack` the generated default. If no safe default is proven, omit `default_plan`
and require a named command. `dva up *` is never a valid selection strategy.

When an injected platform binding selects a provider, apply it after explicit project
ownership and before a generic local fallback. Use it only through a verified parent-owned
stack entry, separately imported plan, or documented external lifecycle target. Preserve
an existing local provider during a preserve audit; do not invent a sibling path, command,
or second database service. Binding metadata is generation context, not a `dva.yml` key.

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
- A present `.gz-git.yaml` child with a dev/app surface has no `dva.yml` or root `subprojects` entry.
- A DVA-owned action still has Make/`docker compose` as its implementation.
- `interaction` uses reserved command names (`logs`, `build`, `status`, `show`, etc.) as plain commands.
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
