---
id: TASK-302
title: "Publish DVA v0.1.48 with the manual release workflow"
type: release
priority: P1
effort: S
created-at: 2026-09-04T21:20:00+09:00
source: "recommended work order after the autonomous queue emptied"
scope: "v0.1.48 immutable release identity, reviewed notes, manual publication, postflight evidence, and local output cleanup"
status: todo
---

# Task 302: publish DVA v0.1.48

## Decision

Publish `v0.1.48` as the next minor after `v0.1.47`. This ships Stage A of the interaction
`env_file` deprecation, the env-bridge commands gated at `EnvBridgeIntroducedVersion`,
capability-driven init, and cross-project plan composition. `MinScaffoldVersion` remains
`0.1.44`. Do not include the `kubectl` top-level reserved-name promotion (TASK-255/256).

Publication is an explicit external write. Use the documented manual runbook from a clean
detached worktree at the integrated release commit. The credential is supplied only to the
release command; never record its value, a Keychain result, or an authorization header in
this task, a commit, or logs retained as evidence. Do not move, recreate, or separately
push a tag to recover a partial publication.

## Completion criteria

- [ ] Integrate the reviewed release preparation into `master`, push it, and record one immutable
  `release_commit` equal to the local and remote source tip | verify: human — integration evidence
  must name only the commit, not credential material
- [ ] Create a local lightweight `v0.1.48` tag at `release_commit`; use a clean detached worktree
  at that exact identity | verify: `test "$(git cat-file -t v0.1.48)" = commit && test "$(git rev-list -n1 v0.1.48)" = "$RELEASE_COMMIT"`
- [ ] Record the reviewed `release-notes/v0.1.48.md` SHA-256 and pass `make release-preflight`
  with the approved command-scoped credential | verify: human — preflight output must say no remote
  state was created and must not retain credential values
- [ ] Publish once through pinned GoReleaser with the reviewed release notes; if remote state is
  created, recover only at the same immutable tag and commit | verify: human — external publication
  requires final remote evidence
- [ ] Run `make release-clean` and `make release-postflight`; record the final URL, remote tag and
  Release identity, exact seven assets, checksum verification, source push, and local cleanup | verify: human — postflight evidence must omit credential values

## Non-goals

- No `kubectl` route registration.
- No TASK-266 Stage B schema rejection.
- No `MinScaffoldVersion` bump.
