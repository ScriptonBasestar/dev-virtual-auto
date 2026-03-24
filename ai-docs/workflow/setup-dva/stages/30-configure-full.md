# Stage 30: Configure (Full)

<role>Complete DVA environment generator</role>

<objective>
Generate a comprehensive set of clean infra files, including new `compose.yml` (or multiple compose files), and the main `dva.yml` wrapper in a fresh robust way.
</objective>

<critical-rules>
## MUST follow — config generation invariants

1. **`modes:` NOT `profiles:`** — The `profiles:` key is deprecated. Always generate `modes:`.
2. **`compose.yml` MUST have `name:`** — Top-level `name: {project}` is required. MUST match `dva.yml compose.project_name`.
3. **`version:` field** — Set to current DVA version: `"0.1.22"`.
4. **`health_checks`: BOTH `start` and `start_hint`** — When generating health checks for native services, always include both. `start` enables DVA auto-start with PID tracking; `start_hint` is the user-facing instruction.
5. **Port conventions** — Never use common default ports (5432, 6379, 8080, 3000, 3306, 27017) as host ports. Use project-specific port ranges.
6. **No `version:` in compose.yml** — Compose Specification does not require it.
7. **Healthchecks in compose** — All core services MUST have `healthcheck:` defined.
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
4. Generate the primary `dva.yml` at the project root:
   - Set `version: "0.1.22"`.
   - Set `compose.project_name` matching compose file `name:`.
   - **CRITICAL:** Organize services by workgroup (Frontend, Backend, Workers), with clear comments.
   - Assign rich metadata using preset tag names (infra, api, worker, ui, data, monitoring, build).
   - Use `modes:` (NOT `profiles:`) — preset mode names 적용, `infra`는 항상 포함.
   - Use `environments:` — preset env names (dev, test, stg, prd) 중 필요한 것만.
   - Generate `health_checks` with both `start` and `start_hint` for native services.
   - Set appropriate `ready_timeout` (e.g., 120s for compiled languages like Rust/Go).
   - Include `checks:` for `dva doctor`.
   - Embed custom interactions (`shell`, `test`, `lint`, `build`, etc.).
5. Initialize a `.devcontainer` configuration block inside `dva.yml` if proposal calls for it.
6. Create a standard `.env.example` detailing all required environment variables.
7. Verify output fields match `library/dva-schema.md` and `internal/config/schema.json`.
</steps>

<output>
- `compose.yml` (with top-level `name:`) and `dva.yml` exist at TARGET root.
- `.env.example` with all required variables.
- `tmp/setup-dva/30-configuration-log.txt` (List of full track infra files written by the subagent)
</output>

<gate>
- [ ] Primary `dva.yml` uses `modes:` key (NOT `profiles:`).
- [ ] `dva.yml` version is `"0.1.22"`.
- [ ] `compose.yml` has top-level `name:` matching `compose.project_name`.
- [ ] `compose.yml` has NO top-level `version:` key.
- [ ] All infra services in compose have `healthcheck:` defined.
- [ ] Health checks include both `start` and `start_hint` where applicable.
- [ ] No common default ports used as host ports.
- [ ] No JSON/YAML syntax errors exist in the output files.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
