# Config Merge Semantics

이 문서는 새 DVA 설정 구조에서 `dva.yml`이 여러 레이어(base, module, override)로 합쳐질 때의 규칙을 정의합니다.

관련 구조 설명은 아래 문서를 기준으로 합니다.

- [31-execution-plan-resolution.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/31-execution-plan-resolution.md)
- [40-declarative-stack-and-plans.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/40-declarative-stack-and-plans.md)

## 1. 레이어 우선순위

```text
dva.yml (base) -> modules (순서대로) -> dva.override.yml
```

나중 레이어가 높은 우선순위를 가집니다.

## 2. 기본 원칙

| 타입 | 전략 | 설명 |
|---|---|---|
| map | key별 deep merge | 같은 키가 있으면 내부 필드까지 재귀적으로 병합 |
| list | replace | 나중 레이어 값이 이전 리스트를 통째로 교체 |
| scalar | replace | 나중 레이어 값이 우선 |

핵심 원칙:

- 선언형 map은 key별 deep merge
- 실행 순서 같은 ordered list는 replace
- 구조적 정체성을 바꾸는 필드는 restricted field로 취급

## 3. Nil / Empty 의미론

| 상태 | 의미 | 예시 |
|---|---|---|
| 필드 absent | 상위 레이어 값 유지 | override에서 `default_runner`를 안 쓰면 base 값 유지 |
| 명시적 빈 문자열 `""` | 값을 비움 | `description: ""` |
| 명시적 빈 리스트 `[]` | 리스트를 비움 | `services: []` |
| 명시적 빈 맵 `{}` | 맵을 비움 | `vars: {}` |

핵심: "안 쓴 것"과 "비운 것"은 다릅니다.

## 4. Top-level 섹션별 merge 규칙

### 4-1. Top-level map 섹션

아래 섹션은 top-level에서 key별 deep merge 합니다.

| 섹션 | key | merge 전략 |
|---|---|---|
| `vars` | 변수명 | scalar replace |
| `stack` | 엔트리명 | 필드별 deep merge |
| `plans` | 실행 이름 | 필드별 deep merge |
| `environments` | 환경명 | 필드별 deep merge |
| `sites` | site명 | 필드별 deep merge |
| `interactions` | 명령명 | 필드별 deep merge |
| `subprojects` | subproject명 | 필드별 deep merge |
| `provision.profiles` | 프로필명 | 필드별 deep merge |

### 4-2. Top-level scalar 섹션

| 섹션 | 전략 |
|---|---|
| `version` | replace |
| `default` 계열 단일값 필드 | replace |
| `env_file` | replace |
| `devcontainer` | replace |
| `ssh.agent_image` | replace |

### 4-3. Top-level list 섹션

| 섹션 | 전략 |
|---|---|
| `modules` | replace |
| `suggestion_ignore` | replace |
| `checks` | append |

## 5. 엔트리 내부 merge 규칙

### 5-1. `stack.<name>`

`stack` 엔트리는 logical unit 선언입니다.
여러 runner를 함께 가질 수 있습니다.

예:

```yaml
stack:
  api:
    description: backend api
    default_runner: native
    runners:
      native:
        dir: apps/api
        run: go run ./cmd/api
      docker:
        image: myorg/api:dev
        run: docker run --rm myorg/api:dev
```

| 필드 | 타입 | 전략 | 비고 |
|---|---|---|---|
| `description` | scalar | replace | |
| `default_runner` | scalar | replace | 선언된 runner 이름이어야 함 |
| `vars` | map | key별 merge | stack-level 기본 변수 |
| `health` | map/struct | 필드별 deep merge | |
| `runners` | map | key별 deep merge | runner name 기준 |

#### `stack.<name>.runners`

`runners`는 runner name별로 deep merge 합니다.

예:

```yaml
stack:
  api:
    runners:
      native:
        run: go run ./cmd/api
```

override:

```yaml
stack:
  api:
    runners:
      native:
        build: go build ./cmd/api
```

결과:

```yaml
stack:
  api:
    runners:
      native:
        run: go run ./cmd/api
        build: go build ./cmd/api
```

### 5-2. `plans.<name>`

`plans`는 실제 실행 가능한 이름입니다.

예:

```yaml
plans:
  local-dev:
    environment: dev
    site: local
    vars:
      LOG_LEVEL: debug
    entries:
      - name: core-compose
        order: 10
        services: [postgres, redis]
```

| 필드 | 타입 | 전략 | 비고 |
|---|---|---|---|
| `description` | scalar | replace | |
| `environment` | scalar | replace | environment 이름 |
| `site` | scalar | replace | site 이름 |
| `vars` | map | key별 merge | plan-level vars |
| `entries` | list | replace | ordered execution list |

#### `plans.<name>.entries`

`entries`는 실행 계획의 ordered list 이므로 replace 합니다.
부분 merge 하지 않습니다.

이유:

- 순서가 중요한 데이터이기 때문
- 부분 merge 시 order/depends_on/services 충돌이 쉽게 발생하기 때문
- 실행 계획은 전체를 다시 쓰는 편이 예측 가능하기 때문

### 5-3. `plans.<name>.entries[]`

각 plan entry는 `stack` 선언을 참조합니다.

| 필드 | 타입 | 전략 | 비고 |
|---|---|---|---|
| `name` | scalar | replace | stack 참조명 |
| `runner` | scalar | replace | 선언된 runner 중 하나여야 함 |
| `order` | scalar | replace | 실행 순서 |
| `depends_on` | list | replace | dependency names |
| `services` | list | replace | compose subset 선택 |
| `vars` | map | key별 merge | entry-level 추가 vars |

### 5-4. `environments.<name>`

