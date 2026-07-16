---
id: TASK-016
title: "Document 6 shipped subcommands and default_mode"
type: docs
priority: P3
status: done
archived-at: 2026-07-16T21:05:00+09:00
verified-at: 2026-07-16T21:05:00+09:00
verification-summary: >-
  Verified by orchestrator: all six subcommands present in USAGE.md; default_mode
  documented (5 mentions); documented commands confirmed to exist in the binary.
  Scope grew slightly for coherence: USAGE.md documented ZERO app subcommands and
  barely mentioned stack, so full tables were added rather than extending a list;
  `modes` was also undocumented and default_mode is meaningless without it, so a row
  was added. README.md deliberately untouched (edit-restricted); documenting in
  USAGE.md satisfies the criterion, which accepts either file.
effort: S
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G8
source-severity: LOW
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
---

# Task 016: Document Undocumented Surface

## Summary

Six shipped subcommands and the `default_mode` config key exist but appear in no
user-facing doc. Docs are incomplete here, not wrong — no correctness risk, so this is
LOW.

## Evidence

CLI set comparison (binary = ground truth): 51 total / 43 documented / **8 binary-only** /
1 doc-only.

Undocumented but shipped (each verified present via its own `Short` in the parent group's
`Available Commands:`):

| Command | Description |
| ------- | ----------- |
| `dva app build` | Build applications (use `--docker` for container build) |
| `dva app log` | Show recent logs for an application (last 100 lines) |
| `dva app restart` | Restart applications (stop then start) |
| `dva app stop` | Stop applications without removing state |
| `dva stack log` | View logs for a stack entry |
| `dva stack stop` | Stop stack entries without removing resources |

`README.md:71-74` documents only `app ls/up/down`; `README.md:66-68` only
`stack up/down/status`. A reader cannot discover the per-app/per-stack verbs.

`default_mode`: implemented at `internal/config/config.go:27`, validated at
`internal/config/validate.go:103-113`, warned at `internal/config/validate_warnings.go:273-278`.
Grep across `docs/`, `USAGE.md`, `README.md`, `AGENTS.md`, `CLAUDE.md`, `examples/` → **zero hits**.
It is a valid schema key users cannot discover. No documented default exists, so there is
nothing to contradict — a pure omission.

## Out Of Scope

- `dva completion` and `dva help` — cobra built-ins, conventionally undocumented. No action.
- `dva config improve` (`USAGE.md:55`) — does not exist, but the line explicitly labels it
  historical ("과거 `dva config improve --docs-only`와 동일") while documenting the current
  `dva config docs`. **Review-only**: optionally reword to drop the dead command name;
  leaving as-is is acceptable.
- CLI **flags** remain unaudited (`coverage denominator unknown`) — a separate audit.

## Completion Criteria

- [x] All six subcommands appear in `README.md` or `USAGE.md` | verify: `for c in "app build" "app log" "app restart" "app stop" "stack log" "stack stop"; do grep -rqF "dva $c" README.md USAGE.md || { echo "MISSING: dva $c"; exit 1; }; done; echo OK`
- [x] `default_mode` is documented as a config key | verify: `grep -rn "default_mode" USAGE.md docs/ README.md`
- [x] No documented command is absent from the binary (no new doc-only drift) | verify: `./bin/dva app build --help && ./bin/dva stack log --help`

## Dependencies

None. Doc-only.

## References

- [unified.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md) — G8
- [evidence-cli.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/evidence-cli.md) — §3.2
- [code-to-doc.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/code-to-doc.md) — C4, C5
