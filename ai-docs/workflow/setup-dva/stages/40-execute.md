<!-- v:2026-03-24 -->

<constants>
SELF = ai-docs/workflow/setup-dva/stages/40-execute.md
DVA_ROOT = {DVA project root}
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
TARGET = {resolved from entry.md or auto.md — target project path}
</constants>

[EXECUTE IMMEDIATELY - NO QUESTIONS]

<role>DVA infrastructure execution agent — start services, verify health</role>

<input>
| Source | Description |
|--------|-------------|
| compose.yml | Generated in stage 30 at target project |
| dva.yml | Generated in stage 30 at target project |
| .env.example | Generated in stage 30 at target project |
| Config log | `tmp/setup-dva/30-config-log-{project-name}.md` |
</input>

<objective>
Start the development infrastructure using DVA (or docker compose fallback).
Verify all services are healthy and accessible.
Run comprehensive CLI verification: test all DVA commands and flag combinations
(--mode, --env) against the generated configuration to catch config errors early.
</objective>

<steps>
## Phase 1: Pre-flight

```bash
# Ensure .env exists (copy from .env.example if missing)
[ -f $TARGET/.env ] || cp $TARGET/.env.example $TARGET/.env

# Verify compose syntax
docker compose -f $TARGET/compose.yml config --quiet

# Check for port conflicts with running containers
docker ps --format '{{.Ports}}' 2>/dev/null | head -20
```

## Phase 2: Start Infrastructure

### Option A: DVA CLI available
```bash
cd $TARGET
dva up
```

### Option B: DVA from source (development)
```bash
cd $TARGET
go run $DVA_ROOT/cmd/dva/main.go up
```

### Option C: Fallback to docker compose
```bash
cd $TARGET
docker compose up -d
```

Detection logic:
```bash
if command -v dva &>/dev/null; then
  dva up
elif [ -d "$DVA_ROOT/cmd/dva" ]; then
  go run $DVA_ROOT/cmd/dva/main.go up
else
  docker compose up -d
fi
```

## Phase 3: Health Verification

```bash
# Container status
docker compose -f $TARGET/compose.yml ps --format 'table {{.Name}}\t{{.Status}}\t{{.Ports}}'

# Wait for healthy (up to 60s)
for i in $(seq 1 12); do
  UNHEALTHY=$(docker compose -f $TARGET/compose.yml ps --format json 2>/dev/null | grep -c '"Health":"starting"')
  [ "$UNHEALTHY" = "0" ] && break
  sleep 5
done

# Final health report
docker compose -f $TARGET/compose.yml ps
```

## Phase 4: Connectivity Test

For each service with exposed ports:
```bash
nc -z localhost {port} && echo "OK: {service}:{port}" || echo "FAIL: {service}:{port}"
```

## Phase 5: CLI Command Verification

Run each DVA command against the generated configuration.
Record pass/fail for each. A command "passes" if exit code is 0 (or expected non-zero for error-case validation).

> **Note:** `dva up` uses `DisableFlagParsing: true` (cobra), so root-level `--dry-run`
> is NOT available for lifecycle commands. Use `--no-wait` for quick start/stop cycles instead.
> Only `dva run CMD --dry-run` supports dry-run (via `runCmd`).

### 5-1. Config & Metadata Commands

```bash
cd $TARGET

# dva validate — must pass with no errors
dva validate 2>&1; echo "EXIT:$?"

# dva show — must display profiles, environments, interaction commands
dva show 2>&1; echo "EXIT:$?"

# dva show --json — JSON output mode
dva show --json 2>&1; echo "EXIT:$?"

# dva status — must show config path and service state
dva status 2>&1; echo "EXIT:$?"

# dva status --json — JSON output mode
dva status --json 2>&1; echo "EXIT:$?"

# dva ls — must list interaction commands
dva ls 2>&1; echo "EXIT:$?"
```

### 5-2. Mode (--mode/-M) Flag Verification

For **each profile** defined in dva.yml `profiles:` section:

