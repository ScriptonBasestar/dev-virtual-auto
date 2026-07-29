# DVA Dogfood Context Reference

Domain deltas only; invariants live in
[METHODOLOGY.md](./METHODOLOGY.md).

<constants>
PROMPT_ROOT = workflows/dva-dogfood
PACKAGES_ROOT = required user-supplied absolute prmpt-framework root
DVA_ROOT = this repo (repo root)
SKILLS_ROOT = DVA_ROOT/skills
SKILL_TARGETS = DVA_ROOT/skills/_targets.yaml
RUNS_ROOT = <TARGET_PROJECT>/tmp/dogfood-dva
FALLBACK_RUNS_ROOT = ${XDG_STATE_HOME:-$HOME/.local/state}/dogfood-dva/<PROJECT_SLUG>-<PATH_HASH>
RUN_DIR = RUNS_ROOT/<RUN_ID>
</constants>

The loop's canonical skill targets are `skills/config` and `skills/dva` in this
repo. Platform-visible copies are projections described by `skills/_targets.yaml`;
never treat an installed/generated copy as an independent source.

## Runtime variables

| Variable          | Rule                                                             |
| ----------------- | ---------------------------------------------------------------- |
| `TARGET_PROJECT`  | User-supplied absolute path; otherwise current working directory |
| `PACKAGES_ROOT`   | Required user-supplied absolute external root; no workflow default |
| `SKILL_SOURCE`    | Exact canonical directory under `SKILLS_ROOT` selected in stage 10 |
| `SKILL_INSTALLED` | Exact platform projection/cache selected during stage 10          |
| `DVA_ROOT`        | Local DVA CLI/schema/doctor source-of-truth repository           |
| `RUNS_ROOT`       | Git-ignored target temp root or durable user-state fallback      |
| `RUN_DIR`         | Unique evidence directory for one dogfood cycle                  |

Never infer a different target after `state.yaml` is created. If the target
changes, start a new run. Never reuse a `RUN_ID` for a new run.

Record `PACKAGES_ROOT` literally in state. Record its Git HEAD, dirty hash, and
the names of protected dirty paths without reading those paths' contents. A
missing active `config` projection is an `environment` finding. It blocks only
an operation that requires an explicit `config` invocation; it never authorizes
installing, syncing, or synthesizing a projection.

## Source-of-truth ownership

<!-- markdownlint-disable MD013 -->

| Owner                       | Put here                                                                | Do not put here                                        |
| --------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------ |
| `skills/config`, `skills/dva` | Reusable DVA workflows, heuristics, validation, and operation safety | Machine paths, project ports, target-specific commands |
| prmpt framework (external)  | Workstation paths, DVA/Compose boundary, port registry, local templates | Generic DVA schema tutorials duplicated from the skill |
| `dev-virtual-auto`          | CLI behavior, schema parser, discovery/doctor implementation            | Workarounds that belong only to one project            |
| Target project              | `dva.yml`, Compose files, project commands and docs                     | Cross-project policy                                   |
| `workflows/dva-dogfood`     | Evaluation orchestration and references                                 | Product implementation or target-specific fixes        |

<!-- markdownlint-enable MD013 -->

**Worked example — dev vs preview default.** The `default_plan` field and
per-plan run selection are DVA mechanism (`dev-virtual-auto`). The convention
"define `dev`/`preview` plans and default to `dev` for local devbox work" is a
reusable heuristic (`skills/config`). The concrete `default_plan: dev`
value is written into the target's `dva.yml`. Never bake a dev-vs-preview
default into DVA itself or into a per-project workaround.

## Terminology

- **Source skill**: editable canonical skill under `SKILLS_ROOT`.
- **Installed skill**: platform projection or cache derived from the source skill.
- **Prompt**: devenv-specific operational instructions under `PACKAGES_ROOT`.
- **Baseline**: read-only observations captured before cycle mutations.
- **Finding**: evidence-backed mismatch or inefficiency.
- **Defect owner**: the single SSoT that should receive the fix.
- **Forward test**: a fresh session using the skill on a realistic request
  without leaked conclusions.

## Required project context

Before project actions:

1. Read target `AGENTS.md`/`CLAUDE.md` and active module guidance.
2. Identify archived, generated, secret, and user-modified paths.
3. Confirm DVA and related CLI versions from local commands, not memory.
4. Compare the installed DVA build/commit with `DVA_ROOT`; never assume the
   local checkout produced the installed binary.
5. Treat references under this directory as guidance, not target-specific truth.
6. Inventory canonical `dva.yml`, legacy `dva.yaml`, and DVA-related files
   referenced by Makefiles, scripts, or Compose documentation. Legacy presence
   is a migration finding, never automatic rewrite authority.
7. Decide whether DVA is needed independently for the root and every active
   subproject. Parent or sibling usage alone is not justification to create a
   config.
8. When Compose changed, preserve an exact handoff of renamed/added/removed root
   files, service names, profiles, port variables, and env prerequisites.
