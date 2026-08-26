---
id: TASK-230
title: "Install DVA skills into the Agent Mesh flat Markdown namespace"
type: feature
priority: P1
effort: M
created-at: 2026-08-26T18:00:00+09:00
source: "runtime installer follow-up"
scope: "dva skill install/status/uninstall Agent Mesh adapter"
status: doing
---

# Task 230: Agent Mesh flat skill installation

## Decision

Add `agent-mesh` to the existing `dva skill install`, `status`, and `uninstall` runtime surface;
do not add a separate export command. The runtime owns only its DVA namespace:
`~/.config/agent-mesh/skills/dva/` for user scope and `.agent-mesh/skills/dva/` for project scope.

The adapter produces `dva.md` and `dva-config.md` from the embedded canonical skills. It removes
YAML frontmatter, keeps the canonical body, and appends `references/*.md` in deterministic name
order. Relative reference bundle resolution is not assumed, so relative Markdown reference links
are rewritten to the canonical GitHub URL. DVA does not run `am skill sync`; it documents that as
a manual post-install action when the local Agent Mesh setup needs it.

## Completion Criteria

- [x] `agent-mesh` resolves to the documented user and project namespace paths | verify: `go test ./internal/skillinstall -run TestRuntimePaths`
- [x] Install renders deterministic flat `dva.md` and `dva-config.md` without YAML frontmatter | verify: `go test ./internal/skillinstall -run TestAgentMeshInstallsFlatRenderedSkills`
- [x] Collision, receipt, drift, dry-run, uninstall, and mixed-runtime preflight retain the existing safety contract | verify: `go test ./internal/skillinstall`
- [x] Agent Mesh documentation states the renderer degradation policy and manual `am skill sync` boundary | verify: `make doc-check`
- [x] Focused and repository mechanical checks pass | verify: `make test-skill-dogfood && make doc-check && make commit-check`
