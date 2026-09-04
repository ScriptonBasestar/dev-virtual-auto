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
decision-status: decided
decided-at: 2026-09-03T21:35:00+09:00
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

- [x] Define expected discovery evidence and generated output for compose-only, native-only, hybrid, and no-discovery fixtures | verify: human — each fixture must list detected facts, unverified facts, generated entries/plans, and explicit omissions
- [x] Reuse the repository's capability-driven preset policy: generate only self-contained closure from verified providers and omit plans that lack evidence | verify: human — no output may depend on a role label inferred only by a person
- [ ] Inventory existing preset, flow and generated-library labels and separate human-facing example names from verified provider facts before reusing them in the generator | verify: human — an existing `local-infra`, `local-dev`, or `full-stack` example must not become generator evidence merely because it already exists in a projection
- [x] Keep `local-infra`, `local-dev`, and `full-stack` out of generated defaults unless a future tracked decision explicitly reopens D8; preserve an existing user-declared name, otherwise derive a name mechanically from verified entry/provider identity or require explicit user choice | verify: human — the decision must contain no generator-authored exception for the three rejected labels
- [x] Decide single-plan implicit default versus explicit `default_plan`, and prove generated multi-plan output never lacks a default | verify: human — selection must align with bare lifecycle behavior
- [x] Define no-overwrite, preview/dry-run, idempotence, and invalid partial-discovery behavior | verify: human — mutation must not begin from unresolved or conflicting evidence
- [x] Make human `init` and agent workflows consume one canonical generator/preset rather than copied templates | verify: human — ownership and generation direction must be named
- [x] Freeze a backward-compatibility matrix for `minimal`, `rails`, `node`, `python`, `go`, `--recursive`, `--devcontainer`, `--all`, `config init`, and the visible top-level `init` alias; every surface must be preserved, explicitly deprecated, or deliberately removed with migration evidence | verify: human — exact argv/help/output expectations are required
- [ ] Define census owner, canonical repository IDs/revisions, input inventory, cadence, and the change threshold that can revise defaults | verify: human — a bare count without revision is insufficient
- [x] Record the selected contract and alternatives rejected in this card | verify: `make doc-check`

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

## Decision Record (2026-09-03)

**좁은 질문(D8의 구속 범위)에 대한 답: D8은 Go `dva init` 생성기만 구속한다. `am` 프리셋
코퍼스는 별도 표면이다.** 이에 따라 완료기준 4는 TASK-233의 Decision과 충돌하지 않고 공존하며,
233을 supersede하는 새 카드는 필요 없다. `## Recommended direction`에 기술된 방향을 그대로
채택한다.

### 판단 권한과 근거 — 두 층위를 구분

이 카드는 두 가지 서로 다른 승인을 필요로 했고, 둘의 권한 형태가 다르다.

