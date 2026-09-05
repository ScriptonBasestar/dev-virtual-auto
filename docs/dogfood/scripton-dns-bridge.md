# scripton-dns-bridge dva 적용 분석

## 현황
- 파일: `dva.yml` (22,750 bytes — 8개 중 최대, version `0.1.44`, 최근 수정 2026-09-04)
- 사용 섹션: `env_file`, `stack`(compose 1엔트리, 서비스 15개), `checks`, `applications`(api/worker), `default_mode`, `suggestion_ignore`, `modes`(7개), `health_checks`, `interaction`(대규모), `provision`(default/full/reset), `subprojects`(1개), `endpoints`(14개)
- `dva validate` (0.1.48): **ERROR** — schema 거부 5건

## 문제점
1. **최상위 `applications:` (line 95-137)** — 제거된 섹션. api는 `run.native`/`run.docker`에 더해 `dev.native`(cargo watch), `build.native`/`build.docker`까지 구모델 전 기능을 사용. schema 거부.
2. **`modes.*.applications:` (hybrid/dev/full-stack/full-stack-monitoring, line 162/171/177/192)** — schema 거부. kafka/nameserver mode는 applications 없이 profile만 켜는 등 mode마다 축이 달라 분해 난이도를 높인다.
3. **`modes:` 7개 (line 154-207)** — deprecated. dev mode 주석(line 180-187)이 compose_services↔compose_profiles 상호작용 함정을 길게 설명하고 있는데, 이는 mode 모델 자체의 예측 불가능성 기록이다.
4. **`default_mode: infra` (line 139)**, **`plans:` 없음**, **`stack.compose.order`/`tags` (line 22-23)** — 신모델 미전환.
5. **`interaction.clean` replace 훅 (line 350-353)** — 제거된 built-in. `logs`/`build` replace 훅도 plan-aware 동사와 충돌.
6. **lifecycle을 interaction으로 우회**: `infra-up`/`infra-down`/`docker-dev`(line 356-385)가 `docker compose up/down`을 직접 실행 — docs/42 §12-2 "생명주기 명령은 interactions로 우회하지 않음" 위반.

## dva 개선 힌트
- dev mode 주석이 문서화한 함정(서비스 나열이 profile 활성화를 억제)은 plans 모델에서도 `entries[].services` + compose profile 조합으로 재현될 수 있다 — plan entry에 profile 선택 필드가 있는지/문서화됐는지 점검 가치.
- kafka/nameserver처럼 "옵션 인프라 묶음"을 켜는 mode는 plan 조합(plan 합성 또는 entry 재사용) 요구를 보여준다. plan 간 include/합성이 없으면 postgres+redis 서비스 목록이 plan마다 복제된다(nd-stack에서 실제 발생).
- sops 기반 `env` interaction 트리는 secrets 관리가 DVA 표면 밖에 있음을 보여줌 — env_file에 sops 소스 지원 같은 gap 후보.

## 마이그레이션 난이도
**상** — db-orchestrator와 동형이지만 mode 7개 + profile 상호작용 함정 + applications의 dev/build 확장 필드까지 있어 8개 중 가장 무겁다. migrate 자동 변환 후에도 modes 3축 분해와 dev/build/depends_on 수동 이관 필요.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] 1 `applications.api/worker` 제거 → `stack.api` / `stack.worker` native 엔트리 (dir `dns-bridge-rs`; build = 구 `build.native`, run = 구 `run.native`; health_checks 엔트리 안으로). docker 변형(`run.docker`/`build.docker`)은 기존 compose 서비스 `dns-bridge-api-rs`/`-worker-rs`(profile rust)를 plan에서 선택하는 것으로 대체 — 별도 docker 엔트리 불필요. `dev.native`(cargo watch)는 어떤 mode에도 속하지 않던 독립 동사였으므로 이미 존재하는 `interaction.run-api-watch`가 충실한 형태 — 엔트리로 만들지 않음(주석 기록). `port`는 endpoints에 존재.
- [x] 2 `modes:` 7개 → `plans:` infra(default) / hybrid / full-stack / full-stack-monitoring / dev / kafka / nameserver + 구 docker-dev interaction의 `docker-dev` plan. hybrid·dev의 동일 `environment:` → `environments.native-local`.
  - profile-only mode(kafka/nameserver/dev/full-stack*)는 PlanEntry에 `profiles`가 없어 profile 서비스를 `services:`에 명시: kafka → [postgres, redis, zookeeper, kafka], nameserver → [postgres, redis, powerdns, etcd, coredns], dev → [postgres, redis, mock-auth], full-stack → [postgres, redis, dns-bridge-migrate, dns-bridge-api-rs, dns-bridge-worker-rs], full-stack-monitoring/docker-dev는 + [prometheus, grafana, jaeger, otel-collector] + mock-auth. dry-run으로 각 up 명령 확인. **플래그**: compose.yaml 서비스 추가 시 plan 목록도 갱신 필요.
  - 구 dev mode 주석(compose_services가 profile 활성화를 억제하는 함정)은 "명시 서비스명이 profile을 활성화한다"는 plans 주석으로 교체.
  - `dns-bridge-api-rs`가 `dns-bridge-migrate`에 depends_on 하지 않아 migrate를 plan에 명시. one-shot 컨테이너와 `--wait` 조합은 실기동 시 확인 필요 **플래그**.
  - `modes.full-stack*.provision: default` → 동등물 없음 **보류**. `modes.dev.health_checks: [api]` → worker health도 대기(동작 차이 플래그).