| 필드 | 타입 | 전략 |
|---|---|---|
| `description` | scalar | replace |
| `vars` | map | key별 merge |

### 5-5. `sites.<name>`

| 필드 | 타입 | 전략 |
|---|---|---|
| `description` | scalar | replace |
| `vars` | map | key별 merge |
| `entry_overrides` | map | key별 deep merge |

#### `sites.<name>.entry_overrides.<stack-entry>`

site는 특정 stack entry의 runner 선택이나 일부 실행 설정을 override 할 수 있습니다.

| 필드 | 타입 | 전략 | 비고 |
|---|---|---|---|
| `runner` | scalar | replace | 선언된 runner 중 하나여야 함 |
| `vars` | map | key별 merge | |
| 그 외 entry-level override | 타입별 규칙 적용 | 필요 최소 범위만 허용 |

### 5-6. `interactions.<name>`

| 필드 | 타입 | 전략 | 비고 |
|---|---|---|---|
| `runner` | scalar | replace 제한 가능 | backend 정체성과 관련 |
| `description` | scalar | replace | |
| `vars` | map | key별 merge | |
| `subcommands` | map | key별 deep merge | |
| 그 외 scalar | scalar | replace | |
| steps/list 성격 필드 | list | replace | |

주의:

- interaction runner는 실행 backend 정체성과 밀접하므로, 필요하면 restricted field로 유지하는 것이 안전함

### 5-7. `subprojects.<name>`

예:

```yaml
subprojects:
  backend:
    path: services/backend
    import:
      plans: [local-dev]
      interactions: [shell, logs]
      provision: [setup]
```

| 필드 | 타입 | 전략 |
|---|---|---|
| `path` | scalar | replace |
| `exclude_tags` | list | replace |
| `import` | map | key별 deep merge |

#### `subprojects.<name>.import`

| 필드 | 타입 | 전략 |
|---|---|---|
| `plans` | list | replace |
| `interactions` | list | replace |
| `provision` | list | replace |

이유:

- import 대상 목록은 부분 병합보다 명시적 교체가 더 예측 가능함
- alias 지정이 들어가면 list merge가 더 복잡해짐

### 5-8. `provision.profiles.<name>`

| 필드 | 타입 | 전략 |
|---|---|---|
| `description` | scalar | replace |
| `steps` | list | replace |
| `vars` | map | key별 merge |

## 6. Restricted Fields

다음 항목은 구조적 정체성과 관련되므로 주의가 필요합니다.

### 6-1. `stack.<name>.runners`

`stack.<name>.runners` 아래의 각 runner key는 logical backend identity를 나타냅니다.

예:

- `native`
- `docker`
- `helm`

규칙:

- 기존 runner key에 대한 config 변경은 허용
- 선언되지 않은 runner를 plan/site에서 참조하면 hard error
- `default_runner`는 최종적으로 반드시 존재하는 runner key를 가리켜야 함

### 6-2. `plans.<name>.entries[].runner`

`runner`는 자유 문자열이 아니라, 참조하는 `stack.<name>.runners` 안에 선언된 key 중 하나여야 합니다.

유효하지 않은 runner 선택은 validation error입니다.

### 6-3. subproject naming

subproject import는 canonical name을 가집니다.

예:

- `backend/local-dev`
- `backend/shell`
- `backend/setup`

규칙:

- canonical name은 항상 고유해야 함
- alias 충돌은 hard error
- 자동 flatten 금지

## 7. Vars Resolution 관련 merge 참고

merge semantics와 runtime resolution은 다릅니다.

- merge semantics: 여러 config layer를 하나의 최종 config로 합치는 규칙
- vars resolution: 실행 시점에 `env_file`, global vars, environment vars, site vars, plan vars를 적용하는 규칙

vars runtime 우선순위는 [31-execution-plan-resolution.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/31-execution-plan-resolution.md)를 따릅니다.

우선순위 (낮음 → 높음):

```text
env_file < global vars < environment vars < site vars < plan vars < CLI vars < OS 환경 변수
```

OS 환경 변수가 가장 높은 우선순위입니다. 같은 키가 OS에 설정되어 있으면
`dva.yml`의 어떤 레이어(`--var` 포함)도 그 값을 덮어쓰지 못합니다.

## 8. 예시

### Base

```yaml
vars:
  LOG_FORMAT: text

stack:
  api:
    default_runner: native
    runners:
      native:
        run: go run ./cmd/api
      docker:
        run: docker run --rm myorg/api:dev

plans:
  local-dev:
    environment: dev
    site: local
    entries:
      - name: api
        order: 10
```

### Override

```yaml
stack:
  api:
    runners:
      native:
        build: go build ./cmd/api

plans:
  local-dev:
    vars:
      LOG_LEVEL: debug
```

### Result

```yaml
stack:
  api:
    default_runner: native
    runners:
      native:
        run: go run ./cmd/api
        build: go build ./cmd/api
      docker:
        run: docker run --rm myorg/api:dev

plans:
  local-dev:
    environment: dev
    site: local
    vars:
      LOG_LEVEL: debug
    entries:
      - name: api
        order: 10
```

## 9. 하위호환성 메모

새 구조는 기존의 `modes`, 최상위 `applications`, `stack.*.order` 중심 모델에서 벗어납니다.

핵심 변화:

- `modes` 제거
- `applications`를 `stack` 안의 logical unit 선언으로 통합
- 실행 순서를 `plans.entries` 로 이동
- environment/site는 stack 선택이 아니라 vars와 override 해석에 집중

구형 구조와의 마이그레이션 표는 [40-declarative-stack-and-plans.md](/Users/archmagece/myopen/scripton/dev-virtual-auto/docs/40-declarative-stack-and-plans.md)를 따릅니다.