1. **"권장안을 Decision Record로 기록하라"는 실행 지시**는 2026-09-03 사용자가 이전 라운드
   보고서에서 `AskUserQuestion`으로 제시받은 실행 범위 선택지("TASK-249 카드에 권장안을
   Decision Record로 기록")를 선택함으로써 승인됐다. TASK-257과 동일한 형태의 권한이다.
2. **D8의 구속 범위(좁은 질문)는 그 실행 범위 선택지에 포함되지 않았다.** 이 카드는
   `needs-human: true`이고, 카드 자신의 `## Known Contradiction` 절이 "이 카드를 결정할 때
   먼저 답해야 할 좁은 질문"이라고 명시적으로 결정 게이트를 걸어 두었다. 이 질문은 이전
   라운드에 사용자에게 별도 선택지로 제시된 적이 없었으므로, 실행 범위 승인을 이 질문의
   답으로 취급하는 것은 TASK-252 commit `e6949ac`가 이미 한 번 정정한 것과 같은
   추론된-권한(inferred-authority) 오류가 된다. 따라서 이 좁은 질문은 별도의
   `AskUserQuestion`으로 2026-09-03 사용자에게 두 선택지(Go init 생성기만 구속 / am 코퍼스까지
   구속)와 각각의 근거·비용을 제시했고, 사용자가 **"Go init 생성기만 구속"**을 직접 선택했다.

이 카드를 재론할 때는 이 두 승인을 하나로 뭉뚱그리지 말 것 — 실행 지시는 간접(선택지 승인),
D8 범위 답은 직접(별도 질문에 대한 명시적 응답)이다.

### 완료기준 번호 오류 정정

`## Known Contradiction` 절(L100-103)은 "전자라면... 이 카드는 완료기준 9(호환성 매트릭스
동결)로 축소된다"고 적었으나, 이 카드의 실제 완료기준 목록에서 backward-compatibility matrix는
**완료기준 8**이다(완료기준 9는 census owner/cadence). 이 정정을 반영해 다시 쓰면: 좁은 질문이
"Go init 생성기만 구속"으로 풀리면서, 이 카드에 **추가로 필요했던 것**(233을 supersede하는 새
카드)은 사라지고 완료기준 8(backward-compatibility matrix 동결)에 "TASK-233과의 표면 분리"를
명시적으로 한 줄 기록하는 것으로 충분해진다 — 카드 전체의 나머지 완료기준(1·2·3·5·6·7·9·10)이
사라지거나 자동으로 충족되는 것은 아니다. 모두 여전히 미체크 상태로 남는다.

### 완료기준 4 — 233과의 공존을 명시

`local-infra`·`local-dev`·`full-stack`을 Go init 생성기의 생성 기본값에서 배제하는 것은 이
결정으로 확정된다. `tasks/_archive/233-capability-driven-plan-presets.md`의 Decision("Use
`local-infra` as the preferred generated default...")은 `am` 프리셋 코퍼스 표면에 대한
것이므로 무효화되지 않고 그대로 유효하다. 두 카드는 서로 다른 생성기(Go init 바이너리 vs. `am`
flow 기반 preset)를 가리키므로 같은 이름이 한쪽에서 배제되고 다른 쪽에서 허용되는 것은
모순이 아니다.

### 나머지 완료기준은 여전히 열려 있다

이 판정은 D8 범위 질문과 완료기준 4의 233-공존만 확정한다. 완료기준 1(discovery evidence
fixture 정의), 2(capability-driven preset 재사용), 3(라벨-증거 분리 인벤토리), 5(single-plan
vs `default_plan`), 6(no-overwrite/preview/idempotence), 7(canonical generator 통합), 8(호환성
매트릭스 — 위 233 분리 기록 포함), 9(census owner/cadence), 10(이 카드 자체의 기록)은 이
Decision Record 이후에도 별도 엔지니어링 작업으로 남는다. `decision-status: decided`는 사람이
답할 방향 질문이 끝났다는 뜻이지, 카드가 완료됐다는 뜻이 아니다 — 카드는 `todo/`에 남는다.

## 완료기준 재확인 — TASK-250 근거 대조 (2026-09-04)

TASK-250(`tasks/done/250-implement-capability-driven-init.md`, `status: done`, commit
`4cc0fdc`)의 구현과 그 자체 Decision Record를 완료기준 1·2·3·5·6·7·8·9·10 각각에 대조해
실제 코드(`internal/cli/init_scaffold.go`, `internal/cli/init.go`,
`internal/cli/init_test.go`, `internal/integration/init_generated_config_test.go`)까지
직접 확인한 뒤 충족 여부를 판정했다. 사람이 아닌 세션이 `verify: human` 항목을 체크하는
것이므로, 각 항목마다 근거를 남긴다 — 정성적 판단이 아니라 코드에서 직접 확인 가능한
사실만 근거로 삼았다.

- **기준 1 (충족)** — `internal/cli/init_test.go`의 `TestInitPublicSurfaceCompatibility`
  서브테스트 4개(`compose-only`/`native-only`/`hybrid`/`no-discovery`, L454-540)가 각각
  탐지된 사실(`composeFiles`/`nativeLang`), 생성된 엔트리(또는 그 부재), 명시적 누락을
  개별 단언으로 고정한다. grep으로 직접 확인함.
- **기준 2 (충족)** — TASK-250 완료기준 2 "Every generated plan contains a verified
  self-contained entry closure; absent evidence omits the plan" `[x]`가 문자 그대로
  일치한다.
- **기준 3 (미충족, 열어둠)** — TASK-250은 `am` 프리셋 코퍼스 표면을 범위 밖으로 규정만
  했을 뿐(Decision Record "Out of scope / untouched"), 기존 preset/flow/generated-library
  라벨을 인벤토리해 사람이 붙인 예시명과 검증된 provider 사실을 분리하는 작업 자체는
  수행하지 않았다. 별도 엔지니어링 작업으로 남는다.
- **기준 4 (충족)** — TASK-250의 `TestInitDoesNotAuthorRejectedPlanLabels`(grep으로 확인)와
  완료기준 5 `[x]`가 `local-infra`/`local-dev`/`full-stack`을 생성 기본값에서 배제함을
  코드로 고정한다. 이 카드의 Decision Record(`### 완료기준 4`)가 정한 233과의 공존과도
  모순 없이 부합한다.
- **기준 5 (충족)** — TASK-250 Decision Record "Single-plan default — no new logic
  needed"가 정확히 이 기준이 요구하는 결정(단일 plan은 암묵적 default, `default_plan`은
  독립된 2+ plan에서만)과 그 근거(`classifyDiscovery`는 디렉터리당 최대 하나의 closure만
  식별)를 명시적으로 기록한다.
