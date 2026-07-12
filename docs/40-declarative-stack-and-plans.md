# Declarative Stack And Plans

이 문서는 DVA 설정 구조를 단순화하기 위한 새 방향을 정의합니다.
목표는 기능 축소가 아니라, 서로 다른 책임을 분리해서 많은 실행 방식을 지원해도 혼란이 생기지 않도록 만드는 것입니다.

## 1. 문제 정의

기존 구조에서 가장 큰 문제는 `stack`이 너무 많은 책임을 동시에 가진다는 점입니다.

- 실행 대상 선언
- 태그 분류
- 실행 순서 지정
- compose 서비스 부분 선택
- `mode` / `env` 기반 필터링 대상
- 사실상 실행 계획 일부 표현

이 구조에서는 "무엇이 선언이고 무엇이 실행 계획인지"가 섞입니다.
그 결과 설정이 커질수록 예측 가능성이 떨어지고, 작은 변경도 다른 동작에 영향을 주기 쉬워집니다.

## 2. 핵심 원칙

새 구조의 핵심 원칙은 아래와 같습니다.

1. `stack`은 선언 저장소다.
2. 실제 실행 대상은 이름 있는 실행 계획이다.
3. `environment`와 `site`는 실행 계획 해석에 참여하지만, `stack` 선택 책임을 직접 갖지 않는다.
4. 다양한 runner를 계속 지원하되, 선언과 실행 계획을 섞지 않는다.

즉:

- `stack` = 재사용 가능한 부품 정의
- `plans` = 실제 실행 가능한 이름
- `environments` = 환경 변수와 용도 차이
- `sites` = 실행 host 조건 차이

## 3. 용어 정리

### 3-1. stack

`stack`은 실행 가능한 엔트리의 선언 모음입니다.
여기에는 runner별 원본 설정만 둡니다.

`stack`은 다음을 직접 결정하지 않습니다.

- 최종 실행 순서
- compose 부분 서비스 선택
- 특정 환경에서의 선택 여부
- 특정 site에서의 runner 전환

### 3-2. plans

`plans`는 실제 사용자가 실행하는 이름입니다.

예:

- `local-dev`
- `office-stg`
- `remote-backend`

CLI는 이 이름을 직접 받습니다.

```bash
dva up local-dev
dva down local-dev
dva stop local-dev
dva status local-dev
dva ls
dva show local-dev
```

`plan`이라는 용어는 내부 개념으로만 사용하고, CLI에는 굳이 드러내지 않아도 됩니다.

### 3-3. environments

`environments`는 `dev`, `stg`, `prd` 같은 환경 구분입니다.
실행 장소가 아니라, 환경 변수와 외부 대상 차이를 표현합니다.

예:

- `APP_ENV`
- API endpoint
- DB name
- feature flag

### 3-4. sites

`sites`는 cluster 기준이 아니라 host 기준입니다.
즉 "DVA를 어디서 실행하느냐"를 표현합니다.

예:

- `local`
- `office`
- `remote`
- `cloud`

`site`는 보통 아래 차이를 만듭니다.

- 사용 가능한 네트워크
- 로컬 Docker 사용 가능 여부
- 로컬 프로세스 실행 가능 여부
- 기본 kube context 또는 SSH 접근성

cluster 자체는 필요하면 별도 `target` 개념으로 분리할 수 있지만, 이 문서의 기본 구조에서는 우선 `site`를 host 기준으로만 정의합니다.

### 3-5. vars

환경 변수 블록의 공통 필드명은 `environment`가 아니라 `vars`를 사용합니다.

이유:

- `environments`와 `environment`의 이름 충돌을 피하기 위해
- `sites.*.environment` 같은 중첩 표현의 혼란을 줄이기 위해
- 실제 역할이 "환경 구분"이 아니라 "주입할 변수 집합"이기 때문에

예:

```yaml
environments:
  dev:
    environment:
      APP_ENV: dev

sites:
  local:
    vars:
      DVA_SITE: local
```

