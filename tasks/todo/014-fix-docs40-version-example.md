---
id: TASK-014
title: "Fix docs/40 recommended YAML using version: 2"
type: docs
priority: P2
status: todo
effort: XS
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G6
source-severity: MEDIUM
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
---

# Task 014: Fix docs/40 version: 2 Example

## Summary

`docs/40-declarative-stack-and-plans.md:314` opens its **recommended** YAML example
(§9 "권장 YAML 예시") with `version: 2`, treating `version` as a config-format revision.
The code treats it as the minimum required DVA version, as a semver **string**. The
recommended example is schema-invalid and would fail the version gate.

## Evidence

- `docs/40:314` — `version: 2` (integer, format-revision style)
- `internal/config/config.go:736-738` — treated as minimum required DVA version; error
  text "config requires minimum version"
- `internal/config/config.go:1093-1104` — `isVersionCompatible` parses semver `%d.%d.%d`
- `internal/config/schema.json:362` — `"type": "string"`
- `internal/config/version.go:5` — shipped `Version = "0.1.44"`

`version: 2` is a non-string (schema-invalid) and parses to `2.0.0`, which fails the
gate against `0.1.44`.

Every other source is consistent, so this is an isolated stale clause:
`USAGE.md:233` (`version: "0.1.44"  # 최소 DVA 버전`), `README.md:30`, and all
example YAMLs use `version: "0.1.44"`.

## Out Of Scope

- Introducing a real config-format version field. If a format revision is genuinely
  wanted, that is a design decision, not a doc fix — raise it separately.

## Completion Criteria

- [ ] `docs/40` §9 uses a quoted semver string consistent with the rest of the docs | verify: `! grep -nE '^version: *[0-9]+ *$' docs/40-declarative-stack-and-plans.md`
- [ ] The §9 example's version value loads against the shipped binary | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\n' > dva.yml && "$OLDPWD/bin/dva" validate`
- [ ] No doc example uses an unquoted integer version | verify: `! grep -rnE '^\s*version: *[0-9]+ *$' docs/ USAGE.md README.md examples/`

## Dependencies

None. Doc-only. Shares `docs/40` with TASK-013 — sequence to avoid edit conflicts.

## References

- [unified.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md) — G6
- [doc-to-code.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/doc-to-code.md) — L3
