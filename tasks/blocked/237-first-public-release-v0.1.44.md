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

Do not create or push the tag while GitHub release authorization is unavailable. Tag creation,
draft creation, asset uploads, and publication are separate external operations; no procedure can
make them atomic. Minimize and make that partial state recoverable rather than claiming it cannot
happen. The current README and `.goreleaser.yml` snapshot-only note remain truthful until the
blocker is removed.

## Unblock condition

Replace the active token with one whose expiration satisfies the organization policy. A human must
confirm in the GitHub token settings that it grants Contents read/write for
`ScriptonBasestar/dva`; the read-only probes below do not prove write access. Do not record the
token or credential details in this card. These commands must succeed immediately before the
first external mutation:

```sh
gh repo view ScriptonBasestar/dva
remote_tag="$(git ls-remote --tags origin refs/tags/v0.1.44)"
test -z "$remote_tag"
release_json="$(gh release list --repo ScriptonBasestar/dva --limit 100 --json tagName)"
printf '%s\n' "$release_json" | jq -e 'all(.[]; .tagName != "v0.1.44")'
```

## Publication procedure

Prepare the final release commit first. Change `.goreleaser.yml` from its snapshot-only policy to
an explicit publication policy with `release.target_commitish: "{{ .Commit }}"`,
`use_existing_draft: true`, and `replace_existing_artifacts: true`. Keep `draft: false`: GoReleaser
still creates a GitHub draft while uploading and publishes it only after the uploads complete.
These settings let the pinned GoReleaser 2.12.7 create the remote tag at the exact release commit
and resume an interrupted draft without deleting or moving the tag.

After committing and verifying that tree, record `release_commit="$(git rev-parse HEAD)"` before
creating the tag. Treat that value as the publication identity; later documentation commits may
move `HEAD`. Every local and remote tag comparison must resolve `v0.1.44` back to that recorded
commit rather than comparing it with whichever commit happens to be checked out later.

Create a **local lightweight** `v0.1.44` tag at the clean release commit; do not push it separately.
An annotated local tag would not match the commit ref GitHub creates from `target_commitish`.
Before any external write, exercise real release mode locally—not snapshot mode—and validate its
outputs:

```sh
git tag v0.1.44 "$release_commit"
goreleaser release --skip=publish --clean
go run ./tools/releasecheck artifacts --dist dist
```

If this local-only validation requires a source correction, first prove that both remote probes
still report no tag or release, delete only the unpublished local tag, make and verify the final
commit, and create the local tag again. Once any remote state exists, the tag is immutable for
this procedure and recovery stays on that exact commit and tag.

After repeating the unblock probes, run pinned `goreleaser release --clean`. Publication is still
not atomic: interruption can leave a draft and its tag. On failure, inspect the remote tag and
draft, fix authorization or connectivity, and rerun for the **same** local tag. Never delete,
repoint, or recreate a public release tag to conceal a failed attempt. The existing-draft and
artifact-replacement settings make that retry the declared recovery path. Escalate instead of
inventing a different tag if the remote tag points anywhere other than the intended release commit.

## Completion criteria

- [ ] Record the explicit publication and retry policy in `.goreleaser.yml` without implying that CI publishes | verify: `goreleaser check && test "$(/usr/bin/grep -Fxc '  target_commitish: "{{ .Commit }}"' .goreleaser.yml)" -eq 1 && test "$(/usr/bin/grep -Fxc '  use_existing_draft: true' .goreleaser.yml)" -eq 1 && test "$(/usr/bin/grep -Fxc '  replace_existing_artifacts: true' .goreleaser.yml)" -eq 1 && test "$(/usr/bin/grep -Fxc '  draft: false' .goreleaser.yml)" -eq 1`
- [ ] Re-run all repository and six-platform release gates at the exact release commit | verify: `dva lint && dva test && dva test integration && make test-skill-dogfood && make doc-check && make commit-check && mise exec -- make release-check`
- [ ] Validate a local lightweight tag against `internal/config.Version` and build real release-mode artifacts without publishing | verify: `test "$(git cat-file -t v0.1.44)" = commit && go run ./tools/releasecheck version --tag "$(git describe --tags --exact-match)" && goreleaser release --skip=publish --clean && go run ./tools/releasecheck artifacts --dist dist`
- [ ] Publish through GoReleaser, then prove the remote tag resolves to the tagged release commit and the release has exactly six archives plus `checksums.txt` | verify: `release_commit="$(git rev-list -n1 v0.1.44)" && test -n "$release_commit" && remote_tag="$(git ls-remote origin refs/tags/v0.1.44 | cut -f1)" && test "$remote_tag" = "$release_commit" && gh release view v0.1.44 --repo ScriptonBasestar/dva --json isDraft,tagName,assets | jq -e '(.isDraft == false) and (.tagName == "v0.1.44") and (([.assets[].name] | sort) == (["checksums.txt","dva_darwin_amd64.tar.gz","dva_darwin_arm64.tar.gz","dva_linux_amd64.tar.gz","dva_linux_arm64.tar.gz","dva_windows_amd64.zip","dva_windows_arm64.zip"] | sort))'`
- [ ] Verify immediate tagged-module installation directly from GitHub and execute the installed binary | verify: `tmp_bin="$(mktemp -d)"; trap 'rm -rf "$tmp_bin"' EXIT; GOPROXY=direct GOBIN="$tmp_bin" go install github.com/ScriptonBasestar/dva/cmd/dva@v0.1.44 && "$tmp_bin/dva" version | /usr/bin/grep -q '^dva version 0.1.44$'`
- [ ] Add pinned-version, checksum-verifying binary installation documentation only after the assets exist | verify: `/usr/bin/grep -Fq 'github.com/ScriptonBasestar/dva/cmd/dva@v0.1.44' README.md && /usr/bin/grep -Fq '/releases/download/v0.1.44/checksums.txt' README.md && /usr/bin/grep -Eq 'sha256sum|shasum -a 256' README.md && make doc-check`
- [ ] Record release URL, artifact names/checksums, checks, push state, and final external probes; then remove the local `dist/` build output before archiving this card | verify: `test ! -e dist`

## Preserved evidence

- Live-runtime-tested baseline source: `61d101943cc5a480f65715461818f11cdeb28781`
- Installed binary SHA-256: `be7033f2cb8581147b63835cd217a81e97cfd469240ef4cc2ec69f6146277f2e`
- `make release-check`: passed with Darwin/Linux/Windows on amd64/arm64 and `checksums.txt`
- CE/DVA neutral-claim dogfood: passed from the integrated flow devbox target
- Antigravity user projection: receipt, 12-file manifest, neutral claim, and bundle digest passed
  independent review; native IDE discovery remains unverified because no Antigravity host is
  installed, so this is not evidence of a live runtime pass
- GitHub API preflight: blocked before tag creation by the organization token-lifetime policy

These hashes identify the reviewed pre-release state, not the future release commit. If `master`
advances before unblocking, repeat all gates and record the actual tagged revision instead of
treating these hashes as release pins. `GOPROXY=direct` is intentional for the immediate module
probe; the public proxy is the normal user path but may lag the new tag and can be checked later.
