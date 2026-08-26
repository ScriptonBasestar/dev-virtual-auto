# DVA Dogfood — Skill/Prompt/Tool Improvement Loop

An evidence-first loop that improves the canonical DVA skills, this workflow's own
stage prompts, the DVA tool itself, and a target project's configuration — while
applying DVA to a real devbox project. One run tests one measurable hypothesis,
changes exactly one owner, and records append-only evidence that can stop and
resume from `RUN_DIR` without hidden conversation context.

Start at [`00-start.md`](00-start.md) and follow each stage's `next_prompt`
through to [`40-evaluate.md`](40-evaluate.md). The loop is self-contained: it runs
with this repository alone, with no external checkout and no framework gateway.

## Usage

1. Start an AI agent with read access to both this repository (`DVA_ROOT`) and the
   devbox project to improve (`TARGET_PROJECT`).
1. Pass [`00-start.md`](00-start.md) with the inputs below.
1. Run the prompt named by `state.next_prompt`, and only when the current stage is
   PASS.
1. When resuming after a stop, pass only the `RUN_DIR` absolute path.

```text
Use workflows/dva-dogfood/00-start.md.
TARGET_PROJECT=/absolute/path/to/devbox
HYPOTHESIS=DVA should surface its own daemon diagnosis when a compose lifecycle
           command fails because the Docker daemon is unavailable.
MODE=step
```

`MODE=step` stops after every accepted stage and emits `RUN_DIR=` / `NEXT_PROMPT=`,
which keeps each stage inside one context window. `MODE=continuous` follows
`next_prompt` until a stop condition. Neither mode overrides a fresh-session,
approval, failure, or safety boundary.

## Stage order

```text
00 start      → bind target, hypothesis, unique run, revisions, projection status
10 baseline   → before-state, derive cases, freeze requests, select ONE owner
20 improve    → mutate the selected owner only  (skill | prompt | dva_tool)
30 forward    → fresh-session gate, history-free case sessions, target-owned edits
40 evaluate   → score, gate, route every finding to its SSoT, close the run
```

Stage 20 is skipped when the selected owner is `target_project`, `environment`, or
`no_change`; it is marked `not_applicable` with no attempt report. Stage 40 may
route a correction back to stage 20 within the same owner instead of closing.

## Owner model

Every run selects exactly one owner at stage 10 and may mutate only that owner.
A finding belonging to a different owner is backlog for the next run.

| Owner            | Meaning                                            | Mutating stage |
| ---------------- | -------------------------------------------------- | -------------- |
| `skill`          | reusable DVA knowledge in `skills/`                | 20             |
| `prompt`         | this workflow's own stage prompts and references   | 20             |
| `dva_tool`       | DVA CLI, schema, discovery, doctor, runtime        | 20             |
| `target_project` | the analyzed project's own configuration and docs  | 30             |
| `environment`    | tooling/runtime gap outside the three above        | none           |
| `no_change`      | no current gap; proceed to the forward test        | none           |

Never mutate two owners under one run. A different owner requires a new run with a
new hypothesis and a new baseline.

## Core principles

- Do not modify skill, prompt, DVA source, or target project before measuring.
- Keep reusable DVA knowledge in the canonical skills; keep run orchestration here.
- Distinguish project configuration issues from DVA CLI issues.
- Judge DVA necessity independently for the root and each active subproject; do not
  generate a config where none is warranted.
- A case must name project state that was verified to exist. A request about state
  the target does not have is an answer key, not evidence.

## Sources of truth

- Dogfood orchestration: `workflows/dva-dogfood` (this directory)
- Skills: this repo's canonical `skills/dva-config` and `skills/dva`; platform copies
  are projections generated or linked from those sources, never independent sources
- DVA tool: this repository

## References

| File                                   | Purpose                                                |
| -------------------------------------- | ------------------------------------------------------ |
| [ref-context.md](ref-context.md)       | Paths, terms, source-of-truth boundary, run context    |
| [ref-artifacts.md](ref-artifacts.md)   | Run layout, state schema, reports, session and resume  |
| [ref-evaluation.md](ref-evaluation.md) | Evaluation surfaces, case derivation, finding ownership |
| [ref-safety.md](ref-safety.md)         | Safety invariants, protected operations, validation ladder |

Every numbered stage loads all four references whole, once per session, and reuses
them while their path and Git revision are unchanged.

## Completion

A run is `complete` only when the before/after evidence is comparable, every
derived case has an outcome, every finding has exactly one owner, and one
measurable next hypothesis is recorded. Runtime startup is a distinct post-cycle QA
surface: no numbered stage may invoke `provision`, `up`, `down`, `stop`, or
`restart` against a real target.
