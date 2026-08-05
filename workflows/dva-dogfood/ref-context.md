# DVA Dogfood Context Reference

Paths, terminology, and the source-of-truth boundary every stage reasons about.
The owner model and stage order live in [README.md](README.md).

<constants>
PROMPT_ROOT = workflows/dva-dogfood
DVA_ROOT = this repo (repo root)
SKILLS_ROOT = DVA_ROOT/skills
SKILL_TARGETS = DVA_ROOT/skills/_targets.yaml
RUNS_ROOT = <TARGET_PROJECT>/tmp/dogfood-dva
FALLBACK_RUNS_ROOT = ${XDG_STATE_HOME:-$HOME/.local/state}/dogfood-dva/<PROJECT_SLUG>-<PATH_HASH>
RUN_DIR = RUNS_ROOT/<RUN_ID>
</constants>

The loop's canonical skill targets are `skills/config` and `skills/dva` in this
repo. Platform-visible copies are projections described by `skills/_targets.yaml`;
never treat an installed or generated copy as an independent source.

## Runtime variables

| Variable          | Rule                                                              |
| ----------------- | ----------------------------------------------------------------- |
| `TARGET_PROJECT`  | User-supplied absolute path; otherwise current working directory  |
| `DVA_ROOT`        | Local DVA CLI/schema/doctor source-of-truth repository            |
| `SKILL_SOURCE`    | Exact canonical directory under `SKILLS_ROOT` the run selected    |
| `SKILL_INSTALLED` | Exact platform projection or cache for the active host            |
| `RUNS_ROOT`       | Git-ignored target temp root, or the durable user-state fallback  |
| `RUN_DIR`         | Unique evidence directory for one dogfood cycle                   |

Never infer a different target after `state.yaml` is created. If the target
changes, start a new run. Never reuse a `RUN_ID`.

A missing active `config` projection is an `environment` finding. It blocks only an
operation that requires an explicit `config` invocation; it never authorizes
installing, syncing, or synthesizing a projection.

## Source-of-truth ownership

<!-- markdownlint-disable MD013 -->

| Owner                         | Put here                                                             | Do not put here                                        |
| ----------------------------- | -------------------------------------------------------------------- | ------------------------------------------------------ |
| `skills/config`, `skills/dva` | Reusable DVA workflows, heuristics, validation, and operation safety | Machine paths, project ports, target-specific commands |
| `dev-virtual-auto`            | CLI behavior, schema parser, discovery/doctor implementation         | Workarounds that belong only to one project            |
| Target project                | `dva.yml`, Compose files, project commands and docs                  | Cross-project policy                                    |
| `workflows/dva-dogfood`       | Evaluation orchestration, stage routing, and references              | Product implementation or target-specific fixes        |

<!-- markdownlint-enable MD013 -->

**Worked example — the default plan.** The `default_plan` field and per-plan run
selection are DVA mechanism (`dev-virtual-auto`). The convention "define
`dev`/`preview` plans and default to `dev` for local devbox work" is a reusable
heuristic (`skills/config`). The concrete `default_plan: dev` value is written into
the target's `dva.yml`. Never bake a dev-vs-preview default into DVA itself, and
never solve it as a per-project workaround.

**Worked example — a diagnosis DVA already owns.** When a lifecycle command fails
for a reason `dva doctor` already detects, the gap is that DVA does not surface its
own diagnosis at the failure point. That is a `dva_tool` finding. Teaching the
skill to "run doctor first" documents around the defect; adding a target hook hides
it from every other project.

## Terminology

- **Source skill**: editable canonical skill under `SKILLS_ROOT`.
- **Installed skill**: platform projection or cache derived from the source skill.
- **Prompt**: this workflow's own stage prompts and references under `PROMPT_ROOT`.
- **Baseline**: read-only observations captured before any cycle mutation.
- **Finding**: an evidence-backed mismatch or inefficiency.
- **Defect owner**: the single SSoT that should receive the fix.
- **Forward test**: a fresh session using the skill on a realistic request without
  leaked conclusions.

## Required project context

Rules that hold for every project action. The stage prompts own *what* to
inventory; these own *how to read* what they find.

1. Read target `AGENTS.md`/`CLAUDE.md` and active module guidance, and identify
   archived, generated, secret, and user-modified paths.
2. Confirm DVA and related CLI versions from local commands, not memory, and
   compare the installed build/commit with `DVA_ROOT`. Never assume the local
   checkout produced the installed binary — a stale binary makes every observation
   about its output unusable as evidence.
3. Treat references under this directory as guidance, not target-specific truth.
4. Legacy `dva.yaml` presence is a migration finding, never automatic rewrite
   authority.
5. Judge DVA need independently for the root and every active subproject; parent or
   sibling usage alone does not justify creating a config.
6. When Compose changed, preserve an exact handoff of renamed/added/removed root
   files, service names, profiles, port variables, and env prerequisites.
