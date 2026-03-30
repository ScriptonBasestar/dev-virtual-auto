# Stage 10: Verify

<role>User interaction handler and proposal auditor</role>

<objective>
Present the proposed changes and structure (based on the analysis) to the user for explicit approval before any modifications are made. THIS IS THE ONLY USER GATE.
</objective>

<steps>
1. Read `tmp/improve-guided/00-analysis-report.yaml`.
2. Generate a clear markdown proposal outlining what files will be created or modified, the structure of the DVA configuration, the setup track, and infra services.
   - **Crucial:** Highlight how services will be categorized into distinct working groups (e.g., frontend, backend, workers) using `tags` and `related` fields.
3. Present the generated proposal directly to the user.
4. **CRITICAL:** Prompt the user with "Do you approve these changes? (Yes/No/Modify)".
5. **WAIT** for the user to explicitly approve or reject. Do not proceed independently.
6. If the user provides modifications, adapt the proposal and ask again.
7. Upon approval, save the finalized blueprint to `10-proposal-approved.yaml`.
</steps>

<output>
- `tmp/improve-guided/10-proposal-approved.yaml` containing the approved blueprint.
</output>

<gate>
- [ ] User was explicitly asked for permission before proceeding.
- [ ] User gave clear approval for the final provided proposal.
- [ ] The completed proposal document is saved.
</gate>

<return>
{ "artifacts": ["tmp/improve-guided/10-proposal-approved.yaml"], "gate": "PASS|FAIL", "summary": "..." }
</return>
