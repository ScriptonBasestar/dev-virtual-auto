# ARCHITECTURE.md: DVA 설계 구조

이 문서는 DVA의 구현 경계, 설정 해석 과정, 실행 책임을 정의한다.

## 전체 구조

DVA는 선언된 설정을 바로 실행하지 않는다. 설정 레이어를 병합하고 검증한 뒤 이름 있는
실행 항목을 불변 계획으로 해석하고, 각 엔트리를 해당 runner에 위임한다.

```text
dva.yml
  + .sb/dva/*.yml modules
  + dva.override.yml
  + active subprojects
          │
          ▼
Config Loader → Schema/Semantic Validation → Resolver
                                               │
                                               ▼
                                     Immutable ExecutionPlan
                                               │
                                               ▼
                                          Orchestrator
                                               │
                    ┌──────────────────────────┼─────────────────────────┐
                    ▼                          ▼                         ▼
              Compose runner            Cluster runners           Local runner
                    │                          │                         │
                    └──────────── external tools and processes ─────────┘
```

## 역할 분담

각 계층은 설정 해석, 실행 계획, backend 위임을 분리해 소유한다.

### CLI adapter

- `cmd/dva/`는 최소 진입점만 제공한다.
- `internal/cli/`는 Cobra 명령, 동적 interaction 라우팅, 출력 형식을 담당한다.
- CLI는 설정을 직접 해석하거나 backend 명령을 조립하지 않는다.

### Configuration

- `internal/config/`는 `dva.yml`, modules, override, subproject를 로드하고 병합한다.
- `internal/config/schema.json`은 허용되는 설정 구조의 계약이다.
- 구조 검증과 실행 전 semantic warning을 제공한다.

### Resolution and orchestration

- `internal/lifecycle/`는 이름 있는 실행 대상을 불변 `ExecutionPlan`으로 해석한다.
- `order`와 `depends_on`으로 실행 wave를 계산한다.
- teardown은 의존 관계의 역순으로 수행한다.
- legacy app lifecycle은 새 계획 모델로 이동 중인 호환 영역이다.

### Runners and execution

- `internal/runner/`는 Compose, Kubectl, Local interaction을 backend 동작으로 변환한다.
- `internal/exec/`는 외부 프로세스 실행과 process replacement를 담당한다.
- 외부 도구의 리소스 의미를 DVA가 다시 구현하지 않는다.

의존 방향은 CLI에서 설정·수명 주기 계약을 거쳐 runner와 외부 실행으로 향한다.

## Source of Truth

| Concern | Canonical Source |
|---------|------------------|
| 사용자 프로젝트 선언 | `dva.yml`과 활성 module/override |
| 허용 설정 구조 | `internal/config/schema.json` |
| 병합과 interpolation 의미 | `internal/config/` 및 merge semantics 문서 |
| 실행 계획 해석 | `internal/lifecycle/` 및 resolution 문서 |
| 외부 명령 실행 | `internal/runner/`, `internal/exec/` |
| 제품 철학과 범위 | `SOUL.md`, `PRODUCT.md` |
| 에이전트 작업 규칙 | `AGENTS.md`, `CLAUDE.md` |

문서가 구현과 충돌하면 코드와 schema를 먼저 확인하고, 의도된 계약이 바뀐 경우 관련
문서를 함께 갱신한다.

## 핵심 도메인 경계

| Concept | Responsibility | Must Not Own |
|---------|----------------|--------------|
| `stack` | 재사용 가능한 logical unit과 runner 선언 | 이번 실행의 최종 선택과 순서 |
| `plans` | 사용자가 실행하는 이름과 entry 선택 | runner 원본 설정의 중복 |
| `environments` | dev/stg/prd 같은 용도별 변수 차이 | 실행 host 선택 |
| `sites` | local/office/remote/cloud 같은 host 차이 | 애플리케이션 환경 의미 |
| `interaction` | 반복 가능한 단발성 프로젝트 명령 | 서비스 수명 주기 |
| `provision` | 한 번 수행하는 준비·초기화 | 계획을 대신하는 반복 startup |

## 설정 병합 흐름

```text
base dva.yml
    → modules in .sb/dva/
    → dva.override.yml
    → environment/site/plan variables
    → subproject namespace resolution
    → schema and semantic validation
    → effective config
```

- Map section은 key 단위로 병합한다.
- List와 scalar는 뒤 레이어가 교체한다.
- runner나 plugin처럼 구조를 결정하는 필드는 호환되지 않는 override를 거부한다.
- 최종 동작은 원본 파일 하나가 아니라 병합된 effective config를 기준으로 판단한다.

## 실행 데이터 흐름

Lifecycle, interaction, agent discovery는 같은 effective config를 서로 다른 목적으로 사용한다.

### Named lifecycle

```text
dva up/down/stop/status <name>
    → config load and validation
    → plan/environment/site resolution
    → immutable ExecutionPlan
    → dependency waves
    → runner execution
    → structured status and errors
```

### Interaction

```text
dva <interaction> or dva run <interaction>
    → built-in command check
    → interaction tree and subproject namespace resolution
    → owning project directory
    → selected runner
    → external command
```

### Agent discovery

```text
dva manifest / dva ls / dva config show
    → effective project configuration
    → discoverable commands and runners
    → human or agent selects an existing operation
```

에이전트는 manifest에 없는 명령을 추측하지 않으며, mutation 전에 validate와 doctor의
읽기 전용 결과를 우선한다.

## 상세 문서

- [Configuration Merge Semantics](docs/30-config-merge-semantics.md)
- [Execution Plan Resolution](docs/31-execution-plan-resolution.md)
- [Declarative Stack and Plans](docs/40-declarative-stack-and-plans.md)
- [USAGE.md](USAGE.md) — CLI와 설정 레퍼런스
- [AGENTS.md](AGENTS.md) — 에이전트 작업 지침과 repository map
