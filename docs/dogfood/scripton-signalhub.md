# scripton-signalhub dva 적용 분석

## 현황
- 파일: `dva.yml` (11,405 bytes, version `0.1.26`)
- 사용 섹션: `env_file`, `stack`(compose 1엔트리), `checks`, `modes`(3개), `health_checks`, `interaction`, `provision`(default/full/reset), `subprojects`(3개), `endpoints`
- `dva validate` (0.1.48): **ERROR** — `interaction.clean` 제거된 built-in 훅

## 문제점
1. **`interaction.clean` replace 훅 (line 253-259)** — `clean` built-in 제거로 hard error. sadawiki와 동일 패턴.
2. **`modes:` 3개 (line 81-100)** — deprecated. hybrid mode가 `environment:` 주입(DB/Redis host·port)까지 겸해 `environments` 분리 대상. `plans:` 없음.
3. **`stack.compose.order`/`tags` (line 28-29)** — 실행 계획 필드가 선언에 잔류.
4. **lifecycle을 interaction으로 우회**: `dev`(line 130-133, `docker compose up -d`), `infra-up`/`infra-down`(line 262-270), `db reset`의 `down -v && up -d`(line 226-229) — 전부 `dva up/down <plan>`이 정위치. 특히 `dev`는 reserved 계열 이름이면서 compose 직접 실행.
5. **`build`/`logs` replace 훅 (line 150-156, 237-241)** — plan-aware lifecycle 동사와 충돌하는 구세대 우회.
6. **applications 미사용인데 native 앱 실행이 interaction에 산재** — `dev:engine`/`dev:ui`(go run, pnpm dev)가 interaction으로만 존재. 신모델이라면 native runner stack 엔트리 + plan(hybrid)이 정위치. health_checks.engine-api의 `start:`가 그 자리를 임시로 메꾸고 있다.

## dva 개선 힌트
- "engine은 native, UI는 pnpm dev" 같은 장기 실행 dev 서버를 interaction으로 두는 패턴이 반복된다(포그라운드 watch 프로세스). plans의 native runner가 포그라운드/watch 실행 UX(로그 스트리밍, Ctrl-C 전파)를 잘 지원하는지가 이 프로젝트 전환의 관건 — dogfood 시나리오로 적합.
- 훅 제거 hard error(clean)는 modes 경고보다 먼저 파싱을 중단시켜, 사용자가 deprecated 경고 목록 전체를 한 번에 못 본다. validate가 error 후에도 나머지 진단을 이어서 출력하면 마이그레이션 계획 수립이 한 번에 된다.

## 마이그레이션 난이도
**하~중** — 구조는 sadawiki급으로 단순(compose 1엔트리 + mode 3개)하나, native 앱 2개를 stack 엔트리로 새로 선언하고 hybrid mode의 env 주입을 environments로 분리하는 작업이 추가된다.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `dva config migrate` 미리보기: modes 3개 변환 없음. 힌트 "environment → environments.hybrid.vars"는 스키마(`environments.<name>.environment`, docs/40 §3-5)와 불일치 — 그대로 따르면 validate 실패. `provision: default/full` 모드 필드는 "no equivalent".
- [x] `modes:` 제거 → `plans.infra`(db/redis), `plans.full-stack`(engine 컨테이너 포함), `plans.hybrid`(compose→engine native→admin-ui native, depends_on 체인). `default_plan: infra`.
- [x] hybrid 모드의 env 주입 → `environments.hybrid.environment` (DB/REDIS host·port localhost).
- [x] engine/ui → `stack.engine`(native, dir signalhub-engine, build `go build ./...`, run `go run cmd/server/main.go`, health http `${SIGNALHUB_PORT:-11800}/health`), `stack.admin-ui`(native, pnpm build/dev).
- [x] `stack.compose.order` 및 엔트리 tags 제거(runner tags `[infra]` 유지).
- [x] `docker compose`를 직접 부르던 interactions 제거: `dev`(+dev:engine/dev:ui → `dva up hybrid`), `infra-up`/`infra-down`(→ `dva up/down infra`).
- [x] `build`/`logs` replace hook + 서브커맨드 제거 → `dva build hybrid`(native build 포함), `dva logs infra`. core 빌드만 `build-core` interaction으로 유지(plan 엔트리가 아니라 대체 불가).
- [x] `db reset` → steps(`docker compose down -v` → `dva up infra`), `clean` → steps.
- [x] provision: infra step `compose_up`, full 프로필 `compose_up: [signalhub-engine, signalhub-db, signalhub-redis]`, note 문구를 plan 이름으로 갱신.
- [x] subprojects import 정리(engine `watch`, admin-ui `install`; 중복 `generate` 제거).
- [x] version 0.1.44.

### validate 최종 출력
```
[warn] semantic: compose.yaml: missing top-level 'name: signalhub'
✅ dva.yml is valid
EXIT=0   (warning 1 — 의도적 예외)
```

### 보류/예외 항목
- compose.yaml `name:` 누락 경고 — compose.yaml 수정 없이는 해소 불가(다른 파일 금지 규칙) → 예외.
- `${SIGNALHUB_PORT:-11800}` (stack.engine health url) — TASK-303, 그대로 이동만.
- `modes.*.provision` 필드는 대응 개념 없음 → 의미 손실 없이 삭제(provision 프로필은 별도 유지됨).
- Makefile 제안(dev/engine/ui-module/e2e 등) suggestion_ignore로 일괄 처리.

### 발견된 dva 개선점
- **migrate 힌트가 스키마와 불일치**: `environments.hybrid.vars`를 안내하지만 유효 키는 `environment`. (repro: 원본 dva.yml에서 `dva config migrate` 출력 확인 후 힌트대로 작성 → validate 에러.)
- **validate 조기 중단**(sadawiki와 동일): clean replace 하드 에러가 나머지 경고를 가림.
- **`--purge` named volume 미삭제**: db reset/clean이 여전히 `docker compose down -v`를 steps로 호출해야 함.
- **PlanEntry에 profiles 없음**: profile 게이트 서비스는 services에 이름을 명시해야 활성화됨. 동작은 하나 문서화 필요.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `${SIGNALHUB_PORT:-11800}` health url: 유지 확정.
