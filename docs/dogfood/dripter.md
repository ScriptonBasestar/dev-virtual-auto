# dripter dva 적용 분석

## 현황
- 파일: `dva.yml` (490줄), `version: "0.1.44"`
- 섹션: `env_file(files 형식)`, `stack`(infra-compose 단일, runners 형식 + 서비스별 tags + stack-level health_checks), `plans`(7개), `default_plan`, `checks`, `suggestion_ignore`(대량), `interaction`(build replace hook, `project:build` 등 콜론 이름), `provision`, `endpoints`
- `dva validate`: **valid** (warning 1건)

## 문제점
- **중복 plan 쌍**: `infra`==`local-infra` (validate warning, 92–106행). 별칭 중복 선언.
- **plans에 `environment`/`site` 미지정** (91행 이하): 스키마상 유효하지만 `environments`/`sites` 섹션 자체가 없어 향후 stg 분기 시 구조 추가가 필요. canonical 예시(41 §9)와의 형태 차이.
- 앱(backend Ktor / frontend Astro)이 **stack 엔트리로 선언되지 않음** — native 실행이 Make/`hybrid` plan 주석("run backend/frontend natively")으로만 존재. suggestion_ignore 주석(226행)도 "subprojects 복원 전까지 Make-only"라고 부채를 인정. 새 모델에서는 native runner stack 엔트리 + plan entry가 정답.
- `subprojects:` 섹션 없음 — 루트가 서브프로젝트 커맨드를 노출하지 못하고 interaction에 `cd dripter-engine-ktor && …` 하드코딩이 반복됨 (297–402행).
- suggestion_ignore가 110여 항목으로 비대 — Makefile 표면과 DVA 표면의 이중 관리 비용이 그대로 드러남.

## dva 개선 힌트
- plan alias 기능 수요 (cwrapper와 동일).
- **suggestion_ignore 비대화**: glob 패턴은 지원되지만 카테고리 단위 제외(예: 특정 Makefile include 파일 전체 무시)가 없어 목록이 수백 줄로 자람 — ignore 소스 파일 단위 옵션 고려 여지.
- native 앱을 아직 stack으로 못 올린 이유가 "체크아웃이 없을 수 있는 서브프로젝트" — stack 엔트리에 optional/존재 조건(디렉토리 없으면 skip) 개념이 없어서 선언을 미루게 됨.

## 마이그레이션 난이도
**하~중** — 구조는 신형. 남은 작업은 native 앱의 stack 엔트리화 + subprojects 등록으로, 기계적이지만 서브프로젝트 체크아웃 전제가 필요.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] 중복 plan `infra` 삭제, `local-infra` 유지. 근거: README.md, CLAUDE.md, ENV_ARCHITECTURE.md, deploy/README.md, deploy/runbooks/local.md, docs/*가 전부 `dva up local-infra`를 안내하고 `dva up infra` 참조는 0건.
- [x] Ktor/Astro 앱을 native runner stack 엔트리로 선언 — `stack.backend-native`(dir dripter-engine-ktor, build `./gradlew build -x test`, run `./gradlew :server:run`, http health :16200/health), `stack.frontend-native`(dir dripter-frontend-astro, build `pnpm build`, run `pnpm dev`, tcp health :16201). 커맨드는 각 서브프로젝트 dva.yml의 `project:run`/`project:build`에서 가져옴.
- [x] 신규 plan `local-dev` 추가 — infra-compose(postgres, redis) → backend-native → frontend-native (depends_on 체인). 기존 `hybrid`(infra + Adminer/RedisInsight)는 문서가 그 의미로 안내하므로 그대로 유지.
- [x] `subprojects:` 도입 — `backend`(dripter-engine-ktor), `frontend`(dripter-frontend-astro), `exclude_tags: [infra]`, 서브프로젝트 interaction import. `dva ls`에서 `backend/test`, `frontend/project:run` 등 노출 확인.
- [x] `cd … &&` 하드코딩 제거(부분) — 단일 서브프로젝트만 대상으로 하던 subcommand(`test backend/frontend`, `test integration mock`, `lint backend/frontend`, `fmt backend/frontend`, `project:build frontend`)를 삭제하고 import된 `backend/test`, `frontend/test unit`, `frontend/test integration`, `backend/lint`, `frontend/lint`, `backend/fmt`, `frontend/fmt`, `frontend/project:build`로 대체. 두 디렉토리를 순회하는 집계 커맨드(`test`, `lint`, `fmt`, `check`, `project:build all`, `project:clean`)와 `test e2e`(Makefile `test-e2e` suggestion 매핑 유지 필요)는 `cd` 체인을 유지 — 아래 dva 개선점 참조.
- [x] `interaction.build` replace 훅 삭제 — `dva --dry-run build local-dev`가 compose build(postgres, redis) + `./gradlew build -x test` in dripter-engine-ktor + `pnpm build` in dripter-frontend-astro 를 순서대로 계획함을 확인. bare `dva build`는 이제 default_plan(local-infra)의 compose build만 수행하므로 앱 빌드는 `dva build local-dev`로 호출.
- [x] suggestion_ignore 주석("native lifecycle stays Make-only until subprojects are restored") 현행화.
- [ ] suggestion_ignore 110항목 재검토 — 보류 `[dva 선행: TASK-309]`.
- [ ] plans의 `environment`/`site` 미지정 — 스키마상 유효하고 PLAN.md 항목이 아니라 미변경.

### validate 최종 출력
```
✅ dva.yml is valid
EXIT=0
```
warning 0건.

### 보류/예외 항목
- suggestion_ignore 재검토(TASK-309 선행).
- 집계 interaction의 `cd … && cd ../… &&` 체인 유지(local runner가 `workdir`를 무시하므로 대안 없음).
- `docker:build`(및 backend/frontend 하위)는 `docker compose -f deploy/local/compose.yaml --profile apps build`를 직접 호출 — `dva build full-stack backend frontend`로 대체 가능하지만 `--profile apps` 전달 여부가 불확실해 유지.

### 발견된 dva 개선점
1. **`interaction.workdir`가 local runner에서 무시됨.** 스키마는 `workdir`를 interaction 공통 필드로 두지만 `internal/runner/`에서 `Workdir`를 읽는 곳은 `docker_compose.go`뿐(`grep -rn Workdir internal/runner/`). `runner: local` interaction에 `workdir: dripter-engine-ktor`를 줘도 현재 디렉토리에서 실행된다. 이 때문에 devbox 루트의 서브프로젝트 대상 interaction은 전부 `cd X && …` 하드코딩을 강제당한다(funbricks-notifire의 `workdir`+`cd` 중복도 같은 원인으로 추정). Repro: `interaction.x: {runner: local, workdir: dripter-engine-ktor, command: pwd}` → `dva run x`가 루트 경로를 출력.
2. **stack native 엔트리에 optional/존재 조건 부재**(기존 힌트 재확인) — 서브프로젝트 체크아웃이 없으면 `dva up local-dev`가 실패할 수밖에 없음. 이번에는 두 체크아웃이 모두 존재해 선언했다.
3. subcommand 삭제 시 Makefile suggestion 매핑(`test e2e` ↔ `test-e2e`)이 깨져 warning이 생김 — 서브프로젝트 import된 `frontend/test e2e`는 매핑으로 인정되지 않는다. import된 interaction도 suggestion 매칭 대상에 포함하면 루트 중복 선언을 줄일 수 있다.
