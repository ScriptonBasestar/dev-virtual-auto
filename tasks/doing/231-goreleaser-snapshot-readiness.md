---
id: TASK-231
title: "Validate GoReleaser snapshot artifacts without publishing a release"
type: chore
priority: P1
status: doing
effort: S
created-at: 2026-08-26T00:00:00+09:00
decision: "Keep TASK-063's no-public-release decision; make the retained GoReleaser configuration executable as a local and CI snapshot gate."
scope: ".goreleaser.yml, Makefile, CI, pinned GoReleaser tool, tools/releasecheck, and the minimal lifecycle process-group portability boundary"
---

# Task 231: Validate GoReleaser snapshot readiness

## Decision

TASK-063 remains in force: this task does not create a tag, push a tag, request
`contents: write`, create a GitHub release, or add a release-download URL. It only proves that
the already-retained release configuration produces the six supported platform archives and their
checksums locally and in CI.

During the first actual snapshot, Windows compilation exposed direct Unix `syscall.Kill` and
`SysProcAttr.Setpgid` use in local background lifecycle handling. The fix keeps Unix process-group
semantics intact and rejects that unsupported lifecycle mode explicitly on Windows; it does not
pretend that killing one Windows PID safely tears down its child process tree.

## Acceptance criteria

- [x] `make release-check` validates an exact `vX.Y.Z` tag against `internal/config.Version`, but permits an untagged snapshot | verify: `go run ./tools/releasecheck version --tag v0.1.44 && go run ./tools/releasecheck version`
- [x] Snapshot builds inject Version, Commit, and BuildDate with the same meanings as `make build` | verify: `go run ./tools/releasecheck stamping`
- [x] Snapshot output contains linux, darwin, and windows archives for amd64 and arm64, all covered by `checksums.txt` | verify: `go test ./tools/releasecheck`
- [x] Windows builds compile and local background process groups fail explicitly rather than silently changing teardown scope | verify: `GOOS=windows GOARCH=amd64 go build ./cmd/dva`
- [x] CI executes the snapshot gate using the pinned GoReleaser version | verify: `/usr/bin/grep -A8 '^  goreleaser-snapshot:' .github/workflows/ci.yml`

## Verification record

Verified locally with GoReleaser 2.12.7. `make release-check` created and checked
`dva_{linux,darwin}_{amd64,arm64}.tar.gz`, `dva_windows_{amd64,arm64}.zip`, and
`checksums.txt`; the darwin/arm64 binary reported snapshot Version, full Commit, and UTC
BuildDate. The task remains `doing` until independent review and integration.
