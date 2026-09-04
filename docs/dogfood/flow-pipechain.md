# flow-pipechain dva 적용 분석

## 현황
- 파일: `dva.yml` (753줄 — 그룹 A 최대), `version: "0.1.44"`
- 섹션: `env_file(files)`, `stack`(compose + api/agent/portal-ui/admin-ui native 엔트리, 각자 stack-level `health_checks` 보유), `plans`(7개), `default_plan`, `environments`, `sites`, `checks`, `suggestion_ignore`(70+ 항목), `health_checks`(최상위, start 포함), `interaction`(build/logs replace hook, clean은 일반 커맨드로 전환 완료), `provision`, `subprojects`(7개), `endpoints`
- `dva validate`: **valid** (warning 다수)

## 문제점
- **최상위 `health_checks` 4건이 전부 dead** (validate warning, 366–393행): api/agent/portal-ui/admin-ui 모두 `start`를 선언했지만 참조할 `modes`가 없음. 게다가 **같은 체크가 stack 엔트리 안에 이미 중복 선언**돼 있음(예: api — 61–66행 vs 366–372행). 최상위 블록은 modes 시절 잔재이므로 삭제가 답.
- **제거된 CLI 표면을 참조하는 note**: `provision.default` 마지막 note(645행) "Run 'dva dev' … or 'dva up -M full-stack'" — `dev`도 `-M`도 현행 표면에 없음.
- **`interaction.logs.replace`(546–549행)와 provision이 `deploy/local/compose.yaml`을 참조**하는데 stack은 루트 `compose.yaml`을 선언(29행) — 두 compose 경로가 혼재해 어느 쪽이 정본인지 설정이 갈라짐.
- `interaction.clean`은 built-in 제거에 맞춰 `steps:` 일반 커맨드로 올바르게 전환됨(551–559행) — 모범 사례.
- Makefile 커버리지 warning 다수(`check`, `contract-offline` 등) — suggestion_ignore가 70+ 항목인데도 누락이 남는 이중 관리 부담.

## dva 개선 힌트
- **orphan health_checks 감지가 유효하게 작동** — 다만 "stack 엔트리에 동일 이름 체크가 이미 있음"까지 알려주면(중복 감지) 삭제 판단이 즉시 가능해짐.
- suggestion_ignore 비대(dripter와 동일) — 대규모 Makefile 프로젝트에서 ignore 관리 비용이 큼.
- note/description 문자열 속 제거된 명령(`dva dev`, `-M`) lint 부재 (knowchain과 동일).

