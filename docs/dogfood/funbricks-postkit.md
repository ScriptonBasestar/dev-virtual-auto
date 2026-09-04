# funbricks-postkit-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (3,471 bytes, 174줄) — 8개 중 가장 작고 균형 잡힌 config
- 사용 섹션: `version`, `env_file`(files 객체 형식), `stack`(devbox compose + engine/ui native), `plans`(local-infra/local-dev/full-stack), `environments`(dev/ci), `sites`(local + entry_overrides), `suggestion_ignore`, `interaction`(6개), `endpoints`
- 미사용: `default_plan`, `checks`, `provision`
- `dva validate`: **통과** (semantic warn: `default_plan` 미설정, Makefile `check`/`check-archive-drift` suggestion 2건)

## 문제점
- 구조 위반 없음. 교과서적인 hybrid(인프라 Docker + 앱 native) 신규 모델 적용.
- `default_plan` 미설정 (validate warn): plans 3개 — `local-dev`나 `local-infra`를 지정하면 `dva up` 단축이 산다.
- `sites.local.entry_overrides`의 engine/ui native 오버라이드 (L100-104): 두 엔트리의 `default_runner`가 이미 native라 no-op — flow-taskchain과 동일한 군더더기 패턴.
- `stack.engine`/`ui`의 `dir: .` (L35, 44): native runner가 루트에서 make 위임 — 실제 subproject 디렉터리(postkit-engine-fiber 등)가 있는데 Makefile 경유로 우회. 동작엔 문제없으나 선언의 자기서술성이 떨어진다.
- `provision` 부재: README/Makefile에 초기 셋업이 있다면 `dva provision`으로 노출할 후보.

## dva 개선 힌트
- no-op entry_overrides가 두 프로젝트(postkit, flow-taskchain)에서 반복된다 — "override가 default_runner와 동일" semantic warn을 추가하면 복붙 잔재를 걸러낼 수 있다.
- `default_plan` 미설정 warn이 실제로 두 프로젝트에서 방치돼 있다 — `dva init`/migrate가 plan이 1개 이상이면 default_plan 스캐폴드를 넣어주는 편이 낫다.

## 마이그레이션 난이도
**하** — 이미 완전 이행. default_plan 한 줄 추가 수준.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `default_plan: local-dev` 추가 — validate warn 해소.
- [x] `sites.local.entry_overrides`(engine/ui native→native, no-op) 삭제.
- [x] `stack.ui`: `dir: .` + `make build-ui`/`make run-ui` → `dir: postkit-ui-react` + `pnpm install && pnpm build` / `pnpm dev`. Makefile 타겟이 하던 일이 `cd $(POSTKIT_UI_REPO) && pnpm …` 뿐이라 동작 동일 (native run/build는 `sh -c`로 실행돼 `&&` 체인 가능). 잃는 것은 `require_checkout` 안내 메시지뿐이며 디렉터리 부재 시 fail-fast로 대체됨.
- [ ] (보류) `stack.engine`의 `dir: .` + `make build-engine`/`make run-engine` 유지. `run-engine`이 `.env` 존재 검사, `POSTKIT_DATABASE_URL`/`POSTKIT_REDIS_URL` 비어있음 검사, `POSTKIT_*`→`DATABASE_URL`/`REDIS_URL`/`PORT` 이름 매핑, mise 툴체인 선택을 수행하므로 직접 `go run`으로 바꾸면 실동작이 달라진다. `dir: postkit-engine-fiber` + `make -C .. run-engine`은 dir가 장식에 그쳐 채택하지 않음.
- [x] Makefile suggestion warn 2건(`check`, `check-archive-drift`) — interaction 직접 매핑 추가 (둘 다 개발 워크플로우 게이트).
- [ ] (보류) `provision` 부재 — Makefile `prepare`/`setup`이 있으나 셋업 노출은 계획 범위 밖.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 0)
```

### 보류/예외 항목
- engine native 엔트리의 `dir: .` (위 사유).
- `${VAR:-default}` 사용처 없음.

### 발견된 dva 개선점
1. no-op `entry_overrides` (override runner == `default_runner`) semantic warn 후보 — postkit/flow-taskchain 두 곳에서 실증 (기존 힌트 재확인).
2. native 엔트리가 루트 Makefile을 경유해야만 env 매핑을 얻는 구조 — native runner에 `env_file`/`environment` 매핑 표현이 있으면 `make run-engine` 우회가 필요 없어진다. 현재 `stack.<entry>.runners.native`에 `environment:`를 둘 수 있는지 스키마 확인 필요 (있다면 문서화 부족, 없다면 기능 gap).

## 2차 재검증 (2026-09-05, dva ecae43d: TASK-305/306/308 반영 빌드)

- (TASK-308) `environments.ci`를 선택하는 plan이 없어 dead 선언 warning. 결정 대기: ci plan 추가 vs 삭제(권장: 삭제, CI는 dva를 쓰지 않음).
- (결정 반영) `environments.ci` 삭제 — CI는 dva를 쓰지 않고 plan 없이 env를 고를 수단이 없음. validate warn 0.
