# Environment stack_overrides (2단계)

## 배경

1단계에서 `EnvironmentProfile.Stack []string` 필드로 환경별 stack entry 필터링을 구현함.
2단계는 환경별로 stack entry의 설정값 자체를 오버라이드하는 기능.

## 문제

환경별로 동일한 plugin의 설정값(namespace, files, values_files 등)을 바꿔야 하는 경우,
현재는 env var interpolation(`${NAMESPACE}`)에 의존하거나 별도 entry를 만들어야 함.

배열 필드(files, values_files)는 env var로 해결 불가.

## 제안 설계

```yaml
environments:
  stg:
    stack: [kubectl]
    stack_overrides:
      kubectl:
        namespace: myapp-staging
  prd:
    stack: [kubectl, helm]
    stack_overrides:
      kubectl:
        namespace: myapp-production
      helm:
        values_files: [values-production.yaml]
```

## 구현 포인트

- 구조체: `EnvironmentProfile.StackOverrides map[string]*LifecycleEntry`
- 적용 시점: `applyEnv()` 에서 env vars 머지 직후, orchestrator 실행 전
- 머지 방식: deep merge (원본 entry에 override 필드만 덮어씀)
- 오버라이드 대상: plugin config 필드 (namespace, files, values_files, project_name, exports 등)
- 오버라이드 불가: order, plugin, tags 등 구조적 필드

## 복잡도

- plugin config 타입별 merge 함수 필요
- merge 우선순위: base stack → stack_overrides → env var interpolation
- schema.json 업데이트 필요

## 대안

env var interpolation으로 대부분 커버 가능:

```yaml
stack:
  kubectl:
    namespace: ${NAMESPACE:-myapp-development}

environments:
  stg:
    environment:
      NAMESPACE: myapp-staging
```

배열 필드 오버라이드 요구가 실제로 발생하면 그때 구현.

## 판단 기준

- env var interpolation으로 충분하면 → 보류
- files/values_files 배열 오버라이드 요구 발생 시 → 구현
