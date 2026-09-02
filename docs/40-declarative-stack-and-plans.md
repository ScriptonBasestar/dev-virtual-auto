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
dva show
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

변수 블록의 필드명은 섹션마다 다릅니다. 스키마가 각 섹션에서 아래 필드명만 허용하므로(`additionalProperties: false`), 서로 바꿔 쓰면 `dva validate`가 실패합니다.

| 섹션 | 변수 블록 필드명 |
| --- | --- |
| `environments.<name>` | `environment` |
| `sites.<name>` | `vars` |

> 참고: 설계 단계에서 `environments`와 `environment`의 이름 충돌을 피하려고 모든 섹션의 변수 블록을 `vars`로 통일하는 안도 검토했지만, 채택하지 않았습니다. `environments.<name>`에 `vars`를 쓰면 스키마 검증에서 거부됩니다.

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

## 이어서 읽기

- [41-execution-plans-and-cli.md](41-execution-plans-and-cli.md) — 실행 계획 구조, CLI, 권장 YAML, 해석 순서 (§6–10)
- [42-migration-and-compatibility.md](42-migration-and-compatibility.md) — 마이그레이션 원칙과 호환 레이어 (§11–13)
