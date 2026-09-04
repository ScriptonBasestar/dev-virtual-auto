# 57. devbox native-lifecycle 패턴 가이드

> 상태: 권장 패턴 (TASK-310). 실행 예시는 `examples/devbox-native/`.
> 용어와 계층 규칙은 [docs/40](40-declarative-stack-and-plans.md), subproject import는
> USAGE.md "subprojects" 절이 정본이다. 이 문서는 **어디에 무엇을 선언하는가**만 다룬다.

## 1. 문제

devbox 루트(여러 서비스 저장소를 함께 체크아웃하는 저장소)에서 native 앱 프로세스를
선언하는 방식이 세 갈래로 갈렸다.

| 유파 | 관찰 | 결과 |
|---|---|---|
| 루트 plan에 native entry | flow-knowchain, flow-pipechain | 루트 dva.yml이 서브프로젝트의 빌드·실행 명령을 복제. 서브프로젝트가 명령을 바꾸면 루트가 조용히 낡음 |
| 서브프로젝트 소유 + import | flow-observechain | 루트는 이름만 import. validate 경고 0 |
| 선언하지 않음 | dripter | native 앱이 dva 밖. `dva status`·health·`down`이 볼 수 없음 |

SOUL의 신념 3 "하나의 동작에는 하나의 소유자만 둔다"가 판단 기준이다. native 프로세스의
빌드·실행 명령을 아는 것은 그 코드를 가진 서브프로젝트이므로, 소유자도 거기다.

## 2. 권장 패턴: 서브프로젝트 소유 + import

역할을 두 층으로 나눈다.

**루트 dva.yml** — 공유 인프라와 컨테이너화된 서비스만 소유한다.

- `stack.compose` 하나가 compose 파일을 소유하고, plan은 그 서비스 부분집합을 고른다.
- `subprojects.<name>.import`로 자식의 `plans`와 `interactions`를 **이름만** 가져온다.
- `exclude_tags: [infra]`로 자식이 자체 인프라 엔트리를 두더라도 루트 namespace에 섞이지
  않게 한다. postgres의 소유자는 하나여야 한다.

**서브프로젝트 dva.yml** — 자기 native 프로세스의 수명 주기를 소유한다.

- `stack.<app>`에 `native` runner(`build`, `run`)와 `health_checks`를 둔다.
- `plans.dev`가 그 엔트리 하나를 선택하고 `default_plan: dev`로 둔다. 서브프로젝트 안에서
  `dva up`, 루트에서 `dva up core/dev`가 **같은 소유자의 같은 프로세스**를 띄운다.
- `interaction`에 test/lint/migrate 같은 개발 명령을 둔다. 루트가 import하면
  `dva run core/test`가 되고 실행 디렉터리는 서브프로젝트 root다.

```yaml
# 루트
subprojects:
  core:
    path: services/core
    exclude_tags: [infra]
    import:
      plans: [dev]
      interactions: [test, lint, migrate]
```

```yaml
# services/core/dva.yml
stack:
  core:
    default_runner: native
    runners:
      native:
        build: "go build -o bin/core ./cmd/core"
        run: "./bin/core --port 8080"
    health_checks:
      core: { type: http, url: "http://localhost:8080/health", ready_timeout: 60 }
plans:
  dev:
    entries: [{ name: core, runner: native, order: 20 }]
default_plan: dev
```

루트에서 보이는 이름은 `dva ls`가 그대로 보여 준다.

```
Plans (dva up <name>):
  core/dev     # Core API as a native process
  local-apps   # Infrastructure plus the containerised builds of both apps
  local-infra  # Shared infrastructure only
  portal/dev   # Portal dev server as a native process
```

### 2-1. 인프라 의존은 순서가 아니라 절차로 푼다

import된 `core/dev`는 자식 plan이라 루트의 `compose` 엔트리에 `depends_on`을 걸 수 없다.
개발 절차는 `dva up`(인프라) → `dva up core/dev`다. 한 명령으로 묶고 싶으면 루트에
`composes:` plan을 두어 루트 plan과 import된 plan을 순서대로 실행한다(USAGE.md "composes").
예시의 `dev-all`이 그 형태다.

