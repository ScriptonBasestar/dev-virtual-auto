# scripton-db-orchestrator dva 적용 분석

## 현황
- 파일: `dva.yml` (21,448 bytes — 8개 중 두 번째로 큼, version `0.1.44`)
- 사용 섹션: `env_file`, `stack`(compose 1엔트리), `checks`, `applications`(api/worker), `default_mode`, `suggestion_ignore`, `modes`(6개), `health_checks`, `interaction`(대규모 — Makefile 위임 위주), `provision`(default/full/reset), `subprojects`(3개), `endpoints`(13개)
- `dva validate` (0.1.48): **ERROR** — schema 거부 5건

## 문제점
1. **최상위 `applications:` 섹션 (line 108-139)** — 제거된 섹션. `stack.<name>.default_runner: native`로 이동해야 한다. schema가 `Additional property applications is not allowed`로 거부.
2. **`modes.*.applications:` 키 (hybrid/dev/full-stack/full-stack-ui, line 186/192/197/207)** — 동일하게 schema 거부. mode가 native/docker 앱 전략을 고르는 구모델 패턴으로, 신모델에서는 plan entry의 runner 선택 책임.
3. **`modes:` 섹션 전체 (line 173-212)** — deprecated. 6개 mode 중 hybrid/dev는 `environment:` 주입까지 겸하고 있어 `environments` 분리 대상.
4. **`default_mode: infra` (line 144)** — `default_plan`으로 대체 대상.
5. **`plans:` 없음** — 실행 이름이 전무한데 mode가 6개로, 선언/실행 혼합의 전형.
6. **`stack.compose.order`/`tags` (line 27-28)** — 실행 계획 성격 필드가 선언에 남아 있음.
7. **`interaction.build`/`clean`/`logs` replace 훅 (line 257-296)** — `clean` built-in 제거로 무의미해졌고(applications 에러에 가려 아직 미보고), `build`/`logs`는 plan-aware 동사와 충돌.
8. **applications의 `depends_on`/`port`/`health` (worker.depends_on: [api] 등)** — `config migrate`가 자동 변환하지 못하는 필드(docs/43 §18-1)라 수동 이관 필요.

## dva 개선 힌트
- 이 설정은 interaction 트리가 사실상 Makefile 전체 미러(약 40개 커맨드)다. Makefile 위임만 하는 interaction의 대량 선언을 줄이는 기능(예: Makefile target 자동 노출/네임스페이스 import)이 있으면 파일이 1/3로 줄어든다. `suggestion_ignore` 19줄이 그 마찰의 증거.
- `modes.hybrid.environment`처럼 mode가 env 주입까지 겸하는 실사용 패턴은 modes→(plans+environments) 분해가 왜 자동화 안 되는지 보여주는 좋은 코퍼스 — migrate 스캐폴드 출력에 이 env 블록을 environments 후보로 제안하면 도움이 된다.
- `applications.*.health.required` 부재(docs/43 §16 기록된 부채)의 실수요 사례: 이 프로젝트 api/worker health는 ready_timeout 120s의 게이트성 체크였다.

## 마이그레이션 난이도
**상** — applications 2개(native/docker 이중 경로 + depends_on + health) + mode 6개 + env 주입 혼합. `dva config migrate` 자동 변환 범위(applications→stack, order→plans)로 절반은 처리되지만, modes 6개의 plans/environments/sites 3축 분해와 depends_on/port 수동 이관이 남는다.

## 적용 결과 (2026-09-05)

### `dva config migrate` 프리뷰 (수정 전 실행, 파일 미변경)
Converted:
- `applications.api → stack.api` (port 11100은 "drove the port-reclaim check in 'dva app up', which no longer exists"로 미이관)
- `applications.worker → stack.worker`

Left for you (원문 요지):
- `applications.api.run.docker` / `worker.run.docker`: compose 서비스(profile rust)는 compose 러너 stack 엔트리 + plan에서 서비스 선택
- `applications.worker.depends_on`: `plans.<plan>.entries[].depends_on`으로
- `stack.*.order` / `stack.compose.order: 10`: plan이 없어 옮길 곳 없음 — plan 선언 후 재실행
- `modes.infra`, `infra-full`: description → plans, compose_services → entries[compose].services
- `modes.full-stack`, `full-stack-ui`: compose_profiles → `runners.compose.up_options`에 `--profile`, applications → runners.native, **provision → no equivalent**
- `modes.hybrid`, `dev`: compose_services/health_checks/environment(→ `environments.<name>`, plan.environment로 선택)/applications
- 즉 migrate는 applications 2개만 변환하고 modes 6개·order·depends_on·health_checks(top-level)는 전부 수동.

### 변경 항목 체크리스트
- [x] 1 `applications:` 제거 → `stack.api` / `stack.worker` (native runner: dir `db-orchestrator-rs`, build `cargo build --release -p …`, run `cargo run -p …`), health_checks를 엔트리 안으로 이동. `port: 11100`은 endpoints에 이미 있어 미이관.
- [x] 2 `modes.*.applications` / 3 `modes:` 6개 → `plans:` infra / infra-full / full-stack / full-stack-ui / hybrid / dev. hybrid·dev의 `environment:` 블록(내용 동일) → `environments.native-local` 하나로 통합, 두 plan이 `environment: native-local`로 참조.
  - hybrid/dev entries: compose(10) → api(20, depends_on compose) → worker(30, depends_on api) — 구 `applications.worker.depends_on: [api]` 반영.
  - full-stack/full-stack-ui: `compose_profiles: [rust]/[rust, ui]`는 profile 서비스명을 `services:`에 명시(`db-orchestrator-api-rs`, `-worker-rs`, `-ui`)로 대체. dry-run: `docker compose … up -d --wait postgres … db-orchestrator-api-rs db-orchestrator-worker-rs` 확인.
  - `modes.full-stack*.provision: default` → 동등물 없음(migrate도 "no equivalent"), `dva provision default` 별도 실행. **보류**.
  - `modes.dev.health_checks: [api]`(worker 미검사) → 신모델은 엔트리 health가 항상 적용되어 dev에서도 worker health(pgrep)를 기다림. 동작 차이 **플래그**.
