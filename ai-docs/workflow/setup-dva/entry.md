<!-- v:2026-03-23 -->

<constants>
TOOLKIT = setup-dva
SELF = ai-docs/workflow/setup-dva/entry.md
DVA_ROOT = {DVA project root — resolved from git root or dva.yml location}
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
SCHEMA_REF = internal/config/schema.json
EXAMPLES_DIR = examples/
STATE_FILE = tmp/setup-dva/state.yaml
TARGET = {user-provided target project path}
</constants>

[EXECUTE IMMEDIATELY]

<role>DVA setup diagnostic router — analyze request, recommend stage or delegate to auto.md</role>

<objective>
Analyze user's DVA setup request and project state.
Route to the appropriate stage or delegate full pipeline to auto.md.
Multi-stage workflow with user confirmation gates — step mode recommended for first-time setup.
</objective>

<stages>
| # | Stage | File | Description |
|---|-------|------|-------------|
| 00 | Analyze | stages/00-analyze.md | Scan target project, detect compose patterns and directory structures |
| 10 | Verify | stages/10-verify.md | Present DVA structure proposal, wait for user confirmation |
| 20 | Transform | stages/20-transform.md | Migrate files/directories to DVA-compatible layout |
| 30 | Configure (full) | stages/30-configure-full.md | compose.yml + dva.yml 신규 생성 (compose 없는 프로젝트) |
| 30 | Configure (adopt) | stages/30-configure-adopt.md | 기존 compose 기반 dva.yml만 생성 (compose 있는 프로젝트) |
| 40 | Execute | stages/40-execute.md | Run dva up, verify container health |
</stages>

<diagnosis>
1. Check STATE_FILE → determine last completed stage
2. Scan target project for existing DVA artifacts (dva.yml, compose.yml)
3. Check DVA CLI availability (dva binary or go run)
4. Determine: fresh setup, partial migration, or re-run needed

## Quick State Assessment
```bash
# DVA CLI available?
command -v dva 2>/dev/null || ls $DVA_ROOT/bin/dva 2>/dev/null
# Target project state
ls $TARGET/dva.yml $TARGET/compose.yml $TARGET/docker-compose.yml 2>/dev/null
# Prior analysis artifacts
ls tmp/setup-dva/ 2>/dev/null
```
</diagnosis>

<routing>
| Condition | Action |
|-----------|--------|
| "auto", "자동", "한번에", "전부" | IMMEDIATELY read and execute auto.md |
| "--dry-run", "분석만", "analyze only" | Execute stage 00 only |
| No prior state, fresh project | Recommend stage 00 (present menu) |
| State shows 00 complete | Recommend stage 10 (present menu) |
| State shows 10 complete (user approved) | Recommend stage 20 |
| State shows 20 complete (transform done) | Recommend stage 30 — route to `30-configure-full.md` or `30-configure-adopt.md` based on `setup_track` in analysis report |
| State shows 30 complete (config generated) | Recommend stage 40 |
| Specific stage requested | Execute that stage directly |

### Stage Selection Menu (step mode)

```text
[setup-dva] Available stages:
  00. Analyze    — Scan target project, detect patterns
  10. Verify     — Review DVA structure proposal (requires user approval)
  20. Transform  — Migrate to standard structure
  30. Configure  — Generate dva.yml (+ compose.yml if needed)
  40. Execute    — Start infrastructure (dva up)

Select stage to run (or "auto" for full pipeline):
```
</routing>

<constraints>
- Do not execute stages directly in step mode — present menu and let user confirm
- Stage 10 (Verify) is a mandatory user-confirmation gate — never skip
- For auto/resume requests, delegate to auto.md
- If DVA CLI not found, warn but continue (docker compose fallback)
- Target project path is required — ask if not provided
- Reference DVA examples from EXAMPLES_DIR and schema from SCHEMA_REF
</constraints>

<trigger>
Start. Analyze request → if auto → IMMEDIATELY read and execute auto.md. Otherwise detect state → present stage menu → wait for user selection.
</trigger>
