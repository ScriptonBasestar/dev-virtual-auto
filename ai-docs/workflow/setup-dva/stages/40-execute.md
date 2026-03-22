<!-- v:2026-03-23 -->

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

## Phase 5: Generate Execution Report

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

## Quick Commands
- Logs: `dva logs` or `docker compose logs -f`
- Stop: `dva down` or `docker compose down`
- Shell: `dva shell` or `docker compose exec {service} /bin/bash`
- Status: `dva status`

## Issues (if any)
{list of failed health checks or unreachable ports}
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
- [ ] Execution report generated
- [ ] No services in "restarting" or "exited" state
</gate>

<output>
| Artifact | Path |
|----------|------|
| Execution Report | `tmp/setup-dva/40-execution-report-{project-name}.md` |
</output>

<trigger>Pre-flight → start infrastructure → verify health → report.</trigger>
