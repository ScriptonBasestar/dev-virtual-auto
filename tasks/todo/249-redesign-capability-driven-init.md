---
id: TASK-249
title: "Redesign init around verified capabilities instead of a fixed three-plan template"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-01T19:25:00+09:00
source: "PLAN-002 tracked D8-compatible scaffold ruling"
scope: "init discovery contract, capability preset integration, plan naming/default rules, human-agent parity, census ownership"
status: todo
needs-human: true
decision-status: pending
---

# Task 249: redesign capability-driven init

## Summary

Replace the rejected fixed three-plan template with an evidence-driven generation contract aligned
with D8 and the repository's capability preset policy.

## Decision required

The proposed unconditional `local-infra`, `local-dev`, and `full-stack` scaffold conflicts with D8,
contains invalid top-level fields, and asks the current Compose-only detector to invent a native runner.
It can also generate empty or duplicate plans that immediately trigger D6.

## Recommended direction

검증된 provider closure 하나에서 plan 하나를 생성하는 보수적 기본값을 권장한다. 단일 plan은 기존 bare
lifecycle의 implicit default를 사용하고, 실제 evidence가 둘 이상의 독립 plan을 정당화할 때만 명시적
`default_plan`을 기록한다. 이름은 기존 사용자 선언을 보존하고, 새 이름은 entry/provider identity에서
기계적으로 도출한다. 충돌하거나 불완전한 discovery에서는 preview만 제공하고 파일을 쓰지 않는다.

현재 다섯 template과 `config init`/top-level `init` surface는 유지하되 모두 하나의 generator를 호출하게
한다. Corpus 빈도는 detector 개선의 입력으로만 사용하고 새로운 archetype이나 plan label의 근거로
사용하지 않는다.

## Completion Criteria

- [ ] Define expected discovery evidence and generated output for compose-only, native-only, hybrid, and no-discovery fixtures | verify: human — each fixture must list detected facts, unverified facts, generated entries/plans, and explicit omissions
- [ ] Reuse the repository's capability-driven preset policy: generate only self-contained closure from verified providers and omit plans that lack evidence | verify: human — no output may depend on a role label inferred only by a person
- [ ] Inventory existing preset, flow and generated-library labels and separate human-facing example names from verified provider facts before reusing them in the generator | verify: human — an existing `local-infra`, `local-dev`, or `full-stack` example must not become generator evidence merely because it already exists in a projection
- [ ] Keep `local-infra`, `local-dev`, and `full-stack` out of generated defaults unless a future tracked decision explicitly reopens D8; preserve an existing user-declared name, otherwise derive a name mechanically from verified entry/provider identity or require explicit user choice | verify: human — the decision must contain no generator-authored exception for the three rejected labels
- [ ] Decide single-plan implicit default versus explicit `default_plan`, and prove generated multi-plan output never lacks a default | verify: human — selection must align with bare lifecycle behavior
- [ ] Define no-overwrite, preview/dry-run, idempotence, and invalid partial-discovery behavior | verify: human — mutation must not begin from unresolved or conflicting evidence
- [ ] Make human `init` and agent workflows consume one canonical generator/preset rather than copied templates | verify: human — ownership and generation direction must be named
- [ ] Freeze a backward-compatibility matrix for `minimal`, `rails`, `node`, `python`, `go`, `--recursive`, `--devcontainer`, `--all`, `config init`, and the visible top-level `init` alias; every surface must be preserved, explicitly deprecated, or deliberately removed with migration evidence | verify: human — exact argv/help/output expectations are required
- [ ] Define census owner, canonical repository IDs/revisions, input inventory, cadence, and the change threshold that can revise defaults | verify: human — a bare count without revision is insufficient
- [ ] Record the selected contract and alternatives rejected in this card | verify: `make doc-check`

## Rejected baseline

Do not generate a fixed three-plan template merely because those names are common in the measured
corpus. Frequency is input evidence, not proof that a particular repository has those capabilities.
Capability evidence can justify a plan's existence, but does not by itself justify one of the three
rejected labels.

## Known Contradiction — TASK-233 (기록만, 판정 아님)

이 카드는 결정되기 전에 알려져야 할 사실 하나를 빠뜨리고 있다. **완료기준 4는 이미 닫힌
결정과 충돌한다.**

완료기준 4는 `local-infra`·`local-dev`·`full-stack`을 "D8을 명시적으로 재개하는 추적된
결정이 없는 한" 생성 기본값에서 배제하라고 요구한다. 그런데
`tasks/_archive/233-capability-driven-plan-presets.md`의 `## Decision`(L54)은 이렇게 말한다.

> Use `local-infra` as the preferred generated default only when all selected providers are
> local, verified, and non-destructive.

TASK-233은 `status: done`, `verification-status: verified`다. 즉 완료기준 4가 요구하는
"추적된 결정"이 반대 방향으로 이미 존재한다. 이 카드는 D8과 세 label은 언급하지만 그 label을
의무화한 결정도, 그것을 고정한 테스트도 이름을 대지 않는다. 그 누락 자체가 결함이다.

### 충돌의 정확한 범위 — 코퍼스도 테스트 pin도 아니다

`internal/config/plan_preset_corpus_test.go`의 `TestPlanPresetPolicyShipsInPromptCorpus`는
`required` 슬라이스에 리터럴 `"default_plan: local-infra"`를 담고
`agent-mesh-flows/shared/library/naming-presets.md`와 `internal/cli/library_reference.txt`
양쪽에 존재하는지 검사한다. 이것이 충돌 지점으로 보이기 쉬우나 아니다.

- 그 pin은 **코퍼스 내용에 대한 문자열 단언**이고, `naming-presets.md:139`는 예시 YAML
  블록 안에 있다.
- 완료기준 3이 정확히 그 이음매를 이미 갈라 놓았다 — "an existing `local-infra` ... example
  must not become generator evidence merely because it already exists in a projection."
- 따라서 코퍼스는 그 문자열을 예시로 계속 가르칠 수 있고, canonical generator를 만들어도
  pin은 그대로 산다. 완료기준 4와 7은 코퍼스를 생성기 출력으로 읽을 때만 충돌하며,
  완료기준 3이 그 독법을 금지한다.

살아남지 못하는 것은 **TASK-233의 Decision 문장 하나**뿐이다. 그것은 예시가 아니라 생성
규칙("preferred generated default")이기 때문이다.

### 재개는 새 카드로만 가능하다

TASK-233은 `tasks/_archive/`에 있다. 완료기준 4를 채택하면서 233의 Decision을 무효화하려면
done 카드를 제자리에서 수정하는 것이 아니라 명시적으로 supersede하는 새 카드가 필요하다.
이 절은 그 판단을 내리지 않는다 — 판단자가 233의 존재를 모른 채 결정하는 일만 막는다.

**따라서 이 카드를 결정할 때 먼저 답해야 할 좁은 질문:** D8은 Go `init` 생성기만 구속하는가,
아니면 `am` 프리셋 코퍼스까지 구속하는가. 전자라면 완료기준 4와 233은 서로 다른 표면을
말하므로 공존하고, 이 카드는 완료기준 9(호환성 매트릭스 동결)로 축소된다. 후자라면 233을
supersede하는 카드가 같은 변경에 포함돼야 한다.
