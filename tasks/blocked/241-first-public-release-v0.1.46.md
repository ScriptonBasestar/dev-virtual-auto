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

The release commit is the clean, integrated `master` tip that passes every repository and
six-platform gate. Verify the task branch against the current source, fast-forward and push
`master`, then require `release_commit == master == origin/master` before creating a local
lightweight `v0.1.46` tag. Do not push the tag separately; validate real release-mode artifacts
with `--skip=publish`. Once remote state exists, recovery stays on that exact tag and commit; never
retag a newer commit to conceal a failed attempt. `.goreleaser.yml` retains `target_commitish`,
existing-draft reuse, artifact replacement, and delayed publication so an interrupted upload can be
resumed without moving the tag.

## Completion criteria

- [x] Set runtime version `0.1.46` while preserving scaffold floor `0.1.44` | verify: `go run ./tools/releasecheck version --tag v0.1.46 && test "$(/usr/bin/grep -Fxc 'const MinScaffoldVersion = "0.1.44"' internal/config/version.go)" -eq 1`
- [x] Re-run repository and six-platform release gates at the exact release commit | verify: `dva lint && dva test && dva test integration && make test-skill-dogfood && make doc-check && make commit-check && mise exec -- make release-check`
- [x] Validate the frozen release identity and its integration into the current local and remote source tips | verify: `release_commit=55d9895afa7e57a84b7e0797a657eddd83fc169c && test "$(git rev-list -n1 v0.1.46)" = "$release_commit" && test "$(git cat-file -t v0.1.46)" = commit && git merge-base --is-ancestor "$release_commit" master && git merge-base --is-ancestor "$release_commit" origin/master`
- [x] Validate real release-mode artifacts without publishing, then remove the local artifacts | verify: human — the immutable release commit, build metadata, six archive checksums, and cleanup probes are recorded in Local preparation evidence below
- [x] Prepare reviewed first-public-release notes independently of automatic historical-tag changelog selection | verify: `test "$(shasum -a 256 release-notes/v0.1.46.md | cut -d ' ' -f 1)" = "720ebc330af83b890487f2c9e03ce91b430219b0bbf4b4e6acd27bbb068d96a9"`
- [ ] Confirm the active fine-grained token grants `ScriptonBasestar/dva` Contents read/write and any required organization approval | verify: human — the token holder must confirm repository scope and permission immediately before publication
- [ ] Confirm the authenticated session and absence of conflicting remote state immediately before publication | verify: `test -n "$GITHUB_TOKEN" && gh auth status && test -z "$(git ls-remote --tags origin refs/tags/v0.1.46)" && gh release list --repo ScriptonBasestar/dva --limit 100 --json tagName | jq -e 'all(.[]; .tagName != "v0.1.46")'`
- [ ] Publish through pinned GoReleaser and prove the remote tag, final release, six archives, and checksum asset all match the release identity | verify: human — external publication requires explicit authorization and post-write evidence
- [ ] Add pinned-module and checksum-verifying binary installation documentation only after public assets exist | verify: human — documentation must not advertise assets before publication
- [ ] Record the release URL, published asset checksums, tag/source push state, and final remote probes; remove local `dist/` and `bin/` before archiving | verify: `test ! -e dist && test ! -e bin`

## Local preparation evidence

The release identity is commit `55d9895afa7e57a84b7e0797a657eddd83fc169c`. Before local tag
creation, the reviewed task tip was fast-forwarded to `master`, pushed, and verified equal to
`origin/master`. The local lightweight `v0.1.46` tag points at that same commit. It was not pushed.

That exact commit passed `dva lint`, race/coverage tests, integration tests, built-executable skill
dogfood, documentation and commit gates, and the six-platform snapshot gate. The untagged snapshot
reported `0.0.0-SNAPSHOT-55d9895`; after tagging, the snapshot reported
`0.1.46-SNAPSHOT-55d9895`.

Real release mode ran with `--skip=publish --clean` and an explicit temporary release-notes file.
Its metadata recorded version `0.1.46`, full commit
`55d9895afa7e57a84b7e0797a657eddd83fc169c`, and build date
`2026-08-29T23:15:19.460916+09:00`. The host `darwin/arm64` binary independently reported version
`0.1.46`, that full commit, and UTC build date `2026-08-29T14:15:19Z`.

The six locally verified archive checksums were:

```text
5731c940a6ac95df14614caa6f0f625406e13ee28518be5b23ef3cf88c372661  dva_darwin_amd64.tar.gz
8d7296a8e414ef849064c2312744f5681ca9359405e5f04548891801ecc06172  dva_darwin_arm64.tar.gz
c5ab2c03c3fa73bbc262196212d27df90313dde041119298ac4f99e98b9477c6  dva_linux_amd64.tar.gz
bc2eae5c261ffc24f709e3bdc7e875e710d612fe1b8c73819f2a1cbc3eb916a1  dva_linux_arm64.tar.gz
0748a58dcf5ffc81d546cd5646efca20834ff0392ccc812dc051d51574fd5c4e  dva_windows_amd64.zip
9de85d9b0cf48177272e111a3ff679707d7a62674a74be9f3c68ff8708397c32  dva_windows_arm64.zip
```

Post-build probes still found no remote `v0.1.46` tag and an empty GitHub release list. The temporary
release-notes input, ignored `dist/`, and ignored `bin/` were removed after verification. The task
remains blocked only on explicit human confirmation of the active token's Contents read/write
permission and the external publication/post-publication criteria above.

The reviewed first-public-release notes now live at `release-notes/v0.1.46.md`. They summarize the
full public product surface instead of relying on GoReleaser's automatic changelog range, which may
start at the unpublished local `v0.1.45` tag. Publication must pass this exact reviewed file with
`--release-notes`; because the frozen `v0.1.46` tag predates the notes, use its absolute path from the
clean preparation worktree and verify the recorded SHA-256 immediately before the release command.

This evidence commit advances `master` beyond the fixed release identity. Future publication must
therefore use a new clean detached worktree at local tag `v0.1.46` and require
`HEAD == v0.1.46 == release_commit` before invoking GoReleaser. Do not publish from the newer
`master` tip and do not move the tag.

## Preserved history

TASK-238 records the local `v0.1.45` tag and successful six-platform snapshot and real release-mode
artifact validation. That tag stays at `39019d1e12b1fb131ed773abde18ce7726f44e08`; no remote
`v0.1.45` tag, draft, or release exists. TASK-237 preserves the earlier `v0.1.44` candidate and its
failed GitHub authorization attempt. Neither historical tag is moved, deleted, or published.
