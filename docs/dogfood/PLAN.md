# mydevbox dva 마이그레이션 계획 (2026-09-05 분석 기준, 구체화판)

dva 0.1.48 기준 23개 프로젝트 분석 종합. 프로젝트별 근거는 개별 리포트 참조.
dva 자체 개선(TASK-303~310)은 별도 세션에서 진행 중이며, 아래 항목 중 `[dva 선행]` 표시는
해당 태스크 완료 후 진행하는 것이 효율적이다.

**원칙**: dva 형태를 강제한다. 각 항목은 수동 검토 후 반영 여부를 결정한다.
**공통 완료 기준**: `dva validate` exit 0 + warning 0 (의도적 예외는 리포트에 기록).

---

## Tier 0 — 변경 불필요 (5)

flow-observechain, scripton-nd-stack, sigdock-pass, flow-taskchain, funbricks-postkit
→ 아래 Tier 1의 "소소한 정리"만 선택 적용.

## Tier 1 — 기계적 정리 (난이도 하)

### 공통 패턴 (여러 프로젝트 반복)

| 패턴 | 대상 프로젝트 | 조치 |
|---|---|---|
| 중복 plan (`infra`≡`local-infra`, `hybrid`≡`local-dev`) | cwrapper(2쌍), dripter, gizzahub, matdosa, scripton-nd-stack | 한쪽 삭제. `[dva 선행: TASK-307 alias]` 도입 시 alias로 전환 |
| `default_plan` 미설정 | flow-knowchain, funbricks-notifire, funbricks-postkit | 한 줄 추가 (`local-dev` 또는 `local-infra`) |
| 최상위 `health_checks` 고아/중복 | flow-knowchain, flow-pipechain(4건, stack과 중복) | 최상위 블록 삭제, 필요 시 stack native 엔트리로 이동 |
| 제거된 CLI 참조 문구 (`dva dev`, `-M`, `dva clean`, `dva app up`) | flow-knowchain, flow-pipechain, gorisa | suggestion_ignore 주석 및 provision note 수정 |
| no-op `sites.local.entry_overrides` | flow-taskchain, funbricks-postkit | 삭제 |
| `interaction.clean` replace 훅 (validate ERROR) | sadawiki, scripton-signalhub | `steps:` 일반 interaction으로 전환 (nd-stack/elemhant 방식) |
| `build`/`logs` replace 훅으로 compose 직접 호출 | cwrapper, funbricks-elemhant, sadawiki, scripton-signalhub, sigdock-idp | `dva build/logs <plan>` 으로 대체 가능 여부 확인 후 훅 삭제 |
| `endpoints` 부재 (포트가 주석/echo에만 존재) | funbricks-notifire, matdosa, sigdock-pass | 주석의 포트를 `endpoints:` 섹션으로 이동 |
| compose 파일 drift (미등록 overlay) | flow-knowchain(3), sigdock-idp(6), sigdock-pass(10) | 의도적 파일은 stack 두 번째 엔트리+plan으로 등록하거나 리포트에 예외 기록 `[dva 선행: TASK-309 ignore]` |

### 프로젝트별 고유 항목

