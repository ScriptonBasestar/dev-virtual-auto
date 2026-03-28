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
16. **Section order** — version → env_file → stack → checks → modes → health_checks → interaction → provision → subprojects.
17. **Naming presets** — Use standard tag names (infra, api, worker, ui, data, monitoring, build) and mode names (infra, full-stack, hybrid, etc.).
18. **Reserved commands use replace:** — `build`, `clean`, `logs` are reserved DVA commands. Use `replace:` hooks.
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
- [ ] Section order follows standard.
- [ ] Naming follows presets (tags, modes, envs).
- [ ] Reserved commands use `replace:` hooks.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
