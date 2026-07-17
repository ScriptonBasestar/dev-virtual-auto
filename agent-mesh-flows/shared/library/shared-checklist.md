# DVA Configuration Self-Review Checklist (Shared)

> Single source of truth for dva.yml validation checklist.
> Referenced by: improve prompt (auto mode), guided improve pipeline gates (interactive mode).

## Mandatory Checks (before finalizing dva.yml)

### Version & Header
- [ ] `version` field matches current DVA CLI version
- [ ] `yaml-language-server: $schema=...` comment on first line

### Structure
- [ ] Section order follows canonical: version → vars → environment → env_file → stack → plans → environments → sites → health_checks → interaction → provision → modules → subprojects → endpoints → infra → ssh → devcontainer
- [ ] `env_file:` uses object format (`files:` array; optional top-level `required:`)
- [ ] `stack:` section present (no legacy `compose:` root-level)
- [ ] New/rewrite config has at least one named `plans:` entry
- [ ] Custom `checks:` do not duplicate built-in doctor checks
- [ ] `provision:` exists only when setup/initialization work is required

### Stack & Compose
- [ ] `stack.{entry}.runners.compose.tags: [infra]` present on primary compose runner
- [ ] All files in `stack.{entry}.runners.compose.files` actually exist
- [ ] Multi-stack entries do not redundantly list same base compose file
- [ ] All services have `tags:`
- [ ] Compose file has top-level `name:` matching `stack.{entry}.runners.compose.project_name`

### Ports & Endpoints
- [ ] All host ports are project-unique (no common defaults: 5432, 6379, 3000, 8080, etc.)
- [ ] User-facing ports declared in `endpoints:` section (not `services.ports`)

### Health Checks
- [ ] All `health_checks` have `start:` and/or `start_hint:` (not both with identical values)
- [ ] Health check URLs use literal values (no `${VAR:-DEFAULT}`)

### Lifecycle ownership
- [ ] Each Compose service has one lifecycle owner: compose stack + plan
- [ ] No matching `applications.*.run.docker.service` for a Compose-owned service
- [ ] No standalone docker runner generated for an existing Compose service
- [ ] Native app processes use native/process runners selected by plans
- [ ] `plans.entries[].depends_on` has no circular references

### Interaction Commands
- [ ] Host build commands use `runner: local`
- [ ] Reserved commands use `replace:` hooks
- [ ] No reserved DVA command names as plain interaction keys
- [ ] No echo wrapper commands

### Provision
- [ ] No `run: "dva <command>"` calls in provision steps
- [ ] No raw compose/docker lifecycle command duplicates a named plan
- [ ] No synthetic default/full/reset profile without setup evidence

### Subprojects (if applicable)
- [ ] Imported subproject `version` matches root
- [ ] Subprojects use `exclude_tags: [infra]`
- [ ] No `description:` field in subprojects (only `path`, `exclude_tags`, `import`)
- [ ] Every imported subproject has its own `dva.yml`; placeholders without import entries (`import` omitted or `import: {}`) may be initialized later

### Plans & runner strategy
- [ ] Plans select only declared runners
- [ ] Compose service subsets use `plans.entries[].services`
- [ ] Environments distinguish dev/stg/prd variables
- [ ] Sites distinguish local/remote host differences

### Final Validation
- [ ] `dva config validate` exits with ERROR 0
