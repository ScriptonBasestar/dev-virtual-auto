# Stage 40: Execute

<role>Configuration execution and verification runner</role>

<objective>
Ensure the newly constructed/adopted DVA environment validates structurally and can boot up correctly. Verify that the CLI hooks load optimally.
</objective>

<steps>
1. Check DVA CLI availability: run `which dva || command -v dva` to determine if the binary is installed.
2. **If DVA CLI is available:**
   a. Run `dva validate` in TARGET to assert `dva.yml` syntax parses correctly.
   b. Unless `--skip-execute` was passed, run `dva up --no-wait` (or `dva up --mode {mode} --no-wait` if `--mode` was provided).
   c. Run `dva stack status` to confirm stack entries reached healthy/running state.
   d. If `applications:` section exists in dva.yml, run `dva app ls` to verify application configuration is valid (does not start apps — only lists declared apps).
3. **If DVA CLI is NOT available (fallback):**
   a. Log: "DVA CLI not found — falling back to docker compose"
   b. Run `docker compose config` to validate compose syntax.
   c. Unless `--skip-execute` was passed, run `docker compose up -d`.
   d. Run `docker compose ps` to confirm containers started.
4. If `--mode` or `--env` flags were provided during the pipeline run, verify the environment parameters are applied:
   - `--mode`: confirm the mode-specific services/profiles are active
   - `--env`: confirm environment vars from the named environment are in effect
5. Collect all CLI output into `tmp/improve-guided/40-execution-report.txt`.
</steps>

<output>
- `tmp/improve-guided/40-execution-report.txt` — CLI logs from validation and startup (dva or docker compose)
</output>

<gate>
- [ ] DVA CLI availability was checked and the correct execution path taken.
- [ ] Configuration validation passed (`dva validate` or `docker compose config`).
- [ ] Containers started successfully (unless `--skip-execute` was passed).
- [ ] If `applications:` exists, `dva app ls` output is included in report.
- [ ] Execution report artifact exists with CLI output.
</gate>

<return>
{ "artifacts": ["tmp/improve-guided/40-execution-report.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
