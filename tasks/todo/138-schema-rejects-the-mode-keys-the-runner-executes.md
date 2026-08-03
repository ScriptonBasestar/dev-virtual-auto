---
id: TASK-138
title: "schema.json rejects modes.<name>.build and .run, the keys the build runner reads"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T12:30:00+09:00
source: "TASK-083 finalize verification — the mirror of the defect 083 fixed, one level up"
depends-on: [TASK-083]
scope: "dva repo — internal/config/schema.json, internal/cli/compose.go"
---

# Task 138: Let the schema accept the mode keys the runner already executes

## Problem

`ModeConfig` declares two strategy keys that drive real behaviour:

```
internal/config/config.go:148  Build string `yaml:"build"`  // "docker" | "native" | custom shell
internal/config/config.go:149  Run   string `yaml:"run"`    // "docker" | "native" | custom shell
```

They are load-bearing — `internal/cli/compose.go:503-524` switches the build strategy on
`ModeConfig.Build`.

`schema.json`'s `modes` object sets `additionalProperties: false` and lists only
`applications`, `compose_profiles`, `compose_services`, `description`, `endpoint_tags`,
`environment`, `health_checks`, `provision`, `stack`. Neither `build` nor `run` is there.

So the schema rejects what the runner executes. Reproduced 2026-08-03 with `bin/dva`
v0.1.44 on a config declaring `modes: {native: {build: native, run: native}}`:

```
$ dva validate
ERROR: schema validation failed in dva.yml:
  - modes.native: Additional property build is not allowed
  - modes.native: Additional property run is not allowed
rc=1

$ dva build --mode native
ERROR: mode "native" build=native but no interaction.build.replace defined
```

The second command read `build: native`, resolved the strategy, and failed on the *next*
step — which is the proof that the key is consumed, not ignored.

## Why this is TASK-083's shape

TASK-083 fixed a step that announced work it never did, and its second half closed the
same schema/Go divergence for `provision_item`. This is the mirror image: the Go side
acts, the schema forbids. `internal/cli/compose.go` was in that task's audit (it names
call site `compose.go:464`), but the schema half was applied only to `provision_item`.

## Acceptance criteria

- [ ] `modes.<name>.build` and `modes.<name>.run` are accepted by `schema.json` with the
      value shape the Go code actually reads (string).
- [ ] The fixture above validates: `dva validate` exits 0 on a config declaring both keys.
- [ ] A test pins schema-vs-Go agreement for `ModeConfig` so the next added field cannot
      diverge silently — the same guard shape TASK-083 used for `provision_item`.
- [ ] The guard is proven non-vacuous: it fails when a field is removed from the schema.
- [ ] `make test` exits 0.

## Notes

Check the rest of `ModeConfig` while here — this sweep found `build`/`run` by exercising
one path, not by comparing the whole struct against the schema. A field-by-field diff of
`ModeConfig` against `schema.json`'s `modes` object is the cheap way to learn whether
these two are the only ones.
