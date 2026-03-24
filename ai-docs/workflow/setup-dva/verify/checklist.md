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

### DVA Config (Static)
- [ ] `dva.yml` version field present
- [ ] `compose.files` lists correct compose files
- [ ] `interaction` entries match running services
- [ ] If `profiles:` defined, each profile references valid compose_profiles/compose_services/health_checks
- [ ] If `environments:` defined, each environment has description and environment map
- [ ] If `provision:` defined, each profile has valid step entries

### DVA CLI — Core Commands (실행 검증)
- [ ] `dva validate` exits 0
- [ ] `dva show` exits 0 and displays config summary
- [ ] `dva show --json` exits 0 and returns valid JSON
- [ ] `dva status` exits 0
- [ ] `dva status --json` exits 0 and returns valid JSON
- [ ] `dva ls` exits 0 and lists interaction commands

### DVA CLI — Lifecycle Commands
- [ ] `dva up` succeeds (containers start in detached mode)
- [ ] `dva down` succeeds (containers stop and remove)
- [ ] `dva up --force` succeeds (force restart)
- [ ] `dva up --no-wait` succeeds (immediate return)

### DVA CLI — Mode Flag (--mode/-M)
- [ ] For each profile in `profiles:`: `dva up --mode {name} --no-wait` runs without error
- [ ] For each profile: `dva down -M {name}` tears down correctly
- [ ] Native mode (`compose_services: []`): compose is skipped, health_checks run
- [ ] Docker mode (`compose_profiles: [...]`): correct `--profile` flags passed to compose
- [ ] Hybrid mode (`compose_services: [svc1, svc2]`): only listed services start
- [ ] Invalid mode name: exits non-zero with "not found" and available list

### DVA CLI — Env Flag (--env/-E)
- [ ] For each environment in `environments:`: `dva up --env {name} --no-wait` runs without error
- [ ] For each environment: `dva up -E {name} --no-wait` (short flag) runs without error
- [ ] Environment vars from profile are merged into compose context
- [ ] Invalid env name: exits non-zero with "not found" and available list

### DVA CLI — Combined Flags
- [ ] `dva up --mode {X} --env {Y}` works for at least one valid combination
- [ ] Mode environment vars + env environment vars merge correctly (env takes precedence if overlap)

### DVA CLI — Provision Profiles
- [ ] `dva provision default` completes (if `provision.default:` exists)
- [ ] `dva provision {other-profile}` completes for each defined profile
- [ ] Invalid provision profile: exits non-zero with "not found" and available list

### DVA CLI — Interaction Commands
- [ ] `dva run {cmd} --dry-run` works for each top-level interaction command
- [ ] `dva {cmd}` dynamic routing works for interaction commands (prefix 'run' omitted)
- [ ] Invalid command name: exits non-zero with suggestion

### Infrastructure
- [ ] All containers in running/healthy state
- [ ] Exposed ports reachable (nc -z localhost PORT)
- [ ] Logs show no critical errors (`docker compose logs --tail 20`)

### Rollback
- [ ] Backup exists at `tmp/setup-dva/backup-*/`
- [ ] Transform log documents all changes made
