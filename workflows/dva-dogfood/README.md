# DVA Dogfood — Prompt/Skill/Project Improvement Loop

The active entrypoint is the numbered stage prompts in this directory. Start at
[`00-start-cycle.md`](00-start-cycle.md) and follow each stage's `next_prompt`
through to [`70-feedback.md`](70-feedback.md). The loop is self-contained: every
stage prompt names the references it needs and the next stage to run.

> **Methodology:** this loop is an **extended variant** of the shared
> self-improve methodology in [`METHODOLOGY.md`](./METHODOLOGY.md)
> (adds skill-audit and DVA-tool stages; splits evaluate/feedback into
> 60/70). That spec defines the stage spine, state model, evaluation
> contract, session rules, and safety invariants. This README and the local
> `ref-*.md` define only the domain contract.

## Overview

An iterative execution package for improving the canonical DVA skills, the devenv
setup prompt, the DVA tool itself, and target project configuration
together while applying DVA to a real devbox project. Each cycle validates
one hypothesis and feeds discovered issues back to the correct source of
truth.

Sources of truth:

- Dogfood orchestration: `workflows/dva-dogfood` (this repo)
- Domain packages: the prmpt framework (external)
- Skills: this repo's canonical `skills/config` and `skills/dva`; platform copies
  are projections generated or linked from those sources.
- DVA tool: this repo

## Core Principles

- Do not modify skill, prompt, DVA source, or target project before
  measuring.
- Keep reusable DVA knowledge in the canonical skills; keep devenv-specific rules in the
  setup prompt.
- Distinguish project configuration issues from DVA CLI issues.
- Judge DVA necessity independently for root and each active subproject;
  do not generate it when unnecessary.

## Usage

1. Start an AI agent in the devbox project to improve.
1. Pass the first stage [00-start-cycle.md](00-start-cycle.md) and specify the
   target project.
1. Run the prompt named by `state.next_prompt` only when each stage is
   PASS.
1. When resuming after a stop, pass only the `RUN_DIR` absolute path to the
   next session.
1. Always verify changed skill metadata and natural triggering in a fresh session.

Treat the numbered stage files as continuation prompts selected by
`state.next_prompt`. Both new runs and resumes start from the self-contained
first stage `00-start-cycle.md`.

Example request:

```text
Use workflows/dva-dogfood/00-start-cycle.md.
TARGET_PROJECT=<TARGET_PROJECT>
HYPOTHESIS=DVA skill should distinguish CLI defects from project config defects.
MODE=continuous
```

## Stage Order

```text
00 initialize → 10 skill audit → 20 baseline + owner routing
                                  ├─ skill → 30 → 40* → 50
                                  ├─ prompt → 40 → 50
                                  ├─ DVA tool → 45 → 50
                                  ├─ target project → 50
                                  └─ environment → 60
50 apply/forward test → 60 evaluate → 70 read-only feedback
```

`40*` is also used as the new-session trigger gate when stage 30 modifies
an installed skill. Change stages that are not selected are marked
`not_applicable` in state and do not produce an attempt report. If stage 60
finds further changes are needed, it returns to the relevant owner stage
instead of proceeding to stage 70.

## References

| File                                   | Purpose                                             |
| -------------------------------------- | --------------------------------------------------- |
| [ref-context.md](ref-context.md)       | Paths, terms, source-of-truth boundary, run context |
| [ref-artifacts.md](ref-artifacts.md)   | Cycle state and evidence file conventions           |
| [ref-evaluation.md](ref-evaluation.md) | Evaluation scores, defect owners, PASS criteria     |
| [ref-safety.md](ref-safety.md)         | Safety rules and DVA verification ladder            |
| [ref-session.md](ref-session.md)       | Unique run path, re-run, session handoff            |

## Directory Structure

```text
workflows/dva-dogfood/
├── README.md
├── METHODOLOGY.md
├── 00-start-cycle.md
├── 10-audit-skill.md
├── 20-capture-baseline.md
├── 30-improve-skill.md
├── 40-improve-prompts.md
├── 45-improve-dva-tool.md
├── 50-apply-to-project.md
├── 60-evaluate.md
├── 70-feedback.md
├── ref-context.md
├── ref-artifacts.md
├── ref-evaluation.md
├── ref-safety.md
└── ref-session.md
```

## Cycle State and Completion

Cycle state (RUN_DIR path, ATTEMPT_ID, handoff update timing) and
completion conditions are defined by METHODOLOGY and
[ref-session.md](ref-session.md) / [ref-evaluation.md](ref-evaluation.md).
Per-stage completion conditions for the forward-test and evaluation gates
are defined by
[50-apply-to-project.md](50-apply-to-project.md) and
[60-evaluate.md](60-evaluate.md).
