# Stage 00: Analyze

<role>Target project configuration and structure analyzer</role>

<objective>
Scan the user's TARGET project to identify existing configurations, docker usage, technology stack, and directory structure. Determine the appropriate setup track ("full" or "adopt").
</objective>

<steps>
1. Identify existing Docker Compose files (`docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, `compose.yaml`) in TARGET.
2. Identify existing `Dockerfile`s and the software technology stack (languages, frameworks, dependencies) used in the project.
3. Determine track:
   - If the project has an extensive, well-maintained docker compose setup: set `setup_track: adopt`.
   - If no valid docker compose file exists, or the user requests a fresh robust environment: set `setup_track: full`.
4. Analyze and identify logically distinct service groups / working groups (e.g., frontend, backend, workers, infrastructure). This is critical for assigning `tags` and `modes` later.
5. Compile findings into `00-analysis-report.yaml`.
</steps>

<output>
- `tmp/setup-dva/00-analysis-report.yaml` containing:
  - `setup_track`: full | adopt
  - `stack`: [list of detected tech]
  - `service_groups`: [list of distinct groups, e.g. ui, backend, worker, db]
  - `existing_compose_files`: [paths relative to TARGET]
</output>

<gate>
- [ ] Analysis report artifact is properly formatted and saved.
- [ ] `setup_track` field is explicitly defined as `full` or `adopt`.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/00-analysis-report.yaml"], "gate": "PASS|FAIL", "summary": "..." }
</return>
