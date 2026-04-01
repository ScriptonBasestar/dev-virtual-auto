# DVA — Dev Virtual Auto

개발 환경 오케스트레이터. `dva.yml` 하나로 Docker Compose, Kubernetes, Helm, 로컬 프로세스를 통합 관리.

## Build

```bash
make build          # bin/dva 생성 (generate 포함)
make install        # ~/.local/bin/dva 설치
make test           # 전체 테스트
make test-integration  # 통합 테스트 (-tags=integration)
make generate       # workflow/*.txt → internal/cli/*.txt 임베드 생성
```

**규칙**: `go build` 직접 실행 금지 → `make build` 사용

## Structure

```
cmd/dva/         # main 진입점 (minimal)
internal/
  cli/           # cobra 명령어 (root, run, compose, stack, ...)
  config/        # dva.yml 파싱·병합·검증
  lifecycle/     # 플러그인 백엔드 (compose/helm/kubectl/...)
  exec/          # 외부 명령어 실행
  logger/        # slog 래퍼
  output/        # 출력 포맷
workflow/        # improve 워크플로우 템플릿 (make generate 소스)
examples/        # dva.yml 예시 파일
tasks/           # 작업 추적
```

## Key Concepts

- **Stack**: `dva.yml`의 `stack:` 섹션 — `LifecycleEntry` 목록, `order`로 실행 순서 결정
- **Plugin**: lifecycle 백엔드 타입 (`compose`, `helm`, `kubectl`, `process`, `script` 등 3-tier)
- **Mode** (`--mode`): 런타임 전략 선택 (dev-only 도구, stg/prd 환경 없음)
- **App**: `applications:` 섹션 — `native`/`docker` 전략으로 앱 프로세스 관리
- **Interaction**: `dva run <name>` 으로 실행되는 사용자 정의 커맨드

## Config File Loading

```
dva.yml (현재 디렉토리) → modules: 병합 → subprojects: 병합
```

환경 변수 우선순위: `env_file` < `environment:` < OS 환경 변수

## Module: github.com/ScriptonBasestar/dva