## 4. stack 엔트리 설계

`stack`은 다양한 runner를 계속 지원할 수 있습니다.
중요한 것은 다양성을 없애는 것이 아니라, 선언 레이어에만 가두는 것입니다.

예상 runner 범위:

- `compose`
- `docker`
- `native`
- `kubectl`
- `helm`
- `kustomize`
- `tilt`
- `skaffold`
- `podman-compose`
- `vagrant`
- `multipass`
- `sam`
- `serverless`
- `script`

`stack` 엔트리는 logical unit 선언입니다.
하나의 엔트리가 여러 runner를 함께 가질 수 있습니다.

```yaml
stack:
  core-compose:
    description: local shared infra
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
          - docker-compose.dev.yml

  api:
    description: backend api
    default_runner: native
    runners:
      native:
        dir: apps/api
        build: go build ./cmd/api
        run: go run ./cmd/api
      docker:
        image: myorg/api:dev
        run: docker run --rm myorg/api:dev
      helm:
        chart: ./charts/api
        release: api
        namespace: default

  app-chart:
    description: app release via helm
    default_runner: helm
    runners:
      helm:
        chart: ./charts/app
        release: app
        namespace: default
```

## 5. Compose 취급 원칙

`compose`는 다른 runner와 다르게 "묶음 실행 단위"가 될 수 있습니다.
이 점을 구조에 반영해야 합니다.

예:

- 하나의 compose entry가 여러 파일을 사용할 수 있음
- 하나의 compose entry가 여러 서비스를 한 번에 띄울 수 있음

따라서 `stack`의 compose 엔트리는 보통 "compose 프로젝트 선언"으로 취급합니다.

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

실제 어떤 서비스만 띄울지는 실행 계획에서 선택할 수 있어야 합니다.
즉 compose 서비스 선택은 선언이 아니라 계획 쪽 책임입니다.

## 6. 실행 계획 구조

실행 계획은 named entry로 정의합니다.
이 문서에서는 이를 `plans`라고 부릅니다.

각 plan은 아래를 조합합니다.

- 어떤 stack 엔트리를 사용할지
- 어떤 environment를 적용할지
- 어떤 site를 적용할지
- 필요한 override
- runner 선택
- 최종 실행 순서와 dependency

```yaml
plans:
  local-dev:
    description: local developer stack
    environment: dev
    site: local
    entries:
      - name: core-compose
        runner: compose
        order: 10
        services: [postgres, redis]

      - name: api
        runner: native
        order: 20
        depends_on: [core-compose]

      - name: web
        runner: native
        order: 30
        depends_on: [api]
```

여기서 중요한 점:

- `name`은 `stack` 엔트리 참조
- `runner`는 해당 stack 엔트리에 선언된 runner 중 하나를 선택
- `services`는 compose 같은 그룹형 runner에만 적용 가능
- `order`와 `depends_on`은 plan 레이어에서 정의

## 7. 왜 `dva stack up`이 아닌가

`stack`은 선언 저장소이므로, 직접 실행 명령의 대상으로 쓰는 것이 맞지 않습니다.

예를 들어:

```bash
dva up local-dev
```

이 명령은 아래를 한 번에 의미합니다.

- 어떤 stack 엔트리를 쓸지
- 어떤 environment를 적용할지
- 어떤 site에서 실행할지
- 어떤 compose 서비스만 올릴지
- 어떤 순서로 실행할지

즉 사용자는 선언 이름이 아니라, 실행 의도를 담은 이름을 실행해야 합니다.

## 8. 권장 CLI

권장 CLI는 아래와 같습니다.

```bash
dva up <name>
dva down <name>
dva stop <name>
dva status [name]
dva ls
dva show <name>
```

의미:

- `dva ls`: 실행 가능한 이름 목록 출력
- `dva show <name>`: 해당 실행 계획 상세 출력
- `dva up <name>`: 실행

