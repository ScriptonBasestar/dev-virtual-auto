# primeno1-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (16,130 bytes, 438줄), `version: "0.1.0"` — **8개 중 유일한 구세대 config**
- 사용 섹션: 최상위 `environment:`(구 형식), `env_file`, `stack`(7 엔트리, `order:`/`script:` 포함), `checks`, `default_mode`, **`modes:`(6개)**, `suggestion_ignore`, `interaction`(clean/logs replace 훅 포함), `provision`, `subprojects`, `endpoints`
- **`plans` 없음, `environments`/`sites` 없음**
- `dva validate`: **실패(ERROR)** — `interaction.clean`: clean built-in 제거로 replace 훅이 붙을 대상이 없음
- `dva config migrate`(preview): "nothing to convert" + `Left for you`로 modes 6개 전부 수동 분해 대상 보고

## 문제점
- `modes:` + `default_mode` (L199, L212-235): docs/42 §11-1에서 "제거 후 분해" 대상인 legacy 축. mode당 stack 목록 선택이 plans의 책임을 대신하고 있다.
- `interaction.clean.replace` (L393-397): **validate 하드 에러**. clean은 built-in에서 제거됐고(`dva down <plan> --purge`로 흡수, docs/43 §16) 훅이 실행될 키가 없다. 일반 interaction(command/steps)으로 전환 필요.
- `stack.*.order` (external-db-contract L80, sigdock-local-runtime L175): 실행 순서는 plan 책임(docs/42 §11-1 "stack.*.order → plans.*.entries[].order") — plans가 없어 migrate도 이 order를 옮길 곳이 없다.
- `interaction.dev-up.command`가 `dva up -M full` 호출 (L280): mode 선택 플래그 의존 — plans 전환 시 함께 수정해야 할 내장 잔재.
- 최상위 `environment:` (L16-27): 신규 예시들은 전역 `vars:` + `environments.*.environment`를 쓴다. SigDock FAPI 계약 변수들이 environment 축 없이 전역 고정.
- 모든 mode가 `project_name: primeno1` 공유 + mode당 전체 -f 목록 복제 (L39-171): compose-observability/tracing이 base services 블록을 통째로 재선언 — plans의 entry 조합으로 표현했으면 사라질 중복.

## dva 개선 힌트
- `config migrate`가 modes를 "보고만" 하는 것은 설계대로지만(docs/43 §18-1), 이 config처럼 mode↔stack이 1:1에 가까운 경우는 `plans.<mode>.entries[].name = modes.<mode>.stack[]` 스캐폴드를 실제 YAML 조각으로 출력해 주면 수동 분해 비용이 거의 0이 된다 — 현재 출력은 필드 대응표뿐이다.
- clean replace 훅 에러 메시지는 훌륭하다(전환 경로 2가지 제시) — 다만 `config migrate`가 이 기계적 변환(replace → down.after 또는 standalone command)을 수행하지 않는 것은 gap.
- script runner 기반 gate 엔트리(order: -10, fail-closed 계약 L79-84, L173-181)는 "cross-entry rollback 부재"를 주석으로 우회 중 — plan 실행에서 선행 entry 실패 시 후속 중단 보장이 문서화/보강될 필요.

