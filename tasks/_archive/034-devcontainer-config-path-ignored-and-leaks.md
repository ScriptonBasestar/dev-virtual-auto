---
id: TASK-034
title: "devcontainer.config_path is ignored AND leaks verbatim into the generated devcontainer.json"
type: bug
priority: P2
status: done
effort: S
completed-at: 2026-07-17T04:05:00+09:00
created-at: 2026-07-17T03:20:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (silent config no-ops)
source-severity: MEDIUM
---

# Task 034: A Key That Does Nothing, And Corrupts The File It Was Supposed To Locate

## Summary

`devcontainer.config_path` is accepted by the schema, validates green, and has **zero** effect on
where DVA writes `devcontainer.json` — the path is hardcoded. Worse than inert: the key is copied
verbatim into the **generated** `devcontainer.json`, where `config_path` is not a devcontainer
spec key.

So the field misses in both directions: it fails to do its one job, and it pollutes the artifact.

## Evidence

Measured at HEAD against a rebuilt `bin/dva`.

```yaml
devcontainer:
  enabled: true
  name: probe-dc
  config_path: custom/somewhere/devcontainer.json
  image: mcr.microsoft.com/devcontainers/base:ubuntu
```

```
$ dva validate           ->  EXIT=0     # the key is ADVERTISED as valid
$ dva validate --fix     ->  EXIT=0

where the file landed:
  .devcontainer/devcontainer.json        <- hardcoded path
  custom/somewhere/devcontainer.json     <- ABSENT

generated .devcontainer/devcontainer.json:
{
  "config_path": "custom/somewhere/devcontainer.json",     <-- LEAKED, not a spec key
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "name": "probe-dc"
}
```

Control — `enabled` **is** stripped from the output, which proves the key filter genuinely runs
and that `config_path`'s presence is a real omission rather than a dead code path:

```
enabled correctly stripped  ·  config_path leaked
```

## Root cause

`internal/cli/devcontainer.go:45-51` copies every key except one:

```go
for k, v := range dc {
    if k == "enabled" {
        continue // DVA-only field, not part of devcontainer spec
    }
    out[k] = v          // <-- config_path rides along
}
```

`config_path` is a DVA-only field by exactly the same logic as `enabled`, but was never added to
the exclusion.

The write path ignores it entirely — `internal/cli/validate.go:55`:

```go
dcPath := filepath.Join(c.FileDir(), ".devcontainer", "devcontainer.json")
```

`grep -rn "\.ConfigPath\b" --include="*.go" internal/ | grep -v _test.go` → **zero** non-test
reads. (Control: the same grep shape finds `.Tags` used at `orchestrator.go:366`, so the pattern
works.)

## Scope — one half is decision-independent, one half is not

**Decision-independent (do this now):** `config_path` must not appear in the generated
`devcontainer.json`. It is not a spec key, and that is true whether DVA later honors the field or
drops it. Same treatment as `enabled`, same one-line shape.

**Needs a decision (do NOT settle it in this task):** should DVA *honor* `config_path` (write to
the configured location) or **remove it from the schema** as dead surface? Honoring it is a
feature and interacts with `doctor.go:100-105` and `init.go`, which also hardcode
`.devcontainer/devcontainer.json`. Filing the leak fix does not commit the project to either.

## Severity: MEDIUM / P2

No infrastructure mutation, so it is not P1. But the emitted `devcontainer.json` is consumed by
VS Code / devcontainer CLI, and DVA is writing a non-spec key into a file it does not own. A user
who sets `config_path` gets silent non-compliance plus a polluted artifact, with `dva validate`
reporting green throughout.

## Completion Criteria

- [ ] `config_path` never appears in the generated `devcontainer.json` | verify: `human — run the Evidence probe; assert the generated file contains no "config_path" key`
- [ ] `enabled` is still stripped and every genuine spec key (`name`, `image`, `features`, …) still passes through unchanged | verify: `human — assert name/image survive and enabled does not; this is the control that the fix did not over-filter`
- [ ] A regression test covers the exclusion and is proven to fail without the fix | verify: `human — restore the leak, confirm the new test FAILS, restore the fix, confirm it passes`
- [ ] `make test` and `go vet ./...` pass | verify: `make test && go vet ./...`
- [x] DECISION — honor `config_path` or delete it from `schema.json`? Not to be resolved by the implementer | verify: `human — decide; if honored, doctor.go:100-105 and init.go hardcode the same path and must follow` — **deferred, not resolved**: split out to TASK-037 so it stays visible in the queue rather than being buried in an archived file. The leak fix does not commit the project to either answer.

## Outcome

Done — the decision-independent half only. `generateDevcontainerJSON` now filters a
`dvaOnlyDevcontainerKeys` set (`enabled`, `config_path`) instead of hardcoding `enabled`.
`config_path` no longer reaches the generated file.

Scope held exactly: `validate.go`, `doctor.go`, `init.go`, and `schema.json` have **zero** lines in
the diff, so the write path is still hardcoded and the honor-vs-remove question is untouched.

Verified independently in a scratch worktree containing only this change (the main worktree held
other agents' in-flight edits to the same package, which would have contaminated `make test`):

- Positive control — restoring the leak makes `TestGenerateJSONOmitsDVAOnlyKeys` **fail**, for the
  right reason and only that reason (`config_path` leaked …); the `enabled` and spec-key control
  assertions stay silent. Restoring the fix makes it pass.
- End-to-end against a rebuilt binary, `dva validate` exiting 0 first so the probe is not vacuous:
  generated file contains `name`, `image`, and expanded `features`; contains neither `config_path`
  nor `enabled`; `custom/` is not created.
- `make test` exit 0, `go vet ./...` exit 0.

`features` expansion (`ghcr.io/devcontainers/features/go:latest`) was the one thing the implementer
flagged as unverified — checked here, and it survives. The test asserts real JSON contents, not that
config parses, which is the anti-pattern TASK-036 documents.

## References

- [035-env-file-interpolate-and-priority-ignored.md](./035-env-file-interpolate-and-priority-ignored.md) — same class: schema advertises, code never reads
- [036-service-related-and-hint-ignored.md](./036-service-related-and-hint-ignored.md) — same class
- [026-shipped-examples-validate-green-runtime-red.md](./026-shipped-examples-validate-green-runtime-red.md) — the run's organizing theme: validate-green / runtime-wrong
