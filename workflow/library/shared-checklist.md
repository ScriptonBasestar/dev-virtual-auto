# DVA Configuration Self-Review Checklist (Shared)

> Single source of truth for dva.yml validation checklist.
> Referenced by: improve prompt (auto mode), guided improve pipeline gates (interactive mode).

## Mandatory Checks (before finalizing dva.yml)

### Version & Header
- [ ] `version` field matches current DVA CLI version
- [ ] `yaml-language-server: $schema=...` comment on first line

### Structure
- [ ] Section order follows canonical: version → environment → env_file → stack → checks → applications → default_mode → modes → environments → health_checks → interaction → provision → subprojects → endpoints
- [ ] `env_file:` uses object format (`files:` array + `interpolate: true`)
- [ ] `stack:` section present (no legacy `compose:` root-level)
- [ ] `default_mode` set and points to minimal infra mode
- [ ] `modes:` has at least `infra` mode
- [ ] `checks:` has `docker_socket` check
- [ ] `provision:` has `default` and `reset` profiles

### Stack & Compose
- [ ] `stack.compose.tags: [infra]` present on primary stack entry
- [ ] All files in `stack.{entry}.files` actually exist
- [ ] Multi-stack entries do not redundantly list same base compose file
- [ ] All services have `tags:`
- [ ] Compose file has top-level `name:` matching `stack.compose.project_name`

### Ports & Endpoints
- [ ] All host ports are project-unique (no common defaults: 5432, 6379, 3000, 8080, etc.)
- [ ] User-facing ports declared in `endpoints:` section (not `services.ports`)

### Health Checks
- [ ] All `health_checks` have `start:` and/or `start_hint:` (not both with identical values)
- [ ] Health check URLs use literal values (no `${VAR:-DEFAULT}`)

### Applications (if project has long-running dev servers)
- [ ] App servers (API, workers, web) declared in `applications:` section
- [ ] Each app has at least `run:` or `dev:` exec path defined
- [ ] Apps with HTTP endpoints have `health:` block
- [ ] `depends_on` reflects startup dependencies (e.g., worker depends on api)

### Interaction Commands
- [ ] Host build commands use `runner: local`
- [ ] Reserved commands use `replace:` hooks
- [ ] No reserved DVA command names as plain interaction keys
- [ ] No echo wrapper commands

### Provision
- [ ] No `run: "dva <command>"` calls in provision steps

### Subprojects (if applicable)
- [ ] Subproject `version` matches root
- [ ] Subprojects use `exclude_tags: [infra]`
- [ ] No `description:` field in subprojects (only `path`, `exclude_tags`)

### Final Validation
- [ ] `dva config validate` exits with ERROR 0
