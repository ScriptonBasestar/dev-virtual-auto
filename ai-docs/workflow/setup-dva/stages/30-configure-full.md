# Stage 30: Configure (Full)

<role>Complete DVA environment generator</role>

<objective>
Generate a comprehensive set of clean infra files, including new `compose.yaml` (or multiple compose files), and the main `dva.yml` wrapper in a fresh robust way.
</objective>

<steps>
1. Evaluate `tmp/setup-dva/10-proposal-approved.yaml` configuring `setup_track` as `full`.
2. Generate all the necessary Docker Compose files within `infra/` or at the root, mapping services to docker compose `profiles` where appropriate.
3. Generate the primary `dva.yml` at the project root.
   - **CRITICAL:** Organize services by workgroup (e.g., Frontend, Backend, Workers), separating them with clear comments like `# --- Frontend Application Services ---`
   - Assign rich metadata to each service using `tags: [app, ui]` or `tags: [infra, db]`, `related: [dependent-service-name]`, and `hint` fields to ensure teams can start specific stacks easily.
   - Embed custom interactions (`app`, `deps`, `start`, etc.).
4. Initialize a `.devcontainer` configuration block inside `dva.yml` (e.g. `devcontainer: enabled: true`) per proposal standards.
5. Create a standard `.env.example` detailing all missing environment variables.
6. Make sure to reference DVA's `internal/config/schema.json` to avoid syntax errors with `dva.yml`.
</steps>

<output>
- Ensure `dva.yml` and newly generated docker compose files exist on the host.
- `tmp/setup-dva/30-configuration-log.txt` (List of full track infra files written by the subagent)
</output>

<gate>
- [ ] Primary `dva.yml` matches project structural requirements.
- [ ] Handlers/commands in `dva.yml` point to correct files.
- [ ] No JSON/YAML syntax errors exist in the output files.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