필요하면 보조 조회 명령을 둘 수 있습니다.

```bash
dva ls stack
dva ls environments
dva ls sites
```

하지만 기본 UX는 `dva ls`만으로 충분해야 합니다.

## 9. 권장 YAML 예시

```yaml
version: 2

env_file:
  - .env

stack:
  core-compose:
    description: shared infrastructure via compose
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
          - docker-compose.dev.yml

  api:
    description: backend api
    default_runner: native
    runners:
      native:
        dir: apps/api
        build: go build ./cmd/api
        run: go run ./cmd/api
      docker:
        image: myorg/api:dev
        run: docker run --rm myorg/api:dev
      helm:
        chart: ./charts/api
        release: api
        namespace: default

  web:
    description: frontend app
    default_runner: native
    runners:
      native:
        dir: apps/web
        build: npm run build
        run: npm run dev
      docker:
        image: myorg/web:dev
        run: docker run --rm myorg/web:dev

environments:
  dev:
    environment:
      APP_ENV: dev
      LOG_LEVEL: debug

  stg:
    environment:
      APP_ENV: stg
      LOG_LEVEL: info

  prd:
    vars:
      APP_ENV: prd
      LOG_LEVEL: warn

sites:
  local:
    vars:
      DVA_SITE: local

  office:
    vars:
      DVA_SITE: office

  remote:
    vars:
      DVA_SITE: remote

plans:
  local-dev:
    description: local development plan
    environment: dev
    site: local
    vars:
      FEATURE_X: "true"
    entries:
      - name: core-compose
        runner: compose
        order: 10
        services: [postgres, redis]

      - name: api
        runner: native
        order: 20
        depends_on: [core-compose]

      - name: web
        runner: native
        order: 30
        depends_on: [api]
```

## 10. 해석 순서

`dva up local-dev` 실행 시 해석 순서는 아래와 같습니다.

1. `plans.local-dev` 조회
2. 참조된 `environment` 로드
3. 참조된 `site` 로드
4. vars 병합
5. plan의 `entries`를 `stack` 선언에 매핑
6. 각 entry의 runner를 결정
7. runner별 override와 compose 서비스 선택 적용
8. `depends_on` + `order`로 실행 순서 계산
9. wave 단위로 실행
10. `down`은 역순 teardown

### 10-1. vars 우선순위

`vars`는 아래 우선순위로 병합합니다.
뒤에 오는 값이 같은 키를 덮어씁니다.

1. OS 환경 변수
2. `env_file`
3. 전역 `vars`
4. `environments.<name>.environment`
5. `sites.<name>.vars`
6. `plans.<name>.vars`
7. CLI 일회성 override

즉:

```text
OS < env_file < global vars < environment vars < site vars < plan vars < CLI vars
```

이 순서를 택하는 이유:

- 전역 값은 기본값이어야 함
- `env_file`은 로컬/공통 기본 입력값을 제공해야 함
- `environment`는 dev/stg/prd 같은 용도 차이를 반영해야 함
- `site`는 local/remote/cloud 같은 실행 위치 차이를 최종 반영해야 함
- `plan`은 특정 실행 이름의 가장 구체적인 설정이어야 함
- CLI override는 명시적 사용자 의도를 최우선으로 반영해야 함

## 11. 마이그레이션 원칙

기존 구조에서 새 구조로 옮길 때의 기본 원칙:

- 기존 `stack` 엔트리는 최대한 유지
- `order`, `tags`, `services` 같은 실행 계획 성격 필드는 plan 쪽으로 이동
- 기존 `modes`는 삭제하거나 점진적으로 `plans` / `sites` / `environments`로 분해
- 기존 `applications`는 가능하면 `stack` 안의 runner 선언으로 통합

### 11-1. 기존 설정에서 사라지거나 이동하는 항목

아래 표는 기존 주요 설정이 새 구조에서 어떻게 처리되는지 정리한 것입니다.

