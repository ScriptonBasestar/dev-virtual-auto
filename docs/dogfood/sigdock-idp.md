# sigdock-idp dva 적용 분석

## 현황
- 파일: `dva.yml` (7,891 bytes, version `0.1.26`) — 지시서 예상과 달리 **도입돼 있음**
- 사용 섹션: `env_file`, `stack`(compose + compose-dev-full 2엔트리), `checks`, `default_mode`, `modes`(3개), `health_checks`, `interaction`, `provision`(default/reset), `subprojects`(3개), `endpoints`(12개)
- `dva validate` (0.1.48): **경고만 있고 통과** (schema 자체는 유효) — 8개 중 "구모델이지만 파싱은 되는" 유일한 케이스

## 문제점
1. **`modes:` + `modes.*.stack:` (line 78-95)** — deprecated. 특히 `stack: [compose]`/`stack: [compose-dev-full]`로 mode가 stack 엔트리를 직접 선택 — 신모델에서 environment/mode는 stack 선택 책임이 없다(docs/42 §11-1의 `environments.*.stack` 제거와 같은 계열).
2. **`plans:` 없음, `default_mode: infra` (line 75)** — validate warn. overlay 선택(compose vs compose-dev-full)은 plan entry가 정위치.
3. **`stack.*.order` (line 15, 47)** — validate warn: plan 경로에서 읽히지 않는 값.
4. **`interaction.db`가 실행 타깃 없이 subcommands만 보유 (line 188-202)** — validate warn: 직접 호출 불가.
5. **config drift (validate warn)** — 루트에 compose 파일 8개가 있는데 dva.yml은 2개만 추적 (`compose.e2e.yaml`, `compose.kafka.yaml`, `compose.observability.yaml` 등 6개 미등록).
6. **`health_checks.*.start_hint` 중복 (line 103)** — `start`가 있으면 무시됨 (validate warn).
7. **`build`/`logs` replace 훅 (line 116-127, 136-139)** — plan-aware 동사 우회 구세대 패턴.

## dva 개선 힌트
- `modes.*.stack:` 목록으로 "기본 compose vs overlay compose" 를 고르는 이 패턴은 migrate의 자동 변환(plan entry의 name 선택)으로 가장 기계적으로 옮길 수 있는 형태인데, modes 전체가 스캐폴드-only 정책에 묶여 있다 — stack 선택만 하는 단순 mode는 자동 변환 가능한 하위 클래스로 분리할 가치가 있다.
- compose drift 경고가 실제 gap을 정확히 잡았다(e2e/kafka/observability overlay 6개 미등록). 이 경고에 "stack 엔트리 스캐폴드 출력" 을 붙이면 발견→수정이 한 단계로 줄어든다.
- `interaction.clean-build`의 주석 "DVA 0.1.44 requires custom interactions to be invoked through dva run" — 사용자가 버전 거동 변화를 주석으로 기록하며 따라오고 있다는 증거로, changelog→config 힌트 채널 수요.

## 마이그레이션 난이도
**하** — 유일하게 validate를 통과하는 구모델 설정. mode 3개가 각각 (서비스 목록, profile, stack 엔트리 선택)이라 plan 3개로 1:1 기계 대응. order 제거와 db interaction 타깃 추가는 사소하다.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `dva config migrate` 미리보기: `default_mode`/modes 변환 없음. `compose_profiles: [mq]` → "`--profile infra-mq`"(모드 이름 오기). `modes.*.stack` 선택은 기계적으로 plan entries로 옮길 수 있는데 자동화되지 않음.
- [x] `modes.*.stack` 선택 → plans: `infra`(postgres/redis/vault/mailpit), `hybrid`(compose + server native), `infra-mq`(+kafka/nats/rabbitmq), `dev-full`(compose-dev-full), `observability`, `tracing`, `kafka-redpanda`. `default_plan: infra`.
- [x] `stack.*.order` 및 엔트리 tags 제거(runner tags `[infra]`만 유지).
- [x] 미등록 overlay 6개 판정: **등록** `compose.observability.yaml`(stack.observability, base + overlay), `compose.jaeger.yaml`(stack.jaeger), `compose.kafka.yaml`(stack.kafka-redpanda) — dev 라이프사이클. **미등록(예외)** `compose.e2e.yaml`, `compose.e2e.delivery-tls.yaml`, `compose.multi-region-postgres.yaml` — CI/테스트 픽스처.
- [x] `server` native 엔트리 추가(cargo build/run, health http :11301/healthz).
- [x] `interaction.db`에 실행 대상 부여(service postgres + psql). 부모와 동일했던 `db shell` 서브커맨드 제거(migrate/reset 유지).
- [x] `build` replace hook → `build-docker`/`build-native` interaction(make 타깃), `logs` replace 제거.
- [x] start_hint 중복 제거(상위 health_checks 블록 삭제).
- [x] provision `compose_up: [postgres, redis, vault, mailpit]`, note 갱신. version 0.1.44.
- [x] Makefile 제안 → suggestion_ignore(`db-shell` 포함).

### validate 최종 출력
```
[warn] config drift: compose.files is … but detected root compose files are … compose.e2e.delivery-tls.yaml, compose.e2e.yaml, … compose.multi-region-postgres.yaml …
✅ dva.yml is valid
EXIT=0   (warning 1 — 의도적 예외)
```

### 보류/예외 항목
- config drift 경고: e2e/multi-region 픽스처를 의도적으로 미등록. 무시 수단이 없어 잔존(TASK-309).
- `${REDIS_PASSWORD:-change-me-redis}`, `${POSTGRES_USER:-scripton}`, `${POSTGRES_DB:-sigdock_idp}` — TASK-303, 미수정.

### 발견된 dva 개선점
- **drift 경고 억제 수단 없음**: 의도적으로 미등록한 overlay를 선언할 방법(예: `compose_ignore:` 또는 suggestion_ignore 확장)이 없다. repro: 위 validate 출력.
- **migrate 힌트 `--profile <mode명>`** 오류 재현(`[mq]` → `infra-mq`).
- **modes.*.stack → plan entries 자동 변환 부재**: 필드 매핑이 1:1인데 수작업 요구. TASK-306 증거.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `${REDIS_PASSWORD:-change-me-redis}`, `${POSTGRES_USER:-scripton}`, `${POSTGRES_DB:-sigdock_idp}`: 유지 확정.

## 2차 재검증 (2026-09-05, dva ecae43d: TASK-305/306/308 반영 빌드)

- (TASK-308) `stack.observability.runners.compose.services`가 overlay 서비스 2개만 선언해 plan `observability`의 base 서비스 5개가 미선언 참조였음 → base 5개 추가. base+overlay 엔트리는 TASK-307 실증.
