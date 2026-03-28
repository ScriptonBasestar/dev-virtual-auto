# Stage 30: Configure (Adopt)

<role>Existing compose environment wrap configurator</role>

<objective>
Generate or upgrade the `dva.yml` mapping seamlessly to the TARGET project's existing Docker Compose infrastructure without rewriting their existing manifests.
Handles both fresh adoption (no dva.yml) and upgrade from legacy format (old dva.yml exists).
</objective>

<critical-rules>
## MUST follow — config generation invariants

1. **`modes:` NOT `profiles:`** — The `profiles:` key is deprecated. Always generate `modes:`.
2. **`compose.yml` needs `name:`** — If missing, ADD it matching `stack.compose.project_name`.
3. **`version:` field** — Set to current DVA version: `"0.1.26"`.
4. **`stack:` NOT top-level `compose:`** — Compose config MUST be under `stack:` section.
5. **`health_checks`: BOTH `start` and `start_hint`** — Always include both for native services.
6. **Port conventions** — Never use common default ports as host ports.
7. **compose.yml `version:` key** — Remove if present.
8. **`runner: local` for host commands** — Build/test/lint/fmt/check MUST use `runner: local`.
9. **Provision: direct commands only** — NEVER call `run: "dva <command>"` (circular dependency). Use direct shell commands.
10. **Mutually exclusive overlays** — MUST NOT be combined in `stack.compose.files`. Create separate stack entries.
11. **Provision completeness** — At least 3 profiles: `default`, `full`, `reset`.
12. **File header** — Start with `yaml-language-server: $schema=...` and pattern description block.
13. **Health check commands verifiable** — Use `pgrep -f {process}` or actual HTTP endpoint. Never invent flags.
14. **Service metadata** — Every service MUST have `tags:` and `ports:` with `label:`.
15. **Package names: EXACT from manifests** — Use `[package] name`, NOT directory name.
16. **Section order** — version → environment → env_file → stack → checks → modes → environments → health_checks → interaction → provision → subprojects → endpoints. Omit sections that are not needed, but included sections MUST follow this order.
17. **Naming presets** — Use standard tag names (infra, api, worker, ui, data, monitoring, build) and mode names (infra, full-stack, hybrid, etc.).
18. **Reserved commands use replace:** — `build`, `clean`, `logs` are hookable reserved DVA commands — use `replace:` hooks. See rule 19 for the full reserved list.
19. **Complete reserved command list** — These DVA command names are ALL reserved and MUST NOT appear as plain interaction commands: `up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`, `status`, `show`, `ls`, `run`, `config`, `doctor`, `provision`, `add`, `version`. If the project needs a similar function, either use `replace:` hooks (for hookable ones: up/down/stop/restart/build/clean/logs) or rename (e.g., `service-status` instead of `status`, `app-show` instead of `show`).
20. **Stack compose.files: verify existence** — Every file listed in `stack.{entry}.files` MUST actually exist in the TARGET project. Run `ls` to verify before including. Do NOT assume overlay files exist.
21. **Multi-stack entries: no duplicate base files** — When creating separate stack entries for overlays (e.g., compose-apps, compose-monitoring), each entry should list ONLY its own overlay file(s). Do NOT repeat the base `compose.yml` in every entry — DVA merges stack entries at runtime. Exception: if an overlay file requires the base to parse, include it, but document why.
22. **Health check URLs: literal values only** — Health check `url:` and `address:` fields must use literal values (e.g., `http://localhost:14000/health`), NOT `${VAR:-DEFAULT}` patterns. DVA resolves environment separately; shell variables in URLs will not be interpolated.
23. **stack.compose.tags: [infra]** — The compose-level `tags:` field MUST be present on the primary stack entry. This sets default tags for all services under that entry. Typically `tags: [infra]` for the main infrastructure compose.
</critical-rules>

<steps>
1. Load `tmp/setup-dva/10-proposal-approved.yaml` with `setup_track: adopt`.
2. Process existing docker compose files from `00-analysis-report.yaml`.
3. **Check for existing dva.yml** — if found:
   - Migrate deprecated patterns (top-level compose → stack, profiles → modes, lifecycle → stack)
   - Convert anti-patterns (echo wrappers → real commands, prefixed commands → replace hooks)
   - Convert non-standard fields (host_command → command + runner:local, etc.)
   - Convert old env_file format (string → object with files array)
   - Preserve user's existing structure while upgrading format
4. **Compose file fixes** (minimal, non-destructive):
   - Add `name:` if missing
   - Remove `version:` key if present
   - Do NOT modify service definitions
5. **Load naming presets** — Apply recommended tags and modes from analysis report.
6. **Reference example lookup** — Select matching section from reference-examples.md:
   - Rust hybrid → "Rust — Hybrid Pattern"
   - Go hybrid → "Go — Hybrid Pattern"
   - Python container-first → "Python/Django — Container-First"
   - Node.js → appropriate subsection
   - Do NOT copy placeholder values — use project-specific values.
