---
id: TASK-235
title: "Shared skill roots have no producer-neutral claim or safe takeover path"
type: feature
priority: P1
effort: L
created-at: 2026-08-26T19:00:00+09:00
source: "multi-installer ownership review"
scope: "skill claim protocol, DVA installer takeover/restore, status, CLI, tests, and user documentation"
status: done
---

# Task 235: producer-neutral skill claims and explicit takeover

## Summary

Codex and Antigravity share `~/.agents/skills`, but neither the directory nor every skill in it can
belong to DVA or ce-agent-kit. Ownership must be claimed per top-level installed skill by an arbitrary
producer. Receipt-less DVA-name collisions remain foreign until the user explicitly requests takeover.

## Completion Criteria

- [x] A versioned producer-neutral XDG claim records one canonical top-level skill destination,
  arbitrary producer ID, format, scope, consumers, source digest, and installed file hashes; malformed,
  symlinked, or another-producer claims fail closed | verify: `go test ./internal/skillclaim`
- [x] DVA publishes/removes only its `dva` and `dva-config` claims and coexists with unrelated claims
  in the same runtime root | verify: `go test ./internal/skillinstall -run 'Claim|Coexist'`
- [x] Default install still rejects receipt-less collisions, while `skill install --takeover` affects
  only requested DVA names, rejects symlinks/special files/other producer claims, and preserves exact
  originals in durable state before replacement | verify: `go test ./internal/skillinstall -run Takeover`
- [x] Takeover dry-run is mutation-free and reports backup intent; status reports takeover backup
  availability or corruption | verify: `go test ./internal/skillinstall -run 'Takeover|Status'`
- [x] Ordinary uninstall removes only verified DVA files and keeps the backup; explicit
  `skill uninstall --restore-takeover-backup` verifies both sides and restores the original | verify:
  `go test ./internal/skillinstall -run RestoreTakeover`
- [x] Receipt schema 1/2 compatibility remains, schema 3 strictly binds takeover metadata, and
  transaction failures roll back or retain a reported recovery artifact | verify: `go test ./internal/skillinstall`
- [x] CLI help/manifest and canonical usage docs describe option boundaries, non-automatic restore,
  claim interoperability, and crash/cross-filesystem limitations | verify: `go test ./internal/cli && make doc-check`
- [x] Repository test, generation, dogfood, and commit gates pass | verify: `make test && make check-generate && make test-skill-dogfood && make commit-check`

## Decision

Use `$XDG_STATE_HOME/agent-skills/claims/v1/<sha256(canonical-skill-destination)>.json` as a neutral
per-skill claim namespace. No producer owns the shared root; a claim authorizes only its exact top-level
destination. DVA uses producer `dva` and keeps its existing destination receipt as the transaction
record. `--takeover` never infers ownership from content, names, or permissions: an existing foreign
claim is refused, while an unclaimed regular DVA-name tree/file is durably backed up and then replaced.
Restore is explicit, never an automatic side effect of ordinary uninstall.
Takeover first atomically renames each live foreign entry into a same-filesystem capture stage, then
creates the durable backup from that immutable snapshot. A failed or uncertain rollback retains both
available recovery artifacts. Preflight spans requested destinations, but late operational failures do
not provide a cross-destination atomic commit; rerunning the command is the convergence mechanism.
