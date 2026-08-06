# DVA — Dev Virtual Auto

개발 환경 오케스트레이터. `dva.yml` 하나로 Docker Compose, Kubernetes, Helm, 로컬 프로세스를 통합 관리.

## Documentation

문서 또는 설계 작업 전 `AGENTS.md`의 **Documentation Ownership**을 따릅니다.

- 철학과 판단 기준: `SOUL.md`
- 제품 가치와 범위: `PRODUCT.md`
- 구현 경계와 데이터 흐름: `ARCHITECTURE.md`

내용을 복제하지 말고 canonical document를 갱신한 뒤 다른 문서에서는 요약과 링크만
사용합니다.

## Build

```bash
make build          # bin/dva 생성 (generate 포함)
make install        # ~/.local/bin/dva 설치
make test           # 전체 테스트
make test-integration  # 통합 테스트 (-tags=integration)
make generate       # agent-mesh-flows/shared/library/ → internal/cli/library_reference.txt 임베드 생성
```

**규칙**: `go build` 직접 실행 금지 → `make build` 사용

## Structure

```
cmd/dva/             # main 진입점 (minimal)
internal/
  cli/               # cobra 명령어 (root, run, compose, stack, ...)
  config/            # dva.yml 파싱·병합·검증
  lifecycle/         # 플러그인 백엔드 (compose/helm/kubectl/...)
  runner/            # interaction 실행 엔진 (compose/local/kubectl 러너)
  exec/              # 외부 명령어 실행
  integration/       # 통합 테스트 (-tags=integration)
  logger/            # slog 래퍼
  output/            # 출력 포맷
agent-mesh-flows/    # agent-mesh flow 정의 (product: dva.yml 분석/개선, am 실행)
skills/              # 포터블 스킬 단일 소스 (SKILL.md → Cursor/Codex/OpenCode 투영, tools/skillgen)
workflows/           # DVA 자체 개선 dogfood 워크플로우 (prmpt에서 import, 단일 소스)
tools/skillgen/      # skills/ → 플랫폼별 아티팩트 변환기 (make generate)
examples/            # dva.yml 예시 파일
tasks/               # 작업 추적
```

## Agent-Mesh Flows

AI 기반 프로젝트 분석, 설정 개선, 진단 워크플로우는 agent-mesh(`am`) CLI를 사용하여 독립적으로 실행합니다.

```bash
# Agent-Mesh DVA 워크플로우 커맨드:
am run dva-discover                   # 프로젝트 분석 및 후보 옵션 탐색
am run dva-improve                    # 기존 dva.yml 개선
am run dva-improve param.mode=rewrite # 처음부터 파일 재생성 (초기화)
am run dva-improve-guided             # 대화형 가이드 모드
am run dva-diagnose                   # 환경 상태 점검 및 문제 자동 진단
```

**의존성**: `am` (agent-mesh) CLI가 PATH에 있어야 함.

Flow 파일: `agent-mesh-flows/` 디렉토리. Library reference: `agent-mesh-flows/shared/library/`.

## Key Concepts

- **Stack**: `dva.yml`의 `stack:` 섹션 — `LifecycleEntry` **선언 저장소** (실행 표면 아님)
- **Plan**: `plans:` 섹션 — 실행 가능한 이름. lifecycle 동사는 전부 `dva <verb> <plan>` 형태
- **Plugin**: lifecycle 백엔드 타입 (`compose`, `helm`, `kubectl`, `process`, `script` 등 3-tier)
- **Mode** (`--mode`): 런타임 전략 선택 (dev-only 도구, stg/prd 환경 없음)
- **Interaction**: `dva run <name>` 으로 실행되는 사용자 정의 커맨드

앱 프로세스는 `native` 러너를 쓰는 stack 엔트리입니다 — `applications:` 섹션과
`dva stack`/`app`/`infra`/`clean` 명령은 제거됐습니다 (docs/43).

## Config File Loading

```
dva.yml (현재 디렉토리) → modules: 병합 → subprojects: import 대상 로드
```

환경 변수 우선순위 (낮음 → 높음): `environment:` < `env_file` < OS 환경 변수

`loadEnv`(`cli/root.go`)가 `environment:`를 먼저 적용한 뒤 `env_file`을 덮어씁니다.
plan 경로(`dva up <plan>`)의 전체 `vars` 우선순위는 USAGE.md를 참조하세요.

## Module: github.com/ScriptonBasestar/dva