7. Generate `dva.yml`:
   - File header with schema comment and pattern description
   - `version: "0.1.26"`
   - `env_file:` with files array and interpolate
   - `stack:` with services grouped by workgroup
   - `checks:` (docker_socket, .env, compose, language toolchain)
   - `modes:` (minimum: infra + full-stack or hybrid)
   - `environments:` if project has multi-env configs (dev, test, stg, prd) — place AFTER modes, BEFORE health_checks
   - `health_checks:` (with start + start_hint, appropriate ready_timeout)
   - `interaction:` organized by category (Database, Build, Test, Quality, Logs, Clean)
   - `provision:` (default, full, reset)
   - `subprojects:` if devbox pattern
8. **Interaction command rules:**
   - Container commands (db, redis, shell): `service:` field
   - Host commands (build, test, lint, fmt, check): `runner: local`
   - Reserved commands (build, clean, logs): `replace:` hooks
   - Never generate echo wrappers
9. **Compose overlay classification** — Include only primary files in stack. Document excluded overlays in comment.
10. **Port metadata validation** — Cross-reference compose ports with dva.yml metadata.
11. **Development pattern commands:**
    - container-first: commands use `service: {app-service}`
    - hybrid: commands use `runner: local`
12. **Subproject cascade:**
    - Version must match root
    - Apply same upgrade rules
    - Subprojects use `exclude_tags: [infra]`
13. Verify output against schema.
14. **MANDATORY SELF-REVIEW** — Before finalizing, check the generated dva.yml against these patterns:
    - ❌ `interaction.status:` → ✅ rename to `service-status` or `ps` (status is reserved)
    - ❌ `interaction.show:` → ✅ rename (show is reserved)
    - ❌ `stack.compose.files: [compose.yml, overlay.yml]` where overlay.yml doesn't exist → ✅ remove non-existent files
    - ❌ Multi-stack entries all listing `compose.yml` → ✅ only include base in primary entry
    - ❌ `url: "http://localhost:${VAR:-8080}/health"` → ✅ `url: "http://localhost:14000/health"` (literal)
    - ❌ `stack.compose:` without `tags:` → ✅ add `tags: [infra]`
    - ❌ `env_file: ".env"` → ✅ `env_file: { files: [...], interpolate: true }`
    - ❌ `build: { command: "make build", runner: local }` → ✅ `build: { replace: [{ step: "...", run: "make build" }] }`
    - ❌ health_checks with `start:` but no `start_hint:` → ✅ add both
    - ❌ Missing `checks:` section → ✅ add docker_socket + file_exists + toolchain checks
    - ❌ Missing `provision.reset:` → ✅ add reset profile
    - ❌ `environments:` placed before `modes:` or after `interaction:` → ✅ place between `modes:` and `health_checks:`
    - ❌ Sections out of canonical order → ✅ reorder: version → environment → env_file → stack → checks → modes → environments → health_checks → interaction → provision → subprojects → endpoints
15. **Compose file existence check** — For each file in `stack.{entry}.files`, verify it exists:
    ```bash
    ls $TARGET/{filename} 2>/dev/null
    ```
    Remove any file that doesn't exist. If an overlay file is referenced but missing, remove it from the list and add a comment.
</steps>

<output>
- `dva.yml` written (or upgraded) at TARGET root.
- Compose file patched if `name:` was missing.
- `tmp/setup-dva/30-configuration-log.txt`
</output>

<gate>
- [ ] `dva.yml` uses `modes:` (NOT `profiles:`).
- [ ] `dva.yml` uses `stack:` section (NOT top-level `compose:`).
- [ ] `dva.yml` version is `"0.1.26"`.
- [ ] Compose file has top-level `name:`.
- [ ] Health checks include both `start` and `start_hint`.
- [ ] Host commands use `runner: local` (no echo wrappers).
- [ ] Port metadata matches compose defaults.
- [ ] Modes section exists (minimum infra + one other).
- [ ] Checks section exists (minimum docker_socket + file_exists).
- [ ] Provision includes default + full + reset profiles.
- [ ] No `dva <command>` calls in provision.
- [ ] File header with schema comment present.
- [ ] All services have tags and ports with labels.
- [ ] Subproject versions match root.
- [ ] Section order follows canonical: version → environment → env_file → stack → checks → modes → environments → health_checks → interaction → provision → subprojects → endpoints.
- [ ] Naming follows presets (tags, modes, envs).
- [ ] Reserved commands use `replace:` hooks.
- [ ] No reserved DVA command names used as plain interaction keys (up/down/stop/restart/build/clean/logs/status/show/ls/run/config/doctor/provision/add/version).
- [ ] All files in stack.compose.files actually exist in TARGET.
- [ ] Health check URLs use literal values (no ${VAR:-DEFAULT}).
- [ ] stack.compose.tags: [infra] is present on primary stack entry.
- [ ] Multi-stack entries do not redundantly list the same base compose file.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
