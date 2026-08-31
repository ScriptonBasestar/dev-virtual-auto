---
id: TASK-243
title: "Publish DVA v0.1.47 with the hardened manual release workflow"
type: release
priority: P1
effort: S
created-at: 2026-08-31T17:43:09+09:00
source: "approved immediate v0.1.47 release decision"
scope: "v0.1.47 immutable release identity, reviewed notes, manual publication, postflight evidence, and local output cleanup"
status: done
started-at: 2026-08-31T17:43:09+09:00
completed-at: 2026-08-31T17:50:37+09:00
completion-summary: "Published immutable v0.1.47 at the release commit through the hardened manual workflow; remote identity, seven assets, and checksums were postflight-verified without retaining credential material."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "make release-preflight; manual GoReleaser publication; make release-clean; make release-postflight; remote tag/release/asset probes"
    result: "all commands passed; the final remote tag and release target matched the immutable commit, exactly seven assets were present, and published checksum verification passed"
archived-at: 2026-08-31T17:50:37+09:00
verified-at: 2026-08-31T17:50:37+09:00
---

# Task 243: publish DVA v0.1.47

## Decision

Publish `v0.1.47` as a patch release for the proven fail-closed manual publication workflow.
`MinScaffoldVersion` remains `0.1.44`: this release changes release-operation safeguards only and
does not require a newer generated `dva.yml` syntax.

Publication is an explicit external write. Use the documented manual runbook from a clean detached
worktree at the integrated release commit. The credential is supplied only to the release command;
never record its value, a Keychain result, or an authorization header in this task, a commit, or
logs retained as evidence. Do not move, recreate, or separately push a tag to recover a partial
publication.

## Completion criteria

- [x] Integrate the reviewed release preparation into `master`, push it, and record one immutable
  `release_commit` equal to the local and remote source tip | verify: human — integration evidence
  must name only the commit, not credential material
- [x] Create a local lightweight `v0.1.47` tag at `release_commit`; use a clean detached worktree
  at that exact identity | verify: `test "$(git cat-file -t v0.1.47)" = commit && test "$(git rev-list -n1 v0.1.47)" = "$RELEASE_COMMIT"`
- [x] Record the reviewed `release-notes/v0.1.47.md` SHA-256 and pass `make release-preflight`
  with the approved command-scoped credential | verify: human — preflight output must say no remote
  state was created and must not retain credential values
- [x] Publish once through pinned GoReleaser with the reviewed release notes; if remote state is
  created, recover only at the same immutable tag and commit | verify: human — external publication
  requires final remote evidence
- [x] Run `make release-clean` and `make release-postflight`; record the final URL, remote tag and
  Release identity, exact seven assets, checksum verification, source push, and local cleanup | verify: human — postflight evidence must omit credential values

## Publication evidence

The immutable release commit is `eb03aaaf65ad24aefc5ac7da15404a5bfa401c0a`. It was integrated
and pushed as `master` before publication; the remote lightweight `v0.1.47` tag and the final
GitHub Release `target_commitish` both resolve to that same commit.

The reviewed release notes digest is
`fa8c6c12207ed117d541cb21e5a15ad52419516dbe53e6f138d4085080c2aa56` (SHA-256). The
documented `make release-preflight`, publication, `make release-clean`, and
`make release-postflight` sequence completed successfully. No credential value, credential lookup,
request header, or shell environment is retained here.

The final, non-draft, non-prerelease release is
<https://github.com/ScriptonBasestar/dva/releases/tag/v0.1.47>. Postflight confirmed exactly these
seven assets and verified the published archives against `checksums.txt`:

```text
checksums.txt
dva_darwin_amd64.tar.gz
dva_darwin_arm64.tar.gz
dva_linux_amd64.tar.gz
dva_linux_arm64.tar.gz
dva_windows_amd64.zip
dva_windows_arm64.zip
```

`make release-clean` removed the local release output after verification.
