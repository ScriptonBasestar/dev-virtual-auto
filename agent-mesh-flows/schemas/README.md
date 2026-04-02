---
title: Agent Mesh V2 JSON Schemas
version: 2.0.0
date: 2026-03-10
status: Stable
---

# Agent Mesh V2 JSON Schemas

`schemas/` 디렉토리는 Agent Mesh V2 Flow Pipeline의 JSON Schema 파일들을 관리합니다.  
`internal/engine/types.go` Go 구조체와 1:1 대응되며, IDE 자동완성·유효성 검증·문서 역할을 겸합니다.

## 스키마 파일 목록

| 파일 | `$id` | 대상 Go 타입 | 대상 YAML |
|------|------|------------|---------|
| `flow.schema.json` | `agent-mesh-flow-v2` | `engine.Pipeline` | `flows/**/*.yaml` |
| `profile.schema.json` | `agent-mesh-profile-v2` | `map[string]engine.Profile` | `flows/shared/profiles/*.yaml` |

> 나머지 파일(`run.schema.json`, `step.schema.json`, `artifact.schema.json` 등)은 V1 시대 유산으로 현재 코드베이스에서 사용되지 않습니다. 필요 시 삭제 가능합니다.

## 스키마 구조 — `flow.schema.json`

`engine.Pipeline` 구조체 기반의 핵심 스키마입니다.

### 최상위 Pipeline 필드

| 필드 | 타입 | 필수 | 설명 |
|------|------|-----|------|
| `name` | string | — | 파이프라인 식별자 (파일명 기본) |
| `description` | string | — | 파이프라인 설명 |
| `executor` | string | — | 전체 Step의 기본 executor (`micro`/`fast`/`smart`/`deep`, provider, tool, named) |
| `imports` | object | — | 외부 profile 파일 import 설정 |
| `parameters` | array | — | 실행 시 입력 파라미터 목록 |
| `variables` | array | — | 파이프라인 상수/변수 목록 |
| `profiles` | object | — | 재사용 가능한 LLM 설정 묶음 |
| `default_profile` | string | — | 모든 Step에 적용되는 기본 프로파일 이름 |
| `interactive` | object | — | Interactive 모드 설정 (`confirm_before`, `auto_decide`) |
| `shell_policy` | object | — | Shell 명령 실행 제한 설정 (`mode`, `allow`, `deny`) |
| `pipeline_role` | string | — | 파이프라인 역할 제한 (`orchestrator`/`worker`/`analyzer`) |
| `mcp_config` | string | — | 전체 LLM Step의 기본 MCP 설정 (파일경로 또는 프로필명) |
| `steps` | array | **✅** | 실행 단위 목록 |

### Step 필드

| 필드 | 타입 | 설명 |
|------|------|------|
| `id` | string | 고유 식별자 (필수, `^[a-z][a-z0-9_-]*$`) |
| `type` | string | Step 실행 유형 (필수): `shell`/`llm`/`pipeline`/`context`/`file`/`apply`/`frontmatter`/`move`/`read_file`/`worktree` |
| `executor` | string \| string[] | Executor 지정. 배열이면 순서대로 fallback |
| `profile` | string | 적용할 Profile 이름 |
| `depends_on` | string[] | 의존 Step ID 목록 |
| `context` | object | `key: shell_command` 맵. 결과는 `{{id.key}}`로 참조 |
| `system_prompt` | string | 시스템 프롬프트 (파일명 또는 리터럴) |
| `role` | string | 역할 텍스트. `system_prompt` 앞에 삽입 |
| `instruction` | string | 실제 사용자 지시. `{{...}}` 보간 지원 |
| `action` | string | LLM 응답 후 실행할 Shell 명령 (`{{output}}` 사용 가능) |
| `exit_if_empty` | string | 해당 context key가 비어있으면 파이프라인 성공 종료 |
| `when` | string | 조건식. false면 Step 스킵 |
| `ignore_error` | boolean | true면 Step 실패해도 파이프라인 계속 |
| `temperature` | number (0-2) | LLM 창의성 (기본: 0.2) |
| `json_mode` | boolean | LLM JSON 출력 강제 |
| `cwd` | string | Shell 명령의 작업 디렉토리 |
| `timeout_seconds` | integer | LLM/Shell 타임아웃 |
| `loop` | object | Foreach 반복 설정 (`items`, `item_var`, `max_iterations`) |
| `parallel` | boolean | `run_pipeline`과 함께 사용 시 병렬 실행 |
| `run_pipeline` | string | 다른 파이프라인 파일 위임 호출 |
| `pipeline_params` | object | Sub-pipeline에 전달할 파라미터 (`{{loop.file}}` 등 보간 지원) |
| `claude_code` | object | Claude Code CLI 설정 (`lightweight`, `permission_mode`, `tools`, `backend` 등) |
| `gemini` | object | Gemini CLI 설정 (`approval_mode`, `sandbox`, `model` 등) |
| `file` | object | 파일 연산 설정 (type: file용) |
| `apply` | object | LLM 출력 → 파일 수정 설정 (type: apply용) |
| `frontmatter` | object | YAML Frontmatter 조작 설정 (type: frontmatter용) |
| `src` | string | 소스 파일 경로 (type: move, read_file용) |
| `dest_dir` | string | 대상 디렉토리 (type: move용) |
| `worktree` | object | Git worktree 라이프사이클 설정 (type: worktree용) |
| `on_error` | object | Step 실패 시 정리 작업 (frontmatter, move) |

