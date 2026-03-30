기존 `dva.yml`을 보존하지 않고, 프로젝트 분석 결과를 기반으로 **최적의 dva.yml을 처음부터 새로 작성**하세요.
앱 코드를 바꾸는 것이 아니라 `dva.yml`과 관련 문서를 재작성하는 것입니다.

# Mode: Rewrite (Fresh)

1. **기존 dva.yml의 구조, 명령 이름, 메타데이터에 얽매이지 마세요.** 프로젝트 분석 결과를 기반으로 최적의 구조를 새로 설계하세요.
2. 기존 설정에서 재사용할 가치가 있는 부분(커스텀 provision 스크립트, 복잡한 health check 등)은 참고하되, 구조는 새로 잡으세요.

> 공통 규칙은 DVA Library Reference의 "DVA Configuration Guardrails (Shared)" 섹션을 따르세요.
