---
id: TASK-226
title: "Grandfather two historical installer subjects without moving commit-check baseline"
type: fix
priority: P0
effort: S
created-at: 2026-08-26T16:30:00+09:00
source: "post-TASK-225 repository review: make commit-check reports exactly two post-baseline scope violations"
scope: "tools/commitcheck only; no Git history rewrite and no baseline change"
status: doing
---

# Task 226: grandfather the two historical installer subjects

## Summary

`make commit-check` correctly reads every non-merge commit after baseline
`c100ba06`, but it reports two historical scope-less installer commits:
`d7976538` and `c6ed4eab`. Rewriting published history is disproportionate, and
moving the baseline would stop checking every other commit in the intervening
range.

## Decision

Keep the baseline unchanged. Add only two exact exceptions, each pinned by the
full immutable SHA and its exact subject. The gate must continue to check every
other post-baseline commit. The subject pin is intentionally redundant with a
Git object ID: it makes the exceptional policy auditable and makes a changed
exception table fail its tests.

## Completion Criteria

- [ ] Only `d7976538a9f68dad0c7873ce8c256fb7c60212a0` / `feat: add deterministic skill installer` and `c6ed4eab2750ec4e6aca3e130dfcad61abc3fc6f` / `fix: harden skill installation transactions` are waived | verify: `go test ./tools/commitcheck`
- [ ] A SHA or subject drift, and a future commit with either subject, remain violations | verify: `go test ./tools/commitcheck`
- [ ] The original baseline remains `c100ba06de0e64ebe6079908b8681b993e674a58` | verify: `/usr/bin/grep -Fq 'const baseline = "c100ba06de0e64ebe6079908b8681b993e674a58"' tools/commitcheck/main.go`
- [ ] `make commit-check` is green against the full available history | verify: `make commit-check`

## References

- `tools/commitcheck/main.go` — subject gate and immutable baseline
- `tasks/_archive/225-skill-dogfood-contract-and-hermetic-ci-smoke.md` — installer work that introduced the two historical commits
