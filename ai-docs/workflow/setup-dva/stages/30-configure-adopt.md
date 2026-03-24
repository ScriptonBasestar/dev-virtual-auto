# Stage 30: Configure (Adopt)

<role>Existing compose environment wrap configurator</role>

<objective>
Generate the primary `dva.yml` mapping seamlessly to the TARGET project's existing Docker Compose infrastructure without rewriting their existing manifests.
</objective>

<critical-rules>
## MUST follow — config generation invariants

1. **`modes:` NOT `profiles:`** — The `profiles:` key is deprecated. Always generate `modes:`.
2. **`compose.yml` needs `name:`** — If the existing compose file lacks a top-level `name:` field, ADD it. The value MUST match `dva.yml compose.project_name`.
3. **`version:` field** — Set to current DVA version: `"0.1.22"`.
4. **`health_checks`: BOTH `start` and `start_hint`** — When generating health checks for native services, always include both. `start` enables auto-start; `start_hint` is the user-facing instruction.
5. **Port conventions** — Never use common default ports (5432, 6379, 8080, 3000) as host ports. Verify the existing compose uses project-specific port ranges.
6. **compose.yml `version:` key** — Remove if present. Compose Specification does not require it.
</critical-rules>

<steps>
1. Load `tmp/setup-dva/10-proposal-approved.yaml` with `setup_track: adopt`.
2. Process the project's existing docker compose files mapped in `00-analysis-report.yaml`.
3. **Compose file fixes** (minimal, non-destructive):
   - If missing top-level `name:`, add it matching the proposed `compose.project_name`.
   - If `version:` key exists, remove it (Compose Specification).
   - Do NOT modify service definitions, networks, or volumes.
4. **Load naming presets** from `library/naming-presets.md`:
   - Use `00-analysis-report.yaml`의 `project_archetype`로 mode 선택 가이드 결정.
   - `recommended_tags`와 `recommended_modes`를 참조하여 일관된 네이밍 적용.
5. Generate `dva.yml` at the root directory:
   - Set `version: "0.1.22"`.
   - Link to existing compose via `compose: files: [...]`.
   - Set `compose.project_name` matching compose file `name:`.
   - **CRITICAL:** Group services logically by workgroup with clear dividing comments.
   - Apply DVA metadata using preset tag names (infra, api, worker, ui, data, monitoring, build).
   - Use `modes:` (NOT `profiles:`) — preset mode names 적용, `infra`는 항상 포함.
   - Use `environments:` — preset env names (dev, test, stg, prd) 중 필요한 것만.
   - Generate `health_checks` with both `start` and `start_hint` for native services.
   - Include `checks:` for `dva doctor` (docker_socket, file_exists, command checks).
6. Create DVA native custom interactions (`shell`, `test`, `lint`, etc.) mapped to existing application services.
7. Verify output fields match `library/dva-schema.md` and `internal/config/schema.json`.
</steps>

<output>
- `dva.yml` written at TARGET root.
- Compose file patched if `name:` was missing (minimal change).
- `tmp/setup-dva/30-configuration-log.txt` (List of adopt track config operations executed by the subagent)
</output>

<gate>
- [ ] Existing `docker-compose` ecosystem is fully wrapped without destructive rewrites.
- [ ] `dva.yml` uses `modes:` key (NOT `profiles:`).
- [ ] `dva.yml` version is `"0.1.22"`.
- [ ] Compose file has top-level `name:` matching `compose.project_name`.
- [ ] Health checks include both `start` and `start_hint` where applicable.
- [ ] Resulting `dva.yml` implements proper custom commands against existing services.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
