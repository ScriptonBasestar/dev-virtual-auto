---
id: TASK-264
title: "Restore imported interaction and provision execution ownership"
type: bug
priority: P0
effort: L
exec-tier: strong
created-at: 2026-09-02T15:50:45+09:00
source: "TASK-247 caller audit against the documented subproject execution contract"
scope: "direct and imported child interactions, imported provision profiles, owner metadata, child vars/env_file and working directory, canonical/alias parity, fixtures"
status: todo
depends-on: [TASK-247]
---

# Task 264: restore imported interaction and provision ownership

## Summary

Restore the documented rule that direct and explicitly imported child interactions and provision profiles
execute against the owning child effective config. TASK-262 repaired plans only; current interaction and
provision paths can still omit child `vars`/`env_file`, inherit parent context, or lose child working-directory
identity.

## Problem

`run --project` and `project:interaction` load the child config but construct an environment without the
child global vars or env files. Parent-visible imported interactions carry only a path hint, while imported
provision profiles are copied as bare step slices with no owner. This contradicts tracked documentation and
prevents TASK-248 from applying required-env policy to the correct project.

## Recommended direction

Carry one private immutable child owner for canonical and alias registrations, select that owner before
environment loading, and reuse the same child-rooted environment for item resolution and execution. Do not
flatten child declarations or expose local absolute owner paths in public discovery output.

## Completion Criteria

- [ ] Add parent/child fixtures proving the current failure for `run --project`, `project:interaction`, imported interaction canonical/alias, and imported provision canonical/alias with conflicting root/child vars, env files and relative assets | verify: `/usr/bin/grep -Eq '^func TestImportedInteractionAndProvisionOwnerIsolation\(' internal/cli/imported_command_owner_test.go && go test ./internal/config ./internal/cli -count=1`
- [ ] Represent imported interaction and provision ownership explicitly so canonical and alias registrations share one child effective config without serializing local owner paths into manifest, show or list output | verify: `go test ./internal/config ./internal/cli -count=1`
- [ ] Direct and imported child interactions use child global vars, top-level environment, `env_file` and resolved item environment; imported provision uses child global vars, top-level environment and `env_file`; both use the child config directory and default working directory, same-named parent values never leak, and root env failure does not block a child route | verify: `go test ./internal/config ./internal/runner ./internal/cli -count=1`
- [ ] Resolve owner and validate missing/ambiguous child configuration before any interaction/provision step starts; preserve literal-key precedence, canonical/alias collision errors, CLI args, runner selection and successful-path output | verify: `go test ./internal/config ./internal/runner ./internal/cli -count=1`
- [ ] Provide an owner-aware environment-loading seam for TASK-248 but preserve the current warning-and-continue failure policy until TASK-248 changes it; do not implement partial status/logs or doctor policy here | verify: `go test ./internal/config ./internal/cli -count=1`
- [ ] Keep absolute and parent-relative child paths valid and resolve script, compose, local and provision assets from the child root | verify: `go test ./internal/config ./internal/runner ./internal/cli -count=1`
- [ ] Update architecture and usage documentation only where verified behavior changes, then pass repository gates and independent review | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No new project grammar, automatic registration or import/export decision.
- No plan ownership change; TASK-262 owns imported plans.
- No required-env failure propagation; TASK-248 owns the TASK-247 route policy.
- No cross-project composition or manifest owner schema.
