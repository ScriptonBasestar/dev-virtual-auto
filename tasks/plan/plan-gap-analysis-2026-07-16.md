---
id: PLAN-001
title: "Gap analysis remediation — run 20260716T091912Z-73dc094"
type: plan
scope: "Close the 8 gaps found by the bidirectional gap analysis of dev-virtual-auto @ 73dc094"
parent: null
children: [TASK-009, TASK-010, TASK-011, TASK-012, TASK-013, TASK-014, TASK-015, TASK-016, TASK-017]
progress: 11
total-tasks: 9
completed-tasks: 1
target-date: null
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
---

# Plan: Gap Analysis Remediation (run 20260716T091912Z-73dc094)

## Goal

Close the 8 evidence-backed gaps found by the bidirectional gap analysis of
`dev-virtual-auto` at `73dc094`. Every task carries reproduction evidence with
`file:line` and executable `verify:` bindings.

## Source provenance

| Field | Value |
| ----- | ----- |
| Source run ID | `20260716T091912Z-73dc094` |
| Immutable unified report | `tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md` |
| Unified SHA-256 | `e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2` |
| Manifest digests | all 6 artifacts verified OK before emission |
| Repository snapshot | `dev-virtual-auto` @ `73dc094`, `master`, clean, upstream 0/0, no nested Git roots |
| Toolkit baseline | `/Users/archmagece/devenv` @ `4bc4363`, clean; `hub.md` SHA-256 `09c70385bdcf3106…` |
| Toolkit modified during run | no (`<toolkit-remediation-mode>` not entered) |
| Analysis mode | read-only Phase 1 (no `<remediation-mode>`; no target file modified during analysis) |

## Children

- [x] TASK-009 — Fix plugin resolution for the `runners`/`default_runner` stack shape (P1, bug) — G1/HIGH — **done** `9dce65e`, archived
- [ ] TASK-010 — Validate every runner in `schema.json`, not only compose (P1, bug) — G2/HIGH
- [ ] TASK-011 — Honor `--help` on commands that disable flag parsing (P1, bug) — G3/HIGH
- [ ] TASK-012 — Correct inverted ENV precedence in `USAGE.md`, `docs/30`, `schema.json` (P1, docs) — G4/HIGH
- [ ] TASK-013 — Fix `docs/40` clause mandating `vars` for `environments` (P1, docs) — G5/HIGH
- [ ] TASK-014 — Fix `docs/40` recommended YAML using `version: 2` (P2, docs) — G6/MEDIUM
- [ ] TASK-015 — Fix `schema.json` version example that can never load (P2, docs) — G7/MEDIUM
- [ ] TASK-016 — Document 6 shipped subcommands and `default_mode` (P3, docs) — G8/LOW
- [ ] TASK-017 — Decide stack `runners.docker` / `runners.native` semantics (P2, chore, **needs human**) — discovered in TASK-009, not from Phase 1

## Severity → priority mapping (exact, per emit-tasks contract)

`CRITICAL→P0`, `HIGH→P1`, `MEDIUM→P2`, `LOW→P3`. No CRITICAL was found, so no P0 exists.

| Task | Gap | Severity | Priority | Type |
| ---- | --- | -------- | -------- | ---- |
| TASK-009 | G1 | HIGH | P1 | bug |
| TASK-010 | G2 | HIGH | P1 | bug |
| TASK-011 | G3 | HIGH | P1 | bug |
| TASK-012 | G4 | HIGH | P1 | docs |
| TASK-013 | G5 | HIGH | P1 | docs |
| TASK-014 | G6 | MEDIUM | P2 | docs |
| TASK-015 | G7 | MEDIUM | P2 | docs |
| TASK-016 | G8 | LOW | P3 | docs |

## Recommended execution order

1. TASK-009 → 2. TASK-010 (schema must encode the resolution contract 009 establishes)
3. TASK-011 (independent; user-facing hazard)
4. TASK-012 (highest-value doc fix)
5. TASK-013 → 6. TASK-014 (both edit `docs/40`; sequence to avoid conflicts)
7. TASK-015 (shares `schema.json` with 010/012; sequence)
8. TASK-016 (completeness)

## Directory substitutions (mandatory audit record — no silent creation)