## IDE 통합

### 방법 1: 인라인 주석 (권장 — 에디터 무관)

모든 Flow YAML 최상단:

```yaml
# yaml-language-server: $schema=../../schemas/flow.schema.json
name: my-flow
```

경로는 YAML 파일 위치에 따라 조정합니다:
- `flows/dev/*.yaml` → `../../schemas/flow.schema.json`
- `flows/task/*.yaml` → `../../schemas/flow.schema.json`
- `flows/shared/profiles/*.yaml` → `../../../schemas/profile.schema.json`

### 방법 2: VSCode settings.json

```json
{
  "yaml.schemas": {
    "./schemas/flow.schema.json": ["flows/**/*.yaml"],
    "./schemas/profile.schema.json": ["flows/shared/profiles/*.yaml"]
  }
}
```

Red Hat YAML 확장 (`redhat.vscode-yaml`) 필요.

### 방법 3: `am run validate --fix` (자동 삽입)

```bash
am run validate flows/my-flow.yaml --fix
# 🔧 Injected: # yaml-language-server: $schema=../../schemas/flow.schema.json
```

## 글로벌 설치 시 배포

`am flow install` 실행 시 스키마 파일이 `~/.agent-mesh/schemas/`에 자동 배포되고,  
설치된 YAML의 `$schema` 경로가 `../schemas/flow.schema.json`으로 자동 재작성됩니다.

```bash
am flow install ../flows
# 📋 flow.schema.json → ~/.agent-mesh/schemas/flow.schema.json
# ok  commit.yaml → ~/.agent-mesh/flows/commit.yaml  (경로 재작성됨)
```

## CLI 검증

```bash
# am run validate (권장, 스키마 주석도 체크)
am run validate flows/dev/commit.yaml
am run validate flows/dev/commit.yaml --fix  # 스키마 주석 자동 삽입

# npx ajv-cli (JSON Schema 직접 검증)
npx ajv validate -s schemas/flow.schema.json -d flows/dev/commit.yaml

# check-jsonschema (Python 기반)
check-jsonschema --schemafile schemas/flow.schema.json flows/dev/commit.yaml
```

## 스키마 업데이트

`internal/engine/types.go`의 구조체 변경 시 스키마 파일도 수동 업데이트합니다.

```
types.go 변경  →  schemas/flow.schema.json 수동 업데이트  →  커밋
```

> 향후 `go generate` 태그를 통한 자동 생성 고려 중.

## SchemaStore 등록 (오픈소스화 시)

[SchemaStore.org](https://www.schemastore.org/) 등록 시 에디터가 파일명 패턴만으로 스키마를 자동 인식합니다.

사전 조건 및 PR 절차: [ADR-0024](../docs/90-decisions/ADR-0024-schema-hints-and-schemastore.md)

## 관련 문서

- [docs/50-workflows/05-ide-schema-hints.md](../docs/50-workflows/05-ide-schema-hints.md) — 에디터 설정 가이드
- [docs/90-decisions/ADR-0024-schema-hints-and-schemastore.md](../docs/90-decisions/ADR-0024-schema-hints-and-schemastore.md) — 스키마 전략 ADR
- [docs/20-system/03-data-model.md](../docs/20-system/03-data-model.md) — Pipeline/Step 데이터 모델
