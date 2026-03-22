<!-- v:2026-03-23 -->

<constants>
TOOLKIT = setup-dva
SELF = ai-docs/workflow/setup-dva/auto.md
DVA_ROOT = {DVA project root — resolved from git root or dva.yml location}
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
SCHEMA_REF = internal/config/schema.json
EXAMPLES_DIR = examples/
STATE_FILE = tmp/setup-dva/state.yaml
TARGET = {user-provided or detected target project path}
</constants>

[EXECUTE IMMEDIATELY — NO QUESTIONS (except stage 10 user gate)]

<role>DVA setup pipeline orchestrator — delegate stages to subagents, validate gates, persist state</role>

<objective>
Run full DVA setup pipeline by delegating each stage to a subagent.
Orchestrator does NOT execute stage logic directly — it spawns, collects, validates, persists.
Exception: Stage 10 (Verify) requires user confirmation before proceeding.
</objective>

<stages>
| # | Stage | File | Subagent Role | User Gate |
|---|-------|------|---------------|-----------|
| 00 | Analyze | stages/00-analyze.md | Scan target project, generate analysis report | No |
| 10 | Verify | stages/10-verify.md | Present proposal, collect user approval | **Yes** |
| 20 | Transform | stages/20-transform.md | Migrate directory structure | No |
| 30 | Configure | stages/30-configure.md | Generate compose.yml + dva.yml | No |
| 40 | Execute | stages/40-execute.md | Start infra, verify health | No |
</stages>

<execution>
## Orchestration Loop

For each stage (00 → 10 → 20 → 30 → 40):

### 1. Pre-check
- If --resume: load STATE_FILE, skip stages with `gate: PASS`
- If --stage N: jump to stage N directly (still check input dependencies)
- Verify required input artifacts from previous stage exist

### 2. Delegate to Subagent
- Read the stage file from WORKFLOW_ROOT/stages/
- Spawn a subagent with the stage prompt content + previous stage artifacts
- Provide DVA context: pass DVA_ROOT, EXAMPLES_DIR, SCHEMA_REF paths
- The subagent MUST:
  a. Execute all `<steps>` defined in the stage
  b. Produce all artifacts listed in `<output>`
  c. Self-evaluate against `<gate>` checklist
  d. Return structured result: `{ artifacts: [...paths], gate: PASS|FAIL, summary: "..." }`

### 3. User Gate (stage 10 only)
- After stage 10 subagent returns the proposal:
  - Present the DVA structure proposal to user
  - **WAIT for explicit user approval** before proceeding
  - If user requests changes → re-run stage 10 with modifications
  - If user rejects → halt pipeline, set status REJECTED

### 4. Validate Gate
- Orchestrator verifies:
  a. All required artifacts from `<output>` exist as files
  b. Subagent reported gate: PASS
- If FAIL → set stage status BLOCKED in state.yaml, halt, report blocker

### 5. Persist State
- Update state.yaml: stage status → completed, gate → PASS, artifact paths
- Proceed to next stage

## Pipeline Completion
- After stage 40 PASS: set pipeline status to COMPLETE
- Emit final summary: stages completed, dva.yml path, running containers
</execution>

<flags>
| Flag | Behavior |
|------|----------|
| --resume | Load cached state, skip completed stages |
| --dry-run | Run stage 00 only (analysis), no mutations |
| --target PATH | Override target project path |
| --skip-execute | Stop after stage 30 (config only, no dva up) |
</flags>

<constraints>
- NEVER execute stage logic directly. ALWAYS delegate to subagent.
- Stage 10 is the ONLY stage where user interaction is required — ask for approval.
- All other stages run without questions. Gate FAIL is the stop condition.
- Write state.yaml after each stage for resume support.
- If DVA CLI is unavailable, stage 40 falls back to `docker compose up -d`.
- Reference DVA's own examples/ and schema.json for configuration generation.
</constraints>

<trigger>
Start. Parse flags → initialize state → delegate stage 00 → validate gate → continue pipeline. Stage 10 requires user approval before stage 20.
</trigger>