- [x] 4 `default_mode: infra` → `default_plan: infra`.
- [x] 5 plans 6개 신설 (위).
- [x] 6 `stack.compose.order/tags` 삭제 (`runners.compose.tags: [infra]`는 러너 선언 메타로 유지).
- [x] 7 `build` replace 훅(+subcommands api/worker/docker) 삭제 → `dva build hybrid [api|worker]`(dry-run: cargo build --release in db-orchestrator-rs), docker 이미지는 `interaction.docker-build`(make docker-build)로 분리. `logs` replace 삭제 → `dva logs <plan>` (dry-run 동일 명령). `clean` replace → `steps` (make clean).
  - `make build-api`는 빌드 후 `build/api`로 복사하는데 `dva build`의 native build는 cargo만 실행 — build/ 산출물이 필요하면 `make build` 직접 사용. **플래그**, suggestion_ignore 주석에 기록.
- [x] 8 depends_on/health 수동 이관 (위). 최상위 `health_checks`(start 포함) 삭제.
- [x] provision.full `docker compose --profile rust up --build -d --wait` → `dva build full-stack` + `dva up full-stack` 2단계.
- [ ] provision.default의 `docker compose -f compose.yaml up -d --wait postgres redis kafka` — 동일 서비스 집합의 compose-only plan이 없어(dev는 native 앱까지 기동) 유지. **보류**.
- [ ] provision.reset의 `docker compose -f compose.yaml down -v` — `dva down <plan> --volumes`는 `compose rm --force --stop --volumes <services>`라 프로젝트 전체 down(-v, 네트워크)과 동등하지 않아 유지. 주석 기록. **보류**.
- [x] 섹션 순서 canonical로 재배열 (semantic warning 해소), suggestion_ignore 주석의 `-M` 표기 갱신.
- [x] suggestion_ignore 확장: applications 오류로 validate가 막혀 있던 동안 숨어 있던 Makefile 타깃 ~85개(env-*/office-*/test-colima-*/test-kafka-*/validate*/coverage*/sdk-* 등 CI·evidence·ops)를 glob으로 억제. `build`, `build-api`, `build-worker`는 `dva build` 대체로 억제.
- [ ] `version: "0.1.44"` 미변경. `${VAR:-default}` 표현 없음.
- 문서 후속: deploy/README.md L41 `dva up -M full-stack`, L48 `dva clean`; docs/.claude-context/development-workflow.md L50, docs/reference/11-ports-local-dev.md L17/L32 `dva up -M full-stack` → `dva up full-stack`.

### validate 최종 출력
```
[warn] config drift: compose.files is compose.yaml but detected root compose files are compose.yaml, compose.integration.yaml, compose.test.yaml; …
✅ dva.yml is valid
exit=0
```
warning 1 (기준선: applications 스키마 거부로 validate 실패). 검증은 `--dry-run`만 사용.

### 보류/예외 항목
- compose drift 1건: `compose.integration.yaml`, `compose.test.yaml`은 테스트 전용 — 의도적 예외 (drift ignore 부재, 기보고).
- provision.default/reset의 직접 compose 호출 2건 (위 이유).
- `modes.full-stack*.provision: default` 자동 연계 상실.
- dev plan의 worker health 대기 추가.

### 발견된 dva 개선점
- **`--dry-run up <plan>`이 native 엔트리 health check를 실제로 기다림**: `dva --dry-run up hybrid`가 compose·api dry-run 라인을 출력한 뒤 `http://localhost:11100/health/live` ready_timeout(120s) 동안 블록. Repro: 이 프로젝트에서 `timeout 20 dva --dry-run up hybrid` → api 라인 이후 timeout. dry-run은 health를 건너뛰거나 계획만 출력해야 함.
- **`dva down <plan> --volumes` ≠ `docker compose down -v`**: services가 있는 plan은 `compose rm --force --stop --volumes <svc…>`로 실행되어 named volume·network를 남김. Repro: `dva --dry-run down infra --volumes`. 프로젝트 전체 teardown용 옵션(예: `--all`/plan services 미지정 시 down) 필요.
- **`dva build <plan>`이 build 컨텍스트 없는 compose 서비스까지 `compose build` 인자로 넘김**: `dva --dry-run build hybrid` → `docker compose build postgres redis kafka …`. 이미지 전용 서비스는 제외하거나 stack 선언에 build 대상 표시가 필요.
- **`config migrate`가 `applications.*.port`를 버림**: endpoints가 이미 있으면 무해하지만 없으면 정보 손실 — endpoints 후보로 제안하면 좋음.
- **section-order semantic warning이 사용자 재배열을 요구**: `[warn] semantic: section order … canonical order is [version → env_file → stack → plans → default_plan → environments → checks → …]`. `config migrate --write`가 순서까지 맞춰주거나 `dva fmt`류가 필요.
- **`modes.*.provision`의 대체물 없음**: migrate 자체가 "no equivalent"로 보고. plan에 `provision:` 연계(또는 `dva up --provision`)가 없으면 full-stack 첫 기동 시 seed가 빠짐.
