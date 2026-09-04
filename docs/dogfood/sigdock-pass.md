# sigdock-pass dva 적용 분석

## 현황
- 파일: `dva.yml` (2,155 bytes — 8개 중 최소, version `0.1.22`)
- 사용 섹션: `stack`(compose 1엔트리), `plans`(primary), `default_plan`, `environments`(dev), `suggestion_ignore`, `interaction`(6개), `provision`(init/init-sops)
- `dva validate` (0.1.48): **✅ valid** — 신모델(stack+plans+default_plan) 채택. nd-stack과 함께 2개뿐인 clean pass.

## 문제점
치명 오류 없음. 소소한 미비점만 있다.

1. **`stack.*.runners.compose.tags` (line 12)** — tags는 실행 제어에서 축소된 메타데이터(docs/42 §11-1). 무해하나 잔재.
2. **plan entry `services`에 있는 `sigdock-idp` (line 22)** — 루트에 compose overlay가 10개(`compose-ha.yaml`, `compose-federation.yaml`, `compose-saas-*.yaml` 등) 있는데 stack은 `compose.yml` 하나만 선언. HA/federation/chaos/saas 시나리오가 전부 DVA 밖에 있어 채택 폭이 얕다 — plan 1개(primary)뿐.
3. **`interaction.sigdock:logs`의 `command: ""` (line 74-77)** — 빈 명령 placeholder. 실행하면 무엇이 되는지 불명확하고, logs는 이제 plan-aware `dva logs`가 정위치.
4. **`interaction.minio`가 echo로 URL만 출력 (line 79-82)** — endpoints 섹션(이 파일엔 없음)이 정위치인 정보를 interaction으로 우회.
5. **`env_file`/`checks`/`endpoints` 미사용** — provision init-sops가 `.env` 생성을 다루면서 env_file 선언이 없어 우선순위 체계(environment < env_file < OS env) 밖에서 동작.

## dva 개선 힌트
- "최소 채택" 프로필: 신모델 문법은 따랐지만 plan 1개·overlay 10개 미등록. overlay 파일 다수가 있는 프로젝트에 대해 nd-stack형 다중 plan 스캐폴드를 제안하는 `dva init --from-compose` 류 발견 기능이 있으면 이런 얕은 채택을 끌어올린다 (sigdock-idp의 drift 경고와 같은 계열 — 단 이 프로젝트에서는 drift 경고가 안 떴는데, `compose.yml`(비표준 단수형) 외 `compose-*.yaml` 패턴이 감지 대상인지 확인 가치).
- 빈 `command: ""`가 validate를 통과한다 — 빈 실행 타깃 경고 후보.

## 마이그레이션 난이도
**하(완료)** — 이미 신모델. 남은 것은 채택 심화(overlay용 stack 엔트리·plans 추가, env_file/endpoints 도입)로 마이그레이션이라기보다 확장이다.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `dva config migrate` 미리보기: "nothing to convert" (이미 신모델).
- [x] `sigdock:logs` (`command: ""` placeholder) 제거 — `dva logs primary`가 대체.
- [x] `env_file` 추가: `.env.example`(required) → `.env`(optional, 덮어씀).
- [x] `minio` echo interaction 제거 → `endpoints` 신설(server 11900, sigdock-idp 11990, minio-console 11971, minio-api 11970; 포트는 .env.example 기본값).
- [x] `stack.compose.runners.compose.tags: [infra, primary]` → `[infra]` (plan 이름을 tag로 중복하지 않음).
- [x] overlay 판정: **등록** `compose-ha.yaml`(stack/plan `ha-postgres`), `compose-redis.yaml`(`ha-redis`), `compose-federation.yaml`(`federation`, scripts/test-federation.sh와 짝) — 로컬에서 띄우는 픽스처, .env.example에 포트 존재. **미등록(예외)** `compose-airgap`, `compose-cpd`, `compose-saas`, `compose-saas-aws/azure/gcp`(배포 모드/클라우드 CI 스모크), `compose-chaos`(카오스 테스트 전용).
- [x] version 0.1.44.
- [x] interaction.db 설명의 "(requires postgres profile)" 오기 정정(compose.yml에 profile 없음).

### validate 최종 출력
```
[warn] config drift: compose.files is compose.yml, compose-federation.yaml, compose-ha.yaml, compose-redis.yaml but detected root compose files are compose.yml; …
✅ dva.yml is valid
EXIT=0   (warning 1 — 의도적 예외, dva 결함)
```

### 보류/예외 항목
- 위 drift 경고: overlay를 **등록했더니** 오히려 경고가 발생(감지기가 `compose-*.yaml` 패턴을 모름). 등록을 유지하고 예외로 기록.
- `${MINIO_CONSOLE_PORT:-11971}`은 minio interaction 제거로 소멸. endpoints는 `${}`를 쓰지 않고 리터럴 포트를 씀(TASK-303 회피) — .env에서 포트를 바꾸면 endpoints가 어긋남. 향후 endpoints에서 env 치환이 안정되면 전환.
- 등록한 3개 overlay plan은 미검증(라이프사이클 실행 금지) — 파일 헤더의 `docker compose -f … up -d` 사용법을 그대로 옮긴 것.

### 발견된 dva 개선점
- **drift 감지 비대칭**: `compose.yml` + `compose-*.yaml` 명명은 "detected root compose files"에 포함되지 않아, (a) 미등록 상태에선 overlay 10개가 있어도 경고 없음, (b) 등록하면 "선언 파일이 감지 목록에 없음"으로 경고. repro: 이 프로젝트에서 `dva validate` 전/후 비교. 감지 패턴을 `compose[-.]*.y?(a)ml`로 넓히거나, 선언된 파일이 실존하면 drift로 보지 않아야 함.
- **빈 `command: ""`가 validate 통과** (원본 sigdock:logs). 빈 실행 타깃 경고 필요.
- **endpoints의 `${VAR:-default}` 사용 가능 여부 불명확** — TASK-303 해결 후 endpoints url에 env 치환을 허용/문서화하면 리터럴 포트 중복을 피할 수 있음.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- endpoints를 리터럴 포트 대신 `${MINIO_CONSOLE_PORT:-11971}` 등 env 치환으로 되돌릴지는 수동 결정(현재 리터럴 유지).
- (결정 반영) endpoints 리터럴 포트 유지 확정 — dva가 endpoints url을 치환하지 않음(`dva show`로 확인, TASK-323 기록).
