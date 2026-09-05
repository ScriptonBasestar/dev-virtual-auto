# scripton-gitrump dva 적용 분석

## 현황
- 파일: `dva.yml` (15,068 bytes, version `0.1.45`, 최종 수정 2026-04-01 — 8개 중 가장 오래됨)
- 사용 섹션: `env_file`(priority/interpolate 포함), `stack`(compose 1엔트리 — **legacy 평면 형식**), `checks`, `applications`(gitrumpd), `default_mode`, `suggestion_ignore`, `modes`(2개), `health_checks`, `interaction`, `provision`, `subprojects`(3개), `endpoints`
- schema 주석이 옛 저장소명 `dev-virtual-auto`의 루트 `schema.json`을 가리킴(line 1) — 현재 정본은 `dva`/`internal/config/schema.json`
- `dva validate` (0.1.48): **ERROR** — legacy 평면 compose 선언 거부

## 문제점
1. **stack compose 평면 선언 (line 26-39)** — `files`/`project_name`/`up_options`/`services`가 `runners.compose:` 없이 엔트리에 직접 있음. validate가 `compose must be declared under runners.compose` hard error로 거부. 8개 중 유일하게 stack 선언 형식 자체가 legacy.
2. **최상위 `applications:` (line 72-89)** — 제거된 섹션 (평면 compose 에러에 가려 아직 미보고).
3. **`modes:`/`default_mode:` (line 91, 104-110)** — deprecated, `plans:` 없음. dev-full mode는 서비스 선택이 비어 있어 "전체"라는 암묵 의미에 의존.
4. **`interaction.clean` replace 훅 (line 314-317)** — 제거된 built-in. `dev` replace 훅(line 267-270)도 reserved 우회 패턴.
5. **lifecycle을 interaction으로 우회**: `dev-full`(line 272-282)이 overlay compose를 직접 `up -d` — overlay 파일 조합은 stack의 두 번째 엔트리 + plan이 정위치.
6. **콜론 이름 interaction** (`app:build`, `app:run`, `app:clean`) — 제거된 `dva app` 네임스페이스를 이름으로 흉내 내는 잔재.

## dva 개선 힌트
- 4월 이후 방치된 설정이 세 세대(평면 compose → runners 구조 → plans)를 한 파일에 보여준다. `config migrate`가 평면 compose→runners 변환을 지원하는지 이 파일로 확인할 가치가 있다(validate 에러 메시지는 수동 rewrite 안내만 한다).
- schema URL 주석이 죽은 저장소 경로를 가리켜도 아무 경고가 없다 — `dva doctor`가 schema 주석의 URL 유효성/최신성을 점검하면 이런 장기 방치를 조기에 잡는다.

## 마이그레이션 난이도
**중** — 오류 종류는 많지만 규모가 작다. compose 1엔트리 + overlay 1개 + app 1개 + mode 2개. 평면 compose를 runners로 감싸고, gitrumpd를 native stack 엔트리로 옮기고, plan 2개(infra/dev-full)를 쓰면 된다. 자동 migrate가 평면 형식을 못 받으면 손으로 해야 하는 점이 변수.

## 적용 결과 (2026-09-05)

### 유지 여부 판단
- 저장소 최신 커밋 2026-09-04(e9e29d9, 태스크 런타임 활동 활발), dva.yml 최종 수정 2026-04-01(ff62e6e). 프로젝트는 살아 있고 dva.yml만 방치된 상태 → **유지·마이그레이션 대상**. 전면 재작성은 하지 않았다.

### 적용한 기계적 변경 (동작 변화 없음)
- [x] `stack.compose` 평면 선언 → `default_runner: compose` + `runners.compose:{files, project_name, up_options, services}`. `order: 10`·`tags: [infra]`는 엔트리에 그대로. `dva config migrate` preview가 낸 형태와 동일하되, migrate가 `runners.compose.tags: [infra]`를 중복 삽입하는 부분은 넣지 않았다.

### validate 최종 출력 (평면 compose 에러 해소 후 다음 계층)
```
ERROR: schema validation failed in dva.yml:
  - (root): Additional property applications is not allowed
  - env_file: Additional property priority is not allowed
  - env_file: Additional property interpolate is not allowed
exit=1
```
`env_file.priority`/`interpolate` 제거 항목은 평면 compose 에러에 가려 이전 보고서에 없던 신규 발견.

### 수동 결정용 마이그레이션 제안 (5단계)
1. **env_file 정리**: `priority: before_environment`, `interpolate: true` 삭제. 현재 dva는 우선순위 고정(environment < env_file < OS)·항상 보간이므로 동작 동일.
2. **applications.gitrumpd → stack 엔트리**: migrate preview 그대로 채택(native, dir gitrump-ce, health http :11700/healthz). 잃는 것: `dev:` cargo watch 변형(interaction이 대체), `run.docker.service`(plan으로 대체). 최상위 `health_checks.gitrump-http` 중복 삭제.
3. **modes → plans**: `infra`(compose, services [webhook-ok, webhook-fail, mirror-upstream], order 10) / `dev-full`(compose 전체 서비스 — overlay 포함 gitrumpd Docker) / 신규 `dev`(compose infra 3개 + gitrumpd native, order 20, depends_on compose). `default_mode: infra` → `default_plan: infra`. `stack.compose.order` 는 plan entries로 이동 후 삭제. **결정 필요**: `dev-full`의 "빈 선택 = 전체" 의미를 services 명시로 고정할지.
4. **lifecycle 우회 interaction 정리**: `dev-full`(+rebuild/status)·`dev` replace 훅은 `dva up/build/status`가 대체하므로 삭제, `clean` replace 훅은 `steps:`로, `app:*` 콜론 이름은 `build-app`/`run-app`/`clean-app`으로 개명. **결정 필요**: `app:clean all`의 `down -v` 파괴 동작 존치 여부.
5. **provision/schema 주석 정리**: note의 `dva dev`/`dva up -M dev-full` → `dva up dev`/`dva up dev-full`; provision 스텝의 직접 `docker compose up` 호출을 `dva up infra`/`dva up dev-full`로 교체할지 결정(현재는 provision 안에서 lifecycle 우회). 1행 schema 주석을 `dva/internal/config/schema.json` 경로로 갱신, `version: "0.1.48"`.

