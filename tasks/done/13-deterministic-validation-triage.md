---
title: "Deterministic Validation Triage & JQ Feedback"
priority: P1
effort: M
created: 2026-04-02
status: todo
---

# Deterministic Validation Triage & JQ Feedback

## Description
The auto-healing validation loops (`fix_validation_1`, `fix_validation_2`) pass the entire `dva config validate` text output directly back to the LLM. 

Because some guardrails (e.g., Rule 34: `run.native` and `run.docker` must both be strictly defined) are complex, LLMs sometimes hallucinate fixes or fall into an infinite retry loop failing to provide strict YAML objects.

Instead of only giving the textual validation trace, we should inject a deterministic checking step (e.g. `jq` / `yq` testing critical JSON schema nodes) that pre-screens the YAML. This ensures the LLM receives very direct feedback ("You used string instead of object for run.native") bridging the gap between open-ended LLM retries and strict validation constraints.

## Acceptance Criteria
- [ ] Add a fast shell context check after configuring/improving that deterministically checks rigid rules (like the dual-path structure).
- [ ] Append this deterministic error message to the retry prompt along with the standard `validate` output.
