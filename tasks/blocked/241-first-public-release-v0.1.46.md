---
id: TASK-241
title: "Prepare the first public v0.1.46 release and publish after GitHub authorization is repaired"
type: release
priority: P1
effort: S
created-at: 2026-08-29T23:05:09+09:00
source: "approved recommendation after post-wave doctor and bare lifecycle contract corrections"
scope: "v0.1.46 version identity, repository and release gates, local artifacts, eventual GitHub publication, and post-publication installation documentation"
status: blocked
started-at: 2026-08-29T23:05:09+09:00
blocked-at: 2026-08-29T23:05:09+09:00
blocked-on: "Public publication requires a human-confirmed fine-grained GITHUB_TOKEN with Contents read/write; local release preparation may proceed without it."
supersedes: TASK-238
---

# Task 241: prepare and publish the first public v0.1.46 release

## Decision

Prepare `v0.1.46` as the first public candidate rather than publishing the frozen, unpublished
`v0.1.45` tag. Keep local `v0.1.45` at its original commit and never delete, move, or push it.
No remote DVA tag or GitHub release exists at task creation. The new candidate includes the optional
env-file doctor correction and the accurate bare lifecycle contract integrated after `v0.1.45`.

`MinScaffoldVersion` remains `0.1.44`: these changes correct runtime diagnosis, tests, and public
documentation without changing the minimum configuration syntax emitted by `dva init`. Only
`config.Version` advances to `0.1.46`.

Public release publication is a separate external write. Before publication, a human must confirm
that the active fine-grained token has **Contents: read/write**, and the remote tag/release absence
probes must be repeated. Local preparation does not authorize a tag push, GitHub release, global
binary installation, or post-publication documentation that advertises assets which do not exist.

## Release identity and retry policy

The release commit is the clean task tip that passes every repository and six-platform gate. Create
a local lightweight `v0.1.46` tag at exactly that commit, do not push it separately, and validate
real release-mode artifacts with `--skip=publish`. Once remote state exists, recovery stays on that
exact tag and commit; never retag a newer commit to conceal a failed attempt. `.goreleaser.yml`
retains `target_commitish`, existing-draft reuse, artifact replacement, and delayed publication so
an interrupted upload can be resumed without moving the tag.

## Completion criteria

- [ ] Set runtime version `0.1.46` while preserving scaffold floor `0.1.44` | verify: `go run ./tools/releasecheck version --tag v0.1.46 && test "$(/usr/bin/grep -Fxc 'const MinScaffoldVersion = "0.1.44"' internal/config/version.go)" -eq 1`
- [ ] Re-run repository and six-platform release gates at the exact release commit | verify: `dva lint && dva test && dva test integration && make test-skill-dogfood && make doc-check && make commit-check && mise exec -- make release-check`
- [ ] Validate a local lightweight tag and real release-mode artifacts without publishing | verify: `test "$(git cat-file -t v0.1.46)" = commit && go run ./tools/releasecheck version --tag "$(git describe --tags --exact-match)" && mise exec -- goreleaser release --skip=publish --clean && go run ./tools/releasecheck artifacts --dist dist`
- [ ] Confirm GitHub write authorization and absence of conflicting remote state immediately before publication | verify: `test -n "$GITHUB_TOKEN" && test -z "$(git ls-remote --tags origin refs/tags/v0.1.46)" && gh release list --repo ScriptonBasestar/dva --limit 100 --json tagName | jq -e 'all(.[]; .tagName != "v0.1.46")'`
- [ ] Publish through pinned GoReleaser and prove the remote tag, final release, six archives, and checksum asset all match the release identity | verify: human — external publication requires explicit authorization and post-write evidence
- [ ] Add pinned-module and checksum-verifying binary installation documentation only after public assets exist | verify: human — documentation must not advertise assets before publication
- [ ] Record the release commit, artifact checks, final probes, and cleanup; remove local `dist/` and `bin/` before eventual archiving | verify: `test ! -e dist && test ! -e bin`

## Preserved history

TASK-238 records the local `v0.1.45` tag and successful six-platform snapshot and real release-mode
artifact validation. That tag stays at `39019d1e12b1fb131ed773abde18ce7726f44e08`; no remote
`v0.1.45` tag, draft, or release exists. TASK-237 preserves the earlier `v0.1.44` candidate and its
failed GitHub authorization attempt. Neither historical tag is moved, deleted, or published.
