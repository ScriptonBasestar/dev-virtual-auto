---
title: "Separate Drift/Semantic Warnings from Hard Errors"
priority: P2
effort: M
created: 2026-04-02
status: archived
completed-at: 2026-04-02
verified-at: 2026-04-03
archived-at: 2026-04-03
verification-summary: "Verified partitioning of validate (hard) and validate --strict (semantic) in dva-improve, with explicit LLM triage step added."
---

# Separate Drift/Semantic Warnings from Hard Errors

## Description
Currently, the LLM is prompted to fix all validation outputs, including strict warnings (`validate --strict`). Semantic warnings (e.g., config drift, unconventional port mappings, suggestion warnings) are fundamentally different from hard validation errors (e.g., bad schema, invalid references).

Forcing an LLM to resolve both at once sometimes causes it to radically change parts of `dva.yml` unnecessarily just to silence a semantic warning.

We should split the `fix_validation` strategy into two discrete conceptual phases:
1. **Hard Error Resolution**: Focus exclusively on passing `dva config validate` (ERROR 0). 
2. **Semantic Optimization (Triage)**: Once the config is syntactically sound, handle `validate --strict` as a secondary optimization pass or code-review feedback, preventing destructive configuration changes.

## Acceptance Criteria
- [ ] Divide retry prompts to prioritize fixing fatal errors first.
- [ ] Introduce a phase where semantic warnings are treated as "suggestions for the user" or optional non-destructive fixes.
