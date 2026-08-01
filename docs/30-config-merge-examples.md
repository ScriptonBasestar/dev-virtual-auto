# Config Merge — Examples & Migration Notes

config merge 규칙이 실제로 어떻게 동작하는지 보여주는 예시와, 구형 구조에서의
마이그레이션 메모. 규칙 자체는 [30-config-merge-semantics.md](30-config-merge-semantics.md)를
참조하세요.

## 예시

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

## 하위호환성 메모

새 구조는 기존의 `modes`, 최상위 `applications`, `stack.*.order` 중심 모델에서 벗어납니다.

핵심 변화:

- `modes` 제거
- `applications`를 `stack` 안의 logical unit 선언으로 통합
- 실행 순서를 `plans.entries` 로 이동
- environment/site는 stack 선택이 아니라 vars와 override 해석에 집중

구형 구조와의 마이그레이션 표는 [42-migration-and-compatibility.md](42-migration-and-compatibility.md)를 따릅니다.
