# DVA (Docker Virtual Auto)

Docker Compose / Kubernetes CLI wrapper — `dva.yml` 설정 파일로 복잡한 명령어를 간단하게.

> Hip CLI의 Go 재작성 버전 (v10+).

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

compose:
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
dva ls          # 사용 가능한 커맨드 목록
dva shell       # = dva run shell → docker compose run app /bin/bash
dva test        # = dva run test → docker compose run app bundle exec rspec
dva up          # docker compose up -d --wait
dva down        # docker compose down --remove-orphans
dva validate    # dva.yml 스키마 검증
dva manifest    # LLM용 전체 커맨드 매니페스트 출력
```

## Commands

```bash
dva ls              # 사용 가능한 커맨드 목록
dva shell           # = dva run shell (run 생략 가능)
dva up              # 서비스 시작 (-d --wait)
dva up -M backend   # 프로필 모드 적용
dva down            # 서비스 중지 및 제거
dva status          # 워크스페이스 상태 확인
dva show            # 설정 요약
dva validate        # dva.yml 검증
dva provision       # 프로비저닝 실행
dva init --ai       # AI로 dva.yml 자동 생성
```

전체 커맨드 레퍼런스: **[USAGE.md](USAGE.md)**

## Configuration

- **Profiles** (`--mode/-M`): 운영 모드별 compose profiles + 서비스 필터 + 환경변수
- **Environments** (`--env/-E`): 환경변수 프리셋
- **Health Checks**: 비-compose 서비스 상태 확인 및 자동 시작
- **Subprojects**: 모노레포 서브프로젝트 참조 (`dva api:test`)
- **Modules**: `.dva/*.yml` 파일로 설정 분리
- **Override**: `dva.override.yml`로 로컬 오버라이드

상세 설정 가이드: **[USAGE.md](USAGE.md)**

## LLM Integration

```bash
dva init --ai         # Claude Code CLI로 dva.yml 자동 생성
dva init -p           # LLM용 프롬프트 출력
dva manifest          # 구조화된 커맨드 매니페스트
dva config dump       # 병합된 최종 설정 출력
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
