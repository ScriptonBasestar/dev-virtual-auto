<!-- v:2026-03-23 -->
<!-- track: adopt — 기존 compose가 있는 프로젝트용 -->

<constants>
SELF = ai-docs/workflow/setup-dva/stages/30-configure-adopt.md
DVA_ROOT = {DVA project root}
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
SCHEMA_REF = internal/config/schema.json
EXAMPLES_DIR = examples/
TARGET = {resolved from entry.md or auto.md — target project path}
</constants>

[EXECUTE IMMEDIATELY - NO QUESTIONS]

<role>DVA adopt configuration generator — 기존 compose.yml 분석 후 dva.yml만 생성</role>

<input>
| Source | Description |
|--------|-------------|
| Analysis | `tmp/setup-dva/00-analysis-{project-name}.md` |
| Proposal | `tmp/setup-dva/10-proposal-{project-name}.md` |
| Transform log | `tmp/setup-dva/20-transform-log-{project-name}.md` |
| Existing compose | `$TARGET/compose.yml` (already present) |
| DVA schema | SCHEMA_REF for validation |
| DVA examples | EXAMPLES_DIR for reference patterns |
</input>

<precondition>
This stage is for projects where **compose.yml already exists**.
Analysis report must contain `setup_track: adopt`.
If `setup_track: full`, use `30-configure-full.md` instead.
</precondition>

<objective>
Analyze the existing compose.yml to extract service definitions.
Generate dva.yml that wraps the existing compose configuration.
Do NOT modify or regenerate the existing compose.yml.
</objective>

<steps>
## Phase 1: Analyze Existing Compose

Read and parse the existing compose file:

```bash
# Read existing compose
cat $TARGET/compose.yml

# Validate syntax
docker compose -f $TARGET/compose.yml config --quiet 2>&1

# Extract service names
docker compose -f $TARGET/compose.yml config --services 2>/dev/null
```

Extract:
- Service names and their images
- Port mappings (host:container)
- Healthcheck definitions (present or missing)
- Volume mounts
- Environment variable patterns
- Compose file includes/overrides (if any)

## Phase 2: Compose Health Assessment

Check existing compose quality — report only, do NOT fix:

| Check | Action |
|-------|--------|
| Missing healthchecks | Warn in config log (suggest adding later) |
| `version:` key present | Note as legacy (informational only) |
| Hardcoded ports | Note in config log |
| No .env.example | Generate .env.example from detected vars |

If `.env.example` does not exist, generate one from compose env vars:
```bash
# Extract env vars from compose
docker compose -f $TARGET/compose.yml config 2>/dev/null | grep -E '^\s+\$\{' | sort -u
```

## Phase 3: Detect Compose Files

Identify all compose-related files the project uses:

```bash
ls $TARGET/compose*.yml $TARGET/docker-compose*.yml 2>/dev/null
```

Build the compose files list for dva.yml:
- `compose.yml` — primary
- `compose.override.yml` — local overrides (if exists)
- `compose.tools.yml` — dev tools (if exists)
- Any other `compose.*.yml` variants

## Phase 4: Generate dva.yml

Map existing compose services to DVA interactions:

```yaml
version: "0.1.0"
compose:
  files:
    - compose.yml          # existing
    # - compose.tools.yml  # if exists
```

### Service-to-Interaction Mapping

For each service detected in compose.yml, determine appropriate interactions:

| Service Pattern | Generated Interaction |
|----------------|----------------------|
| postgres/mysql/mariadb | `db:` with appropriate CLI command |
| redis | `redis-cli:` |
| app/api/web/server | `shell:`, `logs:` |
| worker/celery/sidekiq | `worker-logs:` |
| nginx/traefik/caddy | `proxy-logs:` |

```yaml
interaction:
  shell:
    description: "Open shell in {primary-service}"
    service: {primary-service}
    command: /bin/bash
  logs:
    description: "Tail service logs"
    service: {primary-service}
    command: ""
  db:
    description: "Database console"
    service: {db-service}
    command: {db-cli-command}
```

## Phase 5: Write Files

Write ONLY:
- `$TARGET/dva.yml` — DVA configuration wrapping existing compose
- `$TARGET/.env.example` — only if missing

Do NOT write:
- `$TARGET/compose.yml` — already exists, do not touch
- `$TARGET/compose.tools.yml` — only if not already present

If dva.yml already exists, create `dva.yml.new` and note in log.

## Phase 6: Validation

```bash
# Verify compose still works (no side effects)
docker compose -f $TARGET/compose.yml config --quiet 2>&1

# DVA config validation (if dva binary available)
DVA_FILE=$TARGET/dva.yml dva validate 2>/dev/null

# Cross-check: dva.yml references only services that exist in compose
docker compose -f $TARGET/compose.yml config --services 2>/dev/null
```
</steps>

<constraints>
- NEVER modify, overwrite, or regenerate the existing compose.yml
- NEVER change port mappings in existing compose
- dva.yml must reference only services that actually exist in compose.yml
- If compose.yml has issues (missing healthchecks, legacy version key), report in config log but do not fix
- Generated dva.yml must conform to DVA schema (SCHEMA_REF)
- Reference DVA examples for idiomatic interaction patterns
</constraints>

<gate>
- [ ] Existing compose.yml is unmodified (checksum unchanged)
- [ ] dva.yml generated and valid against DVA schema
- [ ] dva.yml references only existing compose services
- [ ] .env.example exists (generated if was missing)
- [ ] Config log generated with compose health assessment
</gate>

<output>
| Artifact | Path |
|----------|------|
| DVA config | `$TARGET/dva.yml` |
| Env template | `$TARGET/.env.example` (only if was missing) |
| Config log | `tmp/setup-dva/30-config-log-{project-name}.md` |
</output>

<trigger>Analyze existing compose → assess health → detect files → generate dva.yml → validate.</trigger>
