# DVA Configuration Guardrails (Shared)

> Single source of truth for dva.yml generation/improvement rules.
> Referenced by: improve prompt (auto mode), guided improve pipeline (interactive mode).

## Critical Rules

1. **`stack:` section required** — Compose config MUST be under `stack:`. Migrate legacy `compose:` root-level fields to `stack.compose:`.
2. **`modes:` required** — Always generate `modes:` for environment selection. Minimum: `infra` + one other (full-stack, hybrid, etc.).
3. **`default_mode` required** — `dva up` (no mode flag) must start minimal infra only. Never include Redis Sentinel/Cluster, Kafka, monitoring, HA replicas in default mode.
4. **`version:` field** — Must match the current DVA CLI version. Subproject versions must also match root.
5. **`health_checks`: `start` and/or `start_hint`** — `start` is the auto-start command (optional). `start_hint` is human-readable hint text shown by `dva status` (optional). If `start` is set, `start_hint` is only needed when it differs from the start command. Do not set both to identical values (validation warning).
6. **Health check URLs: literal values only** — `url:` and `address:` fields must use literal values (e.g., `http://localhost:14000/health`), NOT `${VAR:-DEFAULT}` patterns.
7. **Port conventions** — Never use common default ports as host ports: 2181, 3000, 3306, 5432, 6379, 8080, 8443, 9090, 9092, 9200, 15672, 27017.
8. **`runner: local` for host commands** — Build/test/lint/fmt/check MUST use `runner: local`. Never use `echo 'Run: ...'` wrappers.
9. **Reserved commands** — These are DVA built-in commands and MUST NOT appear as plain interaction keys: `up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`, `status`, `show`, `ls`, `run`, `config`, `doctor`, `provision`, `add`, `version`, `migrate`, `console`, `infra`, `dev`, `app`. Hookable commands (up/down/stop/restart/build/clean/logs/dev) use `replace:` hooks. Others must be renamed (e.g., `service-status`, `db-migrate`).
10. **Provision: direct commands only** — NEVER call `run: "dva <command>"` (circular dependency). Use direct shell commands.
11. **Provision completeness** — At least 3 profiles: `default`, `full`, `reset`.
12. **`env_file:` object format** — Must use `{ files: [...], interpolate: true }`, not plain string.
13. **`checks:` section** — Minimum: `docker_socket` + `.env` file_exists.
14. **Section order (canonical)** — version → environment → env_file → stack → checks → applications → default_mode → modes → environments → health_checks → interaction → provision → subprojects → endpoints. Omit unused sections, but included sections MUST follow this order.
15. **File header** — First line must be `# yaml-language-server: $schema=...` schema comment.
16. **`stack.compose.tags: [infra]`** — Primary stack entry MUST have compose-level `tags:` field.
17. **Service metadata: tags required** — Every service MUST have `tags:`. Port metadata (label, http, paths) belongs in `endpoints:` section, NOT in `services.ports`.
18. **Compose `name:` required** — compose.yml must have top-level `name:` matching `stack.compose.project_name`.
19. **Compose `version:` key** — Remove if present (Compose Specification does not require it).
20. **Stack files: verify existence** — Every file listed in `stack.{entry}.files` MUST actually exist. Run `ls` to verify.
21. **Multi-stack entries: no duplicate base files** — Do NOT repeat base `compose.yml` in every entry.
22. **Package names: EXACT from manifests** — Use `[package] name` from Cargo.toml, module path from go.mod, `name` from package.json. NOT directory names.
23. **Naming presets** — Tags: infra, api, worker, ui, data, monitoring, build. Modes: infra, full-stack, hybrid, backend, server, worker, ui. Environments: dev, test, stg, prd.
24. **No echo wrappers** — Never generate `echo 'Run: ...'` dummy commands.
25. **No code changes** — Only modify `dva.yml` and related config. Do not touch app code or Dockerfiles.
26. **Subprojects** — `exclude_tags: [infra]` to avoid parent infra duplication. Only allowed fields: `path`, `exclude_tags`.
27. **`applications:` for long-running app processes** — Application servers (API, workers, web frontends) that the developer actively develops MUST be declared in the `applications:` section, NOT buried in `interaction:` or only in `health_checks.start`. The `applications:` section supports both native and docker execution strategies via `run:`, `dev:`, and `build:` paths.
28. **`applications:` naming** — Use short lowercase names: `api`, `worker`, `web`, `scheduler`. These become the PID/log identifiers.
29. **`applications:` health** — Each application SHOULD have a `health:` block for readiness checking. `dva dev` waits for health before reporting success.
