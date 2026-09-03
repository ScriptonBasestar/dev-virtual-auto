---
id: TASK-263
title: "Decide qualified-project addressing and exposure"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-02T11:20:00+09:00
source: "PLAN-003 separation of addressing from composition"
scope: "interaction machine route and shorthand, imported item names, exposure, collision precedence, discovery surfaces, migration, and rollback"
status: todo
needs-human: true
decision-status: decided
depends-on: [TASK-259, TASK-264]
---

# Task 263: decide qualified-project addressing

## Summary

Use TASK-259 evidence to freeze project addressing and exposure independently of cross-project plan
composition. One separator does not need to serve direct interactions, imported items and configuration
references if doing so weakens compatibility or literal-key precedence.

## Recommended direction

Keep `dva run --project <project> <interaction>` as the collision-safe explicit machine route and retain
`project:interaction` as the human shorthand. Keep explicitly imported plan, interaction and provision names
as `project/item`, with aliases only when declared. Do not automatically expose every child item merely because
a subproject is registered.

This mixed grammar reflects different operations: direct child selection versus a parent-visible, parent-registered
imported name whose execution state remains child-owned.
The fail-closed fallback is the exact current grammar and explicit import policy. A `/` unification or automatic
reachability requires separate measured compatibility evidence and must not be smuggled into composition.

## Completion Criteria

- [x] Use TASK-259's pinned grammar and consumer corpus to decide a canonical explicit route and allowed shorthand separately for direct interactions, imported lifecycle plans, imported interactions and imported provision profiles; canonical and alias lifecycle routes must preserve the same child owner | verify: human — every surface must name accepted, rejected and ambiguous examples
- [x] Freeze literal `:` and `/` key precedence, reserved-prefix rejection, canonical/alias collision handling, missing project behavior, lazy child loading and working-directory selection | verify: human — no parser fallback may silently select a different project or command
- [x] Decide explicit import/export versus automatic registration; the recommended default is explicit import with no flattening or automatic reachability, and `/` denotes only an explicitly imported parent-visible item | verify: human — any broader exposure requires a bounded namespace and compatibility proof
- [x] Freeze help, completion, ls, show, status and manifest representation, including the collision-safe explicit invocation an agent should use | verify: human — machine discovery must not require guessing whether `:` or `/` applies
- [x] Define compatibility duration, migration diagnostics and rollback for any change from the current mixed grammar; insufficient evidence selects the current grammar | verify: human — dynamic invocation findings remain unresolved rather than green
- [x] Obtain independent product and compatibility review, append an approved `## Decision Record`, and change `decision-status` from `pending` to `decided` before TASK-260 begins | verify: `make doc-check`

## Non-goals

- No route, schema, resolver or completion implementation.
- No imported-plan ownership repair; TASK-262 owns it.
- No plan-composition decision or vocabulary rename.

## Decision Record (2026-09-03)

**현행 혼합 문법을 그대로 동결한다: `--project` 명시 · `:` 축약 · `/` 명시적 import.
Automatic registration과 flattening은 계속 거부한다. Subproject 이름은 예약된 내장 명령과
충돌할 수 없도록 `config validate`가 새로 막는다. 자식의 자체 validator가 거부하는 키는
부모의 qualified route로도 도달할 수 없도록 통일한다.**

### 1. 판단 권한

2026-09-03 사용자가 세 개의 독립된 질문으로 이 카드를 직접 제시받고 답했다. 세 질문 모두
TASK-259의 실측 권고안(§5)과 카드 자체의 `## Recommended direction`이 일치하는 방향을 그대로
선택했다 — 권고를 뒤집은 판단이 아니라, 권고를 확정한 판단이다.

### 2. 완료기준 1 — canonical route와 허용 축약

**Option A + explicit import를 채택한다.** TASK-259 §4가 세 옵션을 실측 비교했다:

- **B (`/` 단일 canonical) 기각** — `p/item`을 통일하면 (i) 모든 자식 항목이 자동
  도달 가능해져 기존 유효 설정의 의미를 바꾸거나, (ii) import 존재 여부에 따라 두 의미를
  오가게 된다. 리터럴 `/` 키가 오늘 합법(`weird/slash` 실측)인데 우선순위 규칙이 없어
  충돌하고, `:`는 `USAGE.md`·`warnLiteralKeyShadowsSubproject` 경고·manifest 항목·5개
  추적 테스트에 박혀 있어 deprecation 비용이 크며 채택 후 롤백이 깨끗하지 않다.
- **C (`--project` 단독 canonical) 기각** — 가장 모호한 형식을 없애지만, 8개 명령에
  플래그를 추가하고 파괴적 플래그와의 상호작용을 전부 새로 정해야 하는데 그게 측정되지
  않았다. `/`는 라우트가 아니라 map 키이므로 제거되지도 않아 형식 수가 줄지 않는다.

세 형식은 한 연산의 세 철자가 아니다 — `--project`는 이름 해석 전 프로젝트 선택,
`:`는 그 선택의 축약, `/`는 부모가 등록한 항목의 부모 소유 이름이다. Canonical과 alias
lifecycle route는 동일한 child owner를 보존한다(변경 없음, 이미 그렇다).

