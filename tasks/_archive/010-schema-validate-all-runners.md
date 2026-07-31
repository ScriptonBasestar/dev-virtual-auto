---
id: TASK-010
title: "Validate every runner in schema.json, not only compose"
type: bug
priority: P1
status: done
archived-at: 2026-07-16T21:15:00+09:00
verified-at: 2026-07-16T21:15:00+09:00
verification-summary: >-
  Verified by orchestrator in BOTH directions. Rejects: unknown runner name, unknown key
  in a non-compose runner, and whitespace-padded runner keys. Accepts: valid script config,
  each runner's real fields (process/helm/kubectl probed), and all 18 shipped examples
  (TestExamplesValidateAgainstSchema passes unmodified). make test and go vet green.
  16 keys declared for 15 runners (podman-compose and podman_compose both accepted, per
  normalizeRunnerName). runners.additionalProperties is now false.
  A test assertion was widened, not weakened: the whitespace case is now rejected by the
  closed runners map before reaching the Go-side check, so the test accepts either layer's
  message while still requiring an error - independently re-proven above.
  Not treated as a finding: validate.go:98's whitespace check is now unreachable for
  YAML-loaded configs, but Validate() also runs on programmatically built configs, so it
  remains defense in depth rather than dead code. Left in place deliberately.
effort: M
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G2
source-severity: HIGH
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
depends-on: [TASK-009]
---

# Task 010: Schema Must Validate All Runners

## Summary

`schema.json` defines only the `compose` runner under `stack.<entry>.runners`, with
`additionalProperties: true`. Every other runner (`script`, `process`, `kubectl`,
`helm`, …) therefore accepts arbitrary content and reports `✅ dva.yml is valid`,
which is exactly why TASK-009's runtime failure is silent.

## Evidence

`internal/config/schema.json` — `stack.<entry>.runners`:

- `properties` = `['compose']`
- `additionalProperties` = `true`

Observed asymmetry (same bogus keys in both):

```
runners.compose → ERROR: stack.web.runners.compose: Additional property up is not allowed
runners.script  → ✅ dva.yml is valid          (then fails at runtime)
```

14 plugins are implemented (`internal/lifecycle/registry.go` reports: compose, process,
docker, podman-compose, kubectl, skaffold, serverless, script, helm, kustomize, vagrant,
tilt, sam, multipass) but only 1/14 is schema-covered.

This is a separate defect from TASK-009: TASK-009 is resolution, this is validation.
Fixing only one leaves either a broken runtime or a blind gate.

## Criteria revised after TASK-009 (2026-07-16)

The original criterion asserted "a schema for every implemented plugin (>=14)". TASK-009's
independent review proved that assumption wrong, so it was corrected before implementation:

- `decodeRunnerNode` (`internal/config/lifecycle.go:255`) accepts **15** names — the 14
  registry plugins **plus `native`**, which is not a plugin at all.
- `native` and `docker` decode to **application-style** shapes (`NativeRunnerConfig`:
  dir/build/run/env at `lifecycle.go:68`; `DockerRunnerConfig`: image/run/build/command at
  `lifecycle.go:75`), not stack plugin configs. `NativeRunnerConfig` is consumed only by the
  plan path for `WorkingDir` (`internal/lifecycle/resolver.go:219`); `DockerRunnerConfig` is
  consumed by nothing.
- So the runner set is **not** simply "the plugin registry". Asserting `>=14` would have
  forced a schema that contradicts the decoder.

The criteria now assert the schema matches **what the decoder actually accepts**, and that
unknown runner names and unknown keys are rejected. Whether `runners.docker` should map to
the docker plugin is a **design decision**, deliberately excluded — see TASK-017.

## Out Of Scope

- Plugin resolution behavior (TASK-009).
- Adding new plugins.
- Deciding `runners.docker` / `runners.native` semantics for the stack path (TASK-017).

## Completion Criteria

- [x] `runners` declares a schema for every runner name `decodeRunnerNode` accepts, and no declared name is missing | verify: `python3 -c "import json,re;s=json.load(open('internal/config/schema.json'));r=s['properties']['stack']['additionalProperties']['properties']['runners'];ks=set(r.get('properties',{}));src=open('internal/config/lifecycle.go').read();body=src[src.index('func decodeRunnerNode'):];body=body[:body.index('\ndefault:')] if '\ndefault:' in body else body;want={m for m in re.findall(r'case \"([a-z_-]+)\":',body)};missing=want-ks;print('declared:',sorted(ks));assert not missing, f'missing: {sorted(missing)}'"`
- [x] An unknown runner name is rejected instead of silently accepted | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\nstack:\n  web:\n    runners:\n      not_a_runner:\n        up: "true"\n' > dva.yml && ! "$OLDPWD/bin/dva" validate`
- [x] Unknown keys inside a non-compose runner are rejected rather than silently accepted | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\nstack:\n  web:\n    default_runner: script\n    runners:\n      script:\n        bogus_key_xyz: 1\n' > dva.yml && ! "$OLDPWD/bin/dva" validate`
- [x] All shipped examples still validate against the tightened schema | verify: `go test ./internal/config/ -run 'Example|Schema' -v`
- [x] Full suite and vet stay green | verify: `make test && go vet ./...`

## Dependencies

- TASK-009 — the schema must encode the resolution contract that task establishes.

## References

- `unified.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — G2
- `code-to-doc.md` (gap-analysis run `20260716T091912Z-73dc094`, untracked) — C3
