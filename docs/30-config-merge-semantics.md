# Config Merge Semantics

dva.yml 설정이 여러 레이어(base, module, override)에서 합쳐질 때의 규칙을 정의합니다.

## 레이어 우선순위

```
dva.yml (base) → modules (순서대로) → dva.override.yml
```

나중 레이어가 높은 우선순위를 가집니다.

## 기본 원칙

| 타입 | 전략 | 설명 |
|---|---|---|
| **map** | key별 deep merge | 같은 키의 값을 재귀적으로 병합 |
| **list** | replace | 나중 레이어의 리스트가 이전 것을 통째 교체 |
| **scalar** | replace | 나중 레이어의 값이 우선 |

## Nil/Empty 의미론

| 상태 | 의미 | 예시 |
|---|---|---|
| 필드 absent (nil) | 상위 레이어 값 유지 | override에서 `order`를 안 쓰면 base 값 유지 |
| 명시적 빈 문자열 `""` | 값을 비움 | `command: ""` → 명시적으로 비움 |
| 명시적 빈 리스트 `[]` | 리스트를 비움 | `tags: []` → 태그 없음 |
| 명시적 빈 맵 `{}` | 맵을 비움 | `environment: {}` → 환경변수 없음 |

핵심: **"안 쓴 것"과 "비운 것"은 다릅니다.**

## 섹션별 적용 규칙

### Top-level map 섹션 (key별 deep merge)

이 섹션들은 top-level에서 key별로 합칩니다.
같은 key가 있으면 **엔트리 내부 필드까지 재귀적으로 deep merge**합니다.

| 섹션 | key | 엔트리 내부 merge |
|---|---|---|
| `environment` | 변수명 | scalar replace (map[string]string이므로) |
| `stack` | 엔트리명 | 필드별 deep merge |
| `interaction` | 명령명 | 필드별 deep merge |
| `modes` | 모드명 | 필드별 deep merge |
| `environments` | 환경명 | 필드별 deep merge |
| `applications` | 앱명 | 필드별 deep merge |
| `health_checks` | 체크명 | 필드별 deep merge |
| `endpoints` | 엔드포인트명 | 필드별 deep merge |
| `infra` | 인프라명 | scalar replace (단순 struct) |
| `subprojects` | 서브프로젝트명 | scalar replace (단순 struct) |
| `provision.profiles` | 프로필명 | list replace (steps 리스트) |

### Top-level scalar 섹션 (replace)

| 섹션 | 전략 |
|---|---|
| `version` | replace |
| `default_mode` | replace |
| `env_file` | replace (통째 교체) |
| `devcontainer` | replace (통째 교체) |
| `ssh.agent_image` | replace |

### Top-level list 섹션

| 섹션 | 전략 | 이유 |
|---|---|---|
| `modules` | replace | 모듈 목록은 base에서만 정의 |
| `suggestion_ignore` | replace | 패턴 목록 교체 |
| `checks` (doctor) | **append** | 기존 규칙 유지 — 체크는 누적 |

## 엔트리 내부 Deep Merge 규칙

`stack`, `interaction`, `modes` 등의 엔트리 내부 필드 merge 규칙:

### LifecycleEntry (stack 엔트리)

| 필드 | 타입 | 전략 | 비고 |
|---|---|---|---|
| `plugin` | scalar | **override 금지** | 구조적 필드 |
| `order` | scalar | replace | |
| `tags` | list | replace | |
| `exports` | map | key별 merge | |
| `health_checks` | map | key별 merge | |
| plugin config (compose, helm 등) | struct | 필드별 deep merge | |

### InteractionCommand (interaction 엔트리)

| 필드 | 타입 | 전략 | 비고 |
|---|---|---|---|
| `runner` | scalar | **override 금지** | 구조적 필드 |
| `service` | scalar | replace | |
| `command` | scalar | replace | |
| `description` | scalar | replace | |
| `environment` | map | key별 merge | |
| `tags` | list | replace | |
| `subcommands` | map | key별 merge | |
| 그 외 scalar | scalar | replace | |

### ModeConfig (modes 엔트리)

| 필드 | 타입 | 전략 |
|---|---|---|
| `description` | scalar | replace |
| `compose_profiles` | list | replace |
| `compose_services` | *list | replace (nil=전부, []=없음) |
| `health_checks` | list | replace |
| `endpoint_tags` | list | replace |
| `environment` | map | key별 merge |
| `stack` | list | replace |
| `build` | scalar | replace |
| `run` | scalar | replace |
| `applications` | any | replace |
| `provision` | scalar | replace |

### Plugin Config 내부

| 필드 타입 | 전략 | 예시 |
|---|---|---|
| scalar | replace | `namespace`, `command`, `project_name` |
| list | replace | `files`, `manifests`, `values`, `ports` |
| map | key별 merge | `env`, `set`, `services` |

## Override 금지 필드 (Restricted Fields)

다음 필드는 override 시 **hard error**를 발생시킵니다:

- `stack.*.plugin` — 플러그인 타입 변경은 사실상 다른 엔트리
- `interaction.*.runner` — 실행 백엔드 변경은 사실상 다른 명령

이유: 이 필드들이 바뀌면 "같은 엔트리의 부분 수정"이 아니라 사실상 다른 엔트리를 정의하는 것입니다.

## 예시

### Base dva.yml

```yaml
stack:
  compose:
    plugin: compose
    order: 10
    files: [docker-compose.yml]
    project_name: myapp
    tags: [core]
    exports:
      DB_HOST: postgres
```

### dva.override.yml (부분 override)

```yaml
stack:
  compose:
    files: [docker-compose.yml, docker-compose.dev.yml]  # list replace
    exports:                                               # map merge
      REDIS_HOST: redis
```

### 결과

```yaml
stack:
  compose:
    plugin: compose          # base 유지
    order: 10                # base 유지 (override에서 absent)
    files:                   # list replace
      - docker-compose.yml
      - docker-compose.dev.yml
    project_name: myapp      # base 유지 (override에서 absent)
    tags: [core]             # base 유지 (override에서 absent)
    exports:                 # map merge
      DB_HOST: postgres      # base 유지
      REDIS_HOST: redis      # override에서 추가
```

## 하위호환성

- `checks` (DoctorChecks)는 기존 append 동작을 유지합니다.
- `devcontainer`, `env_file`은 기존 통째 교체 동작을 유지합니다.
- **변경점**: `stack`, `interaction`, `modes`, `environments`, `applications`, `health_checks`, `endpoints` 엔트리가 기존에는 같은 키일 때 통째 교체였으나, 이제 필드별 deep merge로 변경됩니다.
- 기존에 override에서 전체 엔트리를 다시 쓰던 사용자는 동작 차이가 없습니다 (모든 필드를 명시했으므로).
- 부분만 쓰던 사용자는 이제 의도한 대로 부분 override가 적용됩니다.