## 마이그레이션 난이도
**상** — 유일하게 validate가 실패하는 config. modes 6개→plans 수동 분해, stack order 이동, clean 훅 전환, `dva up -M` 호출부 수정, environment→environments 재배치가 모두 수작업이며, sigdock-local/external-db 같은 fail-closed gate 순서 의미가 plans 전환 후에도 보존되는지 검증까지 필요하다.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `dva config migrate` 미리보기: "nothing to convert". modes 6개 각각 "description → plans.<m>.description, stack → plans.<m>.entries[].name" 대응표만 출력(1:1 매핑인데 자동 변환 없음). `stack.*.order`(-10 두 곳), clean replace, 최상위 `environment`, `dva up -M` 언급 없음.
- [x] `interaction.clean` replace(validate 하드 에러) → `steps:` (`docker compose -f … down -v`, DESTRUCTIVE 설명).
- [x] modes 6개 + `default_mode` → plans: `minimal`, `full`(default_plan), `observability`, `tracing`, `sigdock-local`(sigdock-local-runtime order 10 → compose order 20 depends_on), `external-db`(external-db-contract order 10 → compose-external-db order 20 depends_on). `stack.*.order: -10` 제거 후 plan entry order로 이관(게이트 선행 유지).
- [x] **entry 조합 시도 → 되돌림**: overlay 엔트리를 base와 plan에서 조합하면 TASK-288 경고(validate_warnings.go:893)로 거부되어 원래 형태(엔트리당 전체 -f 목록)로 복귀. 엔트리 이름을 `observability`/`tracing`으로 정리, 미사용 앵커 제거.
- [x] `dev-up`: `dva up -M full` → `dva up full`. sigdock-browser-e2e 주석의 `-M sigdock-local`도 갱신.
- [x] 최상위 `environment:` → `vars:` (loadEnv 순서 vars < environment < env_file이라 interaction에 보이는 값 동일). `environments.dev.environment: {PRIMENO1_ENV: development}`로 환경 축 부여.
- [x] `env_file` map 형식 → 리스트 형식. `logs` replace 제거(`dva logs full`). provision `docker compose up` → `compose_up: [postgres, redis, zookeeper, broker, schema-registry]` (PrimaryComposeEntry = order 없을 때 이름순 최소 = `compose`이므로 base 파일 사용).
- [x] version 0.1.44. Makefile 제안 `validate*`/`version` suggestion_ignore 추가.

### validate 최종 출력
```
[warn] semantic: env/docker-compose/docker-compose.yml: missing top-level 'name: primeno1'
✅ dva.yml is valid
EXIT=0   (warning 1 — 의도적 예외)
```

### 보류/예외 항목
- compose 파일 `name:` 누락 경고 — 다른 파일 수정 금지 → 예외.
- `scripts/dev-run.sh`, `scripts/seed-data.sh`, `scripts/sigdock-local-up.sh`가 여전히 `dva up -M <mode>`를 호출(다른 파일 → 미수정, **plans 전환 후 깨짐** — 사용자 후속 필요).
- `${POSTGRES_DB:-primeno1_dev}` ×2, `${TOPIC:-test}` — TASK-303, 미수정.
- `docker-compose.task44.verify.yml` / `task45.verify.yml` 미등록(검증 픽스처). env/docker-compose/ 하위라 drift 감지 대상이 아니어서 경고 없음.

