# workflows/

Development workflows for **DVA itself** — multi-stage prompt sequences an AI agent
runs to improve DVA's skill, prompts, and tool. Distinct from product workflows and
from the skills, which have their own homes:

| Home | Purpose | Runs via |
| ---- | ------- | -------- |
| `workflows/` (here) | **Improve DVA itself** — dogfood loops that audit/improve the skill, prompts, and CLI while applying DVA to a real project | agent reads the numbered stage prompts |
| `agent-mesh-flows/` | **Product** — analyze/improve/diagnose a *user's* `dva.yml` | `am run dva-discover \| dva-improve \| dva-diagnose` |
| `skills/` | The canonical DVA **skills** (single source, projected to platforms) | loaded by the AI host; see `skills/README.md` |

## Workflows

- **[`dva-dogfood/`](dva-dogfood/)** — Prompt/Skill/Project improvement loop.
  Numbered stages `00-start-cycle` → `70-feedback`; start at
  [`dva-dogfood/00-start-cycle.md`](dva-dogfood/00-start-cycle.md). `METHODOLOGY.md`
  is the shared self-improve spine; `ref-*.md` define the DVA domain contract.

## Provenance & single source

`dva-dogfood/` was **imported from** the `prmpt` framework
(`scripton/iac/devenv`, `packages/dva/dogfood`) and **decoupled** from its CE
controller / catalog / gateway so it stands alone here. This repo is now the
**canonical source**; `prmpt` keeps a reference pointer only (no dual management —
see `tasks/todo/054-thin-prmpt-dva-dogfood-after-workflow-import.md`).

These are plain Markdown stage prompts — inherently host-portable, so they need no
per-platform conversion (unlike `skills/`, which `tools/skillgen` projects).
