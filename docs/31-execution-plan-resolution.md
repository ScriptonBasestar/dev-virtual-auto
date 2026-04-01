# Execution Plan Resolution For Mode And Environment

이 문서는 DVA 환경 파라미터(`mode`, `env`, `tags` 등)가 실행 계획(Execution Plan)으로 결합되고 해석되는 원칙과 책임 경계를 정의합니다.
이를 통해 향후 `internal/lifecycle` 리팩터링 및 `dva plan` 명령과 같은 기능 확장을 원활하게 합니다.

## 1. 해석 순서 (Resolution Order)

모든 실행 명령(`dva up`, `dva down`, `dva restart`, ...)은 다음 순서로 해석되어야 합니다.

### 1-1. 모드 (`mode`) 결정 우선순위
1. CLI 플래그: `--mode`, `-M`
2. Config 기본값: `dva.yml` 내의 `default_mode` 명시 값. (명시된 값이 없으면 모드 미적용)

### 1-2. 환경 (`env`) 결정 우선순위
1. CLI 플래그: `--env`, `-E` (여러 개 지정 불가, 단일 값)
2. 환경 미지정 시: 베이스 `dva.yml` 및 `dva.override.yml`에 정의된 기본 동작 수행

### 1-3. 환경 변수 (Environment Variable) 합병 우선순위
동일한 키의 환경 변수가 충돌할 경우 위에서 아래 순으로 덮어씁니다. (아래가 가장 높은 우선순위)
1. **호스트 OS Env**: DVA를 실행하는 현재 프로세스의 쉘 환경.
2. **`env_file`**: `dva.yml`에 선언된 공통 `.env` 파일들의 내용. (선언된 배열 순서대로 합병)
3. **기본 env (`environment`)**: `dva.yml` 최상위에 선언된 기본 환경 변수 블록.
4. **환경별 env (`environments[env].environment`)**: `--env`로 활성화된 특정 환경의 변수 모음.
5. **모드별 env (`modes[mode].environment`)**: 활성화된 `mode`에 종속적인 변수 모음 (실행 전략에 직접적인 영향을 주기 때문에 환경보다 더 높은 우선순위 부여).

### 1-4. 스택 엔트리 필터링 (`Lifecycle` Filtering)
`Stack` 필터링은 다음 조건들을 모두 평가한 교집합(`AND`) 형태로 적용됩니다. `excludeTags`는 나중에 차감(-)되어 최종 엔트리를 결정합니다.
1. 명시적 대상(Names): 명령어 인자로 들어온 특정 컴포넌트 이름 목록.
2. 환경 스택: `--env`에서 지정한 `stack` 목록.
3. 모드 스택: `--mode`에서 지정한 `stack` 목록.
4. 포함 태그(`includeTags`): `--tag`를 만족하는 엔트리.
5. 제외 태그(`excludeTags`): `--exclude-tag`에 포함되면 무조건 최종 목록에서 **제거**.
6. **동적 오버라이드 매핑 (`stack_overrides`)**: 필터링을 통과한 엔트리에 한해서, 활성화된 `--env`의 `stack_overrides` 내용을 Deep Merge하여 플러그인 속성값을 교체합니다.

> **💡 주의**: `stack_overrides` 적용 시 `plugin` 타입 속성의 변경은 아키텍처 상 치명적 오류를 유발하므로 런타임 이전에 검출되어 중단되어야 합니다(Fail-Fast).

### 1-5. 애플리케이션 필터링 (`Application` Filtering)
어플리케이션이 구동될 Strategy("native" | "docker")는 다음 우선순위를 갖습니다.
1. `--docker` 등의 CLI 명시적인 전역 Flag Strategy.
2. `--mode`에 정의된 개별 어플리케이션 Strategy (`modes[mode].app_strategy[name]`).
3. App 기본 설정: `native`가 기본값. (`devMode` 플래그에 따라 `run` 또는 `dev` 명령어 경로 선택).

---

## 2. 책임 경계 (Responsibility Boundaries)

기존 파편화된 파싱 로직을 모으고, CLI와 Orchestrator 간 책임을 완벽히 분리합니다.

### 2-1. CLI Layer (`internal/cli`)
사용자와 상호작용 및 플래그 파싱에 집중합니다.
- 사용자의 명령줄 인자(`--mode`, `--env`, `--tag`, `--exclude-tag` 등)를 파싱합니다.
- 참조된 `mode`나 `env`가 `dva.yml`에 존재하지 않으면 **입력 검증(Validation)** 단계에서 즉시 안내 메시지를 출력하고 종료합니다.
- 모든 컨텍스트가 수집되면, `Resolver`를 호출하여 결정론적이고 불변(Immutable) 상태인 `ExecutionPlan` 객체를 생성합니다.
- `ExecutionPlan` 객체를 `internal/lifecycle`의 Orchestrator/AppManager로 전달하여 실행을 요구합니다.

### 2-2. Orchestrator Layer (`internal/lifecycle`)
계산된 계획을 단순 수행하는 역할에 집중합니다.
- 더 이상 환경 변수를 동적으로 찾아 참조하거나, `filterEntries` 등의 긴 필터 판단 알고리즘을 소유하지 않습니다.
- 전달받은 `ExecutionPlan` 구조체 내에 정리된 `StackEntries`, `EnvVars`, `Applications` 정보만 순회하며 단순 실행 및 생명주기를 관리합니다.

---

## 3. ExecutionPlan 구조체 초안

향후 `dva plan` 등의 플러그인 시뮬레이션 및 Explain 출력을 위한 `internal/config` 또는 `internal/lifecycle/resolver` 모델의 초안입니다.

```go
package lifecycle

import "github.com/ScriptonBasestar/dva/internal/config"

// ExecutionPlanBuilder resolves configuration and flags into a distinct, immutable execution plan.
// It removes the need for lifecycle packages to parse raw input strings.
type ExecutionPlan struct {
	// 1. Meta Context
	Mode       *config.ModeConfig
	EnvProfile *config.EnvironmentProfile

	// 2. Resolved Environment
	// All merged OS + File + Config + Overrides mapping.
	EnvVars map[string]string

	// 3. Determined Stacks
	// The exact ordered list of stack entries to spin up/down,
	// with all their `stack_overrides` deep-merged properly.
	StackEntries []config.LifecycleEntry

	// 4. Determined Applications
	// The final strategies resolved to start targeted applications.
	Applications []AppExecution

	// 5. Audit Log (Optional)
	// A human-readable step-by-step resolution trace for 'dva plan' debugging.
	ResolutionTrace []string
}

// AppExecution pairs an Application with its final resolved execution strategy.
type AppExecution struct {
	App      *config.ApplicationConfig
	Strategy string
	Command  string
}
```

### 3-1. Resolver 구상
CLI는 다음과 같이 하나의 호출로 ExecutionPlan을 도출하게 됩니다.

```go
plan, err := lifecycle.ResolveExecutionPlan(cfg, env, lifecycle.ResolveOptions{
    Mode:        "lite",
    Env:         "stg",
    IncludeTags: []string{"infra"},
    ExcludeTags: []string{"cache"},
    TargetNames: []string{}, // all matched
    GlobalStrategy: "docker",
})
if err != nil {
    return fmt.Errorf("invalid execution plan requested: %w", err)
}
```

이 규칙과 기반 아키텍처 문서는 이후의 모든 플로우 통폐합에 있어서 필수적 기초 자료로 활용됩니다.
