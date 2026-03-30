# Stage 30: Configure (Full)

<role>Complete DVA environment generator</role>

<objective>
Generate a comprehensive set of clean infra files, including new `compose.yml` (or multiple compose files), and the main `dva.yml` wrapper in a fresh robust way.
</objective>

<critical-rules>
## Rules

Follow ALL rules from `library/shared-guardrails.md` (DVA Configuration Guardrails).

### Full-specific additions:

1. **Generate compose.yml from scratch** — All infra services must have `healthcheck:` blocks.
2. **Healthchecks in compose** — All core services MUST have `healthcheck:` defined.
3. **Generate `.env.example`** — Detail all required environment variables.
4. **May create directory structure** — If proposal calls for devcontainer config, include it.
</critical-rules>

<steps>
1. Evaluate `tmp/improve-guided/10-proposal-approved.yaml` configuring `setup_track` as `full`.
2. Generate Docker Compose file(s):
   - **MUST** include top-level `name: {project-name}`.
   - **MUST NOT** include top-level `version:` key.
   - Map services to docker compose `profiles` for optional services.
   - All infra services must have `healthcheck:` blocks.
   - Use project-specific host port ranges (not default ports).
3. **Load naming presets** from `library/naming-presets.md`.
4. **Load reference example** from `library/reference-examples.md` — select matching section.
5. Generate the primary `dva.yml` at the project root following shared guardrails section order and rules.
6. Embed custom interactions:
   - **Container commands** (db, redis, shell): use `service:` field
   - **Host commands** (build, test, lint, fmt, check): use `runner: local`
   - **Reserved DVA commands** (build, clean): use `replace:` hooks
7. Create `.env.example` detailing all required environment variables.
8. Verify output fields match `library/dva-schema.md` and `internal/config/schema.json`.
9. **MANDATORY SELF-REVIEW** — Run through `library/shared-checklist.md` before finalizing.
</steps>

<output>
- `compose.yml` (with top-level `name:`) and `dva.yml` exist at TARGET root.
- `.env.example` with all required variables.
- `tmp/improve-guided/30-configuration-log.txt`
</output>

<gate>
All items from `library/shared-checklist.md` must pass, plus:
- [ ] `compose.yml` has NO top-level `version:` key.
- [ ] All infra services in compose have `healthcheck:` defined.
- [ ] `.env.example` created.
- [ ] No common default ports used as host ports.
- [ ] Services section is tags-only (no ports/label/http/paths).
</gate>

<return>
{ "artifacts": ["tmp/improve-guided/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
</output>
