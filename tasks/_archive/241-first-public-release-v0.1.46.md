---
id: TASK-241
title: "Prepare the first public v0.1.46 release and publish after GitHub authorization is repaired"
type: release
priority: P1
effort: S
created-at: 2026-08-29T23:05:09+09:00
source: "approved recommendation after post-wave doctor and bare lifecycle contract corrections"
scope: "v0.1.46 version identity, repository and release gates, local artifacts, eventual GitHub publication, published-binary dogfood, and post-publication installation documentation"
status: done
started-at: 2026-08-29T23:05:09+09:00
supersedes: TASK-238
completed-at: 2026-08-31T13:56:46+09:00
completion-summary: "Published the immutable v0.1.46 tag and seven verified assets, dogfooded the downloaded host binary across all six skill runtimes, and added pinned/checksum-verifying installation documentation."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "remote tag/release/asset probes; published checksum verification; make dogfood-skill-install; make doc-check; make commit-check"
    result: "remote identity and seven-asset set matched; downloaded archive and binary identity passed; isolated skill dogfood passed"
quality-review: pass
quality-reviewed-at: 2026-08-31T14:58:40+09:00
quality-review-evidence:
  - "Independent final review confirmed the remote tag, final release state, seven-asset set, checksums, archive-to-binary evidence, and absence of credential values."
  - "Review found and the implementation corrected the missing ~/.local/bin creation step before approving the completion archive."
  - "Follow-up review made checksum failure fail closed and documented the Go installation directory instead of assuming it is already on PATH."
  - "The tagged module probe was independently repeated in an isolated GOBIN and reproduced version 0.1.46 with the expected non-GoReleaser metadata."
archived-at: 2026-08-31T13:56:46+09:00
verified-at: 2026-08-31T13:56:46+09:00
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
- [x] Re-run repository and six-platform release gates at the exact release commit | verify: human — the immutable release commit and its successful lint, test, integration, skill-dogfood, documentation, commit, and six-platform gate evidence are recorded below
- [x] Validate the frozen release identity and its integration into the current local and remote source tips | verify: `release_commit=55d9895afa7e57a84b7e0797a657eddd83fc169c && test "$(git rev-list -n1 v0.1.46)" = "$release_commit" && test "$(git cat-file -t v0.1.46)" = commit && git merge-base --is-ancestor "$release_commit" master && git merge-base --is-ancestor "$release_commit" origin/master`
- [x] Validate real release-mode artifacts without publishing, then remove the local artifacts | verify: human — the immutable release commit, build metadata, six archive checksums, and cleanup probes are recorded in Local preparation evidence below
- [x] Prepare reviewed first-public-release notes independently of automatic historical-tag changelog selection | verify: `test "$(shasum -a 256 release-notes/v0.1.46.md | cut -d ' ' -f 1)" = "54ddd64d75fce021672b61dd871b470ef1b717ef03025fc0c5c9ee5b51708ce4"`
- [x] Confirm the active fine-grained token grants `ScriptonBasestar/dva` Contents read/write and any required organization approval | verify: human — the token holder confirmed scope and the non-persisting release-notes API probe requiring Contents write succeeded
- [x] Confirm the authenticated session and absence of conflicting remote state immediately before publication | verify: human — scoped-token authentication, remote tag absence, and release absence were repeated immediately before the successful attempt
- [x] Publish through pinned GoReleaser and prove the remote tag, final release, six archives, and checksum asset all match the release identity | verify: human — the final publication evidence below records the remote identity and complete asset set
- [x] Run the checksum-pinned published host binary through the black-box skill installer dogfood against the clean canonical flow repository | verify: human — the published archive was checksum-verified before extracting and hashing the executable passed to `make dogfood-skill-install`
- [x] Add pinned-module and checksum-verifying binary installation documentation only after public assets exist | verify: `/usr/bin/grep -F 'github.com/ScriptonBasestar/dva/cmd/dva@v0.1.46' README.md USAGE.md && /usr/bin/grep -F 'checksums.txt' README.md USAGE.md`
- [x] Record the release URL, published asset checksums, tag/source push state, and final remote probes; remove local `dist/` and `bin/` before archiving | verify: `test ! -e dist && test ! -e bin`

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

## Publication attempt — 2026-08-31

Publication was attempted from a clean detached worktree at the immutable `v0.1.46` commit with
GoReleaser `2.12.7` and this explicit release-notes file:

```bash
goreleaser release --clean --release-notes /absolute/path/release-notes/v0.1.46.md --verbose
```

GoReleaser successfully built the six platform archives and `checksums.txt`, then GitHub rejected
release creation with `403 Resource not accessible by personal access token`. A post-failure probe
confirmed that neither remote tag `v0.1.46` nor a GitHub release for that tag exists. Local `dist/`
was removed and the detached worktree was removed; the local lightweight tag remains unchanged.

Do not retry with the same credential. The token holder must replace or repair `GITHUB_TOKEN` with
repository-scoped fine-grained **Contents: read/write** permission (and obtain any required
organization approval), then repeat the absence probes and publish from a newly created detached
worktree at this exact tag. This failed authorization does not authorize moving, recreating, or
pushing the tag separately.

