# DVA Plugin for Claude Code

DVA (Dev Virtual Auto) CLI integration — Claude Code가 DVA를 통해 컨테이너 운영, 빌드, 테스트, 인프라 관리를 수행하도록 안내하는 플러그인.

## 설치

로컬 개발 시:

```bash
claude --plugin-dir ./claude-plugin
```

## 요구사항

- DVA CLI (`dva` 바이너리가 PATH에 존재해야 함)
- `dva.yml` 설정 파일이 프로젝트 루트에 존재

## 핵심 개념

**DVA는 `dva.yml` 기반으로 동작합니다.** Docker Compose, Kubernetes, 로컬 실행 등을 추상화하여 단일 명령어로 제공합니다.

```bash
dva manifest -f json   # 사용 가능한 명령 확인 (LLM 최적화)
dva show               # 모드, 환경, 명령어 요약
dva up                 # 서비스 시작
dva test               # 테스트 실행
dva build              # 빌드
dva down               # 서비스 중지 및 제거
dva doctor             # 환경 진단
am run dva-improve     # AI 기반 dva.yml 개선
```

## Skills

| Skill | 경로 | 역할 |
|-------|------|------|
| **dva** | `skills/dva/SKILL.md` | DVA CLI 실행 — build/test/run/lifecycle, 설정 작성 패턴, 반복 작업 템플릿 |
| **config** | `skills/config/SKILL.md` | dva.yml 저작·감사·마이그레이션·진단 (ownership 모델, 변경 모드, 위험순 검증). 자동 트리거 |

`dva` skill은 Claude Code가 raw docker/compose/kubectl 대신 DVA를 사용하도록 강제하고,
`config` skill은 dva.yml 설정 작업 시 자동으로 트리거됩니다 (`user-invocable: false`).

### 스킬 구조

```
skills/
├── dva/                      # CLI 실행
│   ├── SKILL.md              # 핵심 규칙 + 워크플로우
│   ├── assets/
│   │   └── templates/
│   │       ├── root-devbox-plan.yml
│   │       ├── subproject-local.yml
│   │       └── migrate-modes-to-plans.yml
│   └── references/
│       ├── commands.md       # 전체 명령어 레퍼런스
│       ├── advanced.md       # 모드/환경/서브프로젝트/설정 패턴
│       └── patterns.md       # 표준 구조, 마이그레이션, 검증 게이트
└── config/                   # 설정 저작·진단 (canonical)
    ├── SKILL.md              # ownership 모델 + 변경 모드 + 위험순 검증
    └── README.md
```
