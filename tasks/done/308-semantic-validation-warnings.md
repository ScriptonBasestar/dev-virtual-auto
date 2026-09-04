---
id: TASK-308
title: "validate: plan→stack reference check and semantic warnings"
type: feature
priority: P2
effort: M
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "reports/{gizzahub,matdosa,funbricks-postkit,funbricks-notifire,flow-knowchain,gorisa,sigdock-pass}.md"
status: done
---

# Task 308: 참조 검증 및 semantic warning 추가

## Summary

validate가 침묵하는 실증 사례들:

- plan이 stack services 맵에 없는 서비스를 참조해도 침묵 (gizzahub: temporal/kafka/prometheus)
- 미참조 environments/sites dead 선언 (matdosa)
- no-op entry_overrides (flow-taskchain, funbricks-postkit)
- default_plan 미설정 (multi-plan인데 기본 없음 — 3개 프로젝트)
- 빈 command: "" (sigdock-pass)
- note/주석 문자열 속 제거된 CLI(`dva dev`, `-M`, `dva clean`, `dva app up`) 참조 (3개 프로젝트)
- 최상위 고아 health_checks (flow-knowchain, flow-pipechain)

## Completion Criteria

- [x] 위 각 케이스에 대한 warning 규칙 + 픽스처 테스트 (docs/51-flowcheck-rules.md 갱신 포함) | verify: `make test`
- [x] gizzahub/matdosa 설정에서 해당 warning이 실제 출력됨 | verify: human — 출력 첨부

## Dogfood evidence (2026-09-05 실행)

- plan services → stack 미선언 참조: gizzahub 11건 실증 후 복구.
- 미참조 environments: matdosa `test` 삭제. no-op entry_overrides: postkit·flow-taskchain 삭제.
- 빈 `command: ""`: sigdock-pass. built-in 동등 replace 훅 경고는 TASK-320.

## Completion Evidence (2026-09-05)

`internal/config/validate_warnings_refs.go`에 6개 규칙을 추가하고 `ValidateWarnings()`에 등록했다
(`warnPlanServicesNotDeclared`, `warnUnreferencedEnvironmentsAndSites`, `warnNoOpEntryOverrides`,
`warnEmptyInteractionCommands`, `warnRemovedCLIReferences`, `warnOrphanHealthChecks`).
픽스처 테스트는 `validate_warnings_refs_test.go`. `default_plan` 미설정은 기존
`warnMultiplePlansWithoutDefault`가 이미 경고하므로 새 규칙을 두지 않고 문서(USAGE.md)에만 명시했다.

설계 메모:
- plan `services` 검사는 엔트리의 `runners.compose.services` 맵이 있을 때만 동작한다. compose 파일은
  읽지 않는다(TASK-248의 "validate는 파일 I/O 없음" 원칙).
- `environment: ${ENV:-dev}`처럼 런타임에 선택되는 축은 판단 불가라 그 축의 미참조 경고를 생략한다.
- 제거된 CLI 참조는 로더가 보존하는 문자열만 본다. YAML 주석은 검사할 수 없다. 사용자가 같은 이름의
  interaction을 정의한 동사(예: `interaction.clean`)는 정상 명령이므로 제외한다.
- `docs/51-flowcheck-rules.md`는 flowcheck 규칙 문서라 규칙 목록 대신 `RemovedCommands()`와
  `phantom-command`의 분업만 한 단락 추가했다. 경고 목록은 USAGE.md "config validate"에 있다.
- 새 규칙이 `examples/` 4개에서 실제 죽은 선언(no-op `entry_overrides`, 미참조 `stg`/`cloud`/
  `docker-host`)을 잡아 예시를 수정했다. `TestExamplesStrictCleanExceptComposeAbsence`가 그 회귀 가드다.

human 증거 — gizzahub/matdosa는 소유자가 이미 dogfood 중 수정했으므로(위 Dogfood evidence) `git show HEAD:dva.yml`
버전으로 실행했다. 현재 working tree에서는 두 저장소 모두 새 경고가 0건이다.

gizzahub HEAD (`bin/dva validate`, 5 plan에서 11개 서비스):

```
[warn] semantic: plans.apps.entries[0].services: temporal-postgres, temporal, temporal-ui, zookeeper, kafka, prometheus, grafana, alertmanager, postgres-exporter, redis-exporter, node-exporter not declared under stack.compose.runners.compose.services (declared: adminer, backend, frontend, grabber, infocenter, jaeger, portainer, postgres, redis, redis-insight, sigdock-idp); ...
[warn] semantic: plans.full.entries[0].services: (같은 11개) ...
[warn] semantic: plans.monitoring.entries[0].services: prometheus, grafana, postgres-exporter, redis-exporter, node-exporter not declared ...
[warn] semantic: plans.tools.entries[0].services: (같은 11개) ...
[warn] semantic: plans.workflow.entries[0].services: temporal-postgres, temporal, temporal-ui, zookeeper, kafka not declared ...
```

matdosa HEAD:

```
[warn] semantic: environments.dev: no plan selects it via environment:, so its values never apply; reference it from a plan or remove it
[warn] semantic: environments.test: no plan selects it via environment:, so its values never apply; reference it from a plan or remove it
```

검증: `make build`, `make test`, `make lint`, `make doc-check` 통과.