| 기존 개념/키 | 새 구조 상태 | 설명 |
|---|---|---|
| `modes` | 제거 후 분해 | 기존 `mode`의 책임을 `plans`, `environments`, `sites`로 분리 |
| `applications` | 최상위 섹션 제거 | 앱 정의를 `stack` 내부 runner 선언으로 통합 |
| `stack.*.order` | `plans.*.entries[].order` 로 이동 | 실행 순서는 선언이 아니라 실행 계획 책임 |
| `stack.*.tags` | 축소 또는 선택적 메타데이터화 | 핵심 실행 제어 수단으로는 사용하지 않음 |
| `stack` 내부 compose 서비스 선택 | `plans.*.entries[].services` 로 이동 | compose 부분 선택은 선언이 아니라 계획 책임 |
| `mode` 기반 app strategy | `plan` / `site` 해석으로 이동 | `native` / `docker` / 기타 runner 선택을 mode에서 분리 |
| `environments.*.stack` | 제거 | environment는 stack 선택 책임을 갖지 않음 |
| `environments.*.stack_overrides` | 제거 또는 대폭 축소 | environment는 vars 중심으로 단순화 |
| `dva stack up/down/stop/status` | `dva up/down/stop/status <name>` 으로 대체 | 실행 대상은 stack이 아니라 named execution entry |

### 11-2. 유지되지만 의미가 바뀌는 항목

| 기존 개념/키 | 새 구조 상태 | 설명 |
|---|---|---|
| `stack` | 유지 | 실행 대상 집합이 아니라 선언 저장소 |
| `environments` | 유지 | stack 필터가 아니라 `vars` 중심 구성 |
| `subprojects` | 유지 | 여러 DVA 설정 공간을 연결하는 계층 |
| `interaction` | 유지 | 단발성 편의 명령 |
| `provision` | 유지 | 준비/초기화 절차 |

## 12. Compatibility Layers

새 구조에서도 기존의 핵심 편의 요소는 유지해야 합니다.
다만 실행 모델과 동일 레이어에 두지 않고, 보조 레이어로 분리해서 다뤄야 합니다.

### 12-1. subprojects

`subprojects`는 계속 유지할 가치가 있습니다.
역할은 "여러 DVA 설정 공간을 연결하는 네임스페이스/집계 계층"입니다.

즉 `subprojects`는 실행 계획 자체를 정의하기보다, 여러 프로젝트의 `stack` / `plans` / `interactions`를 연결하는 구조입니다.

예상 활용:

- 상위 프로젝트에서 하위 프로젝트의 실행 가능한 이름을 함께 노출
- 특정 subproject의 실행 계획만 대상으로 실행
- `backend/local-dev` 같은 qualified name 지원

기본 연결 대상:

- `plans`
- `interactions`

선택 연결 대상:

- `provision`
- `stack` 조회용 노출

권장 원칙:

- `subprojects`는 설정 공간 분리와 재사용을 담당
- 실행 모델 자체는 각 subproject 내부에서 동일한 구조를 따름
- 기본 canonical name은 항상 `subproject/name` 형식을 사용
- parent top-level로 자동 flatten 하지 않음
- alias는 선택 사항으로만 허용
- 이름 충돌은 자동 해결하지 않고 hard error로 처리
- subproject 선언은 미리 둘 수 있지만, non-empty `import` 대상이 있거나 직접 실행할 때는 해당 subproject의 `dva.yml`이 존재해야 함

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

parent에서 노출되는 canonical name:

- `backend/local-dev`
- `backend/shell`
- `backend/logs`
- `backend/setup`

필요하면 alias를 명시적으로 둘 수 있습니다.

```yaml
subprojects:
  backend:
    path: services/backend
    import:
      plans:
        - name: local-dev
          as: backend-dev
```

이 경우에도 canonical name은 여전히 `backend/local-dev` 이고, `backend-dev` 는 추가 alias일 뿐입니다.

