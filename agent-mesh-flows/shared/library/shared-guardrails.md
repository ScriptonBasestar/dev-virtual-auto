# DVA Configuration Guardrails (Shared)

> Single source of truth for dva.yml generation/improvement rules.
> Referenced by: improve prompt (auto mode), guided improve pipeline (interactive mode).

## Critical Rules

1. **`stack:` section required** — Compose config MUST be under `stack.<entry>.runners.compose`. Migrate legacy root-level `compose:` and `stack.<entry>.compose` fields to `runners.compose`.
2. **`plans:` current model** — New/rewrite configs MUST use named `plans:`.
   Preserve legacy `modes:` only until an explicit migration; never generate new
   modes/applications merely to satisfy an old example.
3. **Plan scope** — Provide explicit minimal and full/hybrid plans as needed.
   Service selection belongs in `plans.*.entries[].services`.
4. **`version:` field** — Must match the current DVA CLI version. Subproject versions must also match root.
5. **`health_checks`: `start` and/or `start_hint`** — `start` is the auto-start command (optional). `start_hint` is human-readable hint text shown by `dva status` (optional). If `start` is set, `start_hint` is only needed when it differs from the start command. Do not set both to identical values (validation warning).
6. **Health check URLs: literal values only** — `url:` and `address:` fields must use literal values (e.g., `http://localhost:14000/health`), NOT `${VAR:-DEFAULT}` patterns.
7. **Port conventions** — Never use common default ports as host ports: 2181, 3000, 3306, 5432, 6379, 8080, 8443, 9090, 9092, 9200, 15672, 27017.
8. **`runner: local` for host commands** — Build/test/lint/fmt/check MUST use `runner: local`. Never use `echo 'Run: ...'` wrappers.
9. **Reserved commands** — These are DVA built-in commands and MUST NOT appear as plain interaction keys: `up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`, `status`, `show`, `ls`, `run`, `config`, `doctor`, `provision`, `version`, `console`, `infra`, `app`, `stack`, `help`, `compose`, `validate`, `manifest`, `ktl`, `ssh`, `completion`, `init`. Hookable commands (up/down/stop/restart/build/clean/logs) use `replace:` hooks. Others must be renamed (e.g., `service-status`, `db-migrate`). Canonical source: `internal/config/reserved.go`.
10. **Provision: direct commands only** — NEVER call `run: "dva <command>"` (circular dependency). Use direct shell commands.
11. **Provision completeness** — At least 3 profiles: `default`, `full`, `reset`.
12. **`env_file:` object format** — Must use `{ files: [...], interpolate: true }`, not plain string.
13. **Built-in checks first** — Do not generate custom Docker socket, Compose
    file, or env-file checks already emitted by `dva doctor`. Add `checks:` only
    for project-specific prerequisites.
14. **Section order (canonical)** — version → vars → environment → env_file →
    stack → plans → environments → sites → health_checks → interaction →
    provision → modules → subprojects → endpoints → infra → ssh → devcontainer.
    Legacy checks/applications/default_mode/modes remain in place during preserve
    mode but are not generated for new configs.
15. **File header** — First line must be `# yaml-language-server: $schema=...` schema comment.
16. **`stack.<entry>.runners.compose.tags: [infra]`** — Primary compose runner MUST have compose-level `tags:` field.
17. **Service metadata: tags required** — Every service MUST have `tags:`. Port metadata (label, http, paths) belongs in `endpoints:` section, NOT in `services.ports`.
18. **Compose `name:` required** — compose.yml must have top-level `name:` matching `stack.<entry>.runners.compose.project_name`.
19. **Compose `version:` key** — Remove if present (Compose Specification does not require it).
20. **Stack files: verify existence** — Every file listed in `stack.{entry}.runners.compose.files` MUST actually exist. Run `ls` to verify.
21. **Multi-stack entries: no duplicate base files** — Do NOT repeat base `compose.yml` in every entry.
22. **Package names: EXACT from manifests** — Use `[package] name` from Cargo.toml, module path from go.mod, `name` from package.json. NOT directory names.
23. **Naming presets** — Tags: infra, api, worker, ui, data, monitoring, build. Modes: infra, full-stack, hybrid, backend, server, worker, ui. Environments: dev, test, stg, prd.
24. **No echo wrappers** — Never generate `echo 'Run: ...'` dummy commands.
25. **No code changes** — Only modify `dva.yml` and related config. Do not touch app code or Dockerfiles.
26. **Subprojects** — `exclude_tags: [infra]` to avoid parent infra duplication. Allowed fields: `path`, `exclude_tags`, `import`. Only add non-empty `import` entries after the child `dva.yml` exists; placeholders may omit `import` or use `import: {}`.
27. **App declarations** — Model native app processes as native/process stack
    runners and select them from plans. Compose-hosted apps remain services of
    the compose stack entry.
28. **Single lifecycle owner** — A Compose service MUST NOT also appear as
    `applications.*.run.docker.service`, a standalone docker runner, a raw
    `docker compose up` interaction, or a provision lifecycle step.
29. **Runner meaning** — `compose` manages Compose services. `docker` manages a
    standalone `docker run` container. Merely being containerized does not
    justify generating both runners.
30. **Plan dependencies** — Express cross-entry startup order with
    `plans.entries[].depends_on` and `order`.
31. **Health ownership** — Compose healthchecks remain in Compose; native
    process readiness belongs to its stack runner/health configuration.
32. **`stop` vs `down` semantics** — `dva stack stop` / `dva app stop` sends SIGTERM but preserves state (PID files, volumes) for quick restart. `dva stack down` / `dva app down` sends SIGTERM AND removes resources (PID files, log files, optionally volumes with `-v`). Prefer `stop` during development iteration; use `down` for full cleanup.
33. **Named lifecycle** — Use `dva up <plan>`/`down <plan>` for declarative
    lifecycle. Legacy `dva app` and `--mode` behavior is migration-only.
34. **No synthetic dual path** — Do not invent native and docker alternatives.
    Declare only execution paths proven by project files and developer workflow.
35. **Environment switching** — Put dev/stg/prd variables in `environments:`
    and host/location differences in `sites:`; plans select both.
36. **Provision boundary** — Provision prepares files, dependencies, directories,
    or credentials. Lifecycle startup belongs to plans.
37. **Interaction boundary** — Interactions are one-shot developer commands,
    not alternate lifecycle wrappers.
38. **Compose command reuse** — Container exec interactions should use
    `service:` + `command:`. Avoid repeating declared Compose file/project flags.
39. **Preserve migration** — In preserve mode, report legacy ownership overlap
    before migration; do not silently delete working commands.
40. **Rewrite migration** — In rewrite mode, remove legacy applications/modes
    only after equivalent stack runners and plans exist.
