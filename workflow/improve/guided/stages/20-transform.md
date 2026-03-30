# Stage 20: Transform

<role>Directory and structure transformer</role>

<objective>
Modify the TARGET project's directory structure safely according to the approved proposal, preparing it for the final DVA configuration files.
</objective>

<steps>
1. Read the finalized blueprint from `tmp/improve-guided/10-proposal-approved.yaml`.
2. Create necessary infrastructure directories outlined in the proposal (e.g. `.sb/dva/`, `infra/`, `.devcontainer/`).
3. If necessary, relocate colliding files safely (with `.bak` extensions).
4. If `setup_track` is `full`, prepare empty scaffolding files as required by the blueprint.
5. Log all applied directory/file mutations to `20-transform-log.txt`.
</steps>

<output>
- `tmp/improve-guided/20-transform-log.txt` (Log file listing applied directory/file movements/creations)
</output>

<gate>
- [ ] All directories defined in the blueprint were successfully created.
- [ ] Any file movements were completed without errors or data loss.
- [ ] Transform log artifact is successfully written.
</gate>

<return>
{ "artifacts": ["tmp/improve-guided/20-transform-log.txt"], "gate": "PASS|FAIL", "summary": "..." }
</return>
