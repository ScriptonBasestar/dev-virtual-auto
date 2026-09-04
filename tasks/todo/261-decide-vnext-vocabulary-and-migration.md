---
id: TASK-261
title: "Decide vNext vocabulary and migration commitment"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-02T10:13:00+09:00
source: "PLAN-003 final vocabulary and migration decision"
scope: "public nouns and namespaces, compatibility strategy, migration tooling, corpus gate, rollback, and follow-up plan"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-254, TASK-256, TASK-258, TASK-260]
---

# Task 261: decide vNext vocabulary and migration

## Summary

After incremental route work and composition semantics are known, decide whether DVA keeps its current
vocabulary, introduces compatibility-first aliases, or performs a versioned hard break. This decision does
not implement a vNext schema or namespace.

## Recommended direction

Current-compatible evolution을 권장한다. `stack`, `plans`, `interaction`, `subprojects`는 이미 제품 문서와
schema에서 서로 다른 책임을 가지므로 측정된 이해도 문제 없이 rename하지 않는다. `run`과 top-level
lifecycle verbs도 유지하며 `plan`/`exec`/`tool` namespace를 추가하지 않는다.

필요한 개선은 stable machine command identity, explicit route와 additive alias metadata로 해결하고,
configuration noun이나 route를 hard break하지 않는다. Evidence가 rename 이익을 증명하지 못하면 이
권장안을 최종 결정으로 채택한다.

## Completion Criteria

