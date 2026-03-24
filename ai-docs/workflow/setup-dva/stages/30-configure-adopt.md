# Stage 30: Configure (Adopt)

<role>Existing compose environment wrap configurator</role>

<objective>
Generate the primary `dva.yml` mapping seamlessly to the TARGET project's existing Docker Compose infrastructure without rewriting their existing manifests.
</objective>

<steps>
1. Load `tmp/setup-dva/10-proposal-approved.yaml` with `setup_track: adopt`.
2. Process the project's existing docker compose files mapped in `00-analysis-report.yaml`.
3. Do NOT modify the core structures of the user's legacy `docker-compose.yml` natively.
4. Generate the `dva.yml` wrapper at the root directory linking to the existing docker-compose structure using the `compose: files: [...]` directive.
   - **CRITICAL:** Group the existing services logically by workgroup (e.g., Frontend, Backend) with clear dividing comments.
   - Apply DVA features onto existing services by adding `tags: [app, ui]` or `tags: [infra, db]`, `related: [dependency-name]`, and `hint` descriptions so specific workgroups can be started dynamically.
5. Create DVA native custom interactions (e.g., `deps`, `lint`, `sh`) mapped to existing application services.
6. Verify output fields map tightly against the parameters shown in `internal/config/schema.json`.
</steps>

<output>
- Ensure `dva.yml` wrappers are written matching user inputs.
- `tmp/setup-dva/30-configuration-log.txt` (List of adopt track config operations executed by the subagent)
</output>

<gate>
- [ ] Existing `docker-compose` ecosystem is fully wrapped without destructive rewrites.
- [ ] Resulting `dva.yml` implements proper custom commands against existing services.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/30-configuration-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
