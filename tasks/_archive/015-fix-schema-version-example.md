---
id: TASK-015
title: "Fix schema.json version example that can never load"
type: docs
priority: P2
status: done
archived-at: 2026-07-16T20:55:00+09:00
verified-at: 2026-07-16T20:55:00+09:00
verification-summary: >-
  Verified by orchestrator: schema version example is now "0.1.0" and validates (exit 0).
  Chose a low floor rather than pinning 0.1.44 so the example cannot rot back into this
  same defect at the next release. Also stated the constraint in the description.
  schema.json parses; schema/example tests green.
effort: XS
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G7
source-severity: MEDIUM
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
---

# Task 015: Fix schema.json Unreachable version Example

## Summary

`internal/config/schema.json:365` advertises `"examples": ["8.1.0"]` for `version`.
Copying that example yields a config that always fails the version gate, because the
shipped DVA version is `0.1.44`. Editors surface schema examples as completions, so this
is a trap in the most-used tooling path.

## Evidence

- `internal/config/schema.json:365` — `"examples": ["8.1.0"]`
- `internal/config/version.go:5` — `Version = "0.1.44"`
- `internal/config/config.go:1093-1104` — `isVersionCompatible`: `0.1.44 < 8.1.0` → fails
- `internal/config/config.go:736-738` — "config requires minimum version"

It is an example rather than a stated default, hence MEDIUM.

## Suggested Approach

Use a value that actually loads. Prefer one that does not need bumping every release —
e.g. a low floor like `"0.1.0"` — rather than pinning the current version, so the example
does not silently rot into this same defect after the next release.

## Out Of Scope

- Changing the version gate semantics.

## Completion Criteria

- [x] The `version` example is a value that loads against the shipped binary | verify: `cd "$(mktemp -d)" && V=$(python3 -c "import json;print(json.load(open('$OLDPWD/internal/config/schema.json'))['properties']['version']['examples'][0])") && printf 'version: "%s"\n' "$V" > dva.yml && "$OLDPWD/bin/dva" validate`
- [x] `schema.json` remains valid JSON | verify: `python3 -c "import json;json.load(open('internal/config/schema.json'));print('ok')"`
- [x] Schema/example tests stay green | verify: `go test ./internal/config/ -run 'Schema|Example'`

## Dependencies

None. Shares `schema.json` with TASK-010 and TASK-012 — sequence to avoid edit conflicts.

## References

- `unified.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — G7
- `doc-to-code.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — L4
