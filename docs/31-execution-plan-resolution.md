# Execution Plan Resolution

이 문서는 DVA의 새 실행 해석 모델을 정의합니다.
핵심은 `stack`을 선언 저장소로 유지하고, 실제 실행은 이름 있는 실행 계획을 통해 수행하는 것입니다.

관련 배경과 용어는 [40-declarative-stack-and-plans.md](40-declarative-stack-and-plans.md)를 기준 문서로 삼습니다.
실행 계획·CLI 상세는 [41-execution-plans-and-cli.md](41-execution-plans-and-cli.md)를 봅니다.

## 1. 목표

새 해석 모델의 목적은 아래와 같습니다.

- 선언과 실행 계획을 분리
- `mode`에 섞여 있던 책임을 분해
- runner 다양성을 유지하면서 예측 가능성 확보
- CLI와 runtime 레이어의 책임 분리

## 2. 실행 대상

실행 명령의 직접 대상은 `stack`이 아닙니다.
실행 명령의 대상은 이름 있는 실행 계획입니다.

예:

```bash
dva up local-dev
dva down local-dev
dva stop local-dev
dva status local-dev
```

여기서 `local-dev`는 `plans.<name>`에 해당하는 논리적 실행 이름입니다.

## 3. 입력 모델

실행 계획 해석에 사용되는 주요 입력은 아래와 같습니다.

- `stack`
- `plans`
- `environments`
- `sites`
- `env_file`
- 전역 `vars`
- CLI override

보조 레이어:

- `subprojects`
- `interactions`
- `provision`

## 4. 해석 순서

모든 실행 명령은 아래 순서로 해석되어야 합니다.

### 4-1. 실행 이름 결정

1. CLI 인자로 `<name>`을 받음
2. `plans.<name>` 또는 import된 canonical name을 조회
3. 없으면 즉시 validation error
4. 예외: plan이 정확히 하나일 때 이름 없는 `dva up`/`down`/`stop`/`restart`/`status`는 그 plan을 기본 선택한다. 앞에 플래그가 오면 기본 선택을 조용히 건너뛰지 않고, plan 이름을 명시하라고 오류를 낸다 (`dva up p1 --dev`).

예:

- `local-dev`
- `backend/local-dev`

### 4-2. plan 본문 로드

선택된 plan에서 아래를 로드합니다.

- `environment`
- `site`
- `vars`
- `entries`

### 4-3. vars 병합

같은 키가 충돌하면 뒤의 값이 우선합니다.

1. `env_file`
2. 전역 `vars`
3. `environments.<name>.environment`
4. `sites.<name>.vars`
5. `plans.<name>.vars`
6. CLI 일회성 override (`--var`)
7. OS 환경 변수

즉:

```text
env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS 환경 변수
```

OS 환경 변수가 가장 높은 우선순위입니다. 같은 키가 OS에 설정되어 있으면 `dva.yml`의
어떤 레이어(`--var` 포함)도 그 값을 덮어쓰지 못합니다 (`internal/config/environment.go`
`MergeVars`가 해석된 map 위에 OS 값을 적용).

