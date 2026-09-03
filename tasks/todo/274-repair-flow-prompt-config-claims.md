---
id: TASK-274
title: "Repair agent-mesh-flows prompt claims that reject or emit invalid config"
type: bug
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-03T00:20:00+09:00
source: "Session audit of agent-mesh-flows/ prompt bodies against internal/config and schema.json"
scope: "dva-improve.yaml fabricated version pre-screen, kubectl stack shape in dva-improve.yaml and 00-analyze.yaml, doctor-check-skipped masking in 40-execute.yaml"
status: todo
---

# Task 274: repair flow prompt config claims

## Summary

Three defects in `agent-mesh-flows/` prompt bodies teach an executing agent to either reject
valid `dva.yml` config or emit config the schema does not accept. An agent following these
prompts produces a failure for the user: a valid config gets hard-rejected before the real
validator ever runs, or a suggested `stack:` shape fails schema validation, or a genuine
environment problem is silently downgraded to "skipped".

## Problem

1. `agent-mesh-flows/dva-improve.yaml` contains a fabricated deterministic pre-screen that
   hard-errors when `version:` is absent — anchor:
   `grep -n 'ERROR: .version field is missing' agent-mesh-flows/dva-improve.yaml`. `schema.json`
   has no root `required` array (verify: `python3 -c "import json;
   print(json.load(open('internal/config/schema.json')).get('required'))"` prints `None`), so
   this pre-screen rejects configs the actual validator accepts.

   **Preserve, do not touch**: the block in the same file headed `## CRITICAL: version 필드`
   (`grep -n 'CRITICAL: version 필드' agent-mesh-flows/dva-improve.yaml`) is correct — it says
   an existing `version:` field must be preserved as-is. Only the pre-screen that hard-errors
   on *absence* of the field is in scope.

2. Both `agent-mesh-flows/dva-improve.yaml`
   (`grep -n 'stack.kubectl.*dir:' agent-mesh-flows/dva-improve.yaml` — line reads
   `` K8s manifests 감지 → `stack.kubectl`에 `dir:` 지정 ``) and
   `agent-mesh-flows/dva-improve-guided/00-analyze.yaml`
   (`grep -n 'stack.kubectl.*dir:' agent-mesh-flows/dva-improve-guided/00-analyze.yaml` — line
   reads `` K8s manifests detected ... → suggest `stack.kubectl` with `dir:` ... ``) instruct
   the agent to place a kubectl entry directly at `stack.kubectl` with a `dir:` key. This is
   wrong on both axes:
   - `kubectl_runner_config` in `schema.json` accepts only `manifests`, `namespace`, `context`,
     `kubeconfig` — no `dir` (verify: `python3 -c "import json; d=json.load(open('internal/config/schema.json'));
     print(sorted(d['definitions']['kubectl_runner_config']['properties'].keys()))"`).
   - The correct nesting is `stack.<entry-name>.runners.kubectl`, not a bare `stack.kubectl`
     key (verify: `sed -n '636,655p' internal/config/schema.json` shows `runners` as the
     object holding the `kubectl` ref, and `internal/config/lifecycle.go` decodes
     `Kubectl *KubectlPluginConfig` under a runner map, not as a top-level stack key —
     `grep -n 'Kubectl \*KubectlPluginConfig' internal/config/lifecycle.go`).

3. `agent-mesh-flows/dva-improve-guided/40-execute.yaml` runs
   `dva doctor 2>&1 || echo "doctor check skipped"` (anchor:
   `grep -n 'doctor check skipped' agent-mesh-flows/dva-improve-guided/40-execute.yaml`). This
   swallows a genuine `checks:` failure and reports it to the pipeline as merely "skipped",
   so a broken environment (missing Docker socket, missing toolchain, etc.) reads as fine
   instead of surfacing the actual doctor failure.

## Completion Criteria

- [ ] The fabricated `.version field is missing` hard-error pre-screen is gone from `dva-improve.yaml` | verify: `! /usr/bin/grep -q 'version field is missing' agent-mesh-flows/dva-improve.yaml`
- [ ] An absent `version:` field is no longer rejected before the real validator runs, and the `## CRITICAL: version 필드` preservation block is untouched | verify: `human — read the diff: the preserve-if-present rule must survive verbatim; only the absence-triggered hard error is removed`
- [ ] No flow prompt instructs emitting a bare `stack.kubectl` key | verify: `! /usr/bin/grep -rq 'stack\.kubectl' agent-mesh-flows/`
- [ ] Both prompts name the correct `stack.<entry>.runners.kubectl` nesting and only schema-valid kubectl properties (manifests/namespace/context/kubeconfig) | verify: `human — the replacement text must round-trip through 'dva validate' on a config an agent would produce from it`
- [ ] `40-execute.yaml` no longer swallows a `dva doctor` failure behind a "skipped" message | verify: `! /usr/bin/grep -q 'doctor check skipped' agent-mesh-flows/dva-improve-guided/40-execute.yaml`
- [ ] A real `checks:` failure is surfaced as a failure, distinct from doctor genuinely not being applicable (binary absent, no `checks:` declared) | verify: `human — confirm the two outcomes are reported differently, not merged into one branch`
- [ ] Generated flow artifacts are current after any edit to shared library sources (`dva-improve.yaml` and `00-analyze.yaml`/`30-configure.yaml` are flowgen injection targets — edits to shared sources must be followed by `make generate`) | verify: `make check-generate`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- The `## CRITICAL: version 필드` preservation block stays exactly as-is — this task removes
  only the fabricated absence-triggered pre-screen, not the preserve-if-present rule.
- No flowcheck rule additions to catch this class of defect going forward — docs/51 declares
  prompt prose out of scope for flowcheck.
- No restructuring of the guided flow's step graph (stage boundaries, `depends_on` wiring,
  `interactive.auto_decide` gates).
- Investigated but dropped: a claimed mismatch between `10-verify.yaml`'s declared proposal
  JSON shape and what downstream stages consume. `20-transform.yaml` and `30-configure.yaml`
  both pass `{{load_proposal.output}}` through to their LLM instruction as opaque context
  rather than destructuring it by key, and the two fields downstream shell steps do parse by
  name (`selected_plan` in `40-execute.yaml:91`, `setup_track` in `30-configure.yaml:78`) are
  both declared in `10-verify.yaml`'s required JSON shape. No concrete mismatch was found, so
  this item is out of scope for this card.
