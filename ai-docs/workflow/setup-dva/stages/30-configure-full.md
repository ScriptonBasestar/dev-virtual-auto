# Stage 30: Configure (Full)

<role>Complete DVA environment generator</role>

<objective>
Generate a comprehensive set of clean infra files, including new `compose.yml` (or multiple compose files), and the main `dva.yml` wrapper in a fresh robust way.
</objective>

<critical-rules>
## MUST follow — config generation invariants

1. **`modes:` NOT `profiles:`** — The `profiles:` key is deprecated. Always generate `modes:`.
2. **`compose.yml` MUST have `name:`** — Top-level `name: {project}` is required. MUST match `stack.compose.project_name`.
3. **`version:` field** — Set to current DVA version: `"0.1.26"`.
4. **`stack:` NOT top-level `compose:`** — Compose configuration MUST be under `stack:` section. Top-level `compose:` is deprecated.
5. **`health_checks`: BOTH `start` and `start_hint`** — When generating health checks for native services, always include both. `start` enables DVA auto-start with PID tracking; `start_hint` is the user-facing instruction.
6. **Port conventions** — Never use common default ports (5432, 6379, 8080, 3000, 3306, 27017) as host ports. Use project-specific port ranges.
7. **No `version:` in compose.yml** — Compose Specification does not require it.
8. **Healthchecks in compose** — All core services MUST have `healthcheck:` defined.
9. **`runner: local` for host commands** — Interaction commands that run on the host MUST use `runner: local`. Never use `echo 'Run: ...'` wrappers.
10. **Package names: EXACT from manifests** — Health check `start` and build `command` MUST use the EXACT package/binary name from the project's package manifest (Cargo.toml `[package] name`, go.mod, package.json). For Rust: `cargo run -p {exact-package-name}` where the name comes from `[package] name = "..."` in each crate's Cargo.toml. Common pitfall: directory names differ from package names (e.g., directory `db-orchestrator-api-rs` but package name `db-orchestrator-api`). **ALWAYS use `[package] name`, NOT the directory name.**
</critical-rules>

<steps>
1. Evaluate `tmp/setup-dva/10-proposal-approved.yaml` configuring `setup_track` as `full`.
2. Generate Docker Compose file(s):
   - **MUST** include top-level `name: {project-name}`.
   - **MUST NOT** include top-level `version:` key.
   - Map services to docker compose `profiles` for optional services (e.g., app services).
   - All infra services must have `healthcheck:` blocks.
   - Use project-specific host port ranges (not default ports).
3. **Load naming presets** from `library/naming-presets.md`:
   - Use `00-analysis-report.yaml`의 `project_archetype`로 mode 선택 가이드 결정.
   - `recommended_tags`와 `recommended_modes`를 참조하여 일관된 네이밍 적용.
4. **Load reference example** from `library/reference-examples.md`:
   - Select the section matching the project's primary language and `development_pattern`.
   - Use as structural guide for section ordering, mode patterns, health check patterns, interaction coverage, and provision profiles.
   - Do NOT copy placeholder values — replace with project-specific values.
5. Generate the primary `dva.yml` at the project root:
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
   - **CRITICAL:** Organize services by workgroup (Frontend, Backend, Workers), with clear comments.
   - Assign rich metadata using preset tag names (infra, api, worker, ui, data, monitoring, build).
   - Use `modes:` (NOT `profiles:`) — preset mode names 적용, `infra`는 항상 포함.
   - Use `modes.*.stack` to filter stack entries per mode where appropriate.
   - Use `environments:` — preset env names (dev, test, stg, prd) 중 필요한 것만.
   - Generate `health_checks` with both `start` and `start_hint` for native services.
   - Set appropriate `ready_timeout` (e.g., 120s for compiled languages like Rust/Go).
   - Include `checks:` for `dva doctor`.
5. Embed custom interactions:
   - **Container commands** (db, redis, shell): use `service:` field, no `runner:`
   - **Host commands** (build, test, lint, fmt, check): use `runner: local`, no `service:`
   - **Reserved DVA commands** (build, clean): use `replace:` hooks if needed
   - Never generate `echo 'Run: ...'` wrappers
6. Initialize a `.devcontainer` configuration block inside `dva.yml` if proposal calls for it.
7. Create a standard `.env.example` detailing all required environment variables.
8. Verify output fields match `library/dva-schema.md` and `internal/config/schema.json`.
</steps>

<output>
- `compose.yml` (with top-level `name:`) and `dva.yml` exist at TARGET root.
- `.env.example` with all required variables.
- `tmp/setup-dva/30-configuration-log.txt` (List of full track infra files written by the subagent)
</output>

<gate>
- [ ] Primary `dva.yml` uses `modes:` key (NOT `profiles:`).
- [ ] `dva.yml` uses `stack:` section (NOT top-level `compose:`).
- [ ] `dva.yml` version is `"0.1.26"`.
- [ ] `compose.yml` has top-level `name:` matching `stack.compose.project_name`.
- [ ] `compose.yml` has NO top-level `version:` key.
- [ ] All infra services in compose have `healthcheck:` defined.
- [ ] Health checks include both `start` and `start_hint` where applicable.
- [ ] Host commands use `runner: local` (no `echo 'Run: ...'` wrappers).
- [ ] No common default ports used as host ports.
- [ ] No JSON/YAML syntax errors exist in the output files.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