- **cwrapper**: `interaction.start`(Django native 중복 선언) 삭제 — `hybrid` plan이 이미 담당
- **dripter**: Ktor/Astro 앱을 native runner stack 엔트리로 선언, `subprojects:` 도입해 `cd … &&` 하드코딩 제거. suggestion_ignore 110항목 재검토 `[dva 선행: TASK-309]`
- **flow-knowchain**: `subprojects:` 도입 (backend/ai/frontend)
- **flow-pipechain**: compose 경로 이원화 해소 — `compose.yaml` vs `deploy/local/compose.yaml` 중 정본 하나로 통일
- **funbricks-elemhant**: build/logs 훅의 compose 파일명·project_name 하드코딩 제거
- **funbricks-notifire**: 30~70줄 인라인 셸/파이썬 스크립트를 `scripts/*.sh`로 추출, `workdir`+`cd` 중복 제거, stack services 빈 객체(`postgres: {}`) 정리
- **funbricks-postkit**: engine/ui `dir: .` → 실제 subproject 디렉토리로
- **gizzahub**: plan이 참조하는 temporal/kafka/prometheus 등을 stack services에 선언 (선언 드리프트 복구) `[dva 선행: TASK-308 검증]`, suggestion_ignore 190항목 재검토
- **matdosa**: dead `environments`(dev/test)를 plan `environment:`에 연결하거나 삭제
- **sadawiki**: `modes:` 2개 → plans 전환 (compose_services형이라 기계 변환 가능 `[dva 선행: TASK-306]`), `stack.order`/`tags` 제거, health_checks `start:`의 compose 직접 호출 제거
- **scripton-signalhub**: `modes:` 3개 → plans (hybrid의 env 주입은 `environments`로 분리), engine/ui를 native stack 엔트리로, `dev`/`infra-up`/`infra-down`/`db reset` interaction의 compose 직접 실행을 plan으로 대체
- **sigdock-idp**: `modes.*.stack` 선택 → plan entry (기계 변환 가능 `[dva 선행: TASK-306]`), `stack.order` 제거, `interaction.db` 실행 타깃 추가, `start_hint` 중복 제거
- **sigdock-pass**: `command: ""` placeholder 제거, `env_file` 선언 추가, HA/federation/saas overlay를 plan으로 등록할지 결정
- **scripton-nd-stack**: 중복 plan 정리, destructive interaction의 agent-deny 연동 확인, Makefile 미매핑 6건

## Tier 2 — 구조 전환 (난이도 중)

- **scripton-gitrump** (4월 이후 방치 — 유지 여부 먼저 결정)
  1. stack compose 평면 선언 → `runners.compose:` 하위로 이동 (validate ERROR 해소)
  2. `applications:` → `stack.<name>.default_runner: native`
  3. `modes:`/`default_mode` → plans/`default_plan`
  4. `dev-full` overlay compose를 stack 두 번째 엔트리 + plan으로
  5. `interaction.clean`/`dev` replace 훅 제거, `app:build`/`app:run`/`app:clean` 콜론 이름 정리
- **gorisa**
  1. `dva app up` 참조 문구 수정
  2. `stack.*.tags` → `subprojects.*.exclude_tags` 의존 관계 확인 후 정리
  3. provision `dev-full-setup`의 수제 멱등성(`tmp/.setup-state`) 재검토
  4. `${VAR:-default}` 사용처 확인 `[dva 선행: TASK-303 필수]`
- **flow-agent-mesh**
  1. `interaction.clean` replace → steps (ERROR 해소)
  2. `modes:` → plans, `environments.*.stack` 제거
  3. `stack.compose.order` → `plans.*.entries[].order`
  4. 최상위 `health_checks.am-server` → native stack 엔트리로
  5. 헤더 주석 `dva up -M` 문구 수정, `version: 0.1.26` 갱신

## Tier 3 — 전면 재작성 (난이도 상)

- **familybook** `[dva 선행: TASK-304 dva.yaml 인식, TASK-305]`
  1. `dva.yaml` → `dva.yml` 개명
  2. flat stack → `runners.compose`, `modes:` 3개 → plans, `environments.test.compose_files` 제거
  3. `interaction.clean` replace 제거, 최상위 health_checks 정리, `version` 갱신
- **scripton-db-orchestrator** `[dva 선행: TASK-305, 306]`
  1. `applications:` 4개 → native stack 엔트리 (depends_on/port/health는 수동 이관)
  2. `modes:` 6개 → plans (hybrid/dev의 env 주입은 `environments`로)
  3. `stack.order`/`tags` 제거, build/clean/logs replace 훅 제거
