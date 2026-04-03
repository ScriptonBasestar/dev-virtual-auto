---
title: "Monorepo Workspace Analysis for Subprojects"
priority: P2
effort: M
created: 2026-04-02
status: archived
completed-at: 2026-04-02
verified-at: 2026-04-03
archived-at: 2026-04-03
verification-summary: "Verified pnpm-workspace, Cargo.toml, and go.work scanning in scan_subprojects context for dva-improve and 00-analyze workflows."
---

# Monorepo Workspace Analysis for Subprojects

## Description
The current subproject detection in `00-analyze.yaml` and `dva-improve.yaml` checks `1-depth` directories for markers like `package.json` or `Cargo.toml`. 

In modern monorepos, explicitly defined workspace structures (`pnpm-workspace.yaml`, `Cargo.toml [workspace]`, `go.work`) provide an accurate map of packages and their dependencies. 

Parsing these workspace files via `jq`/`awk` in the `scan_subprojects` stage will provide the LLM with definitive boundaries. This allows for far more accurate generation of the `applications:` section, especially determining topological launch order (`depends_on:` fields) for topological waves.

## Acceptance Criteria
- [ ] Update `scan_subprojects` context block to check for `pnpm-workspace.yaml`, `Cargo.toml` `[workspace.members]`, and `go.work`.
- [ ] If present, extract package structures and pass them directly to the LLM context.
- [ ] Include instructions to use this workspace hierarchy to structure `applications:` `depends_on:` accurately instead of guessing.
