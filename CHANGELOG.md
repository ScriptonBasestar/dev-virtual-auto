# Changelog

All notable changes to DVA are documented here.

## [Unreleased]

### Added
- **Named plan이 유일한 lifecycle 모델입니다** (docs/43): 모든 lifecycle 동사가
  `dva <verb> <plan>` 한 형태로 수렴했습니다
  - `dva build <plan>` / `dva logs <plan>` — 기존 compose/service 기준에서 plan 기준으로 전환.
    엔트리 단위 지정(`dva logs <plan> <entry>`)과 플러그인별 라우팅(compose는 passthrough,
    process/script는 `.dva/logs/<name>.log`) 유지
  - `dva down <plan> --purge` — 볼륨 + 로컬 빌드 이미지 + provision 마커 제거.
    확인 프롬프트를 거치며 `--force`가 답합니다. 비-tty에서는 EOF를 거부로 취급해 중단
  - `--purge` / `-v`는 모든 plan 동사가 **파싱**한 뒤 `down` 밖에서 거부합니다.
    `dva up <plan> --purge`가 파괴적 플래그를 조용히 무시하는 대신 에러로 멈춥니다
- **`stack:` 섹션**: 플러그인 기반 인프라 오케스트레이션 파이프라인 (기존 `compose:`/`kubectl:` 최상위 섹션 대체)
- Stack 플러그인 시스템: compose, kubectl, helm, kustomize, tilt, skaffold, podman-compose, process, script, docker, vagrant, sam, serverless, multipass
- 플랫 포맷: 플러그인별 설정을 중첩 없이 최상위에 기술 + `plugin:` 필드로 타입 명시
- 엔트리 이름 기반 플러그인 자동추론 (이름이 플러그인명과 일치하면 `plugin:` 생략 가능)
- `modes.*.stack` 필드: 모드별 특정 stack 엔트리만 실행
- `environments.*.stack` 필드: 환경별 stack 엔트리 필터링
- **설정 병합 시스템** (`mergeFrom`): 필드 레벨 deep merge (모듈/오버라이드 적용 시)
  - map은 key별 merge, list/scalar는 replace
  - `plugin`, `runner` 등 구조적 필드 override 금지
- **시맨틱 검증 경고** (`dva config validate`): 13개 비-치명 검사
  - 중복 stack order, 무거운 인프라 기본 모드 경고, 미해결 환경변수
  - 깊은 서브커맨드 중첩, 정규 섹션 순서 검증
- `dva doctor` command: environment prerequisite checks and setup diagnostics
- Command hooks system (`before`/`replace`/`after`) for hookable lifecycle commands (`up`, `down`, `stop`, `restart`, `build`, `logs`)
- `DVA_CURRENT_UID` special variable (numeric user ID); `DVA_CURRENT_USER` now returns username (string)
- `--exclude-tags` flag on `up`/`down` to skip tagged services at runtime
- `env_file` loading now active in config pipeline

### Changed
- `compose:` / `kubectl:` 최상위 섹션 → `stack:` 섹션으로 통합 마이그레이션
- 모듈 디렉토리 `.dva/` → `.sb/dva/`로 변경
- CLI 구조 변경: lifecycle 동사가 backend 기준(`stack`/`app`/`compose`)에서 **intent 기준
  (named plan)** 으로 수렴 — `dva <verb> <plan>` 단일 세대 (docs/43)
- **`dva doctor` exits non-zero when a user-defined `checks:` entry fails** (built-in checks stay advisory):
  text and `--json` still print full results first; user prerequisites gate `dva doctor && dva up`
- **`dva status` exits non-zero when any entry is unrunnable** (TASK-041):
  post-up status summaries still swallow errors so a successful `up` stays exit 0
- **`dva up --force`**: compose only — passes `--force-recreate` (TASK-040);
  help text states the scope; other plugins ignore Force
