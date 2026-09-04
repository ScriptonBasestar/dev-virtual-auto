# matdosa-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (5,730 bytes, 226줄)
- 사용 섹션: `version`, `env_file`(`files: [.env]` 축약 리스트), `stack`(compose 1개 + health_checks), `plans`(5개), `default_plan`, `environments`(dev/test), `checks`, `suggestion_ignore`, `interaction`(build replace 훅 + 12개), `provision`
- 미사용: `sites`, `endpoints`
- `dva validate`: **통과** (semantic warn: `infra`와 `local-infra` plan 완전 동일)

## 문제점
- 구조 위반 없음. 신규 모델 준수.
- plan 중복: `infra` ≡ `local-infra` (L38-52, validate warn) — gizzahub와 동일한 alias성 중복.
- `environments`를 선언(dev/test)했지만 어떤 plan도 `environment:`를 참조하지 않음 (L37-76): dead 선언 — test 환경 vars(POSTGRES_DB: matdosa_test)가 실제로 얹힐 경로가 없다.
- `endpoints` 부재: 헤더 주석(L4-5)에 포트가 다 있는데 `dva up` 후 안내는 없다 — 주석을 endpoints 섹션으로 옮기면 정본화된다.
- `interaction.db`의 `${POSTGRES_USER:-dev}` (L170): gorisa에서 검증된 dva 확장기 버그(변수 설정 시 `:-` 오염)의 잠복 지점.

## dva 개선 힌트
- **참조되지 않는 `environments`/`sites` 선언을 validate가 잡지 않는다** — "선언됐지만 어느 plan도 참조하지 않음" semantic warn이 있으면 이 dead 선언을 바로 드러냈을 것.
- test 환경을 plan 없이 쓰려는 의도라면(`dva up local-infra --env test`류) 그 사용법이 문서에 없다 — environment를 CLI에서 일회성 선택하는 UX gap일 수 있다.
- plan alias 수요 재확인(gizzahub와 동일).

## 마이그레이션 난이도
**하** — 구조 이행 완료. plan 중복 정리 + environments 연결(또는 삭제) + endpoints 추가 정도.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] 중복 plan 정리 — `local-infra` 삭제, `infra` 유지. 근거: Makefile `up` 타겟이 `dva up infra`를 호출(L59)하고 `local-infra`는 dva.yml 내부 참조뿐. `default_plan: infra`로 변경, provision note의 `dva up local-infra` 문구도 `dva up infra`로 수정. validate warn 해소.
- [x] dead `environments` 연결/삭제 — `dev`는 4개 plan 전부에 `environment: dev`로 연결(값이 .env.example과 동일해 실동작 변화 없음). `test`(POSTGRES_DB=matdosa_test)는 삭제: plan 경로에서 `--env`가 거부되고(`dva up --help`), Makefile/스크립트 어디도 test env를 쓰지 않아 도달 경로가 없었음. test DB가 필요해지면 `infra-test` plan + `environment: test`로 복원 가능.
- [x] `endpoints:` 추가 — 헤더 주석·README·compose.yml 기본 포트를 `url:` 형식으로 등록 (backend 18200, frontend 18201, postgres 18210, redis 18220, adminer 18255, sigdock 18290/18291). `source:` 형식은 compose 포트가 `${POSTGRES_PORT:-18210}`라 TASK-303 영향권이어서 피함.
- [ ] (보류) `interaction.db`의 `${POSTGRES_USER:-dev}`/`${POSTGRES_DB:-matdosa}` (현재 L152) — TASK-303 지시대로 미변경.
- [ ] (보류) `interaction.build` replace 훅(make build-api/build-web) — 이 프로젝트는 계획의 build 훅 정리 대상 목록에 없고 validate ERROR도 아니라 유지.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 0)
```

### 보류/예외 항목
- `${VAR:-default}` 사용처: `interaction.db.command` 1건 (위).
- `build` replace 훅 유지 (위).

### 발견된 dva 개선점
1. **참조되지 않는 `environments` 선언을 validate가 잡지 않는다** — `test` env가 어떤 plan에도 연결되지 않고 plan 경로에서는 `--env`도 거부되므로 완전한 dead 선언인데 warn 없음. repro: environments에 `x:` 추가 후 `dva validate` → 침묵.
2. plan 경로에서 environment를 CLI 일회성 선택할 수단이 없다 (`--env`는 plan 이름을 대면 거부). "같은 plan, 다른 environment"는 plan 복제로만 표현 가능 — TASK-307 extends/alias와 결합해 `dva up infra --env test`류 허용 검토.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `interaction.db`의 `${POSTGRES_USER:-dev}`/`${POSTGRES_DB:-matdosa}` 보류 해제: 유지 확정. `source:` 형식 endpoints 전환은 별도 결정.
- (결정 반영) endpoints 리터럴 유지 확정(위와 같은 이유).