### 3. 완료기준 2 — 우선순위·충돌·부재 동작 동결

`LiteralKeyWins`가 정의하는 `:` 우선순위(리터럴 부모 키 우선), `/`가 라우트가 아니라
map 키라 파싱 시 모호성이 발생하지 않는다는 사실, `loadSubprojectConfig`의 존재하지 않는
project 오류 형태(TASK-267 item 2로 `dva ls --project`와 통일됨), lazy child loading을
현행 그대로 동결한다 — 이번 판정으로 변경되는 것은 아래 두 항목뿐이다.

**신규 결정 (a) — 예약어 charset 충돌.** Subproject 이름이 예약된 내장 명령(`up`, `config`,
`run` 등)과 같으면 오늘은 경고 없이 허용된다. 이 판정은 그것을 **`config validate`가
거부**하도록 바꾼다. interaction 키에는 이미 이 규칙이 있으므로(`reserved.go`의
`ValidateReservedCommands`) subproject 이름에도 같은 규칙을 적용해 일관성을 맞춘다.
구현은 이 카드의 비목표(`No route, schema, resolver ... implementation`)이므로 별도
구현 카드가 필요하다 — 아래 §6 참조.

**신규 결정 (b) — 자식 키의 부모 경유 도달성.** 자식 자신의 validator가 거부하는 예약어·
unroutable-prefix 키가 오늘은 부모의 qualified route(`--project`/`:`/`/`)로는 도달
가능하다. 자식 단독으로는 무효인 키가 부모를 거치면 살아나는 상태였다. 이 판정은 부모
경로에서도 동일하게 거부하도록 통일한다 — "어느 검증이 권위인가"가 모호한 채로 남지
않게 한다. 구현은 마찬가지로 별도 카드 필요.

### 4. 완료기준 3 — explicit import 기본값

**Explicit import, 자동 노출 없음, flattening 없음을 채택한다.** TASK-259 §4가 automatic
registration을 기각한 근거: Fixture E가 측정한 대로 import 하나가 깨지면 부모 명령 전체가
실패하는데, automatic registration은 이 실패 모드를 등록된 모든 subproject 수만큼
곱한다. Declaration-only subproject가 오늘 이름을 하나도 기여하지 않는다는 사실을
조용히 재해석하고, 저자의 결정 없이 모든 자식 키를 잠재적 충돌로 만든다. Flattening은
automatic registration의 상위집합 문제이고 PLAN-003이 이미 child-stack flattening을
배제한다 — 평가 대상에서 제외한다.

### 5. 완료기준 4 — 노출 표현(exposure representation)

**TASK-259 §5가 남긴 6개 수리 항목 중 3개(2·3·6번)는 이미 TASK-267이 닫았다** — 확인:
`internal/cli/manifest.go`의 `subprojectUsage`(부모 네임스페이스 기준 계산, 충돌 시
`dva run --project` fallback), `internal/cli/list.go:68`의 `--project`/`-p` 플래그
등록, `LiteralKeyWins` 주석 정정. 카드가 이를 반영하지 않고 있었으므로 여기 기록한다.

남은 것은 두 항목이며 **어느 문법 선택지에서도 옳으므로 이 결정과 독립적으로 진행
가능하다**:
1. `ls --json`과 manifest의 imported item 항목에 owner 필드와 canonical/alias 마커 추가
   (오늘은 세 항목이 여섯 이름으로 보인다).
2. 세 형식(`:`, `/`, `--project`) 전부에 대한 completion 명세 — 오늘은 root interaction
   키만 completion된다.

머신 판별 가능한 collision-safe invocation은 `dva run --project <project> <item>`으로
동결한다 — `dva ls`, `show`, `status`, `manifest`가 전부 이 형태를 일관되게 가리켜야
한다(TASK-267이 `run.go`/`list.go` 불일치는 이미 해소했다).

### 6. 완료기준 5 — 호환성 기간과 마이그레이션

**해당 없음.** 이 판정은 기존 문법을 바꾸지 않으므로 deprecation 기간, 마이그레이션 진단,
롤백 경로가 필요 없다. §3의 신규 결정 (a)·(b)는 새 거부 규칙 추가이지 기존 유효 설정의
의미 변경이 아니다 — 다만 그 규칙에 걸리는 설정이 오늘 존재할 수 있으므로, 그 두 항목을
구현하는 카드는 `config validate`가 새로 거부하는 경우에 대한 오류 메시지에 근거를
명시해야 한다(완료기준 2의 "no parser fallback may silently select a different project"
조항과 동일한 정신).

### 7. Fail-closed 확인

이 판정 자체가 fail-closed 기본값(현행 그대로)을 선택했으므로 완료기준 5의 "증거 불충분 시
현행 유지" 조항은 발동하지 않는다 — 증거가 이미 있었고 그 증거가 현행 유지를 가리켰다.

### 8. 후속 구현 카드 필요

이 카드의 비목표는 "route, schema, resolver, completion 구현 없음"이다. §3의 신규 결정
(a)·(b)를 실제로 강제하려면 새 구현 카드가 필요하다 — 존재하지 않는다. TASK-260을 여는
것 외에, 이 두 항목을 담을 카드를 만드는 것이 다음 groom에서 다뤄야 할 일이다.
