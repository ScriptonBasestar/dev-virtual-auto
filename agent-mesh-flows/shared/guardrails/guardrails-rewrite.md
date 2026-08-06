기존 `dva.yml`을 보존하지 않고, 프로젝트 분석 결과를 기반으로 **최적의 dva.yml을 처음부터 새로 작성**하세요.
앱 코드를 바꾸는 것이 아니라 `dva.yml`과 관련 문서를 재작성하는 것입니다.

# Mode: Rewrite (Fresh)

1. **기존 dva.yml의 구조, 명령 이름, 메타데이터에 얽매이지 마세요.** 프로젝트 분석 결과를 기반으로 최적의 구조를 새로 설계하세요.
2. 기존 설정에서 재사용할 가치가 있는 부분(커스텀 provision 스크립트, 복잡한 health check 등)은 참고하되, 구조는 새로 잡으세요.
3. **앱 서버/워커는 `stack` 선언과 named `plans`로 구성하세요.** `applications:`는
   제거된 키라 쓰면 파일이 로드되지 않고, `modes:`는 legacy이므로 신규 설정에
   만들지 마세요.

## CRITICAL: lifecycle 소유권

Compose 서비스는 compose stack entry가 소유하고 plan이 선택합니다. 동일
서비스를 standalone `docker` runner로 다시 선언하지 마세요. native 개발
프로세스가 필요하면 별도 native/process stack entry로 선언하고 hybrid plan에서
선택하세요.

## CRITICAL: named plans

실행 조합은 named plan으로 표현하세요:

```yaml
plans:
  hybrid:
    entries:
      - name: infra
        runner: compose
        services: [postgres, redis]
      - name: api
        runner: native
  full-stack:
    entries:
      - name: infra
        runner: compose
        services: [postgres, redis, api]
```

## suggestion_ignore 정리

`suggestion_ignore`에는 **현재 프로젝트에 실제로 존재하는** Makefile/package.json 타겟만 포함하세요.
다른 프로젝트의 legacy 패턴(예: Go 프로젝트의 타겟이 Rust 프로젝트에 포함)은 제거하세요.
주석에 해당 타겟이 왜 무시되는지 정확한 이유를 적으세요.

> 공통 규칙은 DVA Library Reference의 "DVA Configuration Guardrails (Shared)" 섹션을 따르세요.
