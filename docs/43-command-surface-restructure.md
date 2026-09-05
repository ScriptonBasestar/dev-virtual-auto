# Command Surface Restructure

CLI 명령어 표면을 선언/실행 분리 모델에 맞춰 정리하는 설계.
선언/실행 모델은 [40-declarative-stack-and-plans.md](40-declarative-stack-and-plans.md), 실행 계획·권장
CLI는 [41-execution-plans-and-cli.md](41-execution-plans-and-cli.md), 마이그레이션 원칙은
[42-migration-and-compatibility.md](42-migration-and-compatibility.md)를 본다.

이 문서는 40-42 시리의 개념 설계를 **현재 코드에 적용하는 재구성 결정**을 소유한다.
철학 판단은 [SOUL.md](../SOUL.md), 제품 범위는 [PRODUCT.md](../PRODUCT.md), 구현 경계는
[ARCHITECTURE.md](../ARCHITECTURE.md)가 각각 소유하며 여기서는 반복하지 않는다.

**구현 상태: 적용 완료.** 예약어 개수는 `internal/config/reserved.go`가 정본이다
(재구성 당시 23개, 이후 `skill`·`agent-deny`·`kubectl` 추가). `dva --help`에 `stack`/`app`/`infra`/`clean`이
없다. §14는 재구성 **이전** 상태의 기록으로 남긴다 — 왜 이렇게 바꿨는지의 근거이므로
현재형으로 읽지 않는다. 기능 하나가 대체 없이 사라졌다: `applications.<name>.health.required`
(엄격 준비 상태 게이트). §16 Tier 1 참조.

## 14. 현재 CLI 부채 (재구성 이전 기록)

재구성 이전 예약어 27개([`internal/config/reserved.go`](../internal/config/reserved.go)) 안에 세 가지 세대가 섞여 있었다.

### 14-1. lifecycle 동사 3중 복제

| 계층 | 명령 | 실행 기준 | 상태 |
|---|---|---|---|
| plan | `dva up/down/stop/restart <plan>` | plan 이름 | 목표 모델 |
| stack | `dva stack up/down/stop/status/build/log/update` | stack 엔트리 | 선언을 직접 실행 |
| app | `dva app up/build/down/restart/stop/log/ls` | `applications:` | legacy |

[42 §11-1](42-migration-and-compatibility.md)이 stack·applications를 실행 대상이 아닌 것으로
규정했으나 CLI 동사는 남아 있다. SOUL 신념 3(하나의 동작엔 하나의 소유자) 위반이다.

### 14-2. 최상위 동사의 세대 불일치

```
up / down / stop / restart  →  plan 기준 (새 모델)
build / logs / clean        →  service·compose 기준 (구 모델)
```

같은 Lifecycle 그룹 안에서 `dva up <plan>`은 동작하지만 `dva logs <plan>`은 동작하지 않는다.

### 14-3. 폐기 잔재와 발견 인터페이스 중복

- `dva infra`는 이미 "folded into stack, use `dva up`" 안내문을 내장한 채 살아 있다.
- `ls` / `show` / `status` / `config show` / `config dump` 가 "설정 보기" 역할을 흩뿌려 갖고 있다.

## 15. 재구성 결정

세 분기점을 두고 아래와 같이 정했다. 근거는 SOUL 신념과 40-42 계약이다.

| 결정 | 선택 | 근거 |
|---|---|---|
| `dva stack` CLI 동사 | **완전 제거** | 선언 조회는 `ls`/`show`가 소유. "stack ls는 되는데 stack up은 안 되는" 혼란 원천 차단 (SOUL 신념 3) |
| `build`/`logs`/`clean` | **전부 plan-aware 통일** | 최상위 lifecycle 동사를 단일 세대로 수렴 (SOUL 신념 2, 예측 가능성). `clean`은 `down --purge`로 흡수 |
| 마이그레이션 방식 | **hard break** | 단계적 폐지 단계 없이 즉시 제거. 대신 `config migrate` 범위 확장이 선행 조건(§18) |

> hard break는 "경고 기간 없이 제거"를 뜻한다. 기존 `dva stack up`/`dva app up`/`applications:`
> 스크립트와 설정은 `config migrate`를 거쳐 plan 모델로 옮겨야 한다. 이 경유지가 없으면
> CLI 제거는 함정이 된다.

## 16. 최종 명령어 표면

예약어 27 → 23 (제거: `stack`, `app`, `infra`, `clean`). "plans"/"stack" 용어는 CLI에 드러내지
않는다([42 §13](42-migration-and-compatibility.md)).

[41 §8](41-execution-plans-and-cli.md)의 권장 핵심 6동사(up/down/stop/status/ls/show)는 발견·실행
최소 UX다. 아래 Tier 1의 `logs`/`build`/`restart`는 41이 금지한 것이 아니라, 이미 존재하는 lifecycle
동사를 같은 `<name>` 기준으로 정렬한 것이다.

### Tier 1 — Lifecycle (전부 `<name>` = plan 기준, 단일 세대)

```
dva up   <name>
dva down <name> [--purge]     # --purge 가 자원까지 파기 (구 clean 흡수)
                              # services 선택 plan에서도 compose 프로젝트 전체를 down -v --rmi local (TASK-311)
dva stop <name>
dva restart <name>
dva status [name]
dva logs  <name>              # plan 엔트리 로그
dva build <name>              # plan 엔트리 빌드 (mode-aware는 runner 책임)
```