- **선언된 환경변수가 compose 컨테이너까지 전달됩니다** (TASK-129): `dva run` 경로에서
  `-e KEY=VALUE`가 `-e`를 받는 모든 compose 서브커맨드(`run`, `exec`)에 주입됩니다.
  이전에는 `method: run`이면서 호스트 OS에도 export된 변수만 전달됐고,
  `method: exec`(설정값 또는 실행 중 컨테이너에서 자동 전환된 경우)와 `steps:` 항목은
  아무것도 받지 못했습니다. `profiles:` 사용 시의 `up`은 `-e` 플래그가 없어 제외입니다.
  전달 대상은 병합된 변수 집합 전체입니다 — `env_file`, global `vars`, `environment:`,
  site vars, plan vars, `--var`, 커맨드 자신의 `environment:`. 단 어딘가에 **선언된** 키만
  해당하며, OS 값은 선언된 키를 덮어쓸 뿐 목록을 늘리지 않으므로 선언하지 않은 호스트
  변수는 전달되지 않습니다. `DVA_*`는 계속 제외되고, 키는 정렬되어 argv가 결정적입니다.
  **주의**: `dva.yml`에만 선언한 변수도 이제 전달되므로 이미지에 내장된 값을 덮어씁니다 —
  `PATH`를 선언했다면 exec 시 컨테이너의 `PATH`가 교체됩니다.
  `kubectl exec`은 env 플래그가 없어 해당 경로는 변경 없음

### Removed
> **Breaking.** 아래 표면은 `0.1.16` 이후 master에만 존재했고 태그된 릴리스에 포함된 적이
> 없습니다. `0.1.16`에서 올라오는 경우 영향받지 않습니다. master 빌드를 쓰고 있었다면
> `dva config migrate`가 `applications:`와 `stack.*.order`를 자동 변환합니다 (`--write`로 적용).

- **`dva stack` / `dva app` / `dva infra` / `dva clean` 커맨드** (docs/43): 예약어가 27개에서
  **23개**로 줄었습니다. lifecycle 동사가 plan/stack/app 3중 복제였고, 그중 plan만이
  사용자가 실제로 표현하려는 것(의도)에 대응했습니다
  - `dva stack up <entry>` → plan에 해당 엔트리만 담아 `dva up <plan>`
  - `dva app up` / `dva app up <app> --dev` → 엔트리를 각각 선언하고 plan이 선택
  - `dva infra up/down` → 위임이었을 뿐이므로 대응하는 plan 동사
  - `dva clean` → `dva down <plan> --purge`
- **`applications:` 섹션** 및 `ApplicationConfig` / `AppVariant` / `AppExecPaths` 타입,
  `modes.*.applications` 필드: 앱 프로세스는 `native` 러너를 쓰는 stack 엔트리입니다.
  `AppManager`(~830줄)와 포트 소유권 추적도 함께 제거됐습니다
- **`--dev` / `--docker` 플래그**: 엔트리는 `run` 명령 하나를 선언합니다. hot-reload 변형은
  별도 엔트리로 선언하고 plan으로 고릅니다
- **`clean`이 hookable에서 빠졌습니다**: hookable 커맨드는 7개에서 **6개**
  (`up`/`down`/`stop`/`restart`/`build`/`logs`)로 줄었습니다. `interaction.clean.before`를
  쓰던 설정은 이제 `dva validate`가 **exit 1로 거부**합니다 — 경고가 아니라 에러인 이유는,
  훅이 붙을 커맨드가 없어진 상태에서 `before: [backup]`이 그냥 실행되지 않으면 출력은
  정상이고 신호는 전혀 없기 때문입니다
- **`applications.<app>.health.required`** (TASK-118 strict readiness): ⚠️ **기능 손실**.
  헬스체크가 끝내 통과하지 않을 때 non-zero exit하던 opt-in 엄격 모드로, plan 경로에
  등가물이 없습니다 — 엔트리 헬스체크는 advisory입니다. `dva config migrate`가 이 키를
  버리면서 리포트에 명시합니다. 대안은 `checks:` 엔트리 또는 interaction 커맨드로 게이팅.
  자세한 내용은 [docs/43](docs/43-command-surface-restructure.md) §16

### Fixed
- **`migrate`가 읽는 곳 없는 필드를 조용히 옮기던 문제**: `applications.<app>.health.required`가
  `stack.<entry>.health_checks`로 그대로 복사됐습니다. `HealthCheckConfig`에 그 필드가 없고
  엔트리 스코프 `health_checks` 스키마에 `additionalProperties: false`가 없어서, 변환 결과는
  `dva validate` exit 0을 받고 `VerifyMigrated`도 통과한 뒤 **아무 동작도 하지 않았습니다**.
  `port`가 눈에 보이게 사라지는 것보다 나쁜 형태였습니다 — 눈에 보이게 남아있고 무력함.
  이제 키를 버리고 리포트에 사유를 적습니다 (클래스 차원의 게이트는 TASK-182)
