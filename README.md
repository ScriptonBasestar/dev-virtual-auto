# DVA (Dev Virtual Auto)

개발 환경 오케스트레이터 — `dva.yml` 하나로 Docker Compose, Kubernetes, Helm, 로컬 프로세스 등을 통합 관리.

## Install

### Binary (추천)

```bash
# From source
go install github.com/ScriptonBasestar/dva/cmd/dva@latest

# Or build locally
make build
./bin/dva version
```

### From Release

```bash
curl -sL https://github.com/ScriptonBasestar/dva/releases/latest/download/dva_linux_amd64.tar.gz | tar xz
sudo mv dva /usr/local/bin/
```

## Quick Start

프로젝트 루트에 `dva.yml` 생성:

```yaml
version: "0.1.0"

stack:
  compose:
    order: 10
    files:
      - docker-compose.yml

interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/bash
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
```

```bash
dva ls              # 사용 가능한 커맨드 목록
dva shell           # = dva run shell → docker compose run app /bin/bash
dva test            # = dva run test → docker compose run app bundle exec rspec
dva up              # stack 전체 시작 (compose up -d --wait 등)
dva down            # stack 전체 중지
dva config validate # dva.yml 스키마 검증
dva manifest        # LLM용 전체 커맨드 매니페스트 출력
```

## Commands

```bash
dva ls                 # 사용 가능한 커맨드 목록
dva shell              # = dva run shell (run 생략 가능)
dva up                 # stack 시작 (order 순서대로)
dva up -M backend      # 모드 적용
dva up -T backend      # 특정 태그 그룹만 실행
dva down               # stack 중지 및 제거
dva status             # 워크스페이스 상태 확인
dva show               # 설정 요약
dva config validate    # dva.yml 검증
dva provision          # 프로비저닝 실행
dva config init --ai   # AI로 dva.yml 자동 생성
dva doctor             # 환경 사전조건 진단
dva migrate            # 레거시 설정 마이그레이션 가이드
```

전체 커맨드 레퍼런스: **[USAGE.md](USAGE.md)**

## Configuration

### Stack (인프라 오케스트레이션)

`stack:` 섹션에서 여러 플러그인을 `order` 순서대로 실행합니다:

```yaml
stack:
  compose:                   # 엔트리 이름 = 플러그인 자동추론
    order: 10
    files: [docker-compose.yml]
    project_name: myapp
  kubectl:
    order: 20
    namespace: myapp-dev
  my-staging:                # 이름이 플러그인과 다르면 plugin: 명시
    plugin: compose
    order: 30
    files: [docker-compose.staging.yml]
```

지원 플러그인: `compose`, `kubectl`, `helm`, `kustomize`, `tilt`, `skaffold`, `podman-compose`, `process`, `script`, `docker`, `vagrant`, `sam`, `serverless`, `multipass`

### 기타 설정

- **Modes** (`--mode/-M`): 운영 모드별 compose profiles + 서비스 필터 + 환경변수 + stack 엔트리 필터
- **Environments** (`--env/-E`): 환경변수 프리셋
- **Tags** (`--tags/-T`): 태그 기반 특정 서비스 그룹 필터링 (`--tag` 별칭 지원)
- **Health Checks**: 비-compose 서비스 상태 확인 및 자동 시작
- **Subprojects**: 모노레포 서브프로젝트 참조 (`dva api:test`)
- **Modules**: `.sb/dva/*.yml` 파일로 설정 분리
- **Override**: `dva.override.yml`로 로컬 오버라이드

상세 설정 가이드: **[USAGE.md](USAGE.md)**

## LLM Integration

```bash
dva config init --ai         # Claude Code CLI로 dva.yml 자동 생성
dva config init -p           # LLM용 프롬프트 출력
dva config improve --print   # 기존 dva.yml 개선용 프롬프트 출력
dva manifest                 # 구조화된 커맨드 매니페스트
dva config show              # 병합된 최종 설정 출력
```

- **`claude-plugin/`**: Claude Code 플러그인 (`claude --plugin-dir ./claude-plugin`)
- **Cursor**: `.cursor/rules/dva.mdc`
- **Antigravity**: `skills/dva/SKILL.md`

## Development

```bash
make build      # Build → ./bin/dva
make test       # Run tests
make lint       # Run linters
make fmt        # Format code
make clean      # Clean build artifacts
```

## License

MIT