#### Subproject execution path

subproject의 `interactions`와 `provision`은 parent의 현재 작업 디렉터리가 아니라, 해당 subproject의 설정 파일이 있는 디렉터리를 기준으로 실행하는 것이 기본 원칙입니다.

즉:

- command resolution 기준 = subproject config
- relative path 기준 = subproject config dir
- default working directory = subproject root

이 규칙이 필요한 이유:

- subproject 내부 상대 경로가 parent 위치에 영향을 받지 않도록 하기 위해
- subproject가 독립적으로도 동일하게 실행되도록 하기 위해
- parent에서 import해도 동작 의미가 바뀌지 않도록 하기 위해

예를 들어 `services/backend/dva.yml` 에 정의된:

```yaml
interactions:
  shell:
    command: ./scripts/dev-shell.sh
```

이를 parent에서 `dva run backend/shell` 로 실행해도, 실제 실행 기준은 `services/backend` 여야 합니다.

`provision`도 동일합니다.
`dva provision backend/setup` 이 호출되면, 해당 provision step들의 상대 경로와 기본 실행 위치는 subproject root를 기준으로 해석합니다.

예외적으로 parent 기준 실행이 필요하다면, 명시적 옵션이나 절대 경로를 사용해야 합니다.

### 12-2. interactions

`interactions`도 유지해야 합니다.
이는 실행 계획이 아니라, 단발성 편의 명령 레이어입니다.

예:

- `dva shell`
- `dva logs`
- `dva db:migrate`
- `dva test`
- `dva kubectl`

권장 원칙:

- `plans`는 장기 실행과 오케스트레이션 담당
- `interactions`는 단발성 작업과 접속/운영 편의 담당
- `up` / `down` / `stop` / `status` 같은 생명주기 명령은 `interactions`로 우회하지 않음

즉:

- 실행은 `plans`
- 작업은 `interactions`

### 12-3. provision

`provision`은 계속 필요합니다.
이는 실행 모델이 아니라 "환경 준비와 초기화" 레이어입니다.

예:

- 개발 도구 설치 확인
- 인증/로그인 준비
- 디렉토리/볼륨 초기화
- 초기 데이터 seed
- 클러스터 시작 전 선행 작업

권장 원칙:

- `provision`은 `up`의 대체가 아님
- `provision`은 실행 전에 필요한 반복 가능한 준비 절차
- `plans`가 장기 실행 환경을 올리고 내리는 역할을 담당한다면, `provision`은 그 전에 필요한 준비를 담당

### 12-4. 계층 요약

새 구조에서 각 요소의 역할은 아래와 같이 구분합니다.

- `stack`: 재사용 가능한 실행 대상 선언
- `plans`: 실제 실행 가능한 조합
- `environments`: dev/stg/prd 같은 환경 차이
- `sites`: local/office/remote/cloud 같은 실행 host 차이
- `subprojects`: 여러 DVA 설정 공간 연결
- `interactions`: 단발성 편의 명령
- `provision`: 환경 준비/초기화 절차

## 13. 결정 사항 요약

- `stack`은 유지한다.
- `stack`은 선언 저장소다.
- `stack` 엔트리는 multi-runner logical unit이 될 수 있다.
- 실행 명령의 대상은 named execution entry다.
- 문서에서는 이를 `plans`로 부르지만, CLI에는 굳이 드러내지 않는다.
- `profiles`는 도입하지 않는다.
- `environments`는 환경 변수와 용도 차이 담당이다.
- `sites`는 host 기준 실행 위치 담당이다.
- `env_file`은 유지한다.
- `compose`는 여러 파일과 여러 서비스를 함께 다루는 묶음 단위로 지원한다.
- runner 다양성은 유지하되, 선언 레이어와 계획 레이어를 분리한다.
- `subprojects`, `interactions`, `provision`은 유지하되 실행 모델과 분리된 보조 레이어로 취급한다.
