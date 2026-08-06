---
id: TASK-182
title: "`VerifyMigrated` proves the migrated config loads, not that its keys still mean anything"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-08-06T08:48:03+09:00
source: "docs/43 command-surface restructure — found while verifying `dva config migrate` against a legacy health fixture"
scope: "dva repo — internal/config/migrate.go, internal/config/schema.json"
---

# Task 182: Make the migration gate catch keys that survive into a config nothing reads

## Problem

`VerifyMigrated` is the only automated gate between `dva config migrate` and the file it
writes over the user's `dva.yml`. It runs two checks:

```go
// internal/config/migrate.go:91-109
func VerifyMigrated(data []byte) error {
	cfg, err := decodeConfig(data)          // check 1: does it parse into Config?
	...
	for name, entry := range cfg.Stack {
		if _, err := ResolvePluginFromName(name, entry); err != nil {   // check 2
	...
```

Neither asks whether a key that decoded successfully still *does* anything. `decodeConfig`
silently discards YAML keys with no matching struct field — that is what `yaml.v3` does by
default — so a migration can emit a key, have it decode "cleanly", and have it reach nothing.

Measured on `0.1.44` / `34cb6f36…`, before the fix in this same change, with the fixture:

```yaml
applications:
  web:
    run: "./bin/web"
    health:
      type: http
      url: "http://localhost:8080/health"
      required: true
```

| | result |
|---|---|
| `dva config migrate` output | contained `required: true` under `stack.web.health_checks.web` |
| `VerifyMigrated` | **passed** |
| `dva validate` on the written file | **exit 0**, `✅ dva.yml is valid` |
| `HealthCheckConfig.Required` | does not exist (`config.go:193-198` — removed with `AppManager.startApp`) |
| non-vacuity control | the same fixture's `type`/`url`/`ready_timeout` **did** reach the decoded check, so the pass is not a decode failure |

Three independent gates said the conversion was clean while a strictness knob sat in the
file with nothing behind it. `validate` could not catch it either: the entry-scoped
`health_checks` object (`schema.json:721-737`) declares `required: ["type"]` but **no
`additionalProperties: false`**, so unknown keys there are legal.

The immediate instance is fixed — `migrateApplicationsNode` now drops `required` and reports
it. This task is the class, not the instance: nothing stops the next migration from doing the
same thing, and nothing in CI would notice.

Note the ordering that makes this worth fixing rather than closing: tightening the schema
alone would **not** have caught this one, because a `required` key inside `health_checks` is
exactly the kind of thing an author would expect to be legal there. Both halves are needed,
and they are needed for different reasons.

## Acceptance criteria

- [ ] Entry-scoped `stack.<entry>.health_checks.<name>` declares `additionalProperties: false`
      in `schema.json`, matching the top-level `health_checks` object which has always had it.
      Verify: `go test ./internal/config/ -count=1`
- [ ] A config with a bogus key under an entry health check now fails `dva validate` naming the
      key and its path. Before this, it passed.
      Verify: `go test ./internal/config/ -count=1`
- [ ] Corpus: every `examples/*.yml` and the repo's own `dva.yml` still validate exit 0 after the
      schema tightens, reported as a count from `dva validate` output rather than grep. A
      non-zero pass count must be printed — an empty loop reads as a pass.
      Verify: `human — the count and the command that produced it are in the Result section`
- [ ] `VerifyMigrated` runs schema validation on the migrated bytes, or the decision not to is
      recorded with its reason. Note it is called from `migrateAppsAndDecode` in tests and from
      the `--write` path; a stricter gate that rejects a legitimate conversion is worse than the
      current one.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] Decide and record whether `decodeConfig` should use `KnownFields(true)` for the migration
      path specifically. That would catch the whole class at once, but it changes what loads for
      every existing config — measure how many of `examples/*` would start failing before
      choosing.
      Verify: `human — the measured count is in the Result section`
- [ ] `make test` and `make doc-check` exit 0.

## References

- `internal/config/migrate.go:91-109` — `VerifyMigrated`, both checks
- `internal/config/config.go:193-198` — the comment recording why `Required` was deleted
- `internal/config/schema.json:721-737` — the entry-scoped `health_checks` object with no
  `additionalProperties` bound
- `internal/config/migrate_applications.go` — the fix for the instance, and the report note
- `internal/config/migrate_applications_test.go` —
  `TestMigrateApplicationsReportsTheDroppedHealthRequired`, which has to assert on emitted
  **text** because a dead key is invisible to `decodeConfig` by construction
- [docs/43](../../docs/43-command-surface-restructure.md) §16 — the capability loss this key
  belonged to

## Notes

Same shape as [TASK-179](179-validate-and-manifest-are-silent-about-health-checks-no-mode-can-reach.md):
a config surface that validates clean while pointing at nothing. 179 is about a live field with
no reachable execution path; this is about a dead field that still looks live. They share a fix
direction — validation should ask "can anything read this?", not only "does this parse?" — and
could reasonably be done together.

The migration reports (`port`, `health.required`) are the manual version of this gate. They work,
but only for the fields whose loss someone remembered to write a note for.
