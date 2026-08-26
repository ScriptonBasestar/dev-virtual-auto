---
id: TASK-231
title: "Validate GoReleaser snapshot artifacts without publishing a release"
type: chore
priority: P1
status: done
effort: S
created-at: 2026-08-26T00:00:00+09:00
decision: "Keep TASK-063's no-public-release decision; make the retained GoReleaser configuration executable as a local and CI snapshot gate."
scope: ".goreleaser.yml, Makefile, CI, pinned GoReleaser tool, tools/releasecheck, and the minimal lifecycle process-group portability boundary"
completed-at: 2026-08-26T18:03:12+09:00
completion-summary: "Exercise release configuration as a non-publishing six-platform snapshot gate and make process-group portability failures explicit."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "dva test && GOOS=windows GOARCH=amd64 go build ./cmd/dva && mise exec -- make release-check && make doc-check && make commit-check"
    result: "passed after rebasing on current master; repository tests, Windows cross-build, six snapshot archives/checksums, executable metadata, documentation, and commit gates exited zero"
quality-review: pass
quality-reviewed-at: 2026-08-26T18:03:12+09:00
quality-review-evidence:
  - "independent ce-judge drove fixes for the CI action contract, Windows PID semantics, exact artifact sets, executable metadata, tagged snapshots, and Git abbreviation parity"
  - "final review found no remaining actionable finding and confirmed core.abbrev=12 snapshot parity"
archived-at: 2026-08-26T18:03:12+09:00
verified-at: 2026-08-26T18:03:12+09:00
verification-summary: "GoReleaser readiness is now tested without tags, write permissions, uploads, or a public release."
---

# Task 231: Validate GoReleaser snapshot readiness

## Decision

TASK-063 remains in force: this task does not create a tag, push a tag, request
`contents: write`, create a GitHub release, or add a release-download URL. It only proves that
the already-retained release configuration produces the six supported platform archives and their
checksums locally and in CI.

Snapshot versions intentionally use GoReleaser's `<base>-SNAPSHOT-<short-commit>` form: an
untagged snapshot has base `0.0.0`, while an exact tag uses its tag version as base. Only a real
exact tag is required to equal `v` plus `internal/config.Version`; an untagged snapshot is not a
public version claim.

During the first actual snapshot, Windows compilation exposed direct Unix `syscall.Kill` and
`SysProcAttr.Setpgid` use in local background lifecycle handling. The fix keeps Unix process-group
semantics intact and rejects that unsupported lifecycle mode explicitly on Windows; it does not
pretend that killing one Windows PID safely tears down its child process tree.

## Acceptance criteria

- [x] `make release-check` validates an exact `vX.Y.Z` tag against `internal/config.Version`, but permits an untagged snapshot | verify: `go run ./tools/releasecheck version --tag v0.1.44 && go run ./tools/releasecheck version`
- [x] Snapshot builds inject Version, Commit, and BuildDate with the same meanings as `make build` | verify: `go run ./tools/releasecheck stamping`
- [x] Snapshot output contains linux, darwin, and windows archives for amd64 and arm64, all covered by `checksums.txt` | verify: `go test ./tools/releasecheck`
- [x] Both the local build and host-runnable snapshot archive report a full 40-hex commit and UTC RFC3339 `Z` BuildDate | verify: `mise exec -- make release-check`
- [x] Windows builds compile and local background process groups fail explicitly rather than silently changing teardown scope | verify: `GOOS=windows GOARCH=amd64 go build ./cmd/dva`
- [x] CI executes the snapshot gate using the pinned GoReleaser version | verify: `/usr/bin/grep -A8 '^  goreleaser-snapshot:' .github/workflows/ci.yml`

## Verification record

Verified locally with GoReleaser 2.12.7. `make release-check` created and checked
`dva_{linux,darwin}_{amd64,arm64}.tar.gz`, `dva_windows_{amd64,arm64}.zip`, and
`checksums.txt`; the darwin/arm64 binary reported snapshot Version, full Commit, and UTC
BuildDate. Independent review also verified the same short-commit contract with a 12-character
Git abbreviation.

The runnable-archive check is intentionally POSIX-host scoped: macOS and Linux use their native
`tar.gz` archive; Windows cross-build output is compiled and checksum-verified but not executed
from those hosts.