### 보류/예외
- (2026-09-05 오전) 1~5단계 미적용 상태였음. 이후 사용자 승인으로 아래 "5단계 적용 결과"에 전부 적용. `${VAR:-default}` 없음.

### 발견된 dva 개선점
- **migrate가 `tags`를 엔트리와 `runners.compose` 양쪽에 복제**한다(`internal/config/migrate.go:161` 주석의 의도적 동작). 결과 파일은 validate를 통과하지만 같은 값이 두 곳에 있어 드리프트 씨앗이 된다 — compose service 기본 태그 용도라면 preview 출력에 그 이유를 한 줄 남기거나 엔트리 태그와 동일할 땐 생략해야 한다. repro: scripton-gitrump-devbox 원본 커밋(ff62e6e)의 dva.yml에서 `dva config migrate` preview — `runners.compose.tags: [infra]`가 엔트리 `tags: [infra]`와 함께 출력됨.
- migrate가 `applications.*.dev`를 "별도 엔트리를 선언하라"고만 안내하고 같은 명령을 가진 기존 interaction(`dev`)과의 중복은 알려주지 않는다.
- validate 에러 계층화(평면 compose → applications/env_file → 다음)로 한 파일에 세 번 이상 validate를 돌려야 전체 그림이 나온다(TASK-305 실증, 세 번째 사례).

## 5단계 적용 결과 (2026-09-05, 사용자 승인 후)

validate: **exit 0 / warn 0** (이전 exit 1). 커밋하지 않음. `--dry-run up/down/build`만 실행.

- [x] 1. `env_file.priority`/`interpolate` 삭제.
- [x] 2. `applications.gitrumpd` → `stack.gitrumpd` (native, dir gitrump-ce, build `cargo build`, run `cargo run --bin gitrumpd …`, health http :11700/healthz). 최상위 `health_checks.gitrump-http` 삭제. `applications.gitrumpd.dev`(cargo watch)는 `dev-watch` interaction으로 존치.
- [x] 3. `modes`/`default_mode` → `plans` 3개 + `default_plan: infra`. `stack.compose.order` 삭제(plan entries로 이동).
  - `infra`: compose [webhook-ok, webhook-fail, mirror-upstream, postgres]
  - `dev`(신규): infra + `gitrumpd` native (order 20, depends_on compose)
  - `dev-full`: compose 서비스 전체 + overlay의 `gitrumpd` — **"빈 선택=전체"를 services 명시로 고정**(결정 사항 1 → 명시 채택)
  - `postgres`가 compose.yaml에 있으면서 stack 선언에 빠져 있던 것을 추가.
- [x] 4. interaction: `dev`/`build`/`clean`/`logs` replace 훅 삭제, `dev-full`(+rebuild/status) 삭제(`dva up dev-full`/`dva build dev-full`/`dva status`). `app:build`→`build-app`, `app:run`→`run-app`, `app:clean`→`clean`(command 형식). **`clean all`의 `docker compose down -v`는 존치**(결정 사항 2 → 존치, description에 destructive 표기).
- [x] 5. provision의 직접 compose 호출 → nested `dva up`/`down`(5개 devbox 관례). note 잔재 치환, schema 주석·`version: "0.1.48"` 갱신.
- [x] 에러에 가려져 있던 Makefile 제안 warning 58건 → `suggestion_ignore` glob 15개로 정리(env-*/release*/k8s-secret-*/validate* 등, 사유 주석). 상위 문서(Makefile, .make/*.mk, CLAUDE.md, README.md, docs/)에 제거된 CLI 잔재 없음.

### dry-run 확인
- `up infra` / `up dev-full`: compose 인자에 선택 서비스만 전달됨. `down dev-full --volumes`: 정상 해석.
- `build dev`: compose 4개 서비스 + native `cargo build` in gitrump-ce (build 컨텍스트 없는 서비스까지 전달 — TASK-314 재현).
- **`--dry-run up dev`가 native health 대기로 200초 넘게 정지** — TASK-312 재현(3번째 사례). 프로세스를 kill함.

### 잔여 결정
- `dva up dev`의 native gitrumpd health ready_timeout 60은 cold `cargo run` 컴파일 시간을 못 넘길 수 있음 → 실기동 후 120~300으로 조정 검토.
- Makefile `dev`/`dev-full` 타깃은 여전히 compose 직접 호출 — dva plan으로 위임할지는 Makefile 소유자 결정.

## docs/57 §4 재점검 (2026-09-05, TASK-310 가이드 기준)

- §4-2 해당: `run-app` interaction이 native 엔트리 `gitrumpd`와 같은 `cargo run --bin gitrumpd` 프로세스를 두 번째 소유자로 선언했다.
  `run-app` 본체를 삭제하고 하위 `check`만 `check-config`(`--check`, EE 디렉토리 = Makefile `run-check`와 동일)로 승격. 단독 기동은 `dva up dev`.
  `build-app`은 `cargo build`가 idempotent라 유지. validate exit 0.
