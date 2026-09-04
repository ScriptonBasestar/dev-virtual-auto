# flow-knowchain dva 적용 분석

## 현황
- 파일: `dva.yml` (517줄), `version: "0.1.44"`
- 섹션: `env_file(files)`, `stack`(compose + backend/ai/frontend/admin native 엔트리 5개), `plans`(4개), `environments`, `sites`, `checks`, `suggestion_ignore`, `health_checks`(최상위), `interaction`(공백 포함 이름 `"db shell"` 다수, build/logs replace hook), `provision`, `endpoints`
- `dva validate`: **valid** (warning 3건)

## 문제점
- **`default_plan` 미설정** (validate warning): plan 4개인데 bare `dva up`이 불가. 한 줄 추가로 해결.
- **compose files drift** (validate warning): `compose.files`는 `compose.yaml`만 선언했지만 루트에 `compose.debug.override.yaml`, `compose.production.yaml`, `compose.test.yaml` 등이 존재 — 어느 쪽이 의도인지 설정이 답하지 못함.
- **제거된 CLI 표면을 참조하는 주석/노트 다수**: `suggestion_ignore` 주석의 "covered by dva up -M full-stack"(202행), "covered by dva clean"(216행), "covered by dva dev"(204행) — `-M`, `clean`, `dev` 모두 현행 표면에 없음. `provision.default` 마지막 note(445행)도 `dva dev` / `dva up -M full-stack` 안내. 실행에는 영향 없지만 사용자를 제거된 명령으로 유도.
- **최상위 `health_checks`(223–246행)가 어디에도 연결 안 됨**: modes가 없으니 참조 주체 부재. start가 없어 validate warning은 안 뜨지만 사실상 advisory dead 선언 — stack native 엔트리별 `health_checks`로 이동하는 것이 신형 형태(pipechain 참조).
- **subprojects 미선언**: backend/ai/frontend 각 저장소가 존재하는데 interaction에 `cd flow-knowchain-backend && …` 하드코딩 반복 (298행 이하).

## dva 개선 힌트
- **drift 감지가 실제로 유효** — compose override 파일을 잡아냈음. 다만 test/production override처럼 의도적으로 dva 밖에 두는 파일을 표시할 방법(예: drift ignore 목록)이 없어 warning이 상주하게 됨.
- 주석·note 속 제거된 CLI 표면은 도구가 잡지 못함 — validate가 provision note/description 문자열 속 `dva clean`, `-M` 패턴을 lint하는 규칙을 추가하면 개밥주기 가치가 있음.

## 마이그레이션 난이도
**하** — 구조는 신형이고 valid. default_plan 지정, 문서 문자열 현행화, health_checks 재배치, subprojects 등록 정도의 정리 작업.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `default_plan: local-dev` 추가 — README.md L88이 local-dev를 "(기본)"으로 안내.
- [x] 최상위 `health_checks` 4건 삭제 → 각 native stack 엔트리(`stack.backend/ai/frontend/admin`)의 `health_checks:`로 이동(url/timeout/ready_timeout 값 그대로). `admin` native runner는 `cd flow-knowchain-admin && pnpm dev` → `dir: flow-knowchain-admin` + `run: pnpm dev`로 정리.
- [x] 제거된 CLI 참조 문구 수정 — suggestion_ignore 주석 5건(`dva up -M full-stack`→`dva up docker-full`, `dva dev / applications`·`dva dev`→`dva up local-dev`, `-M observability`→`dva up observability`, `dva clean`→`dva provision reset`), `provision.default` 마지막 note(`dva dev`/`-M full-stack`→`dva up local-dev`/`dva up docker-full`).
- [x] `subprojects:` 도입 — backend/ai/frontend/admin 4개 선언(`exclude_tags: [infra]`). 4개 체크아웃 모두 자체 dva.yml이 없어 `import:` 없이 선언만 등록(스키마가 허용하는 형태). 따라서 interaction의 `cd flow-knowchain-* && …` 하드코딩은 제거하지 못함 — 각 서브프로젝트가 dva.yml을 갖추면 import로 전환.
- [x] `build`/`logs` replace 훅 삭제 — `dva --dry-run build docker-full backend ai-service frontend`가 훅과 동일한 `docker compose … build backend ai-service frontend`를(project_name 포함) 생성. logs는 `dva logs docker-full -f backend ai-service frontend`처럼 서비스명을 명시해야 profile:full 서비스가 포함됨(아래 개선점 2).
- [x] Makefile `check` suggestion warning 해소 — `interaction.check` (`make check`) 추가.
- [ ] compose files drift warning — 의도적 예외로 유지(아래).

### validate 최종 출력
```
[warn] config drift: compose.files is compose.yaml but detected root compose files are compose.yaml, compose.debug.override.yaml, compose.production.override.yaml, compose.production.yaml, compose.test.yaml; review whether dva.yml is tracking the current project layout
✅ dva.yml is valid
EXIT=0
```
warning 1건(의도적 예외).

### 보류/예외 항목
- **compose drift warning 1건 유지**: `compose.test.yaml`은 독립 실행 테스트 픽스처(`docker compose -f compose.test.yaml up --abort-on-container-exit`, PORT_MAPPINGS.yaml에 "test fixture only"로 명시), `compose.production.yaml`/`compose.production.override.yaml`/`compose.debug.override.yaml`은 production 배포용이라 dev 전용 도구인 dva의 stack에 등록하는 것이 부적절. `[dva 선행: TASK-309 ignore]` 도입 시 해소.
- `provision.default/full/reset`의 `docker compose up/down/pull` 직접 호출은 유지(PLAN 항목 아님). `full`의 `--profile full up -d --wait`는 `dva up docker-full`과 서비스 집합이 동일하지만(docker-full plan이 profile 서비스를 명시), 마이그레이션 없이 provision 내부에서 dva를 재귀 호출하는 형태가 권장인지 불확실해 보류.
- interaction의 `cd <subproject> &&` 하드코딩 유지(서브프로젝트 dva.yml 부재 + local runner `workdir` 미지원, dripter 리포트 개선점 1 참조).

### 발견된 dva 개선점
1. **compose drift ignore 부재**(기존 힌트 재확인, TASK-309) — test fixture/production compose를 dva 밖에 두려는 의도를 표현할 수 없어 warning 0을 달성할 수 없음.
2. **plan 경로 `dva logs <plan>`이 plan의 services로 범위를 좁히지 않음.** `dva --dry-run --debug logs docker-full -f` → `docker compose -f compose.yaml --project-name flow-knowchain logs -f` (서비스 목록 없음). knowchain의 backend/ai-service/frontend는 `profiles: [full]` 뒤에 있어 `--profile` 없는 `logs`에서 제외되므로, plan이 명시적으로 기동한 서비스의 로그가 bare `dva logs docker-full`에 안 나옴. `dva up docker-full`은 서비스명을 명시해 기동하므로 up/logs 사이에 비대칭이 생김. 제안: plan compose entry의 `services:`를 logs 인자로 전달하거나, stack compose runner에 `profiles:` 옵션 추가(현재 `profiles`는 interaction `compose:` 옵션에만 있음, schema.json L421).
3. 주석/note 속 제거된 CLI 문구 lint 부재(기존 힌트 재확인) — 이번에 수동으로 6곳 수정.
