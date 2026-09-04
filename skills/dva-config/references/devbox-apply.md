# Devbox Apply Policy

Canonical rules for applying DVA across a workspace. Skills, diagnosis, plan
templates, and `am run dva-improve*` consume this file. Do not copy the lists
below into other documents.

Parameters (flows: `apply_mode`, `legacy_surface`):

| Parameter | Values | Default |
|-----------|--------|---------|
| `apply_mode` | `propose` — emit a table, do not mutate until accepted; `force` — apply in this run | `propose` |
| `legacy_surface` | `alias` — keep the old name as a one-line handoff to DVA; `remove` — delete it | `alias` |

`apply_mode=force` is not the CLI flag `dva up --force`.

## A. Command migration

DVA owns every workflow it can express: named-plan lifecycle (`up`/`down`/`stop`/`restart`/`logs`/`build`),
`interaction`, `provision`, and `doctor` checks.

1. Inventory Make targets, package scripts, `ops/**` helpers, and raw
   `docker compose`/`skaffold` entrypoints.
2. If DVA can own the work, the implementation moves into `dva.yml`. Wrapping
   the old command (`command: make check`, `run: docker compose up`) is not
   migration.
3. Alias direction is old → DVA (`make api-check` → `dva api-check`). Never
   `dva` → `make` for a DVA-owned action, and never `run: dva …` from provision
   or Make.
4. `propose`: print a table (old command, DVA name, alias or delete, reason).
   Do not edit until the user accepts, unless `apply_mode=force`.
5. `alias`: leave a one-line Make/npm script that calls DVA. `remove`: delete
   the old entrypoint and point docs at `dva ls`.
6. Leave a raw command only when DVA cannot express it; record why
   (`suggestion_ignore` or the apply table).
7. Advertised commands in README, CLAUDE.md, and tasks must match `dva ls` /
   `dva show`.

## B. `.gz-git.yaml` children

When `.gz-git.yaml` exists, every **locally present** `workspaces:` entry with a
dev/app surface (Makefile, package scripts, Compose, language manifest plus a
run/test/build command) **must** be connected to DVA.

1. Child owns `dva.yml` in its repository (interactions/provision and, when it
   runs a long process, a `native` stack entry). Root does not commit child files.
2. Root declares `subprojects.<name>.path`. Do not declare a subproject whose
   child `dva.yml` is missing.
3. Root `import.plans` / `import.interactions` / `import.provision` only for
   names that should appear in the root `dva ls`. Omitting import hides the
   name from the root listing; it does not skip connecting the child.
4. Same-repo nested configs (`prototype/dva.yml`) are also `subprojects`. Do
   not leave a second config that `dva` discovers only by changing cwd.
5. Missing children: report unavailable; do not clone without authority. Do not
   skip a present child because of its name.
6. Exclude only when `.gz-git.yaml`, nearest repo guidance, or a
   generated/vendor/archive path says so.
7. A present child with **no** executable surface: record that assessment; do
   not invent an empty ceremonial `dva.yml`.

Root-only work is allowed only when the user forbids child-repository edits;
then list deferred inventory entries and call the result partial coverage.

## C. Infra and app plans

1. Collect capabilities from Compose (including `ops/**/compose.y*ml`),
   README/Makefile, ADR, and app env: database, cache, object-storage,
   identity, queue.
2. If the app needs a named store (for example RustFS), that provider is a
   stack entry plus an infra plan. Do not substitute a template postgres/redis
   for a different product.
3. App-dependent DB and middleware live on a plan **separate from apps**.
   Default names: `local-infra` (verified providers), `local-dev` (`local-infra`
   plus native apps). Split further when lifecycles differ (`design` is not
   `rustfs`).
4. Design-only Compose (Penpot) is `tools`/`design`, not product infra.
5. Native apps are child `native` runners selected by imported root plans.
   Host ports use a project range; common defaults (8080, 3000, 5432, 6379)
   are a finding, not a silent keep.
6. One lifecycle owner per Compose service. Plans repeat their full closure;
   they do not inherit.

## D. Compose default

1. Scan root compose files, `compose/`, `infra/`, `docker/`, `infrastructure/`,
   and `ops/*/compose.y*ml`. Skaffold/k8s evidence selects those runners; it
   does not replace local Compose when the app runs locally.
2. If local infra is required and no Compose (and no verified skaffold/k8s
   local path) exists, **create modular Compose**:
   - `compose/infra-<capability>.yaml` (and `compose/app-*.yaml` only for
     compose-hosted apps)
   - root `compose.yaml` with `include:`
   - one DVA stack compose runner pointing at that root file
   - top-level `name:` matching `project_name`; healthchecks; loopback publish
3. If `ops/<name>/compose.yml` already exists, adopt it; do not duplicate.
   Make/`docker compose` lifecycle becomes an alias or is removed (section A).
4. Generate only services the evidence names. Do not add a stock postgres
   because a template shows one.
5. Do not generate skaffold unless the repo already has a skaffold/k8s local
   workflow.
