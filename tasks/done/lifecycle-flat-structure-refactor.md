# lifecycle 플랫 구조 리팩토링

## 현재 구조 (중첩)

```yaml
lifecycle:
  compose:
    order: 10
    compose:          # ← 플러그인 타입과 동일한 키가 중첩됨
      files: [docker-compose.yml]
      project_name: myapp
  kubectl:
    order: 20
    kubectl:          # ← 동일 패턴
      namespace: myapp-development
```

- Go 구조체: `LifecycleEntry`에 각 플러그인별 포인터 필드 (`*ComposePluginConfig`, `*KubectlPluginConfig`, ...)
- 플러그인 감지: `DetectPlugin()` — nil이 아닌 필드를 찾아서 타입 반환

## 제안 구조 (플랫)

```yaml
lifecycle:
  compose:
    plugin: compose
    order: 10
    files: [docker-compose.yml]
    project_name: myapp
  compose2:
    plugin: compose
    order: 30
    files: [docker-compose.dev.yml]
  kubectl:
    plugin: kubectl
    order: 20
    namespace: myapp-development
```

## 장점

- 중첩 제거: `compose.compose.files` → `compose.files`
- 동일 플러그인 복수 사용 자연스러움 (compose, compose2)
- `plugin:` 명시로 의도가 명확
- YAML 작성 시 depth 1단계 감소

## 구현 시 고려사항

- `LifecycleEntry`에 `Plugin string` 필드 추가
- custom `UnmarshalYAML` 구현 필요: `plugin` 값 기반으로 나머지 필드를 해당 플러그인 config에 매핑
- `DetectPlugin()` → `Plugin` 필드 직접 참조로 대체
- 기존 포인터 필드 방식(`*ComposePluginConfig` 등)은 내부적으로 유지 가능, YAML 바인딩만 변경
- 플러그인 간 필드명 충돌 검토 필요 (예: `namespace`가 kubectl, helm, kustomize에 모두 존재)

## 관련 파일

- `internal/config/lifecycle.go` — LifecycleEntry 구조체, 플러그인 config 타입들
- `internal/config/lifecycle_helpers.go` — lifecycle 헬퍼 함수
- `internal/config/config.go` — Config.Lifecycle 필드
- `internal/config/schema.json` — YAML 스키마 정의
- `internal/lifecycle/orchestrator.go` — 오케스트레이터 (플러그인 실행)

## 추가 검토: `lifecycle` 이름 변경

`lifecycle`이 "생명주기 단계"보다 "관리 대상 인프라 목록"에 가까움. 대안 후보:

| 후보 | 장점 | 단점 |
|---|---|---|
| `stack` | 간결, 직관적 | docker-stack 혼동 가능 |
| `runtime` | 실행 환경 의미 | 런타임 언어와 혼동 |
| `targets` | 깔끔 | CI/CD 느낌 |
| `lifecycle` (유지) | 변경 비용 없음 | 의미 약간 애매 |
