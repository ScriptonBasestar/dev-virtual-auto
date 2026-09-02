---
id: TASK-245
title: "Freeze the public and filesystem contract for the config env bridge"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-01T19:21:00+09:00
source: "PLAN-002 env bridge decision gate"
scope: "env_file source-target model, exact CLI grammar, Git/path safety, output contract, cross-platform replace spike"
status: todo
needs-human: true
decision-status: pending
---

# Task 245: freeze the env bridge contract

## Summary

Decide the public CLI, configuration shape, filesystem safeguards, and output contract before any
security-sensitive bridge implementation begins.

## Decision required

D9 approved a bridge under the existing `config` group, not the exact `edit`/`unseal` argv or schema.
The current `env_file` accepts string, list, and object shapes and normalizes through `any`. A schema-only
addition can validate and then disappear at runtime. The command also needs a deterministic selector
when more than one env target exists.

No production implementation begins until this card is decided.

## Recommended direction

V1은 effective top-level `env_file`의 object entry에 source↔target 관계를 두고, write command가 선택한
entry의 선언 origin과 path anchor를 하나로 증명할 수 있을 때만 동작하는 방향을 권장한다. Selector는
configured target을 명시하고, encrypted entry가 정확히 하나일 때만 생략을 허용한다. `edit`은 sops가
encrypted source만 편집하게 하고 plaintext target 갱신은 별도 `unseal`에서만 수행한다.

Base/module/override가 합쳐져 provenance가 모호하거나 여러 origin이 같은 target을 주장하면 write를
거부한다. Interaction/subcommand metadata와 owner를 식별할 수 없는 subproject origin도 V1 write 대상에서
제외한다. 이는 load 호환성을 제거한다는 뜻이 아니라, secret-bearing mutation의 대상만 보수적으로
제한한다는 뜻이다. Provenance-preserving loader와 별도 fixture가 승인되면 지원 origin을 넓힐 수 있다.

## Completion Criteria

- [ ] Choose one source↔target representation and show its behavior in every existing top-level `env_file` shape, including load, merge, show, and validation round-trip compatibility; interaction/subcommand `env_file` must reject encrypted-source metadata unless a separate runtime use case is approved | verify: human — the decision must include accepted and rejected YAML examples for both schema locations
- [ ] Freeze the exact command grammar and the zero/one/many encrypted-entry selection rule; ambiguous selection and implicit multi-target writes must fail closed | verify: human — the decision must include an argv table with text and JSON outcomes
- [ ] Define `edit` ownership and the full unseal state matrix across source/target existence, required/optional, and force | verify: human — the matrix must cover every Cartesian branch
- [ ] Define the allowed effective top-level declaration origins and preserved provenance, then define the resolution anchor for root/module/override/subproject declarations plus path containment, absolute paths, Git-outside behavior, tracked/not-ignored targets, symlink/non-regular files, source=target, and permission failures | verify: human — every origin, ambiguous merge, location, and unsafe state must name its exact resolution or fail-closed rejection and whether it is non-overridable
- [ ] Limit `--force` to existing regular-target overwrite unless a separately justified security decision says otherwise; it must not silently bypass tracked, ignore, symlink, type, or path guards | verify: human — rejected alternatives and migration advice must be recorded
- [ ] Run a Linux/macOS/Windows replacement and concurrency spike; specify handle-relative or equivalent TOCTOU defense, file and parent-directory sync, atomicity, durability, cancellation cleanup, SIGKILL/power-loss limits, owned stale-temp recovery, and fail-closed behavior on an unverified platform | verify: human — evidence must include commands, OS/version, results, unresolved guarantees, and the exact supported-OS CI matrix that will keep those guarantees live
- [ ] Freeze success/error text, JSON envelope, exit codes, secret redaction, and stable machine-code policy without inventing a second root error envelope | verify: human — fixture-ready expected documents must contain no decrypted value or raw child output
- [ ] Record the selected option and why alternatives were rejected in this card before changing its status | verify: `make doc-check`

## Non-negotiable baseline

Sops is invoked without a shell, dotenv input/output is explicit, secret material never reaches DVA
output, and an ambiguous selector fails before any write. DVA does not adopt age key/provider ownership.

## Conditional platform rule

지원한다고 결정한 OS는 TASK-246의 safe-writer와 command integration CI에서 계속 검증해야 한다. 현재
CI가 그 matrix를 제공하지 못하고 이를 TASK-246 범위에서 안전하게 추가할 수 없다면, 이 결정 카드가
별도의 bounded CI enablement child를 만들고 PLAN-002의 children·count·graph와 TASK-246 dependency를
같은 변경에서 갱신한다. 그 child가 통합되기 전에는 해당 OS 지원을 선언하지 않으며, 검증되지 않은
platform에서는 mutation을 fail closed한다. 이 조건은 spike를 지속 보증으로 오인하지 않기 위한 것이다.