레이어 1과 2는 `ResolvePlan`이 아니라 config 로드 단계(`internal/cli/root.go` `loadEnv`)가
적용하며, plan이 만든 map은 그 위에 병합됩니다(`e.MergeVars(plan.EnvVars)`). 두 단계가
합쳐져 위 순서를 만듭니다. 특정 실행에서 각 레이어가 실제로 무엇을 얹었는지는
`dva up <plan> --dry-run`이 출력합니다 — 사용법은
[USAGE.md](../USAGE.md#실제-적용-결과-확인)를 참조하세요.

### 4-4. stack entry 매핑

plan의 각 `entries[].name`은 `stack.<name>` 선언을 참조합니다.

이 단계에서:

- 존재하지 않는 stack 참조 검증
- runner별 설정 로드
- subproject import인 경우 source config dir 추적

### 4-5. runner 결정

각 plan entry는 최종 runner를 결정해야 합니다.

권장 우선순위:

1. `stack.<name>.default_runner`
2. `sites.<site>.entry_overrides.<name>.runner`
3. `plans.<plan>.entries[].runner`

같은 key가 있으면 뒤의 값이 우선합니다.
단, 최종 runner는 반드시 해당 `stack.<name>.runners` 안에 선언된 key여야 합니다.

즉:

```text
default_runner < site.entry_overrides.runner < plan.entries[].runner
```

정의되지 않은 runner 선택은 validation error입니다.

### 4-6. plan-level override 적용

실행 계획에서 선언 위에 덧씌울 수 있는 값은 아래로 제한합니다.

- `services`
- `order`
- `depends_on`
- startup 뒤 표시할 `endpoint_tags`
- runner 선택 override
- 추가 `vars`

원칙:

- 선언 자체의 정체성을 바꾸는 override는 제한
- 실행 계획에 필요한 선택과 순서만 허용

### 4-7. 순서 계산

최종 실행 순서는 아래 규칙으로 계산합니다.

1. `depends_on` 기반 DAG 생성
2. 순환 dependency 검증
3. 같은 레벨에서는 `order` 오름차순
4. 여전히 동률이면 이름 정렬
5. 독립 항목은 같은 wave에서 병렬 실행

`down` / `stop`은 역순으로 처리합니다.

## 5. compose 특별 규칙

`compose`는 묶음 단위 실행이 가능하므로 별도 규칙이 필요합니다.

`stack`의 compose 엔트리는 보통 compose 프로젝트 선언입니다.

예:

```yaml
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
          - docker-compose.dev.yml
```

plan에서는 아래를 선택할 수 있습니다.

- compose 전체 실행
- 특정 service subset만 실행

예:

```yaml
plans:
  local-dev:
    entries:
      - name: core-compose
        runner: compose
        services: [postgres, redis]
```

즉 compose 서비스 선택은 선언이 아니라 실행 계획 책임입니다.

## 6. subproject resolution

subproject에서 import된 실행 entrypoint는 canonical namespace 이름을 가집니다.

예:

- `backend/local-dev`
- `backend/shell`
- `backend/setup`

규칙:

- canonical name은 항상 `subproject/name`
- parent top-level로 자동 flatten 하지 않음
- alias는 명시적으로만 허용
- 충돌은 hard error
- `subprojects.<name>` 선언만으로 child `dva.yml`을 즉시 로드하지 않음 (`import` 생략 또는 `import: {}` 포함)
- `import`에 실제 대상이 있거나 `dva run <subproject>:<command>`처럼 직접 실행할 때는 해당 subproject의 `dva.yml`이 필요함
- imported plan의 canonical name과 alias는 같은 child effective config를 실행 owner로 사용
- child stack, environment, site, vars, env_file, lifecycle hooks, endpoints와 readiness를 parent 선언과 섞지 않음

## 7. subproject execution path

subproject의 imported `plans`, `interactions`와 `provision`은 parent 기준이 아니라, 해당
subproject 설정 파일이 있는 디렉터리 기준으로 실행합니다.

즉:

- command resolution 기준 = subproject config
- relative path 기준 = subproject config dir
- default working directory = subproject root
- runner/config 상대 경로와 process state 기준 = subproject root
- imported plan hook·endpoint·readiness 기준 = subproject config

이 원칙은 parent에서 import해도 subproject가 독립 실행될 때와 동일한 의미를 보장합니다.
Subproject `path` 자체는 absolute 또는 parent 밖의 `../` 위치일 수 있으며, import가 새 containment
제약을 추가하지 않습니다.

## 8. 책임 경계

### 8-1. CLI Layer

CLI는 아래를 담당합니다.

- 실행 이름 파싱
- `--var` 같은 명시적 override 수집
- 대상 이름 존재 여부 validation
- Resolver 호출

CLI는 더 이상 raw `stack` 필터링 로직을 직접 가지지 않습니다.

### 8-2. Resolver Layer

Resolver는 아래를 담당합니다.

- plan 로드
- environment/site/profile 확장
- vars 병합
- stack 참조 해석
- dependency/order 계산
- 실행 가능한 immutable plan 생성

### 8-3. Runtime Layer

runtime/orchestrator는 아래만 담당합니다.

- 전달받은 resolved entry 실행
- health check 수행
- reverse teardown 수행

runtime은 raw config 의미 해석을 소유하지 않습니다.

## 9. ExecutionPlan 초안

```go
package lifecycle

import "github.com/ScriptonBasestar/dva/internal/config"

type ExecutionPlan struct {
	Name string

	EnvironmentName string
	SiteName        string

	EnvVars map[string]string

	Entries []ResolvedEntry

	ResolutionTrace []string
}

type ResolvedEntry struct {
	Name       string
	Source     *config.LifecycleEntry
	Runner     string
	Order      int
	DependsOn  []string
	Services   []string
	WorkingDir string
}
```

## 10. 예시

```yaml
vars:
  LOG_FORMAT: text

env_file:
  - .env

stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml, docker-compose.dev.yml]

  api:
    default_runner: native
    runners:
      native:
        dir: apps/api
        run: go run ./cmd/api
      docker:
        image: myorg/api:dev
        run: docker run --rm myorg/api:dev

environments:
  dev:
    environment:
      APP_ENV: dev

sites:
  local:
    vars:
      DVA_SITE: local

plans:
  local-dev:
    environment: dev
    site: local
    vars:
      LOG_LEVEL: debug
    entries:
      - name: core-compose
        runner: compose
        order: 10
        services: [postgres, redis]
      - name: api
        runner: native
        order: 20
        depends_on: [core-compose]
```

`dva up local-dev` 해석 결과:

1. `plans.local-dev` 선택
2. `env_file` 적용
3. `environments.dev.environment` 적용
4. `sites.local.vars` 적용
5. `plans.local-dev.vars` 적용
6. `core-compose`, `api`를 `stack`에서 resolve
7. 각 entry의 최종 runner 결정
8. `depends_on` + `order`로 wave 계산
9. 실행
