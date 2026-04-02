기존 `dva.yml`을 보존하지 않고, 프로젝트 분석 결과를 기반으로 **최적의 dva.yml을 처음부터 새로 작성**하세요.
앱 코드를 바꾸는 것이 아니라 `dva.yml`과 관련 문서를 재작성하는 것입니다.

# Mode: Rewrite (Fresh)

1. **기존 dva.yml의 구조, 명령 이름, 메타데이터에 얽매이지 마세요.** 프로젝트 분석 결과를 기반으로 최적의 구조를 새로 설계하세요.
2. 기존 설정에서 재사용할 가치가 있는 부분(커스텀 provision 스크립트, 복잡한 health check 등)은 참고하되, 구조는 새로 잡으세요.
3. **앱 서버/워커가 있는 프로젝트는 반드시 `applications:` 섹션을 생성하세요.** `interaction:`에 앱 실행 명령을 넣지 말고 `applications:`에 선언하세요. native/docker 실행 경로를 모두 정의하세요.

## CRITICAL: `applications:` 섹션 필수 필드

앱 서버/워커를 `applications:`에 선언할 때 **반드시 아래 필드를 포함**하세요:

- **`port:`** — 리스닝 포트 번호 (정수). `dva app ls`에서 표시됨. 생략 금지.
- **`run:`** — native/docker 이중 경로 권장. string shorthand(`run: "cmd"`)는 native 전용일 때만 사용.
  ```yaml
  run:
    native: "cargo run -p api"
    docker:
      service: api-rs
      profile: rust
  ```
- **`dev:`** — 개발 모드 (hot-reload). native 경로 필수.
- **`build:`** — 빌드 명령. native/docker 이중 경로 권장.
- **`health:`** — 헬스체크 (type, url, timeout, ready_timeout).

## CRITICAL: `modes:` 에 `applications:` 필드 연동

hybrid 또는 앱을 포함하는 모드에는 반드시 `applications:` 필드를 설정하세요:

```yaml
modes:
  hybrid:
    compose_services: [postgres, redis]
    applications: native          # 모든 앱을 네이티브로 실행
  full-stack:
    applications: docker          # 모든 앱을 도커로 실행
  # 앱별 전략 지정도 가능:
  mixed:
    applications:
      api: native
      worker: docker
```

`applications:` 필드가 없으면 해당 모드에서 앱이 자동 시작되지 않습니다.

## suggestion_ignore 정리

`suggestion_ignore`에는 **현재 프로젝트에 실제로 존재하는** Makefile/package.json 타겟만 포함하세요.
다른 프로젝트의 legacy 패턴(예: Go 프로젝트의 타겟이 Rust 프로젝트에 포함)은 제거하세요.
주석에 해당 타겟이 왜 무시되는지 정확한 이유를 적으세요.

> 공통 규칙은 DVA Library Reference의 "DVA Configuration Guardrails (Shared)" 섹션을 따르세요.