## 마이그레이션 난이도
**하** — 구조는 신형이고 valid. 최상위 health_checks 블록 삭제, provision note 현행화, compose 경로 정리 정도의 hygiene 작업.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] 최상위 `health_checks` 4건(api/agent/portal-ui/admin-ui) 삭제 — 동일 url/timeout/ready_timeout이 `stack.<entry>.health_checks.ready`에 이미 있음. `start:`의 `cd … && go run cmd/server/main.go` 등은 stack native `run:`이 담당하므로 손실 없음.
- [x] `provision.default` 마지막 note 현행화 — `dva dev`/`dva up -M full-stack` → `dva up local-dev`/`dva up full-stack`.
- [x] **compose 경로 이원화 해소 — 루트 `compose.yaml`로 통일.** 근거: 루트 `compose.yaml`(4줄)은 `name: pipechain` + `include: deploy/local/compose.yaml`만 가진 shim이고, `deploy/local/compose.yaml`은 헤더에 "Canonical local compose entrypoint"라 자칭하며 README/runbooks/deploy/manifest.yaml이 그 경로를 안내한다. 두 파일은 같은 프로젝트(pipechain)·같은 서비스 집합을 렌더링한다. stack이 이미 루트 `compose.yaml`을 선언하고 있고, dva의 compose drift 감지는 루트 파일을 기준으로 하므로 stack을 `deploy/local/compose.yaml`로 바꾸면 루트 shim이 고아가 되어 drift warning이 생긴다. 따라서 dva.yml 안에서는 루트 `compose.yaml`을 정본으로 삼고, provision 3곳의 `docker compose -f deploy/local/compose.yaml …`에서 `-f` 지정을 제거해 루트 파일(동일 프로젝트명)을 쓰도록 통일. `deploy/local/compose.yaml`은 문서·Makefile(`COMPOSE_PROD`)이 계속 직접 참조하는 파일이므로 유지(dva.yml 외 파일 미수정).
- [x] `interaction.logs` replace 훅 삭제 — `dva --dry-run --debug logs full-stack -f` → `docker compose -f compose.yaml --project-name pipechain logs -f`로 훅과 동일(훅도 profile 미지정).
- [x] `interaction.build` replace 훅 + server/agent/portal/admin 하위 커맨드 삭제 — stack native 엔트리의 `build:`와 1:1 동일 커맨드이므로 `dva build local-dev`(전체) / `dva build local-dev api`(dry-run으로 `go build -o ../build/server ./cmd/server` in flow-pipechain-server 확인)로 대체. `build docker`(이미지 빌드)는 dva 등가물이 없어 `interaction.docker-build`로 보존. Makefile `build-server`/`build-agent` 매핑이 사라져 생긴 suggestion warning은 suggestion_ignore 주석과 함께 등록.
- [x] Makefile `check` 미매핑 → `interaction.check`(`make check`) 추가.
- [x] 그 외 Makefile 미매핑 suggestion 29건 → suggestion_ignore에 그룹 glob(`*-check`, `*-pin`, `contract-offline*`, `k8s-*`, 서브프로젝트별 lint/test 등)으로 등록. 근거: CI/보안 게이트·kind 클러스터·서브프로젝트 전용 타깃으로 devbox interaction 표면이 아님.

### validate 최종 출력
```
✅ dva.yml is valid
EXIT=0
```
warning 0건.

### 보류/예외 항목
- `provision.default/full/reset`의 `docker compose up/down` 직접 호출 유지 — `full`의 `--profile full up`은 README 기준 `--profile cache --profile queue-kafka`와 조합돼야 redis/kafka가 뜨는 구조라 `dva up full-stack`(서비스 명시 기동)과 결과가 다를 수 있어 그대로 둠. 경로만 통일.
- interaction의 `cd flow-pipechain-* &&` 하드코딩 유지 — 서브프로젝트 7개 모두 `import:` 없는 선언만 있고(체크아웃에 dva.yml 부재 여부는 미확인), local runner가 `workdir`를 무시함(dripter 리포트 개선점 1).

### 발견된 dva 개선점
1. **Makefile 파서가 다중 타깃 규칙을 한 이름으로 읽음.** `.make/*.mk`의 `log-search-bench perf-log-search:` 한 줄을 validate가 `Makefile defines "log-search-bench perf-log-search"`로 보고. 공백으로 구분된 타깃 목록을 분리해야 하며, 현재는 `suggestion_ignore`에 공백 포함 문자열을 넣어야 warning이 사라진다. Repro: 임의 Makefile에 `a b:\n\t@true` 추가 후 `dva validate`.
2. **orphan health_checks 감지 시 stack 엔트리 중복 여부 안내 부재**(기존 힌트 재확인) — 4건 모두 stack 엔트리와 동일 값이었음.
3. suggestion_ignore 비대(기존 힌트 재확인) — 이번에 20항목 추가. 카테고리/파일 단위 제외(예: `.make/quality.mk` 전체) 옵션 필요.
4. 루트 shim compose(`include`만 있는 파일)를 stack이 선언할 때, 실제 정본이 include 대상이라는 사실을 dva가 알지 못함 — drift 감지가 루트 파일만 보므로 `deploy/local/*.yaml` 변화는 감지 밖. 사소하지만 기록.
