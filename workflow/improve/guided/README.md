# DVA Improve: Guided Pipeline

사용자 프로젝트에 DVA를 도입하고, 기존 인프라를 DVA 표준으로 마이그레이션하는 5단계 파이프라인.

## Tracks

Stage 00에서 대상 프로젝트의 compose 파일 유무를 감지하여 자동으로 트랙을 결정합니다.

| Track | 조건 | Stage 30 | 설명 |
|-------|------|----------|------|
| **full** | compose 파일 없음 | `30-configure-full.md` | compose.yml + .env.example + dva.yml 전부 신규 생성 |
| **adopt** | compose 파일 존재 | `30-configure-adopt.md` | 기존 compose 유지, dva.yml만 생성 |

## Stages

| # | Stage | File | Description |
|---|-------|------|-------------|
| 00 | Analyze | stages/00-analyze.md | 대상 프로젝트 탐색, compose/구조 패턴 분석, 트랙 결정 |
| 10 | Verify | stages/10-verify.md | 분석 기반 DVA 구조 제안, 사용자 확인 대기 |
| 20 | Transform | stages/20-transform.md | 디렉토리/파일을 표준 구조로 변환 |
| 30 | Configure (full) | stages/30-configure-full.md | compose.yml + dva.yml 신규 생성 |
| 30 | Configure (adopt) | stages/30-configure-adopt.md | 기존 compose 기반 dva.yml만 생성 |
| 40 | Execute | stages/40-execute.md | dva up, 헬스 확인 |

## Entry Points

- `entry.md` — step mode (단계별 확인, 첫 도입 권장)
- `auto.md` — auto mode (전체 파이프라인 자동 실행)

## Usage

```
# Step mode (recommended for first run)
Read and execute: workflow/improve/guided/entry.md

# Auto mode
Read and execute: workflow/improve/guided/orchestrator.md
```

## References

- `library/dva-schema.md` — dva.yml 스키마 레퍼런스
- `library/naming-presets.md` — mode/env/tag 네이밍 프리셋
- `verify/checklist.md` — 파이프라인 완료 후 검증 체크리스트
- `templates/state.template.yaml` — 파이프라인 상태 추적 템플릿
