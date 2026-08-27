---
id: TASK-234
title: "Guided flow drops the approved plan and fails its default execution"
type: fix
priority: P1
effort: M
created-at: 2026-08-26T18:40:00+09:00
source: "independent post-TASK-233 contract review"
scope: "guided proposal persistence, selected-plan resolution, and discovery report freshness"
status: done
completed-at: 2026-08-26T18:57:44+09:00
completion-summary: "Preserve reviewed proposal data, bind approval to the current run, gate every mutation, resolve the approved plan safely, and remove stale automatic discovery input."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "dva test && make check-generate && make doc-check && make commit-check; focused Agent Mesh validation and dry-run"
    result: "race tests, generation, flow/documentation, commit, schema, and plan-path checks passed"
quality-review: pass
quality-reviewed-at: 2026-08-26T18:57:44+09:00
quality-review-evidence:
  - "independent ce-judge found the dropped approval, empty default plan, stale report, interactive continue, and stale artifact bypasses"
  - "final re-review at 3f19cd0 found no remaining blocker and confirmed the gates against Agent Mesh scheduling semantics"
archived-at: 2026-08-26T18:57:44+09:00
verified-at: 2026-08-26T18:57:44+09:00
verification-summary: "Reviewed or explicit batch-approved data is the only mutation input, and lifecycle start repeats strict validation plus declared-plan checks."
---

# Task 234: preserve approved plan decisions through execution

## Summary

The guided flow displays an LLM proposal but saves the original analysis report instead of the
reviewed proposal. Its default execution then forwards an empty plan and fails even after a valid
configuration was generated. The automatic flow can also consume a stale report merely because a
fixed default path exists.

## Completion Criteria

- [x] Stage 10 produces structured proposal JSON, uses the runtime's output-review gate, and persists
  the reviewed output rather than copying Stage 00 | verify: `go test ./internal/config -run TestGuidedFlowPreservesReviewedProposal`
- [x] Rejecting interactive output removes stale approval and gates every child and parent mutation
  branch | verify: `go test ./internal/config -run TestGuidedFlowPreservesReviewedProposal`
- [x] Stage 40 resolves an absent explicit override from the approved proposal and verifies that the
  exact plan exists in the generated config before lifecycle execution | verify: `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [x] An explicit plan remains an override, while an empty or undeclared selected plan fails before
  `dva up` | verify: `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [x] Configuration validation failures stop execution instead of being printed and ignored | verify:
  `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [x] The lifecycle mutation step repeats strict validation and declared-plan checks so Agent Mesh's
  interactive "continue after error" choice cannot bypass them | verify: `go test ./internal/config -run TestGuidedFlowResolvesAndValidatesApprovedPlan`
- [x] The automatic flow always uses its current scan and has no implicit or explicit stale discovery
  report input | verify: `go test ./internal/config -run TestAutomaticFlowAlwaysUsesFreshDiscovery`
- [x] Flow, generation, repository, and commit gates pass | verify: `make doc-check && make check-generate && make test && make commit-check`

## Decision

Use `interactive.auto_decide` for the proposal-producing LLM step and save its reviewed structured
JSON output. Agent Mesh deliberately disables interactive gates under explicit batch mode, so `-y`
is documented here as caller auto-approval rather than being mistaken for an interactive review.
Keep `plan` as an explicit override, but otherwise resolve `selected_plan` from that approved
artifact after configuration and prove it appears in `dva show --json` before starting it.
Remove the fixed default discovery-report path; callers that intentionally reuse a report must pass
it explicitly.
