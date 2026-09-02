---
id: TASK-247
title: "Freeze required env-file behavior for every command path"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-01T19:23:00+09:00
source: "PLAN-002 required-env decision gate"
scope: "all loadEnv callers, observation versus execution semantics, text/JSON/exit contract, doctor strict behavior"
status: todo
needs-human: true
decision-status: pending
---

# Task 247: freeze required env behavior by command

## Summary

Classify every environment-loading command path and freeze its diagnostics, JSON, exit, and child-start
behavior before changing the shared loader.

## Decision required

`loadEnv` currently warns and continues for every error. A single hard-fail would kill doctor and
best-effort observation, but treating every caller as advisory can launch work with missing required
inputs. Optional files still error when present but unreadable or malformed, and a later file can fail
after earlier files have already merged into the environment. The proposal's lifecycle-versus-query
split does not decide `status`, `logs`, teardown, partial merge, or the duplicate JSON status path.

## Recommended direction

외부 process를 시작하거나 resource identity를 해석하는 실행·mutation 경로는 required/inaccessible/
malformed 입력에서 첫 child 전에 fail closed하는 것을 권장한다. 여기에는 teardown도 포함한다. 잘못된
환경으로 다른 resource를 정리하는 것보다 명시적으로 실패하고 진단을 제공하는 편이 안전하다.

`status`·`logs` 같은 관측 경로는 config-only 정보를 계속 보여줄 수 있지만 결과를 `partial`로 명시하고,
필요한 env가 없으면 backend child를 시작하지 않는다. Doctor는 모든 check를 끝까지 수행하고 default는
advisory, `--strict`은 non-zero로 유지한다. Partial merge는 어떤 실행 경로에도 노출하지 않고 버리며,
관측 경로에서도 변수값 자체가 아니라 실패 metadata만 보여준다.

## Completion Criteria

- [ ] Inventory every `loadEnv` caller and classify it by observable command and purpose rather than source filename | verify: `/usr/bin/grep -R -n 'loadEnv(' internal/cli`
- [ ] Freeze a complete matrix for required true/false × missing/inaccessible/malformed × single/multi-file partial merge across text, JSON, exit code, child-process start, and diagnostic completeness | verify: human — all inventoried call sites and every state class must appear exactly once
- [ ] Decide `up`, `restart`, `build`, `run`, hooks, provision, kubectl, ssh, compose passthrough, `down`, `stop`, `status`, `logs`, doctor default/strict, and query surfaces explicitly | verify: human — no row may inherit a generic “lifecycle” label without rationale
- [ ] Ensure any command classified fail-closed stops before its first external child process and uses the existing root JSON failure envelope | verify: human — expected text/JSON/exit fixtures must be written before implementation
- [ ] Preserve doctor diagnostic reachability and define how built-in advisory default differs from `doctor --strict`; do not use default doctor exit 0 as a promotion gate | verify: human — the decision must cite both default and strict outcomes
- [ ] Define whether any caller may observe variables merged before a later file failure; execution paths must never continue on an accidental partial environment, and observation commands must either return an explicitly marked partial result or discard it | verify: human — exact text and single-document JSON shapes are required
- [ ] Record the decision, rejected alternatives, and migration effect in this card | verify: `make doc-check`

## Constraint

Do not implement the policy as an unconditional error inside `loadEnv`. The helper may return richer
status, but the caller contract remains explicit and testable.
