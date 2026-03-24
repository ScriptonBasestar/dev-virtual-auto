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
3. Check for existing `dva.yml` — if found, note version and whether it uses deprecated `profiles:` key.
4. Identify existing `Dockerfile`s and the software technology stack (languages, frameworks, dependencies).
5. Determine track:
   - If the project has an extensive, well-maintained docker compose setup: set `setup_track: adopt`.
   - If no valid docker compose file exists, or the user requests a fresh robust environment: set `setup_track: full`.
6. Analyze and identify logically distinct service groups / working groups (e.g., frontend, backend, workers, infrastructure). This is critical for assigning `tags` and `modes` later.
   - Map each detected service to standard tags from `library/naming-presets.md` (infra, api, worker, ui, data, monitoring, build).
   - Determine the project archetype (web app / service-daemon / microservices / simple app) to guide mode selection.
7. Compile findings into `00-analysis-report.yaml`.
</steps>

<output>
- `tmp/setup-dva/00-analysis-report.yaml` containing:
  - `setup_track`: full | adopt
  - `stack`: [list of detected tech]
  - `service_groups`: [list of distinct groups, e.g. ui, backend, worker, db]
  - `project_archetype`: web-app | service-daemon | microservices | simple-app
  - `recommended_tags`: { service_name: [tag1, tag2] }
  - `recommended_modes`: [list of mode names from naming-presets.md]
  - `existing_compose_files`: [paths relative to TARGET]
  - `compose_issues`: [list of issues found: missing_name, has_version_key, default_ports, missing_healthchecks]
  - `existing_dva`: { found: bool, version: string, has_deprecated_profiles: bool }
</output>

<gate>
- [ ] Analysis report artifact is properly formatted and saved.
- [ ] `setup_track` field is explicitly defined as `full` or `adopt`.
- [ ] `compose_issues` field lists all detected issues (may be empty).
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/00-analysis-report.yaml"], "gate": "PASS|FAIL", "summary": "..." }
</return>
