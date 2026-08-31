---
id: TASK-242
title: "Harden the manual release workflow after the first public launch"
type: chore
priority: P1
effort: M
created-at: 2026-08-31T14:55:16+09:00
source: "approved post-v0.1.46 release review recommendations"
scope: "CHANGELOG release boundary, GoReleaser policy comments, repeatable manual release preflight/postflight, and operator verification"
status: done
started-at: 2026-08-31T14:55:16+09:00
completed-at: 2026-08-31T15:17:55+09:00
completion-summary: "Recorded v0.1.46, codified the current manual publication policy, and added fail-closed preflight, cleanup, and postflight commands with tests."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "make test; make doc-check; make commit-check; git diff --check"
    result: "all checks passed, including race-enabled repository tests and releaseworkflow refusal-path tests"
quality-review: pass
quality-reviewed-at: 2026-08-31T15:17:55+09:00
quality-review-evidence:
  - "Independent security review verified fixed GitHub host, token precedence, origin identity, exact tag/version/commit checks, strict remote absence handling, and non-mutating preflight behavior."
  - "Independent operational review verified the publish, release-clean, and postflight sequence and command-scoped Keychain credential boundary."
archived-at: 2026-08-31T15:17:55+09:00
verified-at: 2026-08-31T15:17:55+09:00
---

# Task 242: harden the manual release workflow

## Decision

Keep publication manual and CI snapshot-only for the next one or two releases. Convert the proven
`v0.1.46` safety checks into a repeatable local command so authentication, immutable tag identity,
remote conflicts, release notes, publication, postflight evidence, and cleanup no longer depend on
an archived task narrative. Use the dedicated release credential only for the release process;
ambient general-purpose GitHub credentials are not accepted as publication evidence.

## Completion criteria

- [x] Record `v0.1.46` as the first public release and leave only post-release work under Unreleased | verify: human — CHANGELOG content must agree with the published release and current repository history
- [x] Replace pre-publication GoReleaser comments with the current approved manual-release policy | verify: human — comments must describe snapshot-only CI and the explicit local publication boundary
- [x] Add a non-secret release preflight/postflight command with immutable-tag, remote-state, write-capability, asset, and cleanup checks | verify: human — tests must cover safe refusal paths without creating a real release
- [x] Expose the workflow through the repository's documented command surface and keep generated help/labels consistent | verify: `make doc-check && make commit-check`
- [x] Independently review the public contract and failure safety, then integrate into the configured source branch and clean the task worktree | verify: human — review and Git evidence are recorded before archive
