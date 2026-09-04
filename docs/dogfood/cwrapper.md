# cwrapper dva 적용 분석

## 현황
- 파일: `dva.yml` (530줄), `version: "0.1.44"`
- 섹션: `env_file(files 형식)`, `stack`(compose / compose-live / django-native, runners 형식), `plans`(9개), `default_plan`, `environments`, `sites`, `suggestion_ignore`, `interaction`(logs replace hook 포함), `provision`, `subprojects`(4개), `endpoints`
- `dva validate`: **valid** (warning 2건)

## 문제점
- **중복 plan 쌍 2건** (validate warning): `hybrid`==`local-dev`, `infra`==`local-infra` — environment/site/entries가 완전히 동일. 별칭 목적으로 보이나 선언 중복이라 한쪽 수정 시 drift 위험. 정본 하나만 남기는 것이 맞음 (plans 60–156행).
- `interaction.start` (269–273행): Django native 실행을 interaction으로 중복 선언. 동일 실행이 `stack.django-native` + `hybrid` plan으로 이미 표현됨 — "실행은 plans, 작업은 interactions" 원칙(docs/42 §12-2)과 어긋나는 legacy 습관.
- `interaction.logs.replace`가 `dva compose compose logs -f django-engine`을 호출 (260행) — 새 표면에서는 `dva logs <plan>`이 plan-aware이므로 replace hook 필요성 재검토 대상.
- 그 외 removed 섹션(`applications:`, `modes:`) 사용 없음. 구조적으로는 신형.

## dva 개선 힌트
- **plan alias 기능 부재**: infra/local-infra 같은 이름 별칭 수요가 실제로 있는데 전체 선언을 복제할 수밖에 없음. `plans.<name>.alias_of` 같은 경량 별칭이 있으면 warning 없이 해결됨.
- validate가 중복 plan을 잘 잡아냄 — 이 warning에 "alias가 목적이면 …" 안내를 붙일 수 있는 지점.

## 마이그레이션 난이도
**하** — 이미 신형 구조. 중복 plan 정리와 `start` interaction 제거 정도의 hygiene 작업만 남음.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] 중복 plan 2쌍 정리 — `local-infra`, `local-dev` 삭제, `infra`, `hybrid` 유지. 근거: README.md, CLAUDE.md, QUICKSTART.md, docs/docker-compose-guide.md, decisions/adr/001이 전부 `dva up infra` / `dva up hybrid`를 안내하고 `local-*` 참조는 0건. `default_plan: infra`로 변경.
- [x] `interaction.start` 삭제 — `hybrid` plan(`stack.django-native`)이 동일 실행을 담당. 단, CLAUDE.md L49/L95, QUICKSTART.md L73이 `dva start`를 안내하므로 문서 후속 수정 필요(dva.yml 외 파일은 미수정).
- [x] `interaction.logs` replace 훅 삭제 — `dva --dry-run --debug logs django-engine -f`(compose passthrough)와 `dva logs full-stack compose -f django-engine`(plan 경로) 모두 훅과 동일한 `docker compose ... logs -f django-engine`으로 확인. 단, bare `dva logs`의 의미가 "Django 로그 follow"에서 "default_plan(infra) 로그"로 바뀜 — CLAUDE.md L97/L189, .make/help.mk L62, .make/dev.mk L106의 안내 문구 후속 수정 대상.

### validate 최종 출력
```
✅ dva.yml is valid
EXIT=0
```
warning 0건.

### 보류/예외 항목
- 없음 (dva.yml 기준). 문서 측 `dva start`, bare `dva logs` 안내는 별도 수정 필요.

### 발견된 dva 개선점
- plan alias 부재(기존 힌트 재확인). 문서가 `infra`/`hybrid`를 쓰는 반면 canonical 예시는 `local-infra`/`local-dev`라 이름 선택을 강제당함 — TASK-307 alias로 해소 가능.

## CLI 잔재 정리 (2026-09-05)
- CLAUDE.md:49 (AGENTS.md는 심링크) `dva start` → `dva up hybrid` (infra + Django native; 별도 start interaction 없음)
- CLAUDE.md:95 `dva start` → `dva up hybrid`
- CLAUDE.md:97, :189 `dva logs` → `dva logs full-stack`
- QUICKSTART.md:73 `dva up hybrid` + `dva start` → `dva up hybrid` 한 줄 (hybrid plan이 Django native 엔트리를 기동)
- .make/help.mk:57-59 `dva up --mode X` → `dva up X` (full-stack / full-stack-tools / full-stack-monitoring), :62 `dva logs` → `dva logs full-stack`
- .make/dev.mk:106 `dva logs` → `dva logs full-stack`
- .make/deploy.mk:39 `dva up --mode …` → `dva up …`
- docs/docker-compose-guide.md:62 `dva clean` → `dva down infra --volumes`
- 보류 0. 참고: bare `dva logs`/`dva down`은 default_plan(infra)이 있어 여전히 유효하지만 plan을 명시했다.
- (결정 반영) bare `dva down` → `dva down infra` (.make/help.mk, .make/dev.mk).
