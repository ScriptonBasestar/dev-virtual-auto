# Execution Plans And CLI

실행 계획 구조, CLI, 권장 YAML, 해석 순서.
선언/용어 배경은 [40-declarative-stack-and-plans.md](40-declarative-stack-and-plans.md)를 본다.
마이그레이션은 [42-migration-and-compatibility.md](42-migration-and-compatibility.md)를 본다.

## 6. 실행 계획 구조

실행 계획은 named entry로 정의합니다.
이 문서에서는 이를 `plans`라고 부릅니다.

각 plan은 아래를 조합합니다.

- 어떤 stack 엔트리를 사용할지
- 어떤 environment를 적용할지
- 어떤 site를 적용할지
- 성공한 startup 뒤 어떤 endpoint를 표시할지
- 필요한 override
- runner 선택
- 최종 실행 순서와 dependency

```yaml
plans:
  local-dev:
    description: local developer stack
    environment: dev
    site: local
    endpoint_tags: [app]
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
dva show
```

의미:

- `dva ls`: 실행 가능한 이름 목록 출력
- `dva show`: 선언된 설정 요약 출력 (plan 이름 인자는 받지 않습니다)
- `dva up <name>`: 실행

`dva ls stack` 같은 주제별 보조 조회 명령은 도입하지 않았습니다 — 기본 UX는
`dva ls`만으로 충분해야 합니다.

## 9. 권장 YAML 예시

```yaml
version: "0.1.44"

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
    environment:
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
어떤 레이어(`--var` 포함)도 그 값을 덮어쓰지 못합니다.

`dva up <plan> --dry-run`은 이 레이어들이 특정 실행에서 실제로 무엇을 얹었는지 출력합니다
([USAGE.md](../USAGE.md#실제-적용-결과-확인)).

이 순서를 택하는 이유:

- 전역 값은 기본값이어야 함
- `env_file`은 로컬/공통 기본 입력값을 제공해야 함
- `environment`는 dev/stg/prd 같은 용도 차이를 반영해야 함
- `site`는 local/remote/cloud 같은 실행 위치 차이를 최종 반영해야 함
- `plan`은 특정 실행 이름의 가장 구체적인 설정이어야 함
- CLI override는 명시적 사용자 의도를 최우선으로 반영해야 함

