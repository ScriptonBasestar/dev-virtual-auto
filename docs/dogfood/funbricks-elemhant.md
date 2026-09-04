# funbricks-elemhant-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (5,817 bytes, 197줄) — Container-First 패턴(전 서비스 Docker Compose)
- 사용 섹션: `version`, `env_file`(files 객체 형식), `stack`(main-compose 1개), `plans`(local-infra/full-stack), `default_plan`, `environments`(dev), `checks`, `suggestion_ignore`(glob 위주 12항목), `interaction`, `provision`(default/reset + `default_profile`), `endpoints`
- `dva validate`: **통과** (Makefile `check` 매핑 suggestion warn 1건)

## 문제점
- 구조 위반 없음. 신규 모델 준수.
- `sites` 미사용, plans에 `site` 미지정 (L36-52): 단일 로컬 환경이라 의도적 생략으로 보이나, 타 devbox와의 일관성은 떨어진다. 스키마상 optional이므로 위반은 아니다.
- `interaction.clean` (L95-99): steps 형식의 일반 파괴적 명령으로 재정의 — 주석에 "clean is no longer a lifecycle built-in" 명시, 올바른 패턴.
- `interaction.build`/`logs` replace 훅이 `docker compose -f docker-compose.yml ...`을 직접 호출 (L90-104): stack 선언(`files: [docker-compose.yml]`, `project_name: funbricks-elemhant`)과 compose 파일/프로젝트명이 중복 하드코딩됨 — stack 선언이 바뀌면 훅이 조용히 어긋난다.
- `endpoints.*.url`이 `${VAR:-default}` 확장에 의존 (L171-195): gorisa 리포트에 기록된 dva 확장기 버그(변수 설정 시 `:-` 형식 오염)의 잠재 영향권.

## dva 개선 힌트
- replace 훅에서 stack 선언의 compose files/project_name을 재사용할 참조 수단(예: `dva compose build` passthrough 또는 훅 내 선언 변수)이 없어 중복 하드코딩을 강제한다.
- Container-First 프로젝트에서 full-stack plan이 services 생략으로 "전체 서비스"를 의미(L48-51) — 이 암묵 의미(services 미지정 = 전부)를 문서에 명시할 가치가 있다.

## 마이그레이션 난이도
**하** — 신규 구조 완전 준수. replace 훅의 하드코딩 정리는 선택 개선.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `interaction.build` replace 훅 삭제 — `dva build full-stack`이 stack 선언에서 `docker compose -f …/docker-compose.yml --project-name funbricks-elemhant build`를 생성함을 `--dry-run`으로 확인 (동일 동작, 하드코딩 제거).
- [x] `interaction.logs` replace 훅 삭제 — `dva logs full-stack -f`가 `… --project-name funbricks-elemhant logs -f`로 실행됨을 `--dry-run --debug`로 확인.
- [x] `interaction.clean` 유지(이름은 `.make/dev-full.mk:39`가 `dva clean`을 호출하므로 보존), 실행 내용을 `docker compose -f docker-compose.yml down -v` → `dva down full-stack --volumes`로 교체. compose 파일명 하드코딩 제거, stack 선언 재사용. `dva down`이 `--volumes/-v`를 지원함을 `dva down --help`와 `--dry-run`(`down --remove-orphans --volumes`)으로 확인.
- [x] `provision.reset` 1단계도 동일하게 `dva down full-stack --volumes`로 교체.
- [x] `interaction.check` (`make check`) 추가 — 유일했던 validate 경고(Makefile `check` 타깃 미매핑) 해소.
- [x] `docker-compose.yml`은 `compose.yaml`의 symlink임을 확인 (`ls -la`). stack이 선언한 `docker-compose.yml` 경로를 그대로 유지, 두 파일이 별개가 아니므로 통일 작업 불필요.
- [ ] `sites` / plan `site` 미지정 — PLAN 항목 아님, 단일 로컬 환경이라 미변경.
- [ ] `endpoints.*.url`의 `${VAR:-default}` (L167, L171, L177, L184, L190: RUSTFS_PORT, RUSTFS_CONSOLE_PORT, ELEMHANT_SIGDOCK_IDP_PORT, GO_GATEWAY_PORT, WORKER_PORT) — TASK-303 규칙에 따라 손대지 않음.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0
```
warning 0 (기준선: warning 1 — `check` 타깃).

### 보류/예외 항목
- 없음 (`${VAR:-default}` 5곳은 규칙상 유지, 위 목록 참조).
- 검증 목적으로 `build`/`logs`/`down`/`run clean`을 `--dry-run`으로만 호출했고 실제 실행은 없었음.

### 발견된 dva 개선점
- `dva --dry-run build <plan>` 출력은 `--project-name`을 포함하지만 앞선 실행(훅 존재 시)에서는 replace 훅이 plan 인자를 무시하고 그대로 실행됨 — 훅이 plan 인자를 받아 stack 선언을 참조할 수단이 없어 사용자가 compose 파일/프로젝트명을 중복 기재하게 됨(기존 리포트 힌트와 동일). 훅 삭제로 회피.
- interaction step에서 `dva down …`을 재귀 호출하는 패턴은 동작하나, `clean` 같은 "down + 추가 정리"를 dva가 1급 개념으로 제공하지 않아(예: `dva down --purge` 후속 hook) 재귀 호출이 유일한 선언 재사용 경로다. Repro: `interaction.clean.steps[0].run: dva down full-stack --volumes` → `dva --dry-run run clean`은 step만 표시하고 내부 down의 dry-run 계획은 보이지 않음.

## CLI 잔재 정리 (2026-09-05)
- .make/dev-full.mk:36, :39 `dva clean` → `dva run clean` (clean interaction = `dva down full-stack --volumes`)
- 보류 1: scripts/tests/test-dva-clean-command.sh:2/58/60/78 — `dva manifest`가 생성하는 `usage_example == "dva clean"`을 단언하는 테스트. `dva clean`은 clean이 reserved에서 빠져 dynamic routing으로 유효한 호출이며, 단언 값은 dva 출력이므로 수정 대상 아님(의도적 유지).

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `endpoints.*.url`의 `${VAR:-default}` 5곳: 유지 확정.
- (결정 반영) bare `dva down` → `dva down full-stack` (.make/dev.mk).
