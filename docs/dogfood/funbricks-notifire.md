# funbricks-notifire-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (10,531 bytes, 304줄)
- 사용 섹션: `version`, `stack`(plan-dispatcher 1개, compose 3파일), `plans`(local-infra/review-backend/local-full), `interaction`(17개, 대부분 `runner: local` + `shell: true` 인라인 스크립트)
- 미사용: `env_file`, `environments`, `sites`, `default_plan`, `checks`, `provision`, `endpoints`
- `dva validate`: **통과** (semantic warn 1건: plans 3개인데 `default_plan` 미설정)

## 문제점
- 구조 위반 없음(신규 stack/plans 모델). 다만:
- `default_plan` 미설정 (validate warn): `dva up`이 plan 이름 필수가 됨 — 편의성 결손.
- `stack.plan-dispatcher.runners.compose.services`가 빈 객체 값(`postgres: {}` 등, L15-20)으로 태그도 없이 나열됨: 선언 가치가 없고 plans의 services 목록과 중복 — 어느 쪽이 정본인지 흐린다.
- 인라인 셸/파이썬 스크립트가 interaction command에 대량 내장 (L131-304, `frontend-quality`/`frontend-mockapi`/`frontend-pwa`/`env-status` 각 30-70줄): dva.yml이 스크립트 저장소가 됐다. `scripts/*.sh`로 빼는 것이 유지보수·diff 가독성에 맞다.
- `workdir:` 지정 후 command 안에서 다시 `cd`(L58-63 backend-quality 등): workdir 의미와 중복 — 어느 쪽이 실효인지 혼란 지점.
- `endpoints` 부재: backend/frontend 서비스가 있는데 `dva up` 후 접근 URL 안내가 없다.

## dva 개선 힌트
- 장문 스크립트를 interaction에 넣는 수요가 실재한다 — `command_file:`(외부 스크립트 참조) 또는 스크립트 길이 lint가 있으면 이 안티패턴을 구조적으로 유도해 줄일 수 있다.
- `workdir` + command 내 `cd` 중복은 `workdir` 실효 여부를 검증/경고하는 semantic check 후보.
- services를 태그 없이 `{}`로 나열해야 하는 형식(맵 강제)이 어색하다 — 태그 없는 서비스는 리스트 표기도 허용하면 자연스럽다.

## 마이그레이션 난이도
**하** — 이미 신규 구조. default_plan 지정 + 스크립트 외부화가 개선 항목이며 구조 변경은 불필요.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `default_plan: local-infra` 추가 — validate warn 해소.
- [x] `stack.plan-dispatcher.runners.compose.services`(`postgres: {}` 등 태그 없는 빈 객체 6개) 삭제 — plans의 `services` 목록을 단일 정본으로. 스키마상 `files`/`up_options`가 있으면 `services`는 선택이라 검증 통과.
- [x] 인라인 스크립트 6건을 `scripts/dva-*.sh`로 추출하고 dva.yml은 `command: bash scripts/<name>.sh`로 호출 (기존 `react-review-*`/`image-scan` 관례와 동일). 추출 대상: `backend-quality`, `backend-notification-controller`, `frontend-quality`, `frontend-mockapi`, `frontend-pwa`, `env-status`. 스크립트는 `cd "$(dirname "$0")/.."`로 루트를 고정해 실행 cwd에 독립적. 내용은 원문 그대로 이관(동작 보존), `sh -n` 문법 검사 통과.
- [x] `workdir:` + `cd` 중복 제거 — `workdir`는 삭제하고 스크립트 내부 `cd`만 유지 (아래 dva 개선점 참조: `runner: local`에서는 `workdir`가 무시되므로 실효는 원래 `cd` 쪽이었음).
- [x] `endpoints:` 추가 — compose.yml/PORT_MAPPINGS.yaml의 기본 호스트 포트를 `url:` 형식으로 등록 (backend 16100, frontend 16130, postgres 16110, nats 16140/16141, clickhouse 16150/16151). `source:` 형식은 compose 포트가 `${BACKEND_PORT:-16100}` 표현이라 TASK-303 확장기 버그 영향권이어서 피함.
- [x] section order warn (endpoints가 interaction 앞) — canonical 순서로 재배치.
- [ ] (보류) compose.monitoring.yml의 prometheus/grafana/loki/tempo: 어떤 plan도 참조하지 않아 endpoints에 넣지 않음. monitoring plan 신설은 범위 밖.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 0)
```

### 보류/예외 항목
- monitoring 서비스는 plan 미참조 상태 그대로 둠 (위).
- `task-validate`/`backend-test` 등 한 줄짜리 interaction의 `shell: true`는 그대로 둠 (동작 보존).

### 발견된 dva 개선점
1. **`interaction.workdir`가 `runner: local`에서 무시된다.** `internal/runner/local.go`의 어떤 form도 `Cmd.Workdir`로 chdir하지 않고, `Workdir`는 `internal/runner/docker_compose.go`(compose exec/run `--workdir`)에서만 소비된다. 이 프로젝트의 `workdir: notifire-backend-phoenix` + 명령 내 `cd notifire-backend-phoenix` 중복은 그 결과. repro: `interaction.x: {runner: local, workdir: sub, command: pwd}` → `dva run x`가 루트 경로를 출력. validate도 경고하지 않음. 수정 후보: local runner가 workdir를 적용하거나, local runner에서 workdir 지정 시 semantic warn.
2. **`script_file:`이 이미 존재하나 실행 방식이 `exec path`라 shebang·실행권한을 요구한다** (`internal/exec/exec.go:230 ExecScriptFile`). 리포트의 "command_file" 힌트는 이미 충족되지만, 문서/`dva init` 안내가 없어 프로젝트들이 `command: bash scripts/x.sh` 관례를 쓴다. `script_file`을 인터프리터 지정(`sh`/`bash`) 가능하게 하거나 문서에 노출하면 인라인 스크립트 안티패턴 유도에 도움.
3. compose `services:` 맵에 빈 객체를 나열하는 형식 — 태그 없는 서비스는 리스트 표기 허용 요구 (기존 힌트 재확인).

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- endpoints `source:` 형식(compose의 `${BACKEND_PORT:-16100}`)으로 전환할지는 수동 결정(현재 `url:` 유지).

## 결정 반영 (2026-09-05)

- `scripts/dva-*.sh` 6개 수용 확정(단독 실행·테스트 가능, interaction 다중행 quoting 회피). 커밋 시 함께 포함할 것.
- endpoints `source:` 전환 보류 확정: dva가 endpoints url/source의 `${VAR}`를 치환하지 않음(TASK-323 기록).
