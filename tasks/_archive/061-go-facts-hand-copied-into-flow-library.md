---
id: TASK-061
title: "Go facts were hand-copied into the AI flow library, and the section order had already drifted"
type: fix
priority: P2
status: done
effort: M
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — tools/libgen (new), Makefile, internal/config/validate_warnings.go, agent-mesh-flows/shared/library/, internal/cli/init.go"
verified-at: 2026-08-03T11:55:33+09:00
archived-at: 2026-08-03T11:55:33+09:00
verification-summary: |
  All 7 criteria MET. The two AUTOGEN blocks in
  agent-mesh-flows/shared/library/shared-guardrails.md are byte-identical to their Go
  sources: :36-39 reserved_commands matches internal/config/reserved.go (27 reserved +
  7 hookable), :53-55 section_order matches the 23 keys of
  validate_warnings.go:20-27 exposed via CanonicalSectionOrder().
  tools/libgen short-circuits to "already up-to-date" when output equals input, and
  replaceBlock returns an error when a marker is missing, so a stale checkout fails
  loudly. Supervisor confirmed idempotency directly: `make check-generate` exits 0 and
  leaves `git status --porcelain` empty after a real generate run — libgen reported
  "already up-to-date".
  The dead //go:embed of library_reference.txt is gone from internal/cli; only the
  unrelated ai_docs.go embed remains, and the build is green.
  One citation has drifted: the task's verify: string cites tools/libgen/main.go:76 for
  the missing-marker error, now at :93 after TASK-067 inserted the version_rule block
  above it. Behaviour is intact; only the line number is stale.
  The deferred Phase 2 (naming presets rule 23, forbidden ports rule 7) was found to
  exist only as README prose with no task tracking it. Filed as TASK-134.
---

# Task 061: Generate the Go-sourced facts in shared-guardrails.md

## Problem

`agent-mesh-flows/shared/library/shared-guardrails.md` is the prompt the AI flows read
before writing a user's `dva.yml`. Two of its rules restated facts that Go already owns:

- **rule 9** — the 27 reserved command names, owned by `internal/config/reserved.go`
- **rule 14** — the canonical top-level section order, owned by
  `internal/config/validate_warnings.go`

Both were hand-transcribed. Nothing compiled the markdown against the Go source, so the
two copies were free to disagree — and one already did.

### Rule 14 had drifted; rule 9 had not

| rule | hand-authored | Go source | verdict |
| --- | --- | --- | --- |
| 9 reserved commands | 27 names | 27 names | identical — no drift yet |
| 14 section order | 17 keys, legacy keys described in prose | 23 keys with fixed positions | **drifted** |

Rule 14 omitted `default_plan` entirely and replaced the six legacy keys' actual
positions (`checks`, `applications`, `default_mode`, `suggestion_ignore`, `modes`) with
the sentence "remain in place during preserve mode". An AI following that prompt could
not reproduce the order `validateCanonicalOrder` checks against, so the generator and the
validator disagreed about the same file.

This is the same defect class as TASK-057: a second copy of knowledge that nothing
compiles. There it was a URL; here it is a list.

## Fix shape

1. Export `CanonicalSectionOrder()` from `internal/config` — `canonicalSectionOrder` was
   already the validator's source, it just had no way out of the package.
2. New `tools/libgen` rewrites the two facts into `<!-- AUTOGEN:NAME:start/end -->`
   marker blocks in `shared-guardrails.md`, reading `ReservedCommands()`,
   `HookableCommands()`, and `CanonicalSectionOrder()`.
3. `make generate` runs libgen first, before the `cat` that builds
   `internal/cli/library_reference.txt`, so the embed-side copy inherits the fix.
4. `make check-generate` also diffs `shared-guardrails.md`, making staleness a CI failure.

## Found while doing it — the embed was dead

`internal/cli/init.go` declared

```go
//go:embed library_reference.txt
var libraryReferenceText string
```

and **nothing referenced it** (`git grep libraryReferenceText` at `9ffa7c8`: one hit, the
declaration). Go only rejects unused *local* variables, so an unused package-level var
compiles forever — the file was baked into every `bin/dva` and read by no one. Removed.

This corrects a claim in [TASK-057](057-dead-self-referencing-urls.md): the dead
`$schema` URL did **not** reach user configs through the binary. The real path is the am
flows reading `shared/library/` (and `library_reference.txt`) from disk at runtime via
`read_file`. The binary was never a link in that chain.

## Non-goals

- Naming presets (rule 23), forbidden ports (rule 7), and `dva-schema.md` have no Go
  source of truth yet — left hand-authored, recorded as Phase 2 in
  `agent-mesh-flows/shared/library/README.md`.
- `AGENTS.md` is only partly generated; it is not added to `.claudeignore`, since that
  would forbid editing its hand-authored half.

## Acceptance criteria

- [x] Reserved list in the markdown is generated from `reserved.go` | verify: `/usr/bin/grep -q 'AUTOGEN:reserved_commands' agent-mesh-flows/shared/library/shared-guardrails.md`
- [x] Section order in the markdown is generated from Go | verify: `/usr/bin/grep -q 'AUTOGEN:section_order' agent-mesh-flows/shared/library/shared-guardrails.md`
- [x] Generated order matches the validator's list exactly | verify: `go run ./tools/libgen && git diff --exit-code agent-mesh-flows/shared/library/shared-guardrails.md`
- [x] `make generate` is idempotent | verify: `make generate && make generate && git diff --stat agent-mesh-flows/shared/library/shared-guardrails.md`
- [x] libgen fails loudly on a missing marker rather than dropping facts | verify: `tools/libgen/main.go:76` returns an error when the marker pair is absent
- [x] Dead embed removed without breaking the build | verify: `make build`
- [x] Full suite green | verify: `make test`

## Result

`make generate` now reports `libgen: ... already up-to-date` on a clean tree, and a second
run produces no diff. `make test` green across all packages; `tools/libgen` compiles as
part of `./...`.

`make check-generate` fails while these files are uncommitted — that is the gate working
as designed: it asserts generated paths are *committed*-fresh, not merely regenerated.

## Evidence

- `git grep -n libraryReferenceText HEAD -- '*.go'` at `9ffa7c8` → single hit, the
  declaration at `internal/cli/init.go:18`.
- `internal/cli/ai_docs.go:11` is the only real `//go:embed` in the package after removal.
- Hand-authored rule 14 (pre-change) listed 17 keys; `CanonicalSectionOrder()` returns 23.