- [x] Compare current vocabulary with proposed alternatives for subprojects/projects, plans/targets, interaction/tasks, stack/components, `run`, and any proposed `plan`/`exec`/`tool` namespace using TASK-254, TASK-256, TASK-258, and TASK-260 evidence | verify: human — no noun may be renamed without a mapped product concept, ambiguity analysis, and measured migration cost
- [x] Choose current-compatible evolution, alias-first migration, or versioned hard break; freeze canonical terms, route examples, configuration keys, compatibility duration, warning channels, and removal gates | verify: human — unspecified terms retain their current contract
- [x] Define migration tooling, version detection, machine-readable report, pinned consumer corpus, generated documentation ownership, release sequencing, rollback, and support horizon | verify: human — dynamic calls, ignored files, and unavailable repositories must remain explicit findings rather than assumed compatibility
- [x] Keep reserved-name collisions as hard errors and the current hook ownership model unless a separate approved decision with equivalent safety evidence changes them | verify: human — vocabulary work must not smuggle in collision or execution-hook policy changes
- [ ] Obtain independent product, architecture, and compatibility review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided`; if migration is selected, create a new plan with bounded implementation and release cards before closing this task | verify: `make doc-check`

## Fail-closed default

If evidence, compatibility contract, rollback, or human approval is incomplete, retain the current vocabulary
and routes. Do not leave a promised hard-break release date open-ended.

## Decision Record (2026-09-04)

**current-compatible evolution을 채택한다.** alias-first migration과 versioned hard break는
기각한다. 이 결정은 스키마나 namespace를 구현하지 않는다.

사용자 권장 작업 순서가 이 카드의 권장안을 실행하라고 승인했고, 선행
TASK-254/256/258/260이 모두 닫힌 뒤에 기록한다.

### 완료기준 1 — 비교 (이 절로 충족)

| 개념 | 현재 정본 | 제안된 대안 | 판정 |
| --- | --- | --- | --- |
| 선언 저장소 | `stack` | components | 유지. schema·USAGE·composition이 이미 이 명사에 묶여 있다 |
| 실행 이름 | `plans` | targets | 유지. lifecycle 동사의 위치 인자 |
| 단발 명령 | `interaction` | tasks | 유지. `dva run`의 입력 |
| 자식 프로젝트 | `subprojects` | projects | 유지. TASK-263 addressing과 구분된 import 단위 |
| 단발 실행 동사 | `run` | `exec` namespace | 유지. 동적 라우팅의 탈출 경로 |
| 생명주기 | `up`/`down`/`stop`/`restart`/`status`/`logs`/`build` | `plan`/`exec`/`tool` namespace | 유지. docs/43이 명사 접두를 제거한 계약 |
| 교차 프로젝트 묶음 | `composes:` (TASK-260) | recursive include / `/` 문법 | 유지. PLAN-005가 이미 구현함 |
| 경로 정체성 | `canonical_name` (TASK-272) | 새 schema 필드 | 유지. `validate`/`config validate`, `kubectl`/`ktl`가 이 필드로 공존 |

rename 이익을 측정한 소비 코퍼스는 TASK-255다. 스캔된 소비자는 `dva ktl`을 쓰고
`dva kubectl`을 쓰지 않았다. 그건 호환 이름을 유지하라는 증거이지, `stack`/`plans`를
바꾸라는 증거가 아니다. `plan`/`exec`/`tool` namespace는 PLAN-003이 비채택으로 이미
적어 두었고, 이후 구현(composition, validate, kubectl)은 그 namespace 없이 경로 정체성을
해결했다.

### 완료기준 2 — 채택과 동결

채택: **current-compatible evolution.**

동결하는 정본:

- 설정 키: `stack`, `plans`, `default_plan`, `environments`, `sites`, `interaction`,
  `provision`, `subprojects`, `composes`, `env_file`, `vars`
- CLI 동사: 현재 reserved set (`ReservedCommands()`). 새 namespace 없음
- 경로 예시: `dva up <plan>`, `dva run <name>`, `dva kubectl` (정본) / `dva ktl` (호환),
  `dva config validate` (정본) / `dva validate` (호환)
- 호환 유지: 이미 도입된 visible compatibility route는 별도 결정 없이 제거하지 않는다
  (`ktl`, `validate`). 제거 날짜를 약속하지 않는다
- 경고 채널: 새 vocabulary deprecation 채널을 만들지 않는다
- 제거 게이트: 이 카드는 어떤 현재 명사·경로의 제거도 열지 않는다

기각:

- **alias-first** — 바꿀 명사의 이익이 측정되지 않은 채 alias를 쌓으면 지원 표면만
  늘어난다. `canonical_name`이 이미 필요한 공존만 표현한다
- **versioned hard break** — 약속할 날짜와 마이그레이션 코퍼스가 없다. fail-closed
  default가 열린 날짜를 금지한다

명시되지 않은 용어는 현재 계약을 유지한다.

### 완료기준 3 — 마이그레이션 도구

마이그레이션할 것이 없으므로 도구·version detection·consumer corpus gate·hard-break
릴리스 순서를 만들지 않는다. 기존 `dva config migrate`는 레거시 선언 변환만 계속
담당한다. 롤백은 "현재 어휘를 유지하는 것" 자체다. 지원 지평은 무기한이다.

TASK-255가 남긴 스캐너 한계(누락 checkout 2곳, origin-unfetched pin, `$cmd`/alias
미전개)는 이 결정에서 **명시적 발견**으로 남긴다. 그 공백을 숨겨 호환을 단정하지
않는다 — 그래서 명사를 바꾸지 않는다.

생성 문서 소유권은 바뀌지 않는다: reserved 목록은 `reserved.go` → libgen →
`make generate`. 산문 사본은 그 생성 경로 밖이며 손으로 맞춘다.

### 완료기준 4 — 충돌과 훅

예약어 충돌은 계속 hard error (`config validate` exit 1, load 경고, bare-name
built-in 우선, `dva run <name>` 탈출). 훅 가능 집합은 `hookableCommands` 그대로
(`up`/`down`/`stop`/`restart`/`build`/`logs`). 이 카드는 둘 다 바꾸지 않는다.

### 후속

구현 계획 없음. 마이그레이션을 고르지 않았으므로 새 plan을 만들지 않는다.
이 카드는 독립 리뷰가 완료기준 5를 닫으면 `done/`으로 옮긴다.
