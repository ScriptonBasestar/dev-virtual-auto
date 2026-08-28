---
id: TASK-238
title: "Prepare the first public v0.1.45 release and publish after GitHub authorization is repaired"
type: release
priority: P1
effort: S
created-at: 2026-08-28T23:38:29+09:00
source: "explicit user approval of the recommended v0.1.45 direction after DVA wave follow-up review"
scope: "v0.1.45 version identity, repository and release gates, local artifacts, eventual GitHub publication, and post-publication installation documentation"
status: blocked
started-at: 2026-08-28T23:38:29+09:00
blocked-at: 2026-08-28T23:38:29+09:00
blocked-on: "Public publication still requires a human-confirmed fine-grained GITHUB_TOKEN with Contents read/write; local release preparation may proceed without it."
supersedes: TASK-237
---

# Task 238: prepare and publish the first public v0.1.45 release

## Decision

Release `v0.1.45` instead of retrying the unpublished `v0.1.44` candidate. The old local tag stays
at its original commit and is never deleted, moved, or pushed. No remote DVA tag or GitHub release
exists at task creation. The new candidate includes the post-tag validator corrections and the
effective default-plan output contract used to verify the completed DVA waves.

`MinScaffoldVersion` remains `0.1.44`: this patch release changes CLI observability and validation,
not the minimum configuration syntax emitted by `dva init`. Only `config.Version` advances to
`0.1.45`.

Public release publication remains a separate external write. It is not authorized by approval of
local preparation alone. Before publication, a human must confirm that the active fine-grained
token has **Contents: read/write**, and the remote tag/release absence probes must be repeated.

## Release identity and retry policy

The release commit is the clean, integrated `master` tip that passes every repository and
six-platform gate. Create a local lightweight `v0.1.45` tag at exactly that commit, do not push the
tag separately, and validate real release-mode artifacts with `--skip=publish`. Once remote state
exists, recovery stays on that exact tag and commit; never retag a newer commit to conceal a failed
attempt. `.goreleaser.yml` retains `target_commitish`, existing-draft reuse, artifact replacement,
and delayed publication so an interrupted upload can be resumed without moving the tag.

## Completion criteria

- [ ] Set runtime version `0.1.45` while preserving scaffold floor `0.1.44` | verify: `go run ./tools/releasecheck version --tag v0.1.45 && test "$(/usr/bin/grep -Fxc 'const MinScaffoldVersion = "0.1.44"' internal/config/version.go)" -eq 1`
- [ ] Re-run repository and six-platform release gates at the exact release commit | verify: `dva lint && dva test && dva test integration && make test-skill-dogfood && make doc-check && make commit-check && mise exec -- make release-check`
- [ ] Validate a local lightweight tag and real release-mode artifacts without publishing | verify: `test "$(git cat-file -t v0.1.45)" = commit && go run ./tools/releasecheck version --tag "$(git describe --tags --exact-match)" && mise exec -- goreleaser release --skip=publish --clean && go run ./tools/releasecheck artifacts --dist dist`
- [ ] Confirm GitHub write authorization and absence of conflicting remote state immediately before publication | verify: `test -n "$GITHUB_TOKEN" && test -z "$(git ls-remote --tags origin refs/tags/v0.1.45)" && gh release list --repo ScriptonBasestar/dva --limit 100 --json tagName | jq -e 'all(.[]; .tagName != "v0.1.45")'`
- [ ] Publish through pinned GoReleaser and prove the remote tag, final release, six archives, and checksum asset all match the release identity | verify: human — external publication requires explicit authorization and post-write evidence
- [ ] Add pinned-module and checksum-verifying binary installation documentation only after public assets exist | verify: human — documentation must not advertise assets before publication
- [ ] Record the release URL, artifact checksums, checks, push state, and final probes; remove local `dist/` before archiving | verify: `test ! -e dist`

## Preserved history

TASK-237 records the earlier `v0.1.44` local artifact validation and the GitHub `403` response. Its
local tag remains at `49d444cc8f4fb128c61b00f6789b577545bc7a21`; post-failure probes found no
remote tag, draft, or published release. That evidence is retained rather than rewritten as a
successful or deleted attempt.
