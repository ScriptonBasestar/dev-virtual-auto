# Stage 30: Configure (Adopt)

<role>Existing compose environment wrap configurator</role>

<objective>
Generate or upgrade the `dva.yml` mapping seamlessly to the TARGET project's existing Docker Compose infrastructure without rewriting their existing manifests.
Handles both fresh adoption (no dva.yml) and upgrade from legacy format (old dva.yml exists).
</objective>

<critical-rules>
## MUST follow — config generation invariants

1. **`modes:` NOT `profiles:`** — The `profiles:` key is deprecated. Always generate `modes:`.
2. **`compose.yml` needs `name:`** — If the existing compose file lacks a top-level `name:` field, ADD it. The value MUST match `stack.compose.project_name`.
3. **`version:` field** — Set to current DVA version: `"0.1.26"`.
4. **`stack:` NOT top-level `compose:`** — Compose configuration MUST be under `stack:` section, not at top-level. Top-level `compose:` is deprecated.
5. **`health_checks`: BOTH `start` and `start_hint`** — When generating health checks for native services, always include both. `start` enables auto-start; `start_hint` is the user-facing instruction.
6. **Port conventions** — Never use common default ports (5432, 6379, 8080, 3000) as host ports. Verify the existing compose uses project-specific port ranges.
7. **compose.yml `version:` key** — Remove if present. Compose Specification does not require it.
8. **`runner: local` for host commands** — Interaction commands that run on the host (build, test, lint, fmt, check) MUST use `runner: local`. Never use `echo 'Run: ...'` as a command.
9. **Provision: `run:` NOT `compose_up:`/`compose_exec:`** — Always convert `compose_up:` to `run: "docker compose up -d --wait svc1 svc2"` and `compose_exec:` to `run: "docker compose exec svc cmd"`. These schema-valid shortcuts create inconsistency.
</critical-rules>

<steps>
1. Load `tmp/setup-dva/10-proposal-approved.yaml` with `setup_track: adopt`.
2. Process the project's existing docker compose files mapped in `00-analysis-report.yaml`.
3. **Check for existing dva.yml** — if `existing_dva.found` is true in analysis report:
   - Identify deprecated patterns: top-level `compose:`, `lifecycle:`, `profiles:`, old version
   - Identify interaction anti-patterns: `echo 'Run: ...'` wrappers, missing `runner: local`
   - **Migrate prefixed command workarounds**: `app-build` → `build` with `replace:` hook, `app-clean` → `clean` with `replace:` hook (these were workarounds for reserved DVA commands)
   - **Convert non-standard fields** (see `library/dva-schema.md` Non-Standard Field Migration):
     - `host_command:` → `command:` + `runner: local`
     - `compose_up:` in interaction → `command: "docker compose up -d ..."` + `runner: local`
     - `compose_logs:` in interaction → `command: "docker compose logs ..."` + `runner: local`
     - `compose_up:` / `compose_exec:` in provision → `run:` preferred (but schema-valid, keep if working)
     - `endpoints:` as array → convert to object-with-keys format with `label:` instead of `name:`
   - **Convert old `env_file` format**: string `".env"` → object with `files` array, add `interpolate: true`
   - **Warn on anti-patterns**:
     - Provision steps calling `run: "dva <command>"` → use direct commands (bootstrap ordering risk)
     - Provision hardcoding compose file paths → should reference stack config, not duplicate file lists
   - Preserve user's existing structure (modes, checks, provision, health_checks) while upgrading format
   - Log all migrations performed
4. **Compose file fixes** (minimal, non-destructive):
   - If missing top-level `name:`, add it matching the proposed `stack.compose.project_name`.
   - If `version:` key exists, remove it (Compose Specification).
   - Do NOT modify service definitions, networks, or volumes.
5. **Load naming presets** from `library/naming-presets.md`:
   - Use `00-analysis-report.yaml`의 `project_archetype`로 mode 선택 가이드 결정.
   - `recommended_tags`와 `recommended_modes`를 참조하여 일관된 네이밍 적용.
