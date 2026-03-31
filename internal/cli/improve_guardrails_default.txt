이미 존재하는 `dva.yml`을 **현재 프로젝트 상태에 맞게 정렬**하세요.
앱 코드를 바꾸는 것이 아니라 `dva.yml`과 관련 문서를 개선하는 것입니다.

# Mode: Default (Preserve)

1. 기존 `dva.yml`을 가능한 한 **최소 변경**으로 개선하되, **누락된 필수 섹션은 반드시 추가**하세요.
2. 기존 명령 이름이 이미 팀에 알려져 있다면, 특별한 이유 없이는 유지하세요.
3. **기존 `stack.compose.services` 메타데이터(tags, ports, related, hint)를 삭제하지 마세요.** 수정/추가만 허용됩니다.
4. **기존 `applications:` 섹션의 앱 정의를 삭제하지 마세요.** 수정/추가만 허용됩니다.

> 공통 규칙은 DVA Library Reference의 "DVA Configuration Guardrails (Shared)" 섹션을 따르세요.
