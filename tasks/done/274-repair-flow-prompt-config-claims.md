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
status: done
---

# Task 274: repair flow prompt config claims

## Summary

Three defects in `agent-mesh-flows/` prompt bodies teach an executing agent to either reject
valid `dva.yml` config or emit config the schema does not accept. An agent following these
prompts produces a failure for the user: a valid config gets hard-rejected before the real
validator ever runs, or a suggested `stack:` shape produces a config that runs and does
nothing, or a genuine environment problem is silently downgraded to "skipped".

**Correction made while implementing.** This card asserted the bad `stack:` shape "fails
schema validation". Measured, it does not — see Evidence. It validates, runs, and applies
zero manifests. The replacement prompt text states the measured behaviour, because telling an
agent the shape is rejected would have been a second false claim in place of the first.

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

- [x] The fabricated `.version field is missing` hard-error pre-screen is gone from `dva-improve.yaml` | verify: `! /usr/bin/grep -q 'version field is missing' agent-mesh-flows/dva-improve.yaml`
- [x] An absent `version:` field is no longer rejected before the real validator runs, and the `## CRITICAL: version 필드` preservation block is untouched | verify: `human — read the diff: the preserve-if-present rule must survive verbatim; only the absence-triggered hard error is removed`
- [x] No flow prompt instructs emitting a bare `stack.kubectl` key | verify: `! /usr/bin/grep -rq 'stack\.kubectl' agent-mesh-flows/`
- [x] Both prompts name the correct `stack.<entry>.runners.kubectl` nesting and only schema-valid kubectl properties (manifests/namespace/context/kubeconfig) | verify: `human — the replacement text must round-trip through 'dva validate' on a config an agent would produce from it`
- [x] `40-execute.yaml` no longer swallows a `dva doctor` failure behind a "skipped" message | verify: `! /usr/bin/grep -q 'doctor check skipped' agent-mesh-flows/dva-improve-guided/40-execute.yaml`
- [x] A real `checks:` failure is surfaced as a failure, distinct from doctor genuinely not being applicable (binary absent, no `checks:` declared) | verify: `human — confirm the two outcomes are reported differently, not merged into one branch`
- [x] Generated flow artifacts are current after any edit to shared library sources (`dva-improve.yaml` and `00-analyze.yaml`/`30-configure.yaml` are flowgen injection targets — edits to shared sources must be followed by `make generate`) | verify: `make check-generate`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

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

## Evidence

Measured against `bin/dva` built from this branch (`make build`), config fixtures under a
scratch directory.

### 1. The absent-`version:` pre-screen rejected valid config

`schema.json` has no root `required` array, so `version:` is optional. The generated
`AUTOGEN:version_rule` block in the same file already said so ("Omit `version:` for no
compatibility gate"), and the hand-written `## CRITICAL: version 필드` block said it again
("없으면 생략하세요"). The pre-screen contradicted both of its own file's rules. Removed
rather than inverted — with `version:` optional there is no absence condition left for a
deterministic pre-screen to assert.

### 2. The bad `stack:` shape is accepted and misread, not rejected

This is the card's own claim corrected. `stack:` entries are `additionalProperties: true`, so
the runner-name-as-entry-key shape validates:

    stack:
      kubectl:
        dir: k8s/

    $ dva config validate
    ✅ dva.yml is valid            # rc=0 — NOT rejected

    $ dva show
    Stack (entry names, referenced by plans and tag filters):
      kubectl  [runner:kubectl]    # an entry NAMED kubectl; runner inferred from the name

    $ dva up --dry-run
    ... msg=dry-run entry=kubectl plugin=kubectl command=kubectl args=[apply]

`kubectl apply` with no `-f` and no namespace: the `dir:` key was ignored, and the run applies
nothing. A silent no-op that passes validation is worse than a rejection, and it is what the
prompts were teaching agents to emit.

The taught replacement shape round-trips (criterion 4's binding) and carries what it promises:

    $ dva config validate            # rc=0
    $ dva config validate --strict   # rc=0
    $ dva up local-dev --dry-run
    [lifecycle] k8s-manifests (kubectl)
    ... args="[apply -f <abs>/k8s -n demo]"
    [lifecycle] demo-chart (helm)
    ... args="[upgrade --install demo ./chart -n demo]"

Runner property sets read from `schema.json` rather than assumed:
`kubectl_runner_config` = manifests, namespace, context, kubeconfig (no `dir`);
`helm_runner_config` = chart, release, namespace, context, values, set.

The `stack.helm` line adjacent to the kubectl one carried the identical nesting defect and was
repaired in the same edit. The card's Problem section named only the kubectl lines, but
criterion 3's `grep -r` binds the whole directory and leaving the helm twin wrong would have
left the two lines teaching contradictory nesting side by side.

### 3. `dva doctor`'s exit code already drew the line criterion 6 asks for

`doctorExitError` (`internal/cli/doctor.go`) counts only user-defined `checks:` failures in the
advisory default: a built-in `[FAIL]` prints and still exits 0, and a config declaring no
`checks:` has nothing user-defined to fail. So non-zero already means precisely "a real
declared check failed", and the `|| echo` fallback discarded that one signal and nothing else.
The repair propagates the exit code and adds an explicit branch only for the case the exit code
cannot express — the `dva` binary not being on PATH, reported as not-applicable with exit 0.
(The preceding `validate_config` stage would normally fail first in that case; the branch is
there so the two outcomes are distinguishable at this stage rather than inferred.)

The first version of that explanatory comment was rejected by `flowcheck` under
`comment-substitution` and `comment-quote`: `am` carries shell quote state across a `#` and
across lines, so a backtick in comment prose gets extracted as a substitution and blocks the
step, and a single apostrophe flips the quote parity of code below it. The comment that
documented the fix would have armed a run-time block in a step nobody touched. Rewritten with
no backticks and no quote characters; `make doc-check` is the gate that caught it.

### Note on the card's stale line reference

Problem item 2 cited `sed -n '636,655p' internal/config/schema.json` for the `runners` nesting.
TASK-246 added 82 lines to that file, so that range now shows site `entry_overrides`. The
`runners` object is at line 720. The claim itself held; only the locator had drifted.

### Out of scope, observed

Three prompt comments in this change had to *describe* a retired spelling rather than quote it,
because criteria 1, 3 and 5 bind `grep` against the whole file and an explanatory comment
naming the old form fails the criterion it documents. This is the same collision recorded in
TASK-273. A convention for "criterion greps must exclude comment lines" would remove the
recurring trap; no card owns it yet.