6. Generate `dva.yml` at the root directory:
   - Set `version: "0.1.26"`.
   - Use `stack:` section (NOT top-level `compose:`):
     ```yaml
     stack:
       compose:
         order: 10
         files: [compose.yml, ...]
         project_name: {project}
         services: { ... }
     ```
   - **CRITICAL:** Group services logically by workgroup with clear dividing comments.
   - Apply DVA metadata using preset tag names (infra, api, worker, ui, data, monitoring, build).
   - Use `modes:` (NOT `profiles:`) — preset mode names 적용, `infra`는 항상 포함.
   - Use `modes.*.stack` to filter stack entries per mode where appropriate.
   - Use `environments:` — preset env names (dev, test, stg, prd) 중 필요한 것만.
   - Generate `health_checks` with both `start` and `start_hint` for native services.
   - Include `checks:` for `dva doctor` (docker_socket, file_exists, command checks).
7. Create DVA native custom interactions:
   - **Container commands** (db, redis, shell): use `service:` field, no `runner:`
   - **Host commands** (build, test, lint, fmt, check): use `runner: local`, no `service:`
   - **Reserved DVA commands** (build, clean): use `replace:` hooks. `replace:` and `subcommands:` can coexist.
   - **Namespace-only parents**: commands with only `description:` + `subcommands:` (no `command:`) are valid for grouping
   - **Mixed-runner subcommands**: a parent with `service:` can have children with `runner: local` (e.g., `db` → container shell, `db migrate` → host command)
   - Never generate `echo 'Run: ...'` wrappers — always execute the actual command
   - Avoid `|| echo '...'` in provision — prefer `|| true` for silent idempotency
8. **Port metadata validation** — For each service in compose files, extract the default host port from `${VAR:-DEFAULT}` patterns. Cross-reference against `stack.compose.services.{name}.ports` entries. Fix any discrepancies so DVA metadata matches compose reality.
9. **Development pattern–aware command generation**:
   - If `development_pattern: container-first` (app runs in Docker, e.g., Django/Rails): interaction commands for build/test/lint use `service: {app-service}`.
   - If `development_pattern: hybrid` (infra in Docker, app native, e.g., Rust/Go): interaction commands for build/test/lint use `runner: local`.
   - Heuristic: if compose services have app profiles (`profiles: [rust]`) or if `modes:` includes `hybrid`/health_checks for native processes → hybrid pattern. If main app has no profile and always runs in Docker → container-first.
10. **Include ALL compose overlay files** — Glob for `compose*.yml` and `docker-compose*.yml` in TARGET root. Include all found files in `stack.compose.files`, not just the primary.
11. **Cascade to subprojects** — if `subprojects:` exists, check each subproject's dva.yml:
   - Version must match root (`"0.1.26"`)
   - Apply same upgrade rules (stack format, runner:local, no echo wrappers)
   - Convert `service: local` (old convention) → `runner: local`
   - Flag version mismatches between root and subproject
12. Verify output fields match `library/dva-schema.md` and `internal/config/schema.json`.
</steps>

<output>
- `dva.yml` written (or upgraded) at TARGET root.
- Compose file patched if `name:` was missing (minimal change).
- `tmp/setup-dva/30-configuration-log.txt` (List of adopt track config operations executed by the subagent)
</output>

<gate>
- [ ] Existing `docker-compose` ecosystem is fully wrapped without destructive rewrites.
- [ ] `dva.yml` uses `modes:` key (NOT `profiles:`).
- [ ] `dva.yml` uses `stack:` section (NOT top-level `compose:`).
- [ ] `dva.yml` version is `"0.1.26"`.
- [ ] Compose file has top-level `name:` matching `stack.compose.project_name`.
- [ ] Health checks include both `start` and `start_hint` where applicable.
- [ ] Host commands use `runner: local` (no `echo 'Run: ...'` wrappers).
- [ ] Port metadata in `stack.compose.services` matches compose file default ports.
- [ ] Prefixed commands (`app-build`, `app-clean`) migrated to reserved names with `replace:` hooks.
- [ ] All compose overlay files included in `stack.compose.files`.
- [ ] Subproject dva.yml versions match root version.
- [ ] Resulting `dva.yml` implements proper custom commands against existing services.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