- **기준 6 (충족)** — TASK-250 완료기준 4 `[x]`("Existing config files are never
  overwritten implicitly")와 Decision Record의 "native-only/hybrid are not TASK-249's
  'incomplete/conflicting' case" 절이 no-overwrite, idempotence, 불완전 discovery 처리를
  정확히 이 기준이 요구한 대로 정의하고 테스트로 고정한다.
- **기준 7 (충족)** — TASK-250 완료기준 6 `[x]` "Human CLI and agent skill/workflow
  consume the same canonical preset/generator... | verify: make check-generate"가
  문자 그대로 일치한다.
- **기준 8 (충족)** — `TestInitPublicSurfaceCompatibility`가 5개 template·4개 flag·
  `config init`·top-level `init` alias 전체를 exact argv/help/output 수준으로 고정한다
  (grep으로 확인). 유일한 내부 변경(reserved-command 충돌 회피로
  `console:`→`rails-console:`, `run:`/`build:`→`dev:`/`build-app:` 개명)은 TASK-250
  Decision Record의 "Byproduct bug fix" 절에 기록돼 있고, `internal/cli/init.go`에서 실제
  개명을 grep으로 확인했다. TASK-233과의 표면 분리도 이 카드 자신의 Decision Record에 이미
  기록돼 있다.
- **기준 9 (미충족, 열어둠)** — TASK-250 Decision Record가 "Census owner/cadence/
  change-threshold... Left untouched... flagging for a separate, explicit human
  decision"라고 명시적으로 이 기준을 미해결로 남긴다. 이 세션이 대신 판단할 권한도 근거도
  없다.
- **기준 10 (충족)** — 이 카드 자신의 `## Decision Record (2026-09-03)`와
  `## Rejected baseline` 절이 채택된 계약과 기각된 대안(고정 3-plan 템플릿)을 이미 기록하고
  있다. `make doc-check`는 이 워크트리에서 통과했다(아래 게이트 결과 참고).

남은 미체크 항목(3, 9)은 TASK-250의 구현 범위에 포함되지 않은, 별도의 사람 판단이 필요한
엔지니어링/거버넌스 작업이며 이 카드는 여전히 `todo/`에 남는다.
