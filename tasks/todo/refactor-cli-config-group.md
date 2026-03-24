---
id: REFACTOR-001
title: "dva config 그룹으로 CLI 구조 통합"
type: refactor
priority: P2
effort: M
status: todo
created: 2026-03-25
---

## Goal

현재 `dva init`에 혼재된 AI/개선 관련 플래그들과, 최상위에 분산된 설정 관련 커맨드들을
`dva config` 서브커맨드 그룹으로 통합하여 CLI UX를 개선한다.

## 문제 상황

- `dva init --improve-prompt`, `dva init --ai-docs`는 "생성" 이 아닌 "유지보수" 목적인데 `init`에 있어 어색
- `dva config dump`는 파일 출력이 아닌 화면 출력인데 `dump`라는 이름이 부정확
- `dva validate`는 설정 파일 전용인데 최상위에 있어 과대 포장됨
- 유지보수/개선 커맨드의 기본 동작이 "프린트"이고 실행이 오히려 플래그라 inconsistent

## 변경 설계

### 최종 커맨드 구조

```
dva config
  ├── show          (기존 dump 대체: 머지된 설정 전체를 JSON/YAML 출력)
  │     -f yaml/json
  ├── init          (기존 최상위 dva init 이동)
  │     -t / --template
  │     --devcontainer / --all
  │     --prompt    (초기 도입용 프롬프트 출력)
  │     --ai        (Claude CLI로 dva.yml 자동 생성)
  │     --no-ai-docs
  │     --verbose
  ├── validate      (기존 최상위 dva validate 이동)
  │     --fix
  │     --strict
  └── improve       (신설: 기존 init --improve-prompt + init --ai-docs 통합)
        기본 동작    → Claude CLI로 즉시 개선 실행  ← init --ai 와 같은 패턴
        --print      → 프롬프트만 stdout 출력 (수동 붙여넣기용)
        --docs-only  → CLAUDE.md/AGENTS.md만 갱신 (기존 --ai-docs)
        --verbose
```

### 기존 커맨드 처리 방침

| 기존 커맨드 | 처리 |
|------------|------|
| `dva init` | `dva config init` alias로 유지 (Breaking Change 방지) |
| `dva validate` | **alias 없이 제거** (설정파일 전용이므로 config 하위로만) |
| `dva config dump` | `dva config show`로 리네임, `dump`는 deprecated alias |

> `dva validate`는 최상위에서 제거. `dva config validate`로 이동.
> CI 스크립트 사용 사례가 있으나, 설정파일 전용이라는 의미 명확화 우선.

## 구현 가이드

### 파일별 작업

| 파일 | 작업 |
|------|------|
| `internal/cli/config_dump.go` | `dump` → `show`로 커맨드명 변경, deprecated alias 추가 |
| `internal/cli/validate.go` | `validateCmd`를 `configCmd.AddCommand()`로 이동, `rootCmd.AddCommand()` 제거 |
| `internal/cli/init.go` | `--improve-prompt`, `--ai-docs`, `--no-ai-docs` 플래그 제거; `initCmd`를 `configCmd.AddCommand()`로 이동; `rootCmd`에는 alias만 |
| `internal/cli/config_improve.go` | 신설: `improve` 서브커맨드. `buildImprovePrompt()`, `generateAIDocs()`, Claude 실행 로직 이동 |
| `internal/cli/root.go` | `validateCmd` 등록 제거 |
| `internal/cli/improve_prompt_template.txt` | 메시지 내 `dva init --improve-prompt` → `dva config improve --print` 로 수정 |
| `internal/cli/validate.go` L44 | 힌트 메시지 `dva init --improve-prompt` → `dva config improve --print` 로 수정 |
| `README.md`, `USAGE.md`, `CHANGELOG.md` | 커맨드 참조 업데이트 |

### `config_improve.go` 설계 핵심

```go
// 기본 동작: Claude CLI 실행 (init --ai와 동일 패턴)
// --print: 프롬프트를 stdout만 출력
// --docs-only: CLAUDE.md/AGENTS.md만 갱신
var improveCmd = &cobra.Command{
    Use:   "improve",
    Short: "Review and improve the current dva.yml via AI",
    RunE: func(cmd *cobra.Command, args []string) error {
        if improvePrint {
            return generateAndPrintImprovePrompt()  // 기존 init.go 함수 이동
        }
        if improveDocsOnly {
            return runAIDocsOnly()  // 기존 init.go 함수 이동
        }
        return runAIImprove()  // Claude CLI 실행 (신규, runAIInit 참고)
    },
}
```

## Acceptance Criteria

- [ ] `dva config show` 가 기존 `dva config dump`와 동일한 출력을 냄
- [ ] `dva config dump` 실행 시 deprecated 경고 후 동일 동작
- [ ] `dva config validate` 가 기존 `dva validate`와 동일하게 동작
- [ ] `dva validate` 실행 시 "command not found" 또는 deprecated 안내 출력
- [ ] `dva config improve` (플래그 없음) 가 Claude CLI로 개선 실행
- [ ] `dva config improve --print` 가 프롬프트를 stdout 출력
- [ ] `dva config improve --docs-only` 가 CLAUDE.md/AGENTS.md 갱신
- [ ] `dva init` alias가 `dva config init`과 동일하게 동작
- [ ] 기존 단위 테스트 전체 통과
- [ ] README, USAGE.md 커맨드 참조 업데이트 완료