- **존재하지 않는 명령을 안내하던 에러 메시지**: git source 캐시가 설정된 ref와 어긋날 때
  `dva infra update <name>` 실행을 권했는데, 그 커맨드는 제거됐습니다. git source는 없을 때만
  clone하고 자동 pull하지 않으므로(재현성), 캐시 디렉토리를 지우는 것이 곧 재clone입니다 —
  메시지가 그렇게 안내합니다
- **`runners.native.build` / `runners.native.env`가 선언만 되고 실행되지 않던 문제**:
  `schema.json`이 두 필드를 광고하고 `decodeRunnerNode`가 디코딩까지 했지만, native→process
  강등 지점(`applyRunnerConfig`, `materializeResolvedEntry`)이 둘 다 버렸습니다
  (`ProcessPluginConfig`에는 `Env` 필드 자체가 없음). `env`는 resolver 병합 사슬의
  `stackEntry.Vars` 직후로 들어가고(러너 한정 선언이므로 엔트리 전역 vars보다 좁게, plan/override
  vars보다는 약하게), `build`는 plan-aware `dva build <plan>`이 실행자가 됩니다
- **실행 중인 컨테이너 감지가 설정을 무시하던 문제** (TASK-133): `dva run`은 서비스가 이미
  떠 있으면 `run` 대신 `exec`으로 전환하는데, 그 판단을 `docker compose ps` 맨몸 호출로
  했습니다 — `-f`도 `--project-name`도 없고 바이너리는 `docker` 고정. 즉 실행과 **다른
  프로젝트**에 질문하고 그 답을 실행에 썼습니다. 컴포즈 파일이 CWD가 아니라 `files:`로
  지정된 경우 감지가 항상 실패해, 실행 중인 컨테이너를 두고 `run --rm` 일회용 컨테이너를
  새로 만들어 명령을 실행했습니다 — 성공하지만 엉뚱한 컨테이너에서, `--rm`이 흔적까지 삭제.
  이제 감지도 다른 모든 compose 호출과 같은 빌더를 거칩니다
- **`--project-name`이 두 번 붙던 문제** (TASK-132): `project_name:`을 선언한 설정에서
  컨테이너가 이미 실행 중이면 — 즉 일반적인 개발 루프 상태 — argv에 플래그가 두 번
  나타났고, 실패한 step의 에러 메시지에 그대로 노출됐습니다. 동작 자체는 옳았지만
  (docker가 마지막 값을 취하고 그 값이 감지된 프로젝트였음) 우선순위가 argv 순서에만
  의존했습니다. 이제 감지된 이름은 플래그를 쓰는 유일한 지점으로 전달됩니다
- `DVA_CURRENT_USER` was returning UID (number) instead of username (string)
- `env_file` field was parsed but never loaded into environment
- Tag filtering (`FilterInteractions`, `exclude_tags`) was implemented but not called for subprojects
- `os.Exit(1)` inside `RunE` replaced with `return err` for consistent cobra error handling
- `dva doctor` always exited 0 after reporting failed checks (TASK-046)
- Removed inert schema surface: `devcontainer.config_path` (TASK-037); provision structured
  `shell`/`sleep`/`docker` (TASK-044; raw-string form still works); provision profiles now
  schema-validated against `provision_item`
- Plan `entries[].runner` honored at execution via plan orchestrator materialization (TASK-039)

## [0.1.16] - 2026-03-24

### Added
- `dva show` command: config summary (profiles, environments, commands)
- `--env` flag: named environment profiles (`environments:` section in dva.yml)
- `--mode` flag: operational mode profiles (`modes:` section in dva.yml)
- `default_profile` field in `provision` config for profile fallback
- `dva provision --list`: list available provision profiles
- USAGE.md: comprehensive command and flag reference

### Changed
- `EnvFile` removed from environment profiles (simplification)

## [0.1.15] and earlier

See git log for full history: `git log --oneline`
