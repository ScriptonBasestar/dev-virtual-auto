# Stage 30: Configure (Adopt)

<role>Existing compose environment wrap configurator</role>

<objective>
Generate or upgrade the `dva.yml` mapping seamlessly to the TARGET project's existing Docker Compose infrastructure without rewriting their existing manifests.
Handles both fresh adoption (no dva.yml) and upgrade from an older dva.yml.
</objective>

<critical-rules>
## Rules

Follow ALL rules from `library/shared-guardrails.md` (DVA Configuration Guardrails).

### Adopt-specific additions:

1. **Minimal changes to existing compose files** — Only add `name:` if missing and remove `version:` key. Do NOT modify service definitions.
2. **Preserve existing structure** — Upgrade format while keeping user's existing interaction names and organization.
3. **Convert anti-patterns** — echo wrappers → real commands, prefixed commands (app-build) → replace hooks, host_command → command + runner:local.
4. **Health check commands verifiable** — Use `pgrep -f {process}` or actual HTTP endpoint. Never invent flags.
</critical-rules>

<steps>
1. Load `tmp/improve-guided/10-proposal-approved.yaml` with `setup_track: adopt`.
2. Process existing docker compose files from `00-analysis-report.yaml`.
3. **Check for existing dva.yml** — if found:
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
7. Generate `dva.yml` following shared guardrails section order and rules.
8. **Applications section generation** — For each detected long-running app server (from `detected_applications` in analysis report):
   - Declare in `applications:` with short lowercase name (api, worker, web, scheduler)
   - Set `run:` with native and/or docker exec paths
   - Set `dev:` for dev mode (hot-reload) if applicable
   - Set `build:` with native and/or docker build commands
   - Set `health:` block with type/url/timeout if HTTP endpoint exists
   - Set `port:` for the app's listening port
   - Set `depends_on:` for startup ordering (e.g., worker depends on api)
   - Set `dir:` if app root differs from config root
   - Migrate apps from `health_checks.start` to `applications:` (remove redundant health_check entries)
9. **Interaction command rules:**
   - Container commands (db, redis, shell): `service:` field
   - Host commands (build, test, lint, fmt, check): `runner: local`
   - Reserved commands (build, clean, logs): `replace:` hooks
   - Never generate echo wrappers
10. **Compose overlay classification** — Include only primary files in stack. Document excluded overlays in comment.
11. **Endpoint completeness check** — Verify all user-facing compose ports are declared in `endpoints:` section.
12. **Development pattern commands:**
    - container-first: commands use `service: {app-service}`
    - hybrid: commands use `runner: local`
13. **Subproject cascade:**
    - Version must match root
    - Apply same upgrade rules
    - Subprojects use `exclude_tags: [infra]`
14. Verify output against schema.
15. **MANDATORY SELF-REVIEW** — Run through `library/shared-checklist.md` before finalizing.
16. **Compose file existence check** — For each file in `stack.{entry}.files`, verify it exists with `ls`.
</steps>

<output>
- `dva.yml` written (or upgraded) at TARGET root.
- Compose file patched if `name:` was missing.
- `tmp/improve-guided/30-configuration-log.txt`
</output>

<gate>
All items from `library/shared-checklist.md` must pass, plus:
- [ ] Existing compose service definitions not modified (adopt mode).
- [ ] Anti-patterns converted (echo wrappers, prefixed commands).
- [ ] Endpoints section covers all user-facing compose ports.
- [ ] Naming follows presets (tags, modes, envs).
</gate>

<return>
{ "artifacts": ["tmp/improve-guided/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
</output>