- **scripton-dns-bridge** `[dva 선행: TASK-305, 306]`
  1. `applications.api`의 run/dev/build native+docker 변형 → native stack 엔트리 + runner 선택 plan
  2. `modes:` 7개 → plans (kafka/nameserver의 profile-only mode 포함, compose_profiles 함정 주석 해소)
  3. `infra-up`/`infra-down`/`docker-dev` interaction → `dva up/down <plan>`
- **primeno1** `[dva 선행: TASK-305, 306]`
  1. `interaction.clean` replace → steps (ERROR 해소)
  2. `modes:` 6개 → plans; mode당 -f 전체 목록 복제를 entry 조합으로 해소
  3. `stack.*.order` → plan entries, `dev-up`의 `dva up -M full` 내장 호출 수정
  4. 최상위 `environment:` → `vars:` + `environments.*`

## 미도입 (6) — 도입 여부 결정 대기

flow-station, lottomaster, mansero, gzh-cli, scripton-code, scripton-dashboard
→ compose/Makefile 존재 여부 확인 후 `dva init`(TASK-249 재설계 결과) 적용 후보.

---

## 실행 순서

| Phase | 내용 | 선행 조건 |
|---|---|---|
| 1 | Tier 1 공통 패턴 일괄 (중복 plan, default_plan, 고아 health_checks, 문구 수정, clean 훅) | 없음 — 즉시 가능 |
| 2 | Tier 1 프로젝트별 고유 항목 | TASK-303 (gorisa/matdosa/elemhant의 `${VAR:-}` 사용처) |
| 3 | Tier 2 (gitrump 유지 결정 → agent-mesh → gorisa) | TASK-305 |
| 4 | Tier 3 4개 | TASK-304, 305, 306 |
| 5 | 미도입 6개 결정 | TASK-249 |

각 프로젝트는 개별 커밋, 리포트의 "문제점" 항목을 체크리스트로 소진하고 validate 출력을 리포트에 첨부.

---

## 현재 상태 (최종 갱신 2026-09-05 오후, dva ecae43d)

프로젝트 변경은 전부 **커밋하지 않은 working tree**. 각 리포트의 "적용 결과"/"결정 반영" 섹션이 diff 검토 체크리스트.

| 구분 | 프로젝트 | validate | 비고 |
|---|---|---|---|
| Tier 0 | flow-observechain | exit 0 / warn 0 | 무변경 |
| Tier 1 | cwrapper, dripter, flow-pipechain, funbricks-elemhant, funbricks-notifire, funbricks-postkit, gizzahub, matdosa, sadawiki, scripton-nd-stack | exit 0 / warn 0 | |
| Tier 1 | flow-knowchain, sigdock-pass | exit 0 / warn 1 | 의도적 drift 예외(TASK-309 대기) |
| Tier 1 | sigdock-idp | exit 0 / warn 1 | 의도적 drift 예외 |
| Tier 1 | scripton-signalhub | exit 0 / warn 1 | compose `name:` 누락 (프로젝트 결정) |
| Tier 1 | flow-taskchain | exit 0 / warn 4 | 사전 존재 Makefile 제안, 범위 밖 |
| Tier 2 | flow-agent-mesh, gorisa, scripton-gitrump | exit 0 / warn 0 | gitrump 5단계 적용 완료 |
| Tier 3 | scripton-dns-bridge | exit 0 / warn 0 | |
| Tier 3 | familybook, scripton-db-orchestrator | exit 0 / warn 1 | 의도적 drift 예외 |
| Tier 3 | primeno1 | exit 0 / warn 1 | compose `name:` 누락 |
| 신규 도입 | scripton-dashboard | exit 0 / warn 0 | dva.yml 신규(native-only) |
| 미도입 | gzh-cli(조건부), flow-station(효용 낮음), mansero(보류), lottomaster·scripton-code(불필요) | — | 리포트만 |