### 발견된 dva 개선점
- **overlay 조합 불가 (설계)**: base+overlay 엔트리 조합은 TASK-288 경고라 overlay마다 base 서비스/태그를 재선언해야 한다(구조적 중복). 개선 후보는 TASK-307의 엔트리 `extends:`/PlanEntry `overlays:`.
- **PrimaryComposeEntry 암묵 선택**: order 제거 후 primary compose 엔트리는 이름순(internal/config/lifecycle_helpers.go:164). compose 엔트리가 여러 개인 config에서 provision compose_up / `dva db`(service 지정 interaction)가 어느 파일 세트로 실행되는지 문서화·명시 수단(`primary: true` 등) 필요.
- **migrate가 modes.*.stack → plans 자동 변환 안 함**(1:1 매핑), `dva up -M` 잔재(interaction command·스크립트) 탐지 없음. TASK-306.
- **drift 감지가 서브디렉터리 compose 파일을 보지 않음**(env/docker-compose/*.verify.yml) — sigdock-pass의 `compose-*.yaml` 미감지와 같은 계열.

## CLI 잔재 정리 (2026-09-05)
실행 파일:
- scripts/dev-run.sh:37, :41, :42 `dva up -M $MODE` → `dva up $MODE` (MODE 값 full/minimal/external-db = plan 이름 그대로)
- scripts/seed-data.sh:96 `dva up -M minimal/full` → `dva up minimal/full`
- scripts/sigdock-local-contract.sh:65 `dva down -M sigdock-local` → `dva down sigdock-local`
- scripts/sigdock-local-up.sh:430 `adjacent_dva up -M infra` → `adjacent_dva up infra` (sigdock-idp-devbox의 plan `infra`)
문서:
- CLAUDE.md:34-37 (AGENTS.md 심링크) `dva up -M X` → `dva up X`; :39 `dva stack down -v` → `dva down full --volumes`; :41 `dva logs` → `dva logs full`; :50 `dva clean` → `dva run clean`; :172 `dva dev` → `dva frontend-dev`; "(via dva mode)" → plan
- README.md:34-37, :70 `dva up -M X` → `dva up X`
- docs/LOCAL_EXECUTION_GUIDE.md:101-113, 172, 191, 223, 245, 273, 336-347, 382-385, 417 `-M` 제거; :118/389/458 `dva logs` → `dva logs full`; :121-123 `dva stack down -v` → `dva down full --volumes`, 흐름 문구 갱신; :122/414 `dva clean` → `dva run clean`
- docs/CICD_GUIDE.md:73, docs/MULTI_REPO_STRUCTURE.md:134 `-M` 제거
- 보류 0. 산문 속 "mode"(SigDock IDP local mode, external DB mode 절 제목)는 CLI 표면이 아니라 유지.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `${POSTGRES_DB:-primeno1_dev}` ×2, `${TOPIC:-test}`: 유지 확정.

## 2차 재검증 (2026-09-05, dva ecae43d: TASK-305/306/308 반영 빌드)

- (TASK-308 semantic warning으로 발견) `interaction.dev-up.command`의 `dva up -M full` 잔재 → `dva up full`. 이전 CLI 잔재 정리에서 dva.yml 내부 문자열은 놓쳤음.
- (결정 반영) bare `dva down` → `dva down full` (CLAUDE.md, docs/LOCAL_EXECUTION_GUIDE.md ×3), sigdock-idp 호출은 `dva down infra`.

## docs/57 §4 재점검 (2026-09-05, TASK-310 가이드 기준)

- §4-1 해당(미적용, 소유자 결정): 앱 프로세스 4종이 stack 엔트리 없이 interaction으로만 기동된다 — `api-run`, `api-run.gateway`,
  `api-run.external-db`, `api-run.stream.external-db`, `frontend-dev`. `dva status`가 앱을 못 보고 `dva down`이 앱을 남긴다.
- 미적용 이유: 각 기동이 SigDock contract gate(`scripts/sigdock-local-contract.sh`), TLS 검증 wrapper(`--exec`), external-db credential wrapper와
  결합돼 있고 health 경로가 확인되지 않아 실기동(TASK-311/312 이후) 없이 옮기면 검증 불가.
- 권장안: native 엔트리 4종 + plan `dev`, 문서 참조 8곳 `dva up dev`로 치환 (아래 적용).

### 권장안 적용 (2026-09-05, 소유자 수용)

- native 엔트리 6종 추가: `api`/`gateway`/`stream`/`frontend`(plan `dev`, `dev-stream`) + `api-external-db`/`stream-external-db`(plan `external-db`).
  `run:`은 gate 스크립트 체인을 그대로 두고 마지막을 `exec`로 넘겨 Gradle/Vite 프로세스가 dva의 추적 대상이 되게 했다(§3 devbox 소유 gate 스크립트 허용).
- interaction `api-run*`/`frontend-dev`/`dev-up` 삭제, 문서·스크립트 11곳을 `dva up dev`/`dev-stream`/`external-db`로 치환. `dva validate` warning 0.
- 검증 한계: `dva --dry-run up dev`는 TASK-312로 health 대기에 걸려 멈춤(kill 필요). 실기동 검증은 TASK-311/312 이후.
