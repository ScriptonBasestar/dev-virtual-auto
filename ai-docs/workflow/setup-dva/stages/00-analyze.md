# Stage 00: Analyze

<role>Target project configuration and structure analyzer</role>

<objective>
Scan the user's TARGET project to identify existing configurations, docker usage, technology stack, and directory structure. Determine the appropriate setup track ("full" or "adopt").
</objective>

<steps>
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
   - Interaction anti-patterns: `echo 'Run: ...'` wrappers (should use `runner: local`) — **count and list affected command names**
   - Missing `runner: local` on host commands (build, test, lint, fmt, check)
   - Prefixed command workarounds: `app-build`, `app-clean` (should migrate to `build`/`clean` with `replace:` hooks)
   - Non-standard fields: `host_command`, `compose_up`, `compose_logs` in interaction (see `library/dva-schema.md` Non-Standard Field Migration)
   - Non-standard provision fields: `compose_up`, `compose_exec` in provision steps
   - Old `env_file` format: string `".env"` instead of object with `files` array
   - Missing sections: `modes:`, `environments:`, `checks:`, `env_file:`, `health_checks:`
   - Subproject dva.yml files (check same issues recursively, **flag version mismatches** with root)
   - **Port metadata validation** — For each service in compose files, extract the default host port from `${VAR:-DEFAULT}` patterns. Cross-reference against dva.yml `stack.compose.services.{name}.ports` entries. Flag any discrepancies.
   - **Development pattern detection** — Determine if the project is container-first (app runs in Docker, e.g., Django) or hybrid (infra in Docker, app runs natively, e.g., Rust/Go). This affects whether interaction commands should use `service:` or `runner: local`.
4. **Classify compose overlay files** — For all `compose*.yml` and `docker-compose*.yml` in TARGET root:
   - Parse service names from each file.
   - Identify mutually exclusive overlays: files that redefine the same service names with incompatible configurations (e.g., `compose.redis-sentinel.yml` vs `compose.redis-cluster.yml`).
   - Classify into `primary_compose_files` (safe to combine) and `mutually_exclusive_overlays` (groups of conflicting files).
5. Identify existing `Dockerfile`s and the software technology stack (languages, frameworks, dependencies).
6. **CRITICAL: Read package manifests to extract EXACT package names.**
   - **Rust**: Read `Cargo.toml` (workspace root) → extract `[workspace] members` list, then read each member's `Cargo.toml` for `[package] name = "..."`. These EXACT names are used in `cargo run -p {name}` for health_check `start` commands. Common pitfall: directory names may differ from package names (e.g., directory `db-orchestrator-api-rs` but package name `db-orchestrator-api`). **Always use the `[package] name` value, NOT the directory name.**
   - **Go**: Read `go.mod` → extract module path. Read `cmd/` directory for binary names.
   - **Node.js**: Read `package.json` → extract `name`, `scripts` (for interaction commands).
   - **Python**: Read `pyproject.toml` or `setup.py` → extract project name.
   - Record these in `package_names` field of the analysis report.
7. Determine track:
   - If existing `dva.yml` found with deprecated patterns: set `setup_track: upgrade` (preserve structure, upgrade format).
   - If the project has an extensive, well-maintained docker compose setup but no dva.yml: set `setup_track: adopt`.
   - If no valid docker compose file exists, or the user requests a fresh robust environment: set `setup_track: full`.
7. Analyze and identify logically distinct service groups / working groups (e.g., frontend, backend, workers, infrastructure). This is critical for assigning `tags` and `modes` later.
   - Map each detected service to standard tags from `library/naming-presets.md` (infra, api, worker, ui, data, monitoring, build).
   - Determine the project archetype (web app / service-daemon / microservices / simple app) to guide mode selection.
8. Compile findings into `00-analysis-report.yaml`.
</steps>

<output>
- `tmp/setup-dva/00-analysis-report.yaml` containing:
  - `setup_track`: full | adopt | upgrade
  - `stack`: [list of detected tech]
  - `service_groups`: [list of distinct groups, e.g. ui, backend, worker, db]
  - `project_archetype`: web-app | service-daemon | microservices | simple-app
  - `recommended_tags`: { service_name: [tag1, tag2] }
  - `recommended_modes`: [list of mode names from naming-presets.md]
  - `existing_compose_files`: [paths relative to TARGET]
  - `compose_issues`: [list of issues found: missing_name, has_version_key, default_ports, missing_healthchecks]
  - `development_pattern`: container-first | hybrid | native
  - `all_compose_files`: [all compose*.yml and docker-compose*.yml found in TARGET root]
  - `mutually_exclusive_overlays`: [groups of files that redefine the same services and cannot be combined, e.g. [[compose.redis-sentinel.yml, compose.redis-cluster.yml]]]
  - `primary_compose_files`: [compose files safe to combine in main stack — excludes mutually exclusive overlays]
  - `port_discrepancies`: [{ service: name, dva_port: N, compose_default: M, compose_var: "VAR_NAME" }]
  - `package_names`:                   # EXACT names from package manifests
      # Rust: { workspace_root: "path", members: [{ name: "exact-pkg-name", path: "relative" }] }
      # Go: { module: "module/path", binaries: ["bin1", "bin2"] }
      # Node: { name: "pkg-name", scripts: { ... } }
  - `existing_dva`:
      found: bool
      version: string                  # e.g. "0.1.0"
      deprecated_patterns:             # list of detected issues
        # profiles | top_level_compose | lifecycle | echo_wrappers
        # missing_runner_local | non_standard_fields | old_env_file_format
        # missing_modes | missing_environments | missing_checks | prefixed_commands
      echo_wrapper_commands:           # [{ name: cmd, count: N }]
      missing_sections:                # [modes|environments|checks|env_file|health_checks]
      non_standard_fields:             # [list of field names found]
      subproject_dvas:                 # [{ path, version, version_mismatch, deprecated_patterns }]
</output>

<gate>
- [ ] Analysis report artifact is properly formatted and saved.
- [ ] `setup_track` field is explicitly defined as `full`, `adopt`, or `upgrade`.
- [ ] `compose_issues` field lists all detected issues (may be empty).
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/00-analysis-report.yaml"], "gate": "PASS|FAIL", "summary": "..." }
</return>
