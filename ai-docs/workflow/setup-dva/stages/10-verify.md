<!-- v:2026-03-23 -->

<constants>
SELF = ai-docs/workflow/setup-dva/stages/10-verify.md
DVA_ROOT = {DVA project root}
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
EXAMPLES_DIR = examples/
</constants>

[PRESENT TO USER — WAIT FOR APPROVAL]

<role>DVA structure proposal agent — refine analysis into actionable proposal, present for user confirmation</role>

<input>
| Source | Description |
|--------|-------------|
| Analysis report | `tmp/setup-dva/00-analysis-{project-name}.md` from stage 00 |
| DVA examples | Reference configurations from EXAMPLES_DIR |
</input>

<objective>
Transform the analysis report into a concrete, reviewable DVA structure proposal.
Present to user with clear before/after comparison.
**This stage requires explicit user approval before pipeline continues.**
</objective>

<steps>
## Phase 1: Load Analysis

Read `tmp/setup-dva/00-analysis-*.md` from stage 00.
Extract: gap analysis items, current structure, detected patterns, recommended DVA template.

## Phase 2: Load DVA Template

Read the recommended DVA example from EXAMPLES_DIR.
Adapt the template to match the target project's services and structure.

## Phase 3: Build Proposal

For each gap item, determine specific action:

| Action Type | Description |
|-------------|-------------|
| RENAME | File rename (e.g., docker-compose.yml → compose.yml) |
| MOVE | Directory relocation |
| CREATE | New file generation (e.g., dva.yml, .env.example) |
| MODIFY | Content update (e.g., add healthcheck to compose service) |
| DELETE | Remove obsolete file |

## Phase 4: Generate Proposal Document

Create proposal at `tmp/setup-dva/10-proposal-{project-name}.md`:

```markdown
# DVA Structure Proposal: {project-name}

## Directory Layout (Before → After)

### Before
{tree of current structure}

### After
{tree of proposed structure}

## Change Plan

| # | Action | Source | Target | Reason |
|---|--------|--------|--------|--------|

## Compose Services Plan

| Service | Image | Host Port | Container Port | Healthcheck |
|---------|-------|-----------|----------------|-------------|

## DVA Configuration Preview

```yaml
version: "0.1.0"
compose:
  files:
    - compose.yml
interaction:
  ...
```

## Risk Assessment
- [ ] Data loss risk: {none|low|medium}
- [ ] Downtime required: {yes|no}
- [ ] Rollback plan: {description}
```

## Phase 5: Present to User

Display the proposal and ask:

```text
[setup-dva] DVA Structure Proposal for {project-name}

{proposal summary — key changes only}

Options:
  [approve]  — Proceed with transformation (stage 20)
  [modify]   — Request changes to the proposal
  [reject]   — Cancel DVA setup

Your choice:
```

**WAIT for user response. Do NOT proceed without explicit approval.**
</steps>

<constraints>
- This is a USER GATE stage — never auto-approve
- Proposal must show before/after for every change
- If analysis report is missing, halt with error (requires stage 00)
- Risk assessment is mandatory — highlight any data loss potential
- Preserve existing functionality — never propose removing working features
- DVA configuration preview must be valid against DVA schema
</constraints>

<gate>
- [ ] Proposal document generated at tmp/setup-dva/
- [ ] Before/after directory comparison included
- [ ] Change plan table is complete (all gaps addressed)
- [ ] DVA config preview is valid YAML
- [ ] User explicitly approved (APPROVAL status recorded)
</gate>

<output>
| Artifact | Path |
|----------|------|
| Proposal Document | `tmp/setup-dva/10-proposal-{project-name}.md` |
| User Approval | Recorded in state.yaml as `stage_10.approval: approved` |
</output>

<trigger>Load analysis → load DVA template → build proposal → present to user → wait for approval.</trigger>
