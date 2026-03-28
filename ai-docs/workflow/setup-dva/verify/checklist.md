# setup-dva Verification Checklist

## Post-Pipeline Verification

### Structure
- [ ] Target project has `compose.yml` (not docker-compose.yml)
- [ ] No `version:` key in compose files (Compose Specification)
- [ ] `compose.yml` has top-level `name:` field matching `dva.yml compose.project_name`
- [ ] `dva.yml` exists and matches compose services
- [ ] `.env.example` exists with all required vars
- [ ] `.env` exists (copied from .env.example)

### Compose Quality
- [ ] All core services have healthchecks
- [ ] Env vars use `${VAR:-default}` fallback pattern
- [ ] No common default ports used as host ports (5432, 6379, 8080, 8000, 3000, 3306, 27017)
- [ ] Port assignments do not conflict with other running projects

### DVA Config (Static)
- [ ] `dva.yml` version is `"0.1.26"` (current)
- [ ] `dva.yml` uses `stack:` section (NOT top-level `compose:` or `lifecycle:`)
- [ ] `dva.yml` uses `modes:` key (NOT deprecated `profiles:`)
- [ ] `stack.compose.files` lists correct compose files
- [ ] `stack.compose.project_name` matches compose file `name:`
- [ ] `interaction` entries match running services
- [ ] Host commands use `runner: local` (no `echo 'Run: ...'` wrappers)
- [ ] Reserved commands (build, clean) use `replace:` hooks if overridden
- [ ] No non-standard fields remain (`host_command`, `compose_up`/`compose_logs` in interaction)
- [ ] `env_file:` uses object format (not string)
- [ ] Provision steps do NOT call `run: "dva <command>"` (bootstrap ordering risk)
- [ ] Provision steps do NOT hardcode compose file paths that duplicate `stack.compose.files`
- [ ] If `modes:` defined, each mode references valid compose_profiles/compose_services/health_checks/stack
- [ ] If `environments:` defined, each environment has description and environment map

### Naming Presets Compliance (`library/naming-presets.md`)
- [ ] Service tags use standard names (infra, api, worker, ui, data, monitoring, build)
- [ ] Mode names follow preset conventions (`infra` is always present as base mode)
- [ ] `backend` and `server` are not both used in the same project
- [ ] Env names use standard names (dev, test, stg, prd) where applicable
- [ ] If `provision:` defined, each profile has valid step entries
- [ ] If `health_checks:` defined, each check has both `start` and `start_hint` (for native services)
- [ ] If `health_checks:` defined, `start` command uses EXACT `[package] name` from Cargo.toml (NOT directory name — e.g., directory `db-orchestrator-api-rs` but package name may be `db-orchestrator-api`)
- [ ] If `subprojects:` defined, each subproject `dva.yml` version matches root

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
- [ ] For each mode in `modes:`: `dva up --mode {name} --no-wait` runs without error
- [ ] For each mode: `dva down -M {name}` tears down correctly
- [ ] Native mode (`compose_services: []`): compose is skipped, health_checks run
- [ ] Docker mode (`compose_profiles: [...]`): correct `--profile` flags passed to compose
- [ ] Hybrid mode (`compose_services: [svc1, svc2]`): only listed services start
- [ ] Invalid mode name: exits non-zero with "not found" and available list

### DVA CLI — Env Flag (--env/-E)
- [ ] For each environment in `environments:`: `dva up --env {name} --no-wait` runs without error
- [ ] For each environment: `dva up -E {name} --no-wait` (short flag) runs without error
- [ ] Environment vars from mode are merged into compose context
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

### Pipeline Flags

#### --resume (cache-based resume)
- [ ] `tmp/setup-dva/state.yaml` exists after at least one stage completes
- [ ] Re-running the pipeline with `--resume` skips stages where `gate: PASS` is recorded
- [ ] Re-running without `--resume` re-executes all stages from the beginning
- [ ] Interrupted pipeline resumes from the last incomplete stage (not from stage 00)

#### --dry-run (analysis only)
- [ ] `--dry-run` runs only stage 00 (Analyze) and exits after producing `00-analysis-report.yaml`
- [ ] No files are created or modified in TARGET when `--dry-run` is used (stages 20/30 are skipped)
- [ ] `00-analysis-report.yaml` is generated with correct `setup_track` and `stack` fields
- [ ] Pipeline reports "dry-run complete — no mutations applied" in the final summary

#### DVA CLI Fallback
- [ ] If `dva` binary is not installed, stage 40 falls back to `docker compose up -d`
- [ ] Fallback is logged explicitly ("DVA CLI not found — falling back to docker compose")
- [ ] `docker compose config` validates successfully before `docker compose up -d` is invoked
- [ ] Fallback containers appear in `docker compose ps` output

### Rollback
- [ ] Backup exists at `tmp/setup-dva/backup-*/`
- [ ] Transform log documents all changes made
