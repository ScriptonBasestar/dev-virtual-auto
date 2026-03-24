# Stage 40: Execute

<role>Configuration execution and verification runner</role>

<objective>
Ensure the newly constructed/adopted DVA environment validates structurally and can boot up correctly. Verify that the CLI hooks load optimally.
</objective>

<steps>
1. Launch the environment config check via `dva validate` down in the TARGET. This will assert that `dva.yml` syntax parses under built-in strictness.
2. If DVA CLI check succeeds but `--skip-execute` was absent, fallback to executing a normal dry-run or syntax ping: `docker compose config` against the files named in the config.
3. Assert that the validation mechanisms pass without yielding fatal syntax crashes.
4. Extract execution metrics and CLI validation logs, appending them into a final report. 
5. Ensure if the user utilized `--mode` or `--env` flags during pipeline run, the environment applies these parameters successfully.
</steps>

<output>
- `tmp/setup-dva/40-execution-report.txt` (Output snippet of CLI logs and verification processes checks)
</output>

<gate>
- [ ] `dva validate` completed properly without throwing parsing anomalies.
- [ ] Environment syntax tests passed (i.e., `docker compose config`).
- [ ] The CLI validation outputs exist in the execution report.
</gate>

<return>
{ "artifacts": ["tmp/setup-dva/40-execution-report.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
