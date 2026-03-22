# setup-dva Verification Checklist

## Post-Pipeline Verification

### Structure
- [ ] Target project has `compose.yml` (not docker-compose.yml)
- [ ] No `version:` key in compose files (Compose Specification)
- [ ] `dva.yml` exists and matches compose services
- [ ] `.env.example` exists with all required vars
- [ ] `.env` exists (copied from .env.example)

### Compose Quality
- [ ] All core services have healthchecks
- [ ] Env vars use `${VAR:-default}` fallback pattern
- [ ] No common default ports used as host ports (5432, 6379, 8080, 8000, 3000, 3306, 27017)
- [ ] Port assignments do not conflict with other running projects

### DVA Config
- [ ] `dva.yml` version field present
- [ ] `compose.files` lists correct compose files
- [ ] `interaction` entries match running services
- [ ] `dva validate` passes (if DVA CLI available)
- [ ] `dva up` or `docker compose up -d` succeeds

### Infrastructure
- [ ] All containers in running/healthy state
- [ ] Exposed ports reachable (nc -z localhost PORT)
- [ ] Logs show no critical errors (`docker compose logs --tail 20`)

### Rollback
- [ ] Backup exists at `tmp/setup-dva/backup-*/`
- [ ] Transform log documents all changes made
