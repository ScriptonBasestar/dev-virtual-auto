# flow-agent-mesh dva 적용 분석

## 현황
- 파일: `dva.yml` (205줄), `version: "0.1.26"`
- 섹션: `env_file(files)`, `stack`(runners 형식이지만 stack-level `order` 잔존), `environments`(dev/test — `stack:` 참조 포함), `checks`, `modes`(infra/full-stack), `health_checks`(최상위, start 포함), `interaction`(build/clean/logs replace hook), `provision`
- `dva validate`: **ERROR** — `interaction.clean` replace hook.

## 문제점
- **`interaction.clean.replace` (123–126행)**: `clean` built-in 제거로 hook이 걸릴 대상이 없음 — validate가 ERROR로 거부. 에러 메시지대로 `interaction.clean.command/steps`(일반 커맨드화) 또는 down hook으로 이동 필요.
- **`modes:` 섹션 (96–104행)**: 제거된 개념. `dva config migrate`도 "modes: split by hand"로 수동 분해 대상으로 보고. 헤더 주석 사용법(`dva up -M full-stack`, 8–9행)도 제거된 `-M` 표면 기준.
- **`plans:` 없음 + `stack.compose.order: 10` (36행)**: migrate가 "no plan entry to move to — plan을 먼저 선언하고 재실행" 하라고 안내하는 전형적 수동 한계 케이스 (docs/42 §12-5).
- **`environments.*.stack: [compose]` (47, 57행)**: 제거된 키 (42 §11-1 "environments.*.stack 제거 — environment는 stack 선택 책임을 갖지 않음").
- **최상위 `health_checks.am-server`** (107–113행): `modes.full-stack.health_checks`가 참조 중이지만 modes 자체가 제거 대상이므로 함께 stack 엔트리(native runner) 쪽으로 이동 필요.
- `version: "0.1.26"`.

## dva 개선 힌트
- migrate 출력이 이 설정에서 정확히 좋은 안내를 냄(order 이동 불가 사유, modes 분해 매핑 표) — 다만 "nothing to convert"인데 Left-for-you가 4건인 경우, **plan 스캐폴드 자동 생성**(entries에 compose를 넣은 draft plan을 주석으로 출력)까지 해주면 재실행 루프가 짧아짐.
- am 바이너리(native 앱)가 health_check `pgrep` + `start`로만 표현됨 — native runner stack 엔트리로 자연스럽게 옮겨지는 사례이므로 migrate의 modes 안내에 native 이동 힌트도 포함할 가치.

## 마이그레이션 난이도
**중** — stack은 이미 runners 형식이고 규모가 작음(205줄). plans 1~2개 선언 + modes 분해 + environments.stack 제거 + clean hook 일반화면 끝. 대부분 migrate 안내문에 이미 매핑이 적혀 있음.

## 적용 결과 (2026-09-05)

### `dva config migrate` preview (적용 전 기록)
```
nothing to convert.
Left for you:
  - stack.*.order: this config declares no plans — declare a plan whose entries[] name these declarations, then re-run
  - stack.compose.order: 10 has no plan entry to move to
  - modes.infra: split by hand — description → plans.infra.description, compose_services → plans.infra.entries[].services
  - modes.full-stack: split by hand — description → plans.full-stack.description, health_checks → stack.<entry>.health_checks
```
migrate는 이 파일에서 아무것도 자동 변환하지 못했고(평면 compose·applications 없음), 5단계 전부 수동 적용.

### 변경 항목
- [x] `interaction.clean` replace 훅 → `steps:` (description 추가). `build`/`logs` replace 훅은 유지(reserved 우회가 아닌 built-in 대체, 경고 없음).
- [x] `modes:` 제거 → `plans:` 3개: `infra`(env dev), `infra-test`(env test — 기존 test 환경이 도달 불능이 되지 않도록 추가), `full-stack`(compose order 10 + am-server order 20, depends_on compose). `default_plan: infra`.
- [x] `environments.dev/test.stack: [compose]` 제거.
- [x] `stack.compose.order: 10` → plan entries의 order로 이동. stack.compose에 description + services(agent-mesh-db, agent-mesh-redis, tags) 선언.
- [x] 최상위 `health_checks.am-server` → 새 stack 엔트리 `am-server`(native: dir agent-mesh, build `make build`, run `./build/am serve`) 의 `health_checks`로 이동. `start:` 필드는 stack health_checks 스키마에 없어 삭제(native runner의 run이 대체), `start_hint`/`ready_timeout: 30` 유지.
- [x] 헤더 주석 사용법(`dva up full-stack`, `dva up infra-test`, `dva build <plan>`, clean은 `dva run clean`)과 `version: "0.1.44"` 갱신.
- [x] ERROR 해소 후 드러난 Makefile 제안 경고 37건 → `suggestion_ignore` 8개 패턴(dev, compose-clean, env-*, k8s-secret-*, ws-*, validate*, version, dogfood-*)으로 대응. 전부 `.make/*.mk` 공유 include에서 오는 타겟.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 0)
```

### 보류/예외
- `interaction.db`의 `${POSTGRES_USER:-postgres}` — TASK-303 대상, 미수정.
- `build`/`logs` replace 훅 유지(동작 보전).

### 발견된 dva 개선점
- ERROR 한 건이 37건의 suggestion 경고를 완전히 가렸다(TASK-305 실증 추가 사례).
- `suggestion_ignore`를 plans 뒤에 두면 section-order 경고. 정본 순서상 위치가 `checks` 뒤·`interaction` 앞인데 docs 예시들이 이를 드러내지 않는다 — 에러 메시지가 정본 순서를 알려주므로 치명적이진 않음.
- 최상위 `health_checks.<name>.start`가 stack 엔트리 health_checks로 옮겨질 때 버릴 필드라는 안내가 migrate/validate 어디에도 없다. TASK-306 범위에 포함할 만함.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `${POSTGRES_USER:-postgres}`: 수정된 확장기로 정상 동작 확인, 변경 없음.
