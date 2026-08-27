---
id: TASK-237
title: "Publish the first public v0.1.44 release after GitHub authorization is repaired"
type: release
priority: P1
effort: S
created-at: 2026-08-27T00:00:00+09:00
source: "explicit user approval after TASK-231 snapshot readiness and multi-runtime dogfood"
scope: "release decision record, v0.1.44 tag, GoReleaser publication, artifact verification, and installation documentation"
status: blocked
blocked-on: "The active GITHUB_TOKEN is rejected by the ScriptonBasestar organization because its fine-grained token lifetime exceeds 366 days."
---

# Task 237: publish the first public v0.1.44 release

## Decision

Proceed with option A from TASK-063: publish `v0.1.44`. The user explicitly approved this
direction after the retained GoReleaser configuration passed the six-platform snapshot gate and
the installed binary passed live skill discovery in Codex, Claude Code, Agent Mesh, OpenCode, and
Grok. TASK-063 remains immutable history of the earlier option-B decision; this task records the
later product decision that supersedes it.

Do not create or push the tag while GitHub release authorization is unavailable. A tag without its
release would recreate the split external state this task is intended to close. The current README
and `.goreleaser.yml` snapshot-only note remain truthful until the blocker is removed.

## Unblock condition

Replace the active token with one whose expiration satisfies the organization policy and which has
Contents read/write access to `ScriptonBasestar/dva`. Both commands must succeed before the first
source or tag mutation:

```sh
gh repo view ScriptonBasestar/dva
gh release list --repo ScriptonBasestar/dva
```

## Completion criteria

- [ ] Record an explicit-release policy in `.goreleaser.yml` without implying that CI publishes | verify: `head -5 .goreleaser.yml`
- [ ] Re-run the clean six-platform snapshot gate at the exact release commit | verify: `make release-check`
- [ ] Create and push annotated tag `v0.1.44`, matching `internal/config.Version`, only after API preflight succeeds | verify: `git tag --points-at HEAD | /usr/bin/grep -qx v0.1.44`
- [ ] Publish six archives plus `checksums.txt` through GoReleaser and verify the GitHub release is non-draft | verify: `gh release view v0.1.44 --repo ScriptonBasestar/dva --json isDraft,tagName,assets`
- [ ] Verify the tagged Go module and one host-native archive install path | verify: `GOBIN="$(mktemp -d)" go install github.com/ScriptonBasestar/dva/cmd/dva@v0.1.44`
- [ ] Add pinned-version, checksum-verifying binary installation documentation only after the assets exist | verify: `make doc-check`
- [ ] Record release URL, artifact names/checksums, checks, push state, and final external probes before archiving this card

## Preserved evidence

- DVA release candidate source: `61d101943cc5a480f65715461818f11cdeb28781`
- Installed binary SHA-256: `be7033f2cb8581147b63835cd217a81e97cfd469240ef4cc2ec69f6146277f2e`
- `make release-check`: passed with Darwin/Linux/Windows on amd64/arm64 and `checksums.txt`
- CE/DVA neutral-claim dogfood: passed from the integrated flow devbox target
- GitHub API preflight: blocked before tag creation by the organization token-lifetime policy

These hashes identify the reviewed pre-release state. If `master` advances before unblocking, repeat
all gates and record the actual tagged revision instead of treating these hashes as release pins.