```bash
# Extract profile names from dva.yml (jq required; fallback to python3 if unavailable)
PROFILES=$(dva show --json 2>/dev/null | jq -r '.profiles // {} | keys[]' 2>/dev/null)
if [ -z "$PROFILES" ]; then
  PROFILES=$(dva show --json 2>/dev/null | python3 -c "import sys,json; [print(k) for k in json.load(sys.stdin).get('profiles',{})]" 2>/dev/null)
fi

for MODE in $PROFILES; do
  echo "=== Testing --mode $MODE ==="

  # Quick start with --no-wait (immediate return, no blocking)
  dva up -M $MODE --no-wait 2>&1; echo "EXIT:$?"

  # Verify status shows correct mode effect
  dva status 2>&1 | head -10

  # Verify down also works with mode
  dva down -M $MODE 2>&1; echo "EXIT:$?"
done
```

**Validation criteria per mode:**
| Mode Type | compose_services | Expected Behavior |
|-----------|-----------------|-------------------|
| native | `[]` (empty) | Skips compose, runs health_checks only |
| docker | not set | Full docker compose with compose_profiles |
| hybrid | `[svc1, svc2]` | Only listed services start |

### 5-3. Environment (--env/-E) Flag Verification

For **each environment** defined in dva.yml `environments:` section:

```bash
ENVS=$(dva show --json 2>/dev/null | jq -r '.environments // {} | keys[]' 2>/dev/null)
if [ -z "$ENVS" ]; then
  ENVS=$(dva show --json 2>/dev/null | python3 -c "import sys,json; [print(k) for k in json.load(sys.stdin).get('environments',{})]" 2>/dev/null)
fi

for ENV_NAME in $ENVS; do
  echo "=== Testing --env $ENV_NAME ==="

  # Quick start + immediate teardown (verifies flag parsing and env application)
  dva up --env $ENV_NAME --no-wait 2>&1; echo "EXIT:$?"
  dva down 2>&1; echo "EXIT:$?"

  # Short flag form
  dva up -E $ENV_NAME --no-wait 2>&1; echo "EXIT:$?"
  dva down 2>&1; echo "EXIT:$?"
done
```

### 5-4. Combined Mode + Env Flag Verification

Test at least one mode+env combination (if both exist):

```bash
if [ -n "$PROFILES" ] && [ -n "$ENVS" ]; then
  FIRST_MODE=$(echo $PROFILES | head -1)
  FIRST_ENV=$(echo $ENVS | head -1)
  echo "=== Testing --mode $FIRST_MODE --env $FIRST_ENV ==="
  dva up --mode $FIRST_MODE --env $FIRST_ENV --no-wait 2>&1; echo "EXIT:$?"
  dva down -M $FIRST_MODE 2>&1; echo "EXIT:$?"
fi
```

### 5-5. Provision Profile Verification

For **each provision profile** defined in dva.yml `provision:` section:

**Prerequisite:** Infrastructure must be running (Phase 2 completed) before testing provision.
Provision steps may have side effects (db:create, db:migrate, npm install, etc.).

```bash
# List provision profiles
PROV_PROFILES=$(dva show --json 2>/dev/null | jq -r '.provision_profiles // [] | .[]' 2>/dev/null)

# Test default profile (full execution — requires running containers)
if echo "$PROV_PROFILES" | grep -q "default"; then
  dva provision default 2>&1 | tail -20; echo "EXIT:$?"
fi

# For non-default profiles, verify they are recognized (not full execution)
for PROV in $PROV_PROFILES; do
  [ "$PROV" = "default" ] && continue
  # Verify profile name is accepted (will fail fast if services not running)
  dva provision $PROV 2>&1 | head -5; echo "PROV=$PROV EXIT:$?"
done
```

### 5-6. Interaction Command Verification

Verify all interaction commands are recognized:

```bash
# List all commands
dva ls 2>&1

# Dry-run each top-level interaction command
# Note: dva run supports --dry-run (unlike lifecycle commands like dva up)
CMDS=$(dva ls 2>/dev/null | grep -E '^\s+\w' | awk '{print $1}')
for CMD in $CMDS; do
  dva run $CMD --dry-run 2>&1; echo "CMD=$CMD EXIT:$?"
done
```