| Purpose | Canonical | Used | Reason |
| ------- | --------- | ---- | ------ |
| Task files | `tasks/todo/` | **`tasks/todo/` (created by this run)** | `tasks/` existed but contained **only** `_archive/` — no entry dir at all. `tasks/todo/` is the canonical entry dir and a valid state dir for the task runner. Created explicitly and recorded here, not silently. |
| Plan file | `tasks/plan/plan-gap-analysis-{date}.md` | **`tasks/plan/plan-gap-analysis-2026-07-16.md` (dir created by this run)** | `tasks/plan/` was absent, so the emit-tasks `tasks/`-root fallback was tried first — the local validator **rejected** it (`❌ Unsupported canonical task directory`), which would have introduced a new failure against a zero-failure baseline. The local validator contract takes precedence over the hub fallback, so the canonical `tasks/plan/` was created explicitly and recorded here. Re-validated: ✅ valid. |
| Filename rule | `{type}-gap-{kebab-summary}.md` | **`NNN-kebab-summary.md`** (009–016) | Detected local convention from `tasks/_archive/001-config-model.md` … `008-tests-and-examples.md`. Local filename rules take precedence; numbering continues from 008. |
| Decision task (TASK-017, added mid-run) | `tasks/decision/` | **`tasks/todo/017-…md`** with `needs-human: true` and `human —` verify bindings | `tasks/decision/` is listed as a valid state dir by the task runner and **is recognized by `ce task board`** (it displayed `decision (1)`), but `ce task validate` rejects it: `❌ Unsupported canonical task directory` — the tool contradicts itself. Keeping `decision/` would have left `ce task validate --all` red against a zero-failure baseline. The decision semantics are preserved in the body (options, recommendation, confidence) and the `human —` bindings prevent auto-resolution. Validator-supported dirs observed: `todo/`, `plan/`, `_archive/`. |
| Body sections | Summary + Completion Criteria | **Summary + Completion Criteria** (+ Evidence, Out Of Scope, Dependencies, References) | Enforced by the local validator `ce task validate`, which errors on "Missing Summary section" / "Missing Completion Criteria section". Note the archived files use an older `Goal`/`Acceptance Criteria` shape that the validator does **not** accept; `_archive/` is excluded from validation, so the canonical shape was used for new tasks. |

## Validation record

| Stage | Command | Result |
| ----- | ------- | ------ |
| Baseline (pre-emission) | `ce task validate --all` | exit 0 — "No task files found" (zero failures; `_archive/` not validated) |
| Baseline listing | `ce task list` | "No matching tasks" |
| Representative draft | `ce task validate <draft>` | Validated one draft first; initial `Goal`/`Acceptance Criteria` shape **failed** (3 errors) → corrected to `Summary`/`Completion Criteria` before emitting the batch |
| Post-emission | see run output | all 8 tasks + plan valid; no new or regressed failures vs baseline |

## Scope boundaries carried from Phase 1

These are declared blind spots, not findings. They do not affect any emitted task —
each task is independently evidenced:

- CLI **flags** were not compared against docs → `coverage denominator unknown`.
- Doc clause coverage is **sample-verified over the highest-risk contract surfaces**,
  not a full sentence-level census of all five current docs.
- `claude-plugin/skills/dva/references/*.md` and `agent-mesh-flows/**` command
  references unverified (non-product roots; `73dc094` touched them).

## Explicitly rejected non-findings

- `docs/40` §7 vs the shipped `dva stack up`: design tension, not a contradiction.
  §8's `dva up <name>` **is** implemented (verified: `dva ls` → "Plans (dva up <name>)";
  `dva up local-dev` executed the plan), so `docs/40` is `current`, and `dva stack up`
  is a supplementary entry-level control.
- `dva engine` (`USAGE.md:33`): `namespace:command` syntax, not a command claim.
- `.dva/` in `CHANGELOG.md:40`: correct historical record of the `.sb/dva` migration.
- `internal/config/environment.go:87` comment: ambiguous in isolation, no behavioral conflict.

## Healthy surfaces (no action)

- Config top-level keys: Go struct ↔ `schema.json` exact **22/22**.
- CI executable contracts run literally: `make build`, `make test` (`-race -cover`),
  `go vet ./...` — all exit 0.
- `.sb/dva` modules path consistent across every live doc and consumer.
- Commands removed by `73dc094` (`dev`, `add`, `migrate`, `cmd`) left no doc references.
