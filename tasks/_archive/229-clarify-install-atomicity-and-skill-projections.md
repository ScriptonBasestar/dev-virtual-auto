---
id: TASK-229
title: "Install help and README blur per-destination atomicity with checkout projections and runtime installation"
type: docs
priority: P1
effort: S
created-at: 2026-08-26T17:08:46+09:00
source: "post-TASK-228 review"
scope: "Makefile install help and README LLM integration contract"
status: done
completed-at: 2026-08-26T17:14:18+09:00
completion-summary: "Clarify per-destination binary replacement and separate checkout projections from runtime skill installation paths."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "make help && make doc-check && make commit-check"
    result: "passed; help exposed the corrected guarantee and documentation and commit gates exited zero"
quality-review: pass
quality-reviewed-at: 2026-08-26T17:14:18+09:00
quality-review-evidence:
  - "independent ce-judge found no blocking contract mismatch and verified the projection and runtime target lists against their implementations"
  - "review improvements now name the verified binary as the replacement and identify the Claude plugin artifact as its internal skills symlink"
archived-at: 2026-08-26T17:14:18+09:00
verified-at: 2026-08-26T17:14:18+09:00
verification-summary: "Checkout projection and runtime installation contracts are distinct and linked to their canonical tables."
---

# Task 229: clarify binary install atomicity and skill projection boundaries

## Summary

The `make install` help can be read as one transaction spanning both destinations even though
the implementation atomically replaces each verified destination. README also lists source
checkout projections beside runtime install locations, which makes generated symlinks look like
the paths that `dva skill install` manages.

## Decision

State the binary guarantee as per-destination replacement. Describe `make generate` outputs only
as DVA checkout projections, and route the runtime installation contract to the canonical path
table in USAGE rather than duplicating it.

## Completion Criteria

- [x] `make help` describes atomic replacement per verified binary destination | verify: `make help`
- [x] README distinguishes checkout-local projections from runtime skill installation | verify: `make doc-check`
- [x] README links to the canonical runtime path table in USAGE | verify: `make doc-check`
- [x] Documentation and commit gates pass | verify: `make doc-check && make commit-check`
