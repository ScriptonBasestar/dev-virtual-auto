# workflows/

Development workflows for **DVA itself** — multi-stage prompt sequences an AI agent
runs to improve DVA's skill, prompts, and tool. Distinct from product workflows and
from the skills, which have their own homes:

| Home | Purpose | Runs via |
| ---- | ------- | -------- |
| `workflows/` (here) | **Improve DVA itself** — dogfood loops that audit/improve the skill, prompts, and CLI while applying DVA to a real project | agent reads the numbered stage prompts |
| `agent-mesh-flows/` | **Product** — analyze/improve/diagnose a *user's* `dva.yml` | `am run dva-discover \| dva-improve \| dva-diagnose` |
| `skills/` | The canonical DVA **skills** (single source, projected to platforms) | loaded by the AI host; see `skills/README.md` |

> **`workflows/` vs `agent-mesh-flows/` are two layers, not competing paradigms** —
> an interactive orchestration loop (here) vs a deterministic `am`-run execution
> primitive. They are intentionally **not** wired together (dogfood exercises the
> skill/CLI directly). See `ARCHITECTURE.md` → "AI 워크플로우 층위".

## Workflows

- **[`dva-dogfood/`](dva-dogfood/)** — Prompt/Skill/Project improvement loop.
  Numbered stages `00-start-cycle` → `70-feedback`; start at
  [`dva-dogfood/00-start-cycle.md`](dva-dogfood/00-start-cycle.md). `METHODOLOGY.md`
  is the shared self-improve spine; `ref-*.md` define the DVA domain contract.

## Why the canonical lives here (not in devenv/prmpt)

This repo (`github.com/ScriptonBasestar/dev-virtual-auto`) is the **public,
distributable artifact** DVA users receive. `prmpt` lives in the **private,
internal** `gitlab.polypia.net/scripton/iac/devenv` repo — DVA users cannot access
it. So any DVA workflow users should run **must be canonical here**; devenv cannot
be the home for user-facing DVA capability. `prmpt` is one *consumer* of this
content, not its owner.

`dva-dogfood/` was **imported from** `prmpt` (`packages/dva/dogfood`) and
**decoupled** from that framework's gateway / catalog / CE controller, so it runs
**standalone with only this repo** — hand `00-start-cycle.md` to any agent; no
`prmpt-gateway` or devenv access is needed. `prmpt` keeps a reference pointer only
(see `tasks/todo/054-thin-prmpt-dva-dogfood-after-workflow-import.md`).

Scope boundary: only `packages/dva` is DVA-specific and belongs here. The rest of
`prmpt` (its gateway CLI, catalog, and ~31 non-DVA domain packages) is
general-purpose framework infrastructure and correctly stays in devenv.

**Canonical location for external references** — `prmpt`'s pointer (and any other
repo referencing this workflow) MUST use the GitHub URL, never a local path:

> `https://github.com/ScriptonBasestar/dev-virtual-auto/tree/master/workflows/dva-dogfood`

These are plain Markdown stage prompts — inherently host-portable, so they need no
per-platform conversion (unlike `skills/`, which `tools/skillgen` projects).
