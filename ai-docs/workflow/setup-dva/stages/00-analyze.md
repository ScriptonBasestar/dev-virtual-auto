# Stage 00: Analyze

<role>Target project configuration and structure analyzer</role>

<objective>
Scan the user's TARGET project to identify existing configurations, docker usage, technology stack, and directory structure. Determine the appropriate setup track ("full", "adopt", or "upgrade"). Also determine if DVA is even needed for this project.
</objective>

<steps>
0. **DVA Necessity Check** — Before full analysis, check if DVA is needed:
   - If no compose files exist AND no infrastructure dependencies (DB, cache, MQ) detected AND no Dockerfile present → set `dva_needed: false` with reason and STOP.
   - If project is purely ops/docs/policy with no buildable code → set `dva_needed: false` and STOP.
   - Otherwise proceed with full analysis.

1. Identify existing Docker Compose files (`docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, `compose.yaml`) in TARGET.
2. If compose files found, check for:
   - Top-level `name:` field (required by DVA — flag as missing if absent).
   - Deprecated `version:` key (flag for removal).
   - Service healthcheck coverage (flag services without `healthcheck:`).
   - Host port range analysis (flag common default ports like 5432, 6379, 3000, 8080).
3. Check for existing `dva.yml` — if found, analyze for:
   - Version (compare against current `0.1.26`)
   - Deprecated `profiles:` key (should be `modes:`)
   - Deprecated top-level `compose:` (should be under `stack:`)
   - Deprecated `lifecycle:` key (should be `stack:`)
   - Interaction anti-patterns: `echo 'Run: ...'` wrappers — **count and list affected command names**
   - Missing `runner: local` on host commands (build, test, lint, fmt, check)
   - Prefixed command workarounds: `app-build`, `app-clean` (should migrate to `build`/`clean` with `replace:` hooks)
   - Non-standard fields: `host_command`, `compose_up`, `compose_logs` in interaction
   - Missing sections: `modes:`, `environments:`, `checks:`, `env_file:`, `health_checks:`
   - Missing `start_hint` in health_checks (both `start` and `start_hint` required)
   - Subproject dva.yml files (**flag version mismatches** with root)
   - **Port metadata validation** — Extract default host port from `${VAR:-DEFAULT}` patterns. Cross-reference against dva.yml ports entries. Flag discrepancies.
   - **Development pattern detection** — container-first (Django, Rails) vs hybrid (Rust, Go) vs native (pure library)
4. **Classify compose overlay files** — For all compose*.yml:
   - Parse service names from each file.
   - Identify mutually exclusive overlays (files redefining same services).
   - Classify into `primary_compose_files` and `mutually_exclusive_overlays`.
5. Identify Dockerfiles and technology stack (languages, frameworks).
6. **CRITICAL: Read package manifests to extract EXACT package names.**
   - **Rust**: Read `Cargo.toml` workspace → extract members, then each `[package] name`. **Use `[package] name`, NOT directory name.**
   - **Go**: Read `go.mod` → module path. Read `cmd/` for binaries.
   - **Node.js**: Read `package.json` → `name`, `scripts`.
   - **Python**: Read `pyproject.toml` or `setup.py` → project name.
   - Record in `package_names` field.
7. Determine track:
   - Existing dva.yml with deprecated patterns: `setup_track: upgrade`
   - Extensive compose but no dva.yml: `setup_track: adopt`
   - No valid compose or user requests fresh: `setup_track: full`
8. Analyze service groups — map to standard tags from naming-presets:
   - Tags: infra, api, worker, ui, data, monitoring, build
   - Determine project archetype: web-app | service-daemon | microservices | simple-app
   - Select recommended modes based on archetype
9. **Naming compliance pre-check** — Verify existing dva.yml (if any) against naming presets:
   - Are mode names from the standard set?
   - Are service tags from the standard set?
   - Flag non-standard names for migration
10. Compile findings into `00-analysis-report.yaml`.
</steps>

<output>
- `tmp/setup-dva/00-analysis-report.yaml` containing:
  - `dva_needed`: bool (false → pipeline stops here)
  - `dva_not_needed_reason`: string (if dva_needed is false)
  - `setup_track`: full | adopt | upgrade
  - `stack`: [list of detected tech]
  - `service_groups`: [list of distinct groups]
  - `project_archetype`: web-app | service-daemon | microservices | simple-app
  - `development_pattern`: container-first | hybrid | native
  - `recommended_tags`: { service_name: [tag1, tag2] }
  - `recommended_modes`: [list of mode names]
  - `existing_compose_files`: [paths]
  - `compose_issues`: [list of issues]
  - `all_compose_files`: [all compose*.yml found]
  - `mutually_exclusive_overlays`: [groups]
  - `primary_compose_files`: [safe to combine]
  - `port_discrepancies`: [{ service, dva_port, compose_default }]
  - `naming_compliance`: { non_standard_modes: [], non_standard_tags: [] }
  - `package_names`: { ... }
  - `existing_dva`:
      found: bool
      version: string
      deprecated_patterns: [...]
      echo_wrapper_commands: [...]
      missing_sections: [...]
      missing_start_hint: [health_check_names]
      subproject_dvas: [{ path, version, version_mismatch }]
</output>

<gate>
- [ ] Analysis report artifact is properly formatted and saved.
- [ ] `dva_needed` field is explicitly defined.
- [ ] If `dva_needed: true`, `setup_track` field is explicitly defined.
- [ ] `compose_issues` field lists all detected issues (may be empty).
- [ ] Package names extracted from actual manifests (not guessed from directories).
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/00-analysis-report.yaml"], "gate": "PASS|FAIL", "summary": "..." }
</return>