### 5-7. Error Case Verification

Verify DVA gives clear errors for invalid inputs:

```bash
# Invalid mode name — should fail with available modes list
dva up --mode nonexistent-mode 2>&1; echo "EXIT:$?"
# Expected: non-zero exit, message contains "not found"

# Invalid env name — should fail with available envs list
dva up --env nonexistent-env 2>&1; echo "EXIT:$?"
# Expected: non-zero exit, message contains "not found"

# Invalid interaction command — should fail with suggestion
dva run nonexistent-command 2>&1; echo "EXIT:$?"
# Expected: non-zero exit
```

### 5-8. Collect Verification Results

Build verification result table:

```markdown
## CLI Verification Results

| # | Command | Expected | Actual | Status |
|---|---------|----------|--------|--------|
| 1 | dva validate | exit 0 | {actual} | PASS/FAIL |
| 2 | dva show | exit 0 | {actual} | PASS/FAIL |
| 3 | dva status | exit 0 | {actual} | PASS/FAIL |
| 4 | dva ls | exit 0 | {actual} | PASS/FAIL |
| 5 | dva up | exit 0 | {actual} | PASS/FAIL |
| 6 | dva up --mode {X} | exit 0 | {actual} | PASS/FAIL |
| 7 | dva up --env {X} | exit 0 | {actual} | PASS/FAIL |
| 8 | dva up --mode {X} --env {Y} | exit 0 | {actual} | PASS/FAIL |
| 9 | dva down | exit 0 | {actual} | PASS/FAIL |
| 10 | dva provision default | exit 0 | {actual} | PASS/FAIL |
| 11 | dva up --mode invalid | exit ≠0 | {actual} | PASS/FAIL |
| 12 | dva up --env invalid | exit ≠0 | {actual} | PASS/FAIL |
```

**Gate rule:** ALL items 1-10 must PASS. Items 11-12 must return non-zero exit (error case validation).

## Phase 6: Generate Execution Report

```markdown
# DVA Execution Report: {project-name}

## Launch Method
{dva up | go run ... up | docker compose up -d}

## Service Status
| Service | Status | Health | Port |
|---------|--------|--------|------|

## Connectivity
| Service | Port | Reachable |
|---------|------|-----------|

## CLI Verification
| # | Command | Status |
|---|---------|--------|

## Mode/Env Matrix
| Mode | Env | Result |
|------|-----|--------|

## Quick Commands
- Logs: `dva logs` or `docker compose logs -f`
- Stop: `dva down` or `docker compose down`
- Shell: `dva shell` or `docker compose exec {service} /bin/bash`
- Status: `dva status`

## Issues (if any)
{list of failed health checks, unreachable ports, or CLI verification failures}
```
</steps>

<constraints>
- If .env does not exist, copy from .env.example (never start without env)
- If port conflict detected, warn but do not change ports (user decision)
- Health check timeout: 60 seconds max
- If DVA CLI not found, try go run from DVA_ROOT, then fall back to docker compose
- Do not modify compose.yml or dva.yml in this stage
</constraints>

<gate>
- [ ] Infrastructure started (dva up or fallback)
- [ ] All core services in "running" or "healthy" state
- [ ] Exposed ports are reachable (nc -z test)
- [ ] `dva validate` passes (exit 0)
- [ ] `dva show` displays all profiles, environments, commands correctly
- [ ] `dva status` runs without error
- [ ] `dva ls` lists all interaction commands
- [ ] `dva up --mode X` works for each defined profile (or dry-run passes)
- [ ] `dva up --env X` works for each defined environment (or dry-run passes)
- [ ] `dva provision default` completes (if provision profiles defined)
- [ ] Invalid `--mode`/`--env` names produce clear error messages (non-zero exit)
- [ ] Execution report generated with CLI verification results
- [ ] No services in "restarting" or "exited" state
</gate>

<output>
| Artifact | Path |
|----------|------|
| Execution Report | `tmp/setup-dva/40-execution-report-{project-name}.md` |
</output>

<trigger>Pre-flight → start infrastructure → verify health → report.</trigger>
