<!-- v:2026-03-23 -->

<constants>
SELF = ai-docs/workflow/setup-dva/stages/00-analyze.md
DVA_ROOT = {DVA project root}
WORKFLOW_ROOT = ai-docs/workflow/setup-dva
SCHEMA_REF = internal/config/schema.json
EXAMPLES_DIR = examples/
TARGET = {resolved from entry.md or auto.md — target project path}
PORT_REGISTRY = {optional — global-port-mappings.yaml path if available}
</constants>

[EXECUTE IMMEDIATELY - NO QUESTIONS]

<role>DVA project analysis agent — scan target project, detect patterns, generate report</role>

<input>
| Source | Description |
|--------|-------------|
| TARGET | Target project path (required) |
| EXAMPLES_DIR | DVA example configurations for reference |
| SCHEMA_REF | DVA schema for validation reference |
| PORT_REGISTRY | Global port allocation (optional) |
</input>

<objective>
Analyze target project's current state — compose files, directory structure, services, ports.
Reference DVA examples to determine best-fit configuration pattern.
Generate analysis report as markdown artifact for stage 10 (Verify).
</objective>

<steps>
## Phase 1: DVA Reference Scan

Load DVA's own examples to understand available patterns:

```bash
# List DVA example configs
ls $DVA_ROOT/examples/*.yml | head -20
# Read DVA schema for valid fields
cat $DVA_ROOT/internal/config/schema.json | head -100
```

Identify which DVA example pattern best matches the target project type.

## Phase 2: Target Project Deep Scan

```bash
# Directory structure
find $TARGET -maxdepth 2 -type f | head -50

# Compose files
cat $TARGET/compose*.yml $TARGET/docker-compose*.yml 2>/dev/null

# Environment files
cat $TARGET/.env.example $TARGET/.env 2>/dev/null

# Existing DVA config
cat $TARGET/dva.yml 2>/dev/null

# Build system
head -30 $TARGET/Makefile 2>/dev/null

# Project type detection
ls $TARGET/go.mod $TARGET/pyproject.toml $TARGET/package.json $TARGET/Cargo.toml $TARGET/Gemfile 2>/dev/null
```

Collect:
- Project language/framework (Go, Python, Node, Ruby, Rust, etc.)
- Compose file naming (compose.yml vs docker-compose.yml)
- Override/overlay conventions
- Services and port assignments
- Existing healthchecks
- Build tooling (Make, scripts, etc.)

## Phase 3: Gap Analysis

Compare target against DVA best practices:
- Missing standard files (compose.yml, .env.example, Makefile)
- Non-standard naming (docker-compose.yml → compose.yml)
- Missing healthchecks in compose services
- Port conflicts (against PORT_REGISTRY if available)
- Missing DVA config (dva.yml)
- Missing Compose Specification compliance (no `version:` key)

## Phase 4: DVA Pattern Matching

Based on project type, recommend DVA example as template:
| Project Type | Recommended Example |
|-------------|---------------------|
| Basic/Single service | examples/basic.yml |
| Full-stack (frontend+backend) | examples/full-stack.yml |
| Node.js | examples/nodejs.yml |
| Kubernetes | examples/kubernetes.yml |
| Multi-env | examples/env-file-multi-env.yml |
| LLM/AI tools | examples/llm-integration.yml |

## Phase 5: Report Generation

Generate analysis report to `tmp/setup-dva/00-analysis-{project-name}.md`:

```markdown
# DVA Analysis: {project-name}

## Project Profile
- Language/Framework: {detected}
- Build system: {make/scripts/none}
- Recommended DVA template: {example name}

## Current State
- Compose: {exists|missing|legacy-naming}
- Services: {list with ports}
- DVA config: {exists|missing}
- Env management: {.env/.env.example status}

## Gap Analysis
| Item | Current | DVA Standard | Action |
|------|---------|-------------|--------|

## Recommended DVA Structure
(preliminary — refined in stage 10)
```
</steps>

<constraints>
- Read-only analysis — do not modify any files
- Limit bash output with | head -N to stay within context
- If TARGET not provided, halt with error
- Skip files that error during scan (permission issues, etc.)
- PORT_REGISTRY is optional — skip port conflict check if unavailable
</constraints>

<gate>
- [ ] Analysis report generated at tmp/setup-dva/
- [ ] Report contains project profile section
- [ ] Report contains current state section
- [ ] Report contains gap analysis table
- [ ] Report contains recommended DVA template
- [ ] No files were modified
</gate>

<output>
| Artifact | Path |
|----------|------|
| Analysis Report | `tmp/setup-dva/00-analysis-{project-name}.md` |
</output>

<trigger>Load DVA references → scan target → analyze gaps → match DVA pattern → generate report.</trigger>