A second attempt on 2026-08-31 followed an explicit human statement that the DVA repository
Contents read/write setting was complete. The active credential was still the shell's
`GITHUB_TOKEN`, byte-identical to the login Keychain item whose service is `mise-github-token`;
`gh api user` identified `archmagece`, and the repository metadata reported `permissions.push=true`.
The retry again built all six archives and reached release creation, where GitHub returned the same
`403 Resource not accessible by personal access token`. Immediate probes again found no remote tag
and no release, and the generated `dist/` plus detached worktree were removed. The push permission
probe is therefore not accepted as evidence of Release API authorization. Before another retry,
repair or replace this specific Keychain credential, reload `GITHUB_TOKEN`, and confirm the new
environment value is the intended token.

A third attempt used the newly created, distinct Keychain credential whose service is
`github-token-scriptonbasestar-dva-release`, injected directly as `GITHUB_TOKEN` for both the
preflight and GoReleaser process. It authenticated as `archmagece`, could read
`ScriptonBasestar/dva`, and still received the same `403` at release creation after building all six
archives. `ScriptonBasestar` is an organization, so another retry requires evidence that this
fine-grained token selects `ScriptonBasestar` as its resource owner (or otherwise has approved
access to that organization's repositories), selects the `dva` repository, grants repository
Contents read/write, and is approved by the organization if approval is required.
Post-failure probes again found no remote tag or release; generated artifacts and the detached
worktree were removed.

## Post-publication dogfood policy

The published `checksums.txt` hashes archives, while `dogfood-skill-install` requires the extracted
executable's SHA-256. The verified chain is therefore: verify the downloaded host archive against
the published checksum, extract it, independently record the binary SHA-256, and pass that value as
`DVA_SHA256`. Resolve `FLOW_ROOT` through the portfolio catalog to the clean canonical
`flow-knowchain-devbox` checkout. The target executes an immutable copy of that binary, proves the
real flow tree and runtime paths are unchanged by project-scope dry-run, and exercises all six
runtime installers, shared claims, uninstall, and takeover in isolated HOME/XDG roots.

## Final publication evidence

GoReleaser `2.12.7` published the immutable `v0.1.46` tag on 2026-08-31. The final release is neither
draft nor prerelease and is available at
<https://github.com/ScriptonBasestar/dva/releases/tag/v0.1.46>. The remote tag and release
`target_commitish` both equal `55d9895afa7e57a84b7e0797a657eddd83fc169c`.

The published `checksums.txt` records exactly six archives:

```text
7e2a9498e0a1e738f1fd038e23aa8d430bcb1fb582a6d3c291519b5209fa0afe  dva_darwin_amd64.tar.gz
e98967433617fea36d0b38e6ebe067cd32c1ac14cbd79f7e0a7a7a9b8165c740  dva_darwin_arm64.tar.gz
e56a34663fa50fd44d9152d54eaba08c2f68448fe83851691e3e65446a19f38c  dva_linux_amd64.tar.gz
999af00733860aa281ff653674c064d64acd8b126405feb1009e2ed55cce4638  dva_linux_arm64.tar.gz
9b9f6d7e2b068265679d94183dad632ec00dd2e441de58503f4c184fd3f0bfbb  dva_windows_amd64.zip
a3ff2cbc2f248369ef86b51bc9d60af7b29ebeedda6b06749356af8c285806d0  dva_windows_arm64.zip
```

The downloaded `dva_darwin_arm64.tar.gz` matched its published checksum. Its extracted binary SHA-256
was `5a0aade6fc2cbd3bbb169b127cad3275c126a1d5c4d41782fb33304cd713703e` and reported version
`0.1.46`, the full release commit, and build date `2026-08-31T04:52:10Z`. Portfolio catalog
resolution located the clean canonical `flow-knowchain-devbox` checkout at
`/Users/archmagece/mydevbox/flow-knowchain-devbox`; `develop == origin/develop == 5c5022e` before
and after dogfood. Real-target dry-run preservation, isolated six-runtime install/status/uninstall,
shared-runtime unlink, and takeover lifecycle all passed. Generated `dist/`, `bin/`, `tmp/`, and the
detached release worktree were removed after verification.

An independent post-publication recheck on 2026-08-31 downloaded only the published
`dva_darwin_arm64.tar.gz` and `checksums.txt` into the repository's ignored `tmp/` directory. The
archive again passed `shasum -a 256 -c -`; its extracted executable again reported `0.1.46`, commit
`55d9895afa7e57a84b7e0797a657eddd83fc169c`, and the recorded executable SHA-256. The tagged module
was also installed with `GOPROXY=direct` and `GOSUMDB=sum.golang.org` into isolated `GOBIN`,
`GOMODCACHE`, `GOPATH`, and `GOCACHE` directories under that temporary root. It reported `dva version
0.1.46`. Its `commit: dev` and `build date: unknown` are expected for `go install`, which compiles
the tagged module without the GoReleaser link-time metadata; the separately downloaded release
archive is the artifact that proves the published commit/build identity. No global binary was
installed, and the temporary root was removed after the probes.

## Preserved history

TASK-238 records the local `v0.1.45` tag and successful six-platform snapshot and real release-mode
artifact validation. That tag stays at `39019d1e12b1fb131ed773abde18ce7726f44e08`; no remote
`v0.1.45` tag, draft, or release exists. TASK-237 preserves the earlier `v0.1.44` candidate and its
failed GitHub authorization attempt. Neither historical tag is moved, deleted, or published.