```yaml
plans:
  dev-all:
    composes:
      - { plan: local-infra, order: 0 }
      - { plan: core/dev, order: 1, depends_on: [local-infra] }
      - { plan: portal/dev, order: 1, depends_on: [local-infra] }
```

루트 plan에 native entry를 직접 넣어 `depends_on: [compose]`를 거는 것은 §3의 유파로
돌아가는 것이므로 택하지 않는다.

### 2-2. 최소 변형: interaction만 export

flow-observechain처럼 자식이 `stack` 없이 `interaction.dev: pnpm dev`만 두고 루트가 그것을
import하는 형태도 경고 없이 동작한다. 이때 native 프로세스는 `dva run core/dev`로 **전경**
실행되며 `status`·health·`down`의 대상이 아니다. 개발 서버를 터미널에 붙여 두는 흐름에는
충분하고, 백그라운드 수명 주기가 필요해지면 위의 stack 엔트리 형태로 올린다. 두 형태를
동시에 두지 않는다(§4-2).

## 3. 루트 plan에 native entry — 허용 조건

앱 코드가 별도 저장소가 아니라 루트 저장소 안의 디렉터리이고 자체 dva.yml을 가질 이유가
없다면(단일 저장소 모노리스), 루트 `stack.<app>`에 native 엔트리를 두는 것이 맞다.
`examples/applications.yml`이 이 형태다. 조건은 다음과 같다.

- `runners.native.dir`로 실행 디렉터리를 명시한다.
- `health_checks`는 최상위가 아니라 **엔트리 아래**에 둔다. 최상위 `health_checks`는
  modes 참조가 없으면 실행되지 않는다(validate가 경고).
- plan에서 `depends_on: [compose]`로 인프라 뒤에 세운다.

서브프로젝트가 자체 dva.yml을 갖게 되는 순간 이 엔트리는 §2로 이전한다. 두 곳에 남기면
소유자가 둘이다.

## 4. 안티패턴

### 4-1. native 앱을 선언하지 않음 (dripter)

앱을 Makefile이나 README 절차로만 띄우면 dva는 인프라만 안다. `dva status`는 앱을 못
보고, `dva down`은 앱을 남기며, endpoints의 health는 확인할 수 없다. §2 또는 §3 중 하나로
선언한다. 명령이 Makefile에 있다면 `run: "make dev-backend"`처럼 그대로 가리키면 된다.

### 4-2. interaction으로 native 실행을 중복 선언 (cwrapper)

```yaml
stack:
  django-native:
    runners: { native: { run: "uv run python manage.py runserver" } }
interaction:
  start:
    command: "uv run python manage.py runserver"   # 같은 프로세스, 둘째 소유자
```

`dva up`으로 뜬 프로세스와 `dva run start`로 뜬 프로세스는 서로를 모른다. 포트 충돌,
`status` 불일치, `down`이 못 내리는 프로세스가 생긴다. interaction을 지우고 stack 엔트리만
남긴다. 짧은 이름이 필요하면 그 엔트리 하나를 고르는 plan에 이름을 붙인다
(`plans.api: {entries: [{name: django-native}]}` → `dva up api`).

### 4-3. 루트가 자식 명령을 복제 (flow-knowchain)

루트 `runners.native.run: "make dev-native-backend"`는 자식 Makefile의 타겟 이름을 루트가
기억하는 형태다. 자식이 타겟을 바꾸면 루트는 validate를 통과한 채 실행에서 깨진다.
자식에 dva.yml을 두고 §2로 import하면 이름 계약만 남고 명령은 소유자에게 돌아간다.

## 5. 검증

`examples/devbox-native/`에서 `dva validate`가 경고 없이 통과하고 `dva ls`가 위 목록을
출력한다. 서브프로젝트 디렉터리에서 단독으로 `dva validate`해도 통과해야 한다. 자식의
plan이 import되는지 확인하려면 루트에서 `dva show`의 "Plans"와 "Subprojects" 절을 본다.
