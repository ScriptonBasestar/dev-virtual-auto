---
id: TASK-310
title: "docs: devbox native-lifecycle pattern guide"
type: docs
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-05T09:00:00+09:00
source: "docs/dogfood/{flow-knowchain,flow-pipechain,flow-observechain,dripter}.md"
status: done
---

# Task 310: devbox native-lifecycle 패턴 가이드 작성

## Summary

native 앱 프로세스 선언에 두 유파가 갈림:

- "루트 plan에 native entry" (flow-knowchain, flow-pipechain)
- "subproject 소유 + import.interactions" (flow-observechain — 레퍼런스, warning 0)

dripter는 native 앱을 stack에 아예 선언하지 않아 dva 관리 밖. 권장 패턴 문서화 필요.

## Direction

flow-observechain 방식을 권장 패턴으로 docs/에 가이드 작성, examples/에 예시 추가.
interaction.start로 native 실행을 중복 선언하는 안티패턴(cwrapper)도 함께 다룬다.

## Completion Criteria

- [x] docs/ 가이드 + examples/ 예시 추가, `make generate` 산출물 갱신 | verify: `make generate && git diff --stat`
- [x] 예시가 validate 통과 | verify: human — 출력 첨부

## Completion Evidence (2026-09-05)

- 가이드: `docs/57-devbox-native-lifecycle-guide.md`. 권장 패턴은 flow-observechain 구조(루트가 compose
  인프라를 소유, 서브프로젝트가 native 수명 주기를 소유, 루트는 `import`로 이름만 가져옴)에 자식
  `stack` native 엔트리 + `plans.dev`를 더한 형태. 루트 plan native entry가 허용되는 조건(§3)과
  안티패턴 3종(미선언 dripter, `interaction.start` 중복 cwrapper, 루트가 자식 명령 복제 knowchain)을 다룸.
- 예시: `examples/devbox-native/` (루트 `dva.yml` + `compose.yaml` + `services/{core,portal}/dva.yml`).
  `composes:` plan `dev-all`이 루트 plan과 import된 plan을 함께 순서화할 수 있음을 확인했다.
  `examples/README.md`에 항목 추가.
- `make generate && git diff --stat`: 생성물 변경 없음(examples/는 생성 입력이 아님; 아래 커밋 diff 참조).

human 증거 — 루트에서:

```
$ dva validate
✅ dva.yml is valid            # 경고 0
$ dva ls
core/lint     # Run golangci-lint
core/migrate  # Apply database migrations
core/test     # Run the Go test suite
db-shell      # Open psql in the shared PostgreSQL service
portal/lint   # Run eslint
portal/test   # Run vitest

Plans (dva up <name>):
  core/dev     # Core API as a native process (infrastructure comes from the devbox root)
  dev-all      # Shared infrastructure, then both native apps from their owners
  local-apps   # Infrastructure plus the containerised builds of both apps
  local-infra  # Shared infrastructure only; run apps from their subprojects
  portal/dev   # Portal dev server as a native process
```

`services/core`에서 단독 `dva validate`도 경고 없이 통과. 부수 관찰: 루트에서 `dva up core/dev --dry-run`이
native health 대기에서 멈춤 — TASK-312가 다루는 결함이라 여기서는 건드리지 않았다.
