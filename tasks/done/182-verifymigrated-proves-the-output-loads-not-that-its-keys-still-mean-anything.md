---
id: TASK-182
title: "`VerifyMigrated` proves the migrated config loads, not that its keys still mean anything"
type: bug
priority: P2
status: done
effort: M
created-at: 2026-08-06T08:48:03+09:00
completed-at: 2026-08-07
source: "docs/43 command-surface restructure — found while verifying `dva config migrate` against a legacy health fixture"
scope: "dva repo — internal/config/schema.json, internal/config/validate.go, internal/config/migrate.go"
---

# Task 182: Make the migration gate catch keys that survive into a config nothing reads

## Problem

`VerifyMigrated` is the only automated gate between `dva config migrate` and the file it
writes over the user's `dva.yml`. Decode silently discards YAML keys with no matching struct
field, so a migration can emit a dead key and still pass. Entry-scoped `health_checks` lacked
`additionalProperties: false`, so `required: true` validated clean while nothing read it.

## Acceptance criteria

- [x] Entry-scoped `stack.<entry>.health_checks.<name>` declares `additionalProperties: false`
      in `schema.json`, matching the top-level `health_checks` object which has always had it.
      Verify: `go test ./internal/config/ -count=1`
- [x] A config with a bogus key under an entry health check now fails `dva validate` naming the
      key and its path. Before this, it passed.
      Verify: `go test ./internal/config/ -count=1`
- [x] Corpus: every `examples/*.yml` and the repo's own `dva.yml` still validate exit 0 after the
      schema tightens, reported as a count from `dva validate` output rather than grep.
      Verify: `human — the count and the command that produced it are in the Result section`
- [x] `VerifyMigrated` runs schema validation on the migrated bytes, or the decision not to is
      recorded with its reason.
      Verify: `human — the decision and its reasoning are in the Result section`
- [x] Decide and record whether `decodeConfig` should use `KnownFields(true)` for the migration
      path specifically. Measure how many of `examples/*` would start failing before choosing.
      Verify: `human — the measured count is in the Result section`
- [x] `make test` and `make doc-check` exit 0.

## Result

### Schema

`stack.*.health_checks` pattern items now set `additionalProperties: false` and list the same
live keys as top-level health checks (`start`, `start_hint`, `ready_timeout` included). A
`required:` under an entry check fails `Validate()` with `Additional property required is not
allowed` (`TestHealthRequiredIsGone/entry_scoped_health_checks_rejects_required`).

`validateYAMLSchema` is extracted so schema validation is not tied only to an on-disk path.

### VerifyMigrated: no full schema (decision)

**Do not** run full JSON-schema validation inside `VerifyMigrated`. Migration may leave residual
`applications:` (and similar) when an entry cannot convert; schema rejects those roots, so a
strict schema gate would fail a legitimate partial conversion (`TestMigrateApplicationsMigratesWhatItCanAndLeavesTheRest`). That is worse than the dead-key hole.

The class is closed at the schema object + `dva validate` / `Config.Validate` on a file the user
keeps. Migration still drops `health.required` at rewrite time so `--write` does not depend on a
later validate.

### KnownFields: measured, not enabled

| Corpus | KnownFields(true) decode |
|--------|--------------------------|
| `examples/**/*.yml` + root `dva.yml` | **total=20 fail=0 pass=20** |

Despite zero example failures, KnownFields is **not** enabled on `decodeConfig` or the migration
path: the same residual `applications:` that blocks full schema would trip a strict decoder, and
schema is already the product contract shared with validate.

### Corpus validate

```bash
# rebuilt bin/dva after schema change
while read f; do DVA_FILE=… ./bin/dva config validate; done
# corpus validate (rebuilt): total=20 pass=20 fail=0
```

### Verification

```
go test ./internal/config/ -count=1   # ok
make test                            # exit 0
make doc-check                       # exit 0
```
