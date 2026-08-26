---
id: TASK-229
title: "Install help and README blur per-destination atomicity with checkout projections and runtime installation"
type: docs
priority: P1
effort: S
created-at: 2026-08-26T17:08:46+09:00
source: "post-TASK-228 review"
scope: "Makefile install help and README LLM integration contract"
status: doing
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

- [ ] `make help` describes atomic replacement per verified binary destination | verify: `make help`
- [ ] README distinguishes checkout-local projections from runtime skill installation | verify: `make doc-check`
- [ ] README links to the canonical runtime path table in USAGE | verify: `make doc-check`
- [ ] Documentation and commit gates pass | verify: `make doc-check && make commit-check`