### 확정된 결정 (2026-09-05)
- reports/ → dva 저장소 `docs/dogfood/`로 이동해 버전 관리(태스크 `source:` 경로 갱신).
- postkit `environments.ci` 삭제. notifire `scripts/dva-*.sh` 수용. bare `dva logs/down`은 plan 명시로 통일.
- endpoints 리터럴 포트 유지(dva가 endpoints url을 치환하지 않음 — TASK-323).
- 의도적 drift warning 5건은 TASK-309 대기.

### 남은 작업
1. 24개 프로젝트 working tree 리뷰·커밋(사용자). warning 0 → 예외 순.
2. 실기동 검증: TASK-311/312 수정 후 primeno1, db-orchestrator, dns-bridge, gitrump, signalhub, dashboard(`make prepare` 선행).
3. dva 태스크: P1 312, 313, 317, 311 → P2 314, 316, 315, 307 → needs-human 319, 321 → P3.

## 이력

### 1차 실행 (2026-09-05 오전)

전 Phase를 실행했다. 프로젝트 변경은 **커밋하지 않은 working tree** 상태이며 각 리포트의 "적용 결과" 섹션이 체크리스트다.

| 구분 | 프로젝트 | validate | 비고 |
|---|---|---|---|
| Tier 0 | flow-observechain | exit 0 / warn 0 | 무변경 |
| Tier 1 | cwrapper, dripter, flow-pipechain, funbricks-elemhant, funbricks-notifire, funbricks-postkit, gizzahub, matdosa, sadawiki, scripton-nd-stack, scripton-dns-bridge | exit 0 / warn 0 | |
| Tier 1 | flow-knowchain, sigdock-idp, sigdock-pass, scripton-signalhub | exit 0 / warn 1 | 의도적 예외(drift 픽스처, compose `name:`) |
| Tier 1 | flow-taskchain | exit 0 / warn 4 | 사전 존재 Makefile 제안, 범위 밖 |
| Tier 2 | flow-agent-mesh, gorisa | exit 0 / warn 0 | |
| Tier 2 | scripton-gitrump | exit 0 / warn 0 | 5단계 마이그레이션 적용 완료(사용자 승인, 2026-09-05) |
| Tier 3 | familybook, primeno1, scripton-db-orchestrator | exit 0 / warn 1 | 의도적 예외 |
| Tier 3 | scripton-dns-bridge | exit 0 / warn 0 | |
| 미도입 | scripton-dashboard(1순위), gzh-cli(조건부), flow-station(효용 낮음), mansero(보류), lottomaster·scripton-code(불필요) | — | 리포트에 골격만, 파일 무생성 |

부수 작업: 5개 프로젝트 20개 파일의 제거된 CLI 잔재(`-M`, `dva app`, `dva clean`, `dva start`) 치환 (리포트 "CLI 잔재 정리" 섹션).
`${VAR:-default}` 사용처는 TASK-303 수정(dva d7636a3, 재빌드 후 23개 재검증 결과 동일) 이후 전부 유지 확정. gorisa만 방어 우회를 걷어내고 기본값 형식으로 복원.

### dva 개선 태스크 (별도 세션)
기존 TASK-303~310에 근거 보강, 신규 TASK-311~323 생성 (tasks/todo/). 우선순위 P1: 311(down teardown), 312(dry-run health 대기), 313(workdir 무시), 317(migrate 힌트 오류).

### 2차 재검증 (2026-09-05 오후, dva ecae43d)

TASK-305(에러 수집)·306(modes→plans migrate)·308(참조 무결성 warning)이 통합된 빌드로 23개 재검증.
신규 semantic warning 3건 발견: primeno1 `-M` 잔재(수정), sigdock-idp observability services 미선언(수정), postkit `environments.ci` dead(결정 대기).
나머지 warning은 의도적 drift 예외(familybook, flow-knowchain, db-orchestrator, sigdock-idp, sigdock-pass)와 compose `name:` 누락(primeno1, signalhub) 뿐.

