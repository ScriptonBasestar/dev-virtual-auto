---
id: TASK-227
title: "make install deletes a working binary before it has verified the replacement"
type: fix
priority: P1
effort: M
created-at: 2026-08-26T16:10:00+09:00
source: "post-dogfood follow-up review of a033987"
scope: "Makefile install target and its isolated destination fixture"
status: doing
---

# Task 227: install the DVA binary atomically per destination

## Summary

`make install` removed each existing destination before copying the built binary. A copy
or filesystem failure could therefore leave one or both established command paths absent,
and it reported no evidence that the two replacements were the built executable.

## Completion Criteria

- [x] Each destination stages a candidate in its own filesystem, verifies executable bytes
  and version, and replaces its final path using an atomic rename | verify: `go test ./tools/installcheck`
- [x] The default local and Go-bin destinations remain supported, while test fixtures can
  override both without changing a real global installation | verify: `go test ./tools/installcheck`
- [x] A same-path destination installs once; directory and symlink-to-directory targets are
  rejected before either replacement | verify: `go test ./tools/installcheck`
- [x] A failed hasher or copy fails visibly, leaves existing destinations untouched, and removes
  every registered staged candidate | verify: `go test ./tools/installcheck`
- [x] A failed second rename reports which earlier destination was replaced and removes the
  remaining staged candidate | verify: `go test ./tools/installcheck`
- [x] Successful installation verifies both final files against the built binary and reports
  version/commit evidence | verify: `go test ./tools/installcheck`
- [x] The installer-only target uses a disposable prebuilt executable and leaves tracked and
  generated checkout content unchanged; `make help` does not invoke Go-bin resolution | verify: `go test ./tools/installcheck`

## Decision

Use a staged file and `mv` in each final directory. `rename(2)` cannot atomically span two
filesystems, so both candidates are staged and verified before the first replacement; if a
later rename fails, the command prints a replacement ledger rather than claiming a
cross-directory transaction it cannot provide. The two paths are compared after resolving
their directories, so a shared destination receives one rename and one verification. Existing
directory-shaped targets are refused before staging because `mv` would otherwise enter them.

The public `install` target still builds the executable, then invokes the prerequisite-free
`install-binary` target. The latter is intentionally narrow: it permits disposable fixtures to
exercise the installer without generating or otherwise writing checkout source files. Go-bin
lookup occurs only in that target's recipe when neither explicit destination override nor
`GOBIN` is present.