**기능 손실 1건**: `applications.<name>.health.required: true`(준비 상태 미달 시 non-zero
exit)에 해당하는 스위치가 이 표면에 없다. 최상위 `health_checks`는 `required`를 지원하지
않고 항상 advisory다. 준비 상태 실패를 CI 실패로 만들려면 `checks:`나 interaction 명령으로
게이트를 직접 세워야 한다. 대체 스위치는 별도 결정이 필요하므로 이 재구성의 범위에 넣지
않았다 — 조용히 사라진 것이 아니라 기록된 부채다.

### Tier 2 — Discovery (사람 + 에이전트 공용)

```
dva ls / dva show / dva manifest / dva validate / dva doctor
```

stack이 CLI에서 사라지므로 `ls`/`show`가 stack 선언을 보여주는 유일한 창이다.

### Tier 3 — Auxiliary (실행 모델과 분리, [42 §12](42-migration-and-compatibility.md))

```
dva run <name>          # interactions; dva <name> 단축형 유지
dva provision <name>    # 1회성 준비/초기화
```

### Tier 4 — Raw passthrough / Tier 5 — Meta

```
dva compose ...   dva ktl ...              # DVA 대응이 없을 때
dva init / version / completion / config(show|docs|migrate|init|validate) / ssh / console / skill
```

## 17. 현재→목표 매핑

| 현재 명령/키 | 처리 | 목표 경유지 |
|---|---|---|
| `dva stack up/down/stop/status/build/log/update` | 제거 | `plans` entry + `dva up/down/... <plan>` |
| `dva app *` + `applications:` 섹션 | 제거 | `stack` runner(native/docker) 선언 + plan |
| `dva infra` | 제거 | 이미 `dva up` 안내 중 |
| `dva clean` | 흡수 | `dva down --purge` |
| `dva build [SERVICE]` | 재정의 | `dva build <name>` (plan-aware) |
| `dva logs [SERVICE]` | 재정의 | `dva logs <name>` (plan-aware) |
| `dva status` | 확장 | plan-aware `dva status [name]` |
| `config dump` | 흡수 | `config show` |
| `config docs` | 흡수 | `docs`(가이드 재생성) |

유지: `up`/`down`/`stop`/`restart`(이미 plan 기준), `run`/`provision`, `ls`/`show`/`manifest`/
`validate`/`doctor`, `compose`/`ktl`, `init`/`version`/`completion`/`config`, `ssh`/`console`/`docs`.

## 18. 구현 의존성

### 18-1. `config migrate` 범위 확장 (선행 조건) — **완료**

[`config_migrate.go`](../internal/cli/config_migrate.go)는 compose 선언 변환만 수행했고,
소스에 `'modes', 'stack.*.order' and 'applications' are migrated by hand.`라고 명시돼 있었다.
§17이 제거 대상으로 삼는 변환 경로가 없는 상태에서 CLI를 먼저 지우면 함정이 되므로,
이 확장을 구현의 첫 단계로 두었다.

- `applications:` 섹션 → `stack` runner 선언 — **자동 변환**.
  `dev`/`variants`/`depends_on`/`port`는 변환되지 않고 리포트가 이름을 댄다.
- `stack` 실행 중심 필드(`order`/`tags`/서비스 선택) → `plans.entries[]` — **자동 변환**.
  참조하는 plan이 없으면 추측하지 않고 거부한다.
- `modes` → `plans`/`environments`/`sites` 분해 — **범위 밖**. 3축 분해가 기계적으로
  유도되지 않아 스캐폴드만 출력하고 거부한다. by-hand로 남는 유일한 항목이다.

### 18-2. 단일 소스 동기화

- [`reserved.go`](../internal/config/reserved.go): 4개 제거 + `hookableCommands` 조정
  (`clean` 이탈, `down`이 `--purge` 의미 흡수). hook은 동사 이름에 묶여 있으므로 같이 이관.
- [`root.go`](../internal/cli/root.go): AddCommand·그룹·help 템플릿에서 stack·app Direct Access 블록 제거.
- 삭제/흡수 파일: `app.go`, `infra.go`, `stack.go`(조회 비트는 `ls`/`show`로), compose.go의 clean.
- `plan_lifecycle.go`: `logs`/`build`/`status` plan-aware 확장 + `down --purge`.
- 정규 문서 동기화: [USAGE.md](../USAGE.md), [ARCHITECTURE.md](../ARCHITECTURE.md) 도메인 경계·실행 흐름,
  [PRODUCT.md](../PRODUCT.md) 현재 상태, `skills/dva`·`skills/dva-config`, manifest/ls 노출 로직.

## 19. 결정 사항 요약

- lifecycle 동사는 `up/down/stop/restart/status/logs/build`로 단일 세대 수렴. 전부 `<name>` 기준.
- `stack`·`app`·`infra`·`clean` 예약어 제거. stack은 YAML 섹션으로만 존재.
- `build`/`logs`/`clean`을 plan-aware로 통일, `clean`은 `down --purge`로 흡수.
- 마이그레이션은 hard break. 단, `config migrate` 범위 확장이 선행 조건.
- CLI 표면에 "plans"/"stack" 용어 노출 금지는 유지([42 §13](42-migration-and-compatibility.md)).
- 상세 구현 순서는 별도 계획에서 다룬다(이 문서는 결정과 범위만 소유).
