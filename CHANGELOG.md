# Changelog

All notable changes to DVA are documented here.

## [Unreleased]

### Added
- **`applications:` 섹션**: 네이티브/Docker 앱 프로세스 라이프사이클 관리
  - `run`, `build`, `dev` 실행 경로 (native/docker 전략)
  - `depends_on` 의존성 기반 토폴로지 정렬 및 동시 시작
  - 앱별 헬스체크, 포트, 환경변수, 작업 디렉토리
- **`dva stack` 서브커맨드**: 인프라 엔트리별 관리
  - `stack up/stop/down/status/log` — 개별 stack 엔트리 제어
  - 역순 teardown (LIFO), 엔트리별 로그 조회
- **`dva app` 서브커맨드**: 애플리케이션 프로세스 관리
  - `app ls/up/stop/down/build/restart/log`
  - `--dev` 모드 (hot-reload), `--docker` 빌드
  - PID 파일 기반 프로세스 추적, 앱별 로그 파일
- **`stack:` 섹션**: 플러그인 기반 인프라 오케스트레이션 파이프라인 (기존 `compose:`/`kubectl:` 최상위 섹션 대체)
- Stack 플러그인 시스템: compose, kubectl, helm, kustomize, tilt, skaffold, podman-compose, process, script, docker, vagrant, sam, serverless, multipass
- 플랫 포맷: 플러그인별 설정을 중첩 없이 최상위에 기술 + `plugin:` 필드로 타입 명시
- 엔트리 이름 기반 플러그인 자동추론 (이름이 플러그인명과 일치하면 `plugin:` 생략 가능)
- `modes.*.stack` 필드: 모드별 특정 stack 엔트리만 실행
- `modes.*.applications` 필드: 모드별 앱 실행 전략 (`"native"`, `"docker"`, 또는 앱별 맵)
- `environments.*.stack` 필드: 환경별 stack 엔트리 필터링
- **설정 병합 시스템** (`mergeFrom`): 필드 레벨 deep merge (모듈/오버라이드 적용 시)
  - map은 key별 merge, list/scalar는 replace
  - `plugin`, `runner` 등 구조적 필드 override 금지
- **시맨틱 검증 경고** (`dva config validate`): 13개 비-치명 검사
  - 중복 stack order, 무거운 인프라 기본 모드 경고, 미해결 환경변수
  - 깊은 서브커맨드 중첩, 정규 섹션 순서 검증
- `dva doctor` command: environment prerequisite checks and setup diagnostics
- Command hooks system (`before`/`replace`/`after`) for hookable lifecycle commands (`up`, `down`, `stop`, `restart`, `build`, `clean`, `logs`)
- `DVA_CURRENT_UID` special variable (numeric user ID); `DVA_CURRENT_USER` now returns username (string)
- `--exclude-tags` flag on `up`/`down` to skip tagged services at runtime
- `env_file` loading now active in config pipeline

### Changed
- `compose:` / `kubectl:` 최상위 섹션 → `stack:` 섹션으로 통합 마이그레이션
- 모듈 디렉토리 `.dva/` → `.sb/dva/`로 변경
- CLI 구조 변경: `up`/`down` → `stack up`/`stack down` + `app up`/`app down` 분리
- `dva status` 출력에 앱 포트 정보 추가
- **`dva doctor` exits non-zero when a user-defined `checks:` entry fails** (built-in checks stay advisory):
  text and `--json` still print full results first; user prerequisites gate `dva doctor && dva up`
- **`dva status` / `dva stack status` exit non-zero when any entry is unrunnable** (TASK-041):
  post-up status summaries still swallow errors so a successful `up` stays exit 0
- **`dva up --force` / `stack up --force`**: compose only — passes `--force-recreate` (TASK-040);
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

### Fixed
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
