# sadawiki dva 적용 분석

## 현황
- 파일: `dva.yml` (7,721 bytes, version `0.1.26`)
- 사용 섹션: `env_file`, `stack`(compose 1엔트리), `environments`(dev/test), `checks`, `modes`(infra/infra-tools/full-stack), `health_checks`, `interaction`, `provision`(default_profile 포함), `endpoints`
- `dva validate` (0.1.48): **ERROR** — `interaction.clean` 제거된 built-in 훅

## 문제점
1. **`interaction.clean` replace 훅 사용 (line 146-149)** — `clean` built-in은 제거됐고 teardown은 `dva down <plan> --purge`. 현재 validate가 hard error로 거부한다.
2. **`modes:` 섹션 사용 (line 99-110)** — deprecated. `plans`/`environments`/`sites`로 분해 대상 (docs/42 §11-1). `compose_services`/`compose_profiles` 선택은 `plans.*.entries[].services`로 이동해야 한다.
3. **`plans:` 없음** — 실행 가능한 이름이 없어 `dva up <plan>` 신모델 경로를 쓸 수 없다.
4. **`stack.compose.order: 10` (line 27)** — 실행 순서는 `plans.*.entries[].order` 책임. plan 경로에서는 읽히지 않는 값.
5. **compose 서비스 아래 `tags` 실행 제어 (line 34-43)** — tags는 메타데이터로 축소된 개념 (docs/42 §11-1).
6. **`interaction.build`/`logs` replace 훅** — `build`/`logs`는 이제 plan-aware lifecycle 동사(docs/43 Tier 1)라 replace로 compose 직접 명령을 우회하는 패턴은 선언/실행 분리에 역행. `logs`에 `command`와 `replace`가 중복 선언(line 151-157)된 점도 모호하다.
7. **health_checks의 `start:`가 `docker compose up -d ...` 직접 호출 (line 119-131)** — DVA를 우회해 compose를 직접 실행. plan 기반이라면 stack 선언을 경유해야 한다.

## dva 개선 힌트
- `clean` 에러 메시지는 명확하나, `dva config migrate`가 `interaction.clean` → `interaction.down.after` 이동까지 자동화하면 이 계열(sadawiki·signalhub 동일 패턴) 전환 비용이 0이 된다.
- modes의 `compose_services`+`compose_profiles` 쌍은 plan entry 하나로 기계적 대응이 가능해 보이는데 migrate가 modes를 스캐폴드만 출력하는 현재 정책(docs/43 §18-1)이 실사용에서 가장 큰 마찰 지점.

## 마이그레이션 난이도
**하** — 단일 compose 스택 + 3개 mode. 각 mode가 서비스 목록 하나로 plan entry에 1:1 대응된다. `clean` 훅 이동과 modes→plans 3개 작성이면 끝.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `dva config migrate` 미리보기: modes 2개(default/full)에 대해 아무것도 변환하지 않음 — 힌트 텍스트만 출력. `compose_profiles: [dev-tools]`를 "up_options에 `--profile full`"로 안내(모드 이름을 profile 값으로 오기). 전부 수작업 이관.
- [x] `modes:` 제거 → `plans.infra`(postgres/redis/elasticsearch), `plans.infra-tools`(+adminer, 기존 full 모드 대응), `plans.test-infra`(env test). `default_plan: infra`.
- [x] `stack.compose.order: 10` 제거. 서비스 tags는 메타데이터로만 유지(제어 용도 아님).
- [x] 상위 레벨 `health_checks` → `stack.compose.health_checks`로 이동(tcp postgres 15210 / redis 15220, http es `_cluster/health`). `start`/`start_hint`는 plan 경로에서 사장이므로 제거.
- [x] `interaction.clean` replace hook → `steps:` (`docker compose down -v --remove-orphans`, DESTRUCTIVE 설명 명시).
- [x] `interaction.build` / `logs` replace hook 제거 — `dva build infra`, `dva logs infra`가 대체.
- [x] provision default의 "Start core infrastructure" step: `run: docker compose up …` → `compose_up: [postgres, redis, elasticsearch]`.
- [x] version 0.1.44로 상향.
- [ ] `full-stack` plan은 infra-tools와 동일 → 중복 plan 경고가 떠서 제거(보류 아님, 정리).

### validate 최종 출력
```
✅ dva.yml is valid
EXIT=0   (warning 0)
```

### 보류/예외 항목
- `${POSTGRES_USER:-dev}` / `${POSTGRES_DB:-sadawiki}` (interaction.db command) — TASK-303 영향으로 미수정.
- `suggestion_ignore`에 `.make/*.mk` 공유 타깃(clean-*, env-*, health, k8s-secret-*, test-e2e, up-*, validate*) 추가 — clean 하드 에러 뒤에 숨어 있던 ~40개 Makefile 제안이 한꺼번에 드러나 일괄 무시 처리.

### 발견된 dva 개선점
- **validate 조기 중단**: `interaction.clean` replace가 하드 에러이면 modes/order/Makefile 제안 경고가 모두 숨겨진다. clean만 고치자 40개 제안이 새로 등장. (repro: 원본 dva.yml에서 `dva validate` → 에러 1줄만; clean을 steps로 바꾸면 경고 다수.) TASK-305.
- **migrate 힌트 오류**: `modes.full.compose_profiles: [dev-tools]` → "`--profile full`"로 안내(profile 값 대신 모드 이름). TASK-306 증거.
- **`--purge`로 named volume 미삭제**: plan이 services 부분집합이면 down이 `compose rm --force --stop --volumes <svcs>`(internal/lifecycle/compose.go composeDownArgs)라 named volume이 남는다. `docker compose down -v` interaction을 `dva down --purge`로 치환할 수 없어 clean interaction을 유지.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `${POSTGRES_USER:-dev}` / `${POSTGRES_DB:-sadawiki}`: 유지 확정.
