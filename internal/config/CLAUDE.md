# internal/config — DVA Configuration

`dva.yml` 파싱·병합·검증 담당. 모든 설정 타입 정의.

## Core Types

- **`Config`** — `dva.yml` 전체 구조 (config.go)
- **`LifecycleEntry`** — `stack:` 항목, `default_runner`+`runners` 기반 실행 선언 (lifecycle.go)
- **`ModeConfig`** — `--mode` 런타임 전략 (`build`/`run`/`applications`) (config.go)
- **`Environment`** — 활성 환경 프로필 및 env_file 병합 결과 (environment.go)

## Plugin Resolution (lifecycle.go)

`LifecycleEntry`의 실행 설정은 우선 `runners`에서 해석됩니다.
Legacy non-compose 플러그인은 명시적 `plugin:` 또는 entry 이름 자동 추론도 지원합니다.

`rawNode`에 YAML 원본 저장 → 타입 확정 후 재파싱.

## Merge (merge.go)

```
base config ← modules[*] ← imported subprojects[*]
```

- `stack:` — 이름 기준 병합, `order` 중복 시 경고
- `environment:` — 후순위 덮어쓰기
- `interaction:` — 이름 충돌 시 에러

## Validation (validate.go)

JSON Schema (`schema.json`) + Go 검증 2단계.
경고(`validate_warnings.go`)는 에러가 아닌 출력으로만 표시.

## Reserved Commands (reserved.go)

`IsReservedCommand(name)` — `interaction:` 키가 내장 커맨드와 충돌하는지 확인.
`cli/root.go`의 동적 커맨드 등록에서 사용.

## env_file Handling (envfile.go)

`env_file:` 필드는 `string | []string | map[string]string` 형태 모두 허용 (`any` 타입).
환경별 env_file 선택 로직 포함.
