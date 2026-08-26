---
id: TASK-234
title: "Guided flow drops the approved plan and fails its default execution"
type: fix
priority: P1
effort: M
created-at: 2026-08-26T18:40:00+09:00
source: "independent post-TASK-233 contract review"
scope: "guided proposal persistence, selected-plan resolution, and discovery report freshness"
status: doing
---

# Task 234: preserve approved plan decisions through execution

## Summary

The guided flow displays an LLM proposal but saves the original analysis report instead of the
reviewed proposal. Its default execution then forwards an empty plan and fails even after a valid
configuration was generated. The automatic flow can also consume a stale report merely because a
fixed default path exists.

## Completion Criteria

- [ ] Stage 10 produces structured proposal JSON, uses the runtime's output-review gate, and persists
  the reviewed output rather than copying Stage 00 | verify: `go test ./internal/config -run TestGuidedFlowPreservesReviewedProposal`
- [ ] Stage 40 resolves an absent explicit override from the approved proposal and verifies that the
  exact plan exists in the generated config before lifecycle execution | verify: `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [ ] An explicit plan remains an override, while an empty or undeclared selected plan fails before
  `dva up` | verify: `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [ ] Configuration validation failures stop execution instead of being printed and ignored | verify:
  `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [ ] The lifecycle mutation step repeats strict validation and declared-plan checks so Agent Mesh's
  interactive "continue after error" choice cannot bypass them | verify: `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [ ] The automatic flow consumes a discovery report only when the caller explicitly supplies its
  path | verify: `go test ./internal/config -run TestAutomaticFlowRequiresExplicitDiscoveryReport`
- [ ] An explicitly supplied discovery report that is absent or not one JSON object fails visibly
  instead of silently falling back to a fresh scan | verify: `go test ./internal/config -run TestAutomaticFlowRequiresExplicitDiscoveryReport`
- [ ] Flow, generation, repository, and commit gates pass | verify: `make doc-check && make check-generate && make test && make commit-check`

## Decision

Use `interactive.auto_decide` for the proposal-producing LLM step and save its reviewed structured
JSON output. Agent Mesh deliberately disables interactive gates under explicit batch mode, so `-y`
is documented here as caller auto-approval rather than being mistaken for an interactive review.
Keep `plan` as an explicit override, but otherwise resolve `selected_plan` from that approved
artifact after configuration and prove it appears in `dva show --json` before starting it.
Remove the fixed default discovery-report path; callers that intentionally reuse a report must pass
it explicitly.