- [x] 4 `default_mode` → `default_plan: infra`; `stack.compose.order/tags` 삭제.
- [x] 5 `build`/`logs` replace 훅 삭제 → `dva build hybrid [api|worker]`, `dva build full-stack`(compose images), `dva logs <plan>`. `clean` replace → steps (cargo clean + `docker compose down -v`; `dva down --volumes`는 plan 서비스만 rm 하므로 compose 직접 호출 유지, 주석 기록).
- [x] 6 `infra-up` → `dva up infra`(dry-run: `up -d --wait postgres redis`, 동일), `infra-down` → `dva down infra`(dry-run: `rm --force --stop postgres redis` — 구 `docker compose down`은 네트워크까지 제거, 차이 **플래그**), `docker-dev` → plan `docker-dev`(`--build`는 plan에서 표현 불가 → `dva build docker-dev && dva up docker-dev`, 주석·suggestion_ignore 기록). 세 interaction 삭제.
- [x] provision.default `docker compose up -d --wait postgres redis` → `dva up infra` (동일 명령). provision.full `--profile rust --profile dev --profile monitoring up --build -d` → `dva build docker-dev` + `dva up docker-dev` (`--wait`가 추가됨). provision.reset `docker compose down -v`는 유지(위 이유).
- [x] 섹션 순서 canonical 재배열. suggestion_ignore: build/build-api/build-worker/logs/infra-up/infra-down/docker-dev(dva 동사 대체) + env-*/validate*/db-rls-provision/test-fast/verify-provider/version(applications 오류 해소 후 드러난 CI·ops 타깃).
- [ ] `version: "0.1.44"` 미변경. `${VAR:-default}` 표현 없음(compose.yaml에는 있으나 대상 아님).
- 문서 후속: ai-docs/workflow/setup-dva/stages/40-execute.md L40 `dva up -M infra-only`.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0
```
warning 0 (기준선: applications 스키마 거부로 validate 실패). 검증은 `--dry-run`만 사용.

### 보류/예외 항목
- `modes.*.provision: default` 자동 연계 상실.
- `dva down infra`가 네트워크를 남김(구 infra-down과 차이), `docker compose down -v`는 clean/reset에 직접 호출 유지.
- docker-dev의 `--build`는 별도 `dva build docker-dev` 필요.
- dev plan의 worker health 대기 추가; full-stack의 migrate one-shot + `--wait` 실기동 확인 필요.
- redis-cluster/redis-sentinel profile은 mode가 없었으므로 plan도 만들지 않음.

### 발견된 dva 개선점
- `dva logs <plan>`은 엔트리가 2개 이상이면 `name one: dva logs hybrid <api|compose|worker>`로 거부 — 합리적이나 native 엔트리 로그 경로가 문서화돼 있는지 확인 필요(구 `dva logs`는 compose 전체 tail).
- (db-orchestrator와 공통) `--dry-run up`이 native health를 대기, `dva down <plan> --volumes`가 프로젝트 전체 down이 아님, `dva build <plan>`이 이미지 전용 compose 서비스를 build 인자로 넘김, `modes.*.provision` 대체물 없음, section-order warning.
- plan 간 서비스 목록 중복(postgres, redis가 8개 plan에 반복). `plans.<name>.composes`(TASK-260)가 있으나 composition plan은 자체 entries를 가질 수 없어 "infra + 추가 서비스" 패턴에는 쓸 수 없음 — leaf plan 상속/alias(TASK-307)가 해법.

## docs/57 §4 재점검 (2026-09-05, TASK-310 가이드 기준)

- §4-2 해당: `run-api`/`run-worker` interaction이 native 엔트리 `api`/`worker`(`plans.dev`/`hybrid`)와 동일한 `cargo run` 프로세스를 중복 선언. 두 interaction 삭제.
  `run-api-watch`(cargo-watch)는 native 러너가 auto-reload를 표현할 수 없어 `dva up`의 대체 수단으로 존치(주석 명시).
  프로젝트 문서는 전부 Makefile 타겟(`make run-api`)을 가리키므로 문서 변경 없음. validate exit 0.
