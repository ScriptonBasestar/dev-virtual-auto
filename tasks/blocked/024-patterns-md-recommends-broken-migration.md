---
id: TASK-024
title: "patterns.md recommends migrating applications to runners.native/docker, which fails at runtime"
type: docs
priority: P2
status: blocked
effort: XS
created-at: 2026-07-16T22:25:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: convergence-check-2
source-severity: MEDIUM
depends-on: [TASK-017]
blocked-reason: dependency
blocked-detail: "The correct content depends on TASK-017's decision about runners.docker/native semantics"
unblock-condition: "TASK-017 decided (Option A -> this doc becomes correct as-is; Option B -> this row must be rewritten)"
blocked-at: 2026-07-17T10:55:00+09:00
---

# Task 024: patterns.md Recommends A Broken Migration

## Summary

`claude-plugin/skills/dva/references/patterns.md:61` instructs users to migrate
`applications.<name>` → `stack.<name>.runners.native/docker`. That shape passes
`dva validate` and then **fails at runtime**.

## Evidence

`patterns.md:57-61`, "Migration Map":

| Old shape | New shape |
| --------- | --------- |
| `applications.<name>` | `stack.<name>.runners.native/docker` |

Following it literally:

```
$ dva validate
✅ dva.yml is valid

$ dva stack up
ERROR: entry "myapp": unknown lifecycle plugin "" (implemented: [...])
```

Cause (established in TASK-009/TASK-017): `native` and `docker` decode to
application-style configs (`NativeRunnerConfig` `lifecycle.go:68`, `DockerRunnerConfig`
`lifecycle.go:75`) that **no lifecycle plugin reads**. `native` is not a registered plugin
at all. So the stack path cannot serve either runner.

`applications:` remains a supported top-level key (one of the 22 in `schema.json`), so this
migration is not only broken — it points away from a working shape toward a non-working one.

This file sits in `claude-plugin/`, a root the original audit classified as input/config and
explicitly declared **unverified** ("`claude-plugin/skills/dva/references/*.md` … command
references unverified"). It was found only by deliberately auditing that blind spot.

## Why this is blocked, not just fixed

The correct content depends on **TASK-017**:

- **If Option A** (map `runners.docker` to the docker plugin): this row becomes correct for
  `docker`, and only the `native` half needs qualifying.
- **If Option B** (reject `docker`/`native` on the stack path): this row must be rewritten to
  point at `applications:` or the nested `process:`/`script:` shapes.

Fixing it now in either direction risks churn or contradicting the decision. It is recorded
as blocked rather than guessed.

## Bearing on TASK-017

This is **evidence for Option A**: a shipped doc already tells users that
`stack.<name>.runners.native/docker` is the intended shape. That is the closest thing found
to a design-intent record, and it points toward `runners.<plugin>` meaning the plugin.
Weigh it against the fact that the runner structs are application-shaped
(`dir`/`build`/`run`), which points the other way.

## Completion Criteria

- [ ] TASK-017 is decided | verify: `human — blocked on the runners.docker/native semantics decision`
- [ ] `patterns.md`'s migration row agrees with the decided behavior, verified by running it | verify: `human — after TASK-017, run the migration the row recommends and confirm dva stack up succeeds`

## References

- [017-runners-docker-native-semantics.md](./017-runners-docker-native-semantics.md) — the blocking decision
- [009-fix-runners-plugin-resolution.md](../_archive/009-fix-runners-plugin-resolution.md) — why the shape fails
