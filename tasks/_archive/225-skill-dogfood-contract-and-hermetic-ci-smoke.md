---
id: TASK-225
title: "The skill-installer dogfood contract excludes stable dirty targets and CI never executes the black-box gate"
type: improvement
priority: P1
effort: M
created-at: 2026-08-26T15:45:00+09:00
source: "post-dogfood review of ada6952 and a successful installed-binary run against flow-knowchain-devbox"
scope: "tools/skilldogfood, its Make targets and workflow documentation, and the existing CI test job"
status: done
completed-at: 2026-08-26T15:55:18+09:00
completion-summary: "Accept stable dirty flow roots while hashing Git-visible content, preserving known ignored runtime paths, and exercising the built installer in hermetic CI."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "make test && make lint && make doc-check && make test-integration && make test-skill-dogfood && make check-generate"
    result: "passed; race/coverage tests, vet, formatting, linters, documentation/CI/flow gates, integration tests, hermetic built-binary round-trip, and generated-file check all exited zero"
  - kind: runtime
    command-or-step: "make dogfood-skill-install with the SHA-pinned 3018fe0 executable against flow-knowchain-devbox"
    result: "passed with real_target_dry_run, fixture_round_trip, and shared_runtime_unlink; target remained unchanged"
quality-review: pass
quality-reviewed-at: 2026-08-26T15:55:18+09:00
quality-review-evidence:
  - "independent ce-judge found and verified fixes for dirty-content mutation detection, receipt wording, bounded diff hashing, and empty runtime ancestor creation"
  - "final review reported no actionable findings after focused tests, hermetic dogfood, doccheck, formatting, and diff checks passed"
archived-at: 2026-08-26T15:55:18+09:00
verified-at: 2026-08-26T15:55:18+09:00
verification-summary: "The public contract, real-flow acceptance, hermetic CI smoke, and dirty-target safety checks agree and pass."
---

# Task 225: make the skill dogfood contract match the workflow

## Summary

The numbered workflow permits a target with pre-existing changes when its Git state
is stable across the dry-run. The black-box helper instead rejects every dirty target
before it can prove that property. Its public wording also says "installed binary"
although stage 30 may deliberately select a SHA-pinned candidate archive.

The black-box gate is available to maintainers but is not exercised by CI. Add one
hermetic smoke that builds the current source executable and runs the same installer
round-trip against a temporary isolated Git repository. It must not depend on any
developer's `~/mydevbox` checkout or derive evidence from globally installed state.

## Completion Criteria

- [x] A repository root with stable pre-existing changes is accepted, while any Git-status or runtime-path change made by the project-scope dry-run still fails | verify: `go test ./tools/skilldogfood`
- [x] User-facing flags, Make help, and workflow documentation consistently describe a selected SHA-pinned executable rather than requiring a global install | verify: `make doc-check`
- [x] Maintainer documentation gives the exact real-flow command, explains externally supplied `DVA_SHA256`, POSIX support, and receipt schema 1 checks | verify: `make doc-check`
- [x] CI runs the built executable through the same black-box round-trip in a temporary Git repository without external flow dependencies | verify: `make test-skill-dogfood`
- [x] Full repository gates pass | verify: `make test && make lint && make doc-check && make test-integration`

## Decision

Keep `DVA_SHA256` caller-supplied for real-flow acceptance. Automatically deriving it
inside the helper would only prove that a file equals itself and would erase the
stage's independent artifact selection boundary. The hermetic CI smoke may compute
the digest of the binary it just built because its purpose is behavior coverage, not
release or installed-artifact provenance.
