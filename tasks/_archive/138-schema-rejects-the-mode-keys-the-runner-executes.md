---
id: TASK-138
title: "schema.json rejects modes.<name>.build and .run, the keys the build runner reads"
type: bug
priority: P2
effort: S
created-at: 2026-08-03T12:30:00+09:00
source: "TASK-083 finalize verification — the mirror of the defect 083 fixed, one level up"
depends-on: [TASK-083]
scope: "dva repo — internal/config/schema.json, internal/cli/compose.go"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: schema modes.build/run; ModeConfigSchema tests PASS
verification-summary: |
  quality-review pass; re-checked deliverables. schema modes.build/run; ModeConfigSchema tests PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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

- [x] `modes.<name>.build` and `modes.<name>.run` are accepted by `schema.json` with the
      value shape the Go code actually reads (string).
- [x] The fixture above validates: `dva validate` exits 0 on a config declaring both keys —
      pinned by `TestValidateAcceptsModeBuildRunKeys` (round-trip: Validate accepts AND the
      fields parse).
- [x] A test pins schema-vs-Go agreement for `ModeConfig` — `TestModeConfigSchemaAgreesWithStruct`
      reflects `ModeConfig`'s yaml tags against the schema's `modes` properties bidirectionally.
- [x] The guard is proven non-vacuous — the `detects a removed schema key` subtest deletes
      `environment` from a schema copy and asserts the comparison catches it.
- [x] `make test` exits 0.

## Resolution

Added `build` and `run` (`type: string`) to the `modes.patternProperties.<pattern>.properties`
object in `internal/config/schema.json`, just before `applications` (matching struct order).
`additionalProperties: false` now admits exactly the eleven `ModeConfig` yaml tags.

Field-by-field diff (the task's Notes asked whether these two are the only ones): ModeConfig's
eleven yaml tags are `description, compose_profiles, compose_services, health_checks,
endpoint_tags, environment, provision, stack, build, run, applications`. Before this change the
schema listed nine and omitted exactly `build` and `run` — these two are the only divergence.
Going forward `TestModeConfigSchemaAgreesWithStruct` enforces the match in both directions: a
field added to the struct without a schema entry fails it, and a schema property with no struct
field fails it.

## Review

Independent `core:code-reviewer` pass: **"Solid fix. No Critical/High."** Validation gate PASS
(score 100). Four review findings applied before commit:

- **M1** — the non-vacuous subtest re-implemented the comparison instead of exercising it, so a
  refactor of the main assertions could make the proof vacuous while the test still passed.
  Extracted `missingFromSchema`/`missingFromStruct`/`modeConfigYamlKeys` helpers; both the main
  test and the subtest now call them, so the proof runs through the real code path.
- **M2** — the agreement test is presence-only, but its docstring promised full divergence
  detection. Scoped the comment: key-set presence only; type drift is the round-trip test's job.
- **L1** — the `build` schema description omitted that `native` delegates to
  `interaction.build.replace` and errors if undefined (compose.go:510-524); corrected.
- **L2** — `modeSchemaPropertyKeys` silently kept the last pattern's properties; added an
  exactly-1-pattern guard so a future second pattern fails loudly rather than returning a partial
  answer.

## Notes

Check the rest of `ModeConfig` while here — this sweep found `build`/`run` by exercising
one path, not by comparing the whole struct against the schema. A field-by-field diff of
`ModeConfig` against `schema.json`'s `modes` object is the cheap way to learn whether
these two are the only ones.
