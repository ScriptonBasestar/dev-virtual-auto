# familybook dva 적용 분석

## 현황
- 파일: `dva.yaml` (288줄, canonical 이름 `dva.yml` 아님), `version: "0.1.26"`
- 섹션: `env_file(files+priority+interpolate)`, `stack`(legacy flat 형식), `modes`(3개), `environments`(dev/test — legacy 필드 포함), `health_checks`(최상위, start 포함), `interaction`, `provision(default_profile)`, `checks`
- `dva validate`: **ERROR — 파싱 실패**. 그룹 A 7개 중 유일하게 현행 구조로 로드조차 안 되는 설정.

## 문제점
- **stack flat 형식 (26–30행)**: `stack.compose`에 `order`/`files`/`project_name`을 직접 선언. validate가 "compose must be declared under runners.compose"로 즉시 거부.
- **`modes:` 섹션 (36–59행)**: 제거된 개념 (docs/42 §11-1 — plans/environments/sites로 분해 대상). `compose_files`/`compose_profiles`까지 mode에 선언.
- **`plans:` 없음**: 실행 표면이 전부 구세대. 문서 주석의 사용법도 구 CLI 기준.
- **`environments.*` legacy 필드**: `test.compose_files` (74–76행) — environment는 vars 중심으로 축소된 개념 (42 §11-1 `environments.*.stack_overrides` 계열 제거).
- **`interaction.clean.replace` (124–129행)**: `clean` built-in이 제거되어 이 hook은 어디에도 걸리지 않음 (다른 프로젝트에서 같은 패턴이 validate ERROR로 잡힘).
- **최상위 `health_checks`의 `start`/`start_hint` (85–108행)**: modes 제거 후 참조 주체가 없어 dead 선언.
- `version: "0.1.26"` — 구버전 명시. 파일명 `dva.yaml`도 rename 경고 대상.

## dva 개선 힌트
- **`dva config migrate`가 `dva.yaml`을 인식하지 못함**: `ERROR: no dva.yml in .` — validate는 dva.yaml을 읽고 rename 경고까지 내는데 migrate는 못 읽음. 가장 마이그레이션이 필요한 설정이 변환기를 못 쓰는 모순. migrate도 dva.yaml fallback(+rename 안내)을 지원해야 함.
- validate가 flat stack 에러에서 rewrite 예시를 보여주는 것은 좋으나, **첫 에러에서 멈춰** modes/clean 등 나머지 legacy 항목을 한 번에 보여주지 못함 — legacy 설정에는 에러 수집 모드가 유용할 것.

## 마이그레이션 난이도
**상** — stack 재작성 + modes 3개의 plans/entries 수동 분해 + environments legacy 필드 정리 + health_checks 재배치 + clean hook 이동 + 파일명 rename까지 전면 개편. migrate 도구도 현재 파일명 문제로 도움을 못 줌.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `dva config migrate` (dva.yaml, rename 전) 원문: `ERROR: no dva.yml in .` (exit 1) — validate는 같은 파일을 읽고 rename 경고를 내는데 migrate는 못 읽음. TASK-304 증거.
- [x] `git mv dva.yaml dva.yml` (커밋 없음). rename 후 migrate: `stack.compose → runners.compose`만 변환. 남긴 항목: `stack.*.order`("plans가 없어 옮길 곳 없음"), modes 3개(description → plans.<mode>.description, `compose_profiles → up_options, as --profile full-stack` / `--profile monitoring` — 모드 이름을 profile 값으로 오기), `environments.test.compose_files` 언급 없음, clean replace 언급 없음.
- [x] flat stack → `stack.compose.runners.compose`, `order` 제거.
- [x] modes 3개 → plans: `infra`(compose), `full-stack`(stack.apps = compose.yaml+compose.apps.yaml, services postgres/redis/backend-dev/frontend-dev), `monitoring`(stack.monitoring = base+overlay, monitoring profile 서비스 12개 명시). 추가로 `hybrid`(compose + native backend + native frontend) 및 `test`(stack.test-stack = compose.test.yaml, project familybook-test). `default_plan: infra`.
- [x] `environments.test.compose_files` 제거 → `stack.test-stack` + `plans.test`. `environments.dev.env_file: .env` 제거(루트 env_file이 이미 .env를 읽음).
- [x] 상위 `health_checks` → `stack.compose.health_checks`(postgres/redis tcp), `stack.backend`/`stack.frontend` native 엔트리의 health_checks(http :15000/health, :15001). `start`/`start_hint` 삭제.
- [x] `interaction.clean` replace → `steps:`; `build`/`logs` replace 제거(`dva build hybrid`가 native build 실행, `dva logs <plan>`). 파일 로그 tail은 `logs-backend`/`logs-frontend` 유지.
- [x] `backend-start`/`frontend-start` interaction 제거 → `dva up hybrid` (native 엔트리). 단독 기동이 필요하면 `dva up hybrid`가 compose까지 같이 올린다는 점 유의(플래그).
- [x] `env_file` map 형식(`files:`/`priority`/`interpolate`) → 리스트 형식. `priority`/`interpolate` 키는 스키마에 없어 삭제(동작 동일: env_file이 environment 위에 덮임).
- [x] `checks`의 `type: tcp` 2개 → `type: command` + `nc -z` (schema가 tcp check를 허용하지 않아 flat-stack 에러 뒤에 숨어 있던 스키마 에러).
- [x] provision `docker compose up -d` → `compose_up: [postgres, redis]`; reset의 `docker compose down -v`는 유지(purge gap).
- [x] version 0.1.44. Makefile 제안 194개 → suggestion_ignore 글로브.

### validate 최종 출력
```
[warn] config drift: compose.files is compose.yaml, compose.apps.yaml, compose.monitoring.yaml, compose.test.yaml but detected root compose files are … compose.devcontainer.yaml, compose.gateway.yaml, compose.local-dev.yaml, … compose.sigdock-dev.yaml, compose.sigdock.yaml …
✅ dva.yml is valid
EXIT=0   (warning 1 — 의도적 예외)
```

### 보류/예외 항목
- drift 경고: compose.devcontainer / gateway / local-dev / sigdock-dev / sigdock 5개는 미등록(devcontainer·gateway·sigdock 연동은 Makefile compose-up-* 타깃 영역). 무시 수단 없음(TASK-309).
- `${REDIS_PASSWORD:-changeme}` (interaction.redis-console) — TASK-303, 미수정.
- monitoring plan은 profile 서비스를 이름으로 명시(elk/tracing profile은 제외 — 원본 mode도 monitoring profile만).
- 등록한 native 엔트리(backend `make run`, frontend `make run-web`)와 plan은 실행 미검증.

### 발견된 dva 개선점
- **migrate가 dva.yaml을 못 읽음**: `ERROR: no dva.yml in .` — validate는 fallback+rename 경고를 내므로 migrate도 동일 fallback 필요. TASK-304.
- **validate 조기 중단이 스키마 에러까지 가림**: flat stack 에러를 고치자 `checks[].type: tcp` 스키마 에러가 새로 등장, 그걸 고치자 경고 195개 등장. 3단계 반복 필요. TASK-305.
- **migrate `--profile <모드명>` 오기** 재현(full-stack, monitoring).
- **migrate가 `environments.*.compose_files`, `env_file` map 형식의 잉여 키(priority/interpolate), 상위 health_checks start/start_hint를 전혀 언급하지 않음**.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `${REDIS_PASSWORD:-changeme}`: 그대로 유지, 정상.
