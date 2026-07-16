---
id: TASK-017
title: "Decide stack runners.docker / runners.native semantics"
type: chore
priority: P2
status: todo
effort: S
created-at: 2026-07-16T20:15:00+09:00
needs-human: true
source-run-id: 20260716T091912Z-73dc094
discovered-in: TASK-009
decision-status: pending
decision-recommendation: "Option A — map runners.docker to the docker plugin"
decision-confidence: medium
---

# Decision 017: stack runners.docker / runners.native 의미 확정

## Summary

`stack.<entry>.runners` 아래의 `docker`와 `native`는 lifecycle 플러그인 설정이 아니라
application 스타일 설정으로 디코딩된다. 그래서 `runners.docker`는 `dva stack up`에서
동작하지 않는다. docker는 실제 등록된 플러그인이고 중첩 `docker:` 형태는 동작하므로
이 비대칭은 사용자를 혼란시킨다. schema(TASK-010)가 이 둘을 허용할지 거부할지
결정하려면 의미를 먼저 확정해야 한다.

## Evidence

| 항목 | 증거 |
| --- | --- |
| docker는 등록된 플러그인 | `internal/lifecycle/plugin_type.go` — registry에 `docker` 포함 |
| 중첩 `docker:` 형태는 동작 | `internal/config/lifecycle.go:388` → `*DockerPluginConfig` |
| `runners.docker`는 다른 타입 | `internal/config/lifecycle.go:265` → `*DockerRunnerConfig` |
| 두 타입은 다름 | `:75` (image/run/build/command/ports/volumes/env/options) vs `:699` (image/name/ports/volumes/env/options) |
| `DockerRunnerConfig` 소비처 없음 | grep 결과 어떤 플러그인도 읽지 않음 |
| `NativeRunnerConfig` 소비처 | `internal/lifecycle/resolver.go:219` — plan 경로 `WorkingDir`만 |
| `native`는 플러그인이 아님 | registry에 없음 → `NewPlugin("native")` 실패 |

TASK-009에서 확인된 실제 영향: `runners.docker`가 플러그인 이름으로 해석되면
`DockerPlugin.Up`이 `cfg == nil`일 때 `&Result{}, nil`을 반환해 **조용히 성공**한다
(`internal/lifecycle/docker.go:19-21`). TASK-009는 `DetectPlugin()`이 이 이름을 반환하지
않게 해 이 회귀를 막았고, 현재는 명시적 오류로 실패한다.

## Options

### 옵션 A — `runners.docker`를 docker 플러그인에 연결 (**추천**, confidence: medium)

`decodeRunnerNode`의 docker case를 `*DockerPluginConfig`로 변경해 stack에서 실제 동작하게 한다.
Effort: S · Risk: medium

- 장점: docker는 이미 등록된 플러그인이고 중첩 `docker:` 형태와 동작이 일치한다;
  `runners.<name>` == 해당 플러그인이라는 단순한 규칙이 성립한다.
- 단점: `DockerRunnerConfig`의 `run`/`build`/`command` 필드가 사라져 이를 쓰는 기존 설정이
  깨질 수 있다 (현재 저장소 `examples/`에는 해당 설정 없음); `native`는 여전히 플러그인이
  아니므로 비대칭이 남는다.

### 옵션 B — `docker`/`native`를 stack runners에서 schema로 거부

이 둘을 applications 전략 전용으로 간주하고 `stack.runners`에서는 검증 단계에서 거부한다.
Effort: S · Risk: medium

- 장점: 현재 코드 동작과 정확히 일치한다 (동작하지 않는 것을 허용하지 않음);
  런타임의 모호한 오류 대신 `validate` 단계에서 명확히 실패한다.
- 단점: `resolver.go:219`가 plan 경로에서 `NativeRunnerConfig`로 `WorkingDir`을 읽으므로
  `runners.native`는 plan에서 의미가 있다 — 단순 거부 시 이 동작이 깨진다.

### 새 증거 (2026-07-16, convergence check 2) — 옵션 A를 강화함

`claude-plugin/skills/dva/references/patterns.md:61`의 Migration Map은 이미 사용자에게
다음 이전을 지시하고 있습니다:

| Old shape | New shape |
| --------- | --------- |
| `applications.<name>` | `stack.<name>.runners.native/docker` |

즉 **출시된 문서가 이미 `stack.<name>.runners.docker`를 의도된 형태로 안내**하고 있습니다.
이는 이 저장소에서 발견된 설계 의도 기록에 가장 가까운 증거이며, `runners.<name>`이
플러그인을 뜻한다는 옵션 A 방향을 가리킵니다. 실제로 이 지시를 그대로 따르면
`dva validate`는 통과하지만 `dva stack up`은 `unknown lifecycle plugin ""`으로 실패합니다
(TASK-024).

반대 방향의 증거도 함께 고려해야 합니다: runner 구조체가 application 형태
(`dir`/`build`/`run`)라는 점은 이들이 원래 `applications:` 전략용으로 설계되었음을 시사합니다.

이 결정은 TASK-024를 blocking합니다.

### 추천 근거

docker는 `registry.go`에 등록된 실제 플러그인이고 중첩 `docker:` 형태는 이미 동작한다.
`runners.docker`만 동작하지 않는 것은 일관성 없는 예외이며, TASK-009에서 확인했듯 조용한
no-op를 유발할 수 있는 구조적 함정이다. 다만 `DockerRunnerConfig`의 `run`/`build`/`command`를
사용하는 기존 설정이 있는지 먼저 확인해야 한다. `native`는 플러그인이 아니므로 옵션 A를
택하더라도 plan 전용(`WorkingDir`)으로 남겨야 한다.

**Confidence: medium** — 코드 증거는 명확하나, docker/native runner 형태가 원래
applications용으로 설계된 것인지 stack용인지에 대한 설계 의도 기록을 찾지 못했다.
제품 소유자의 의도 확인이 필요하다.

## Completion Criteria

- [ ] 옵션 A/B 중 하나가 선택되어 이 파일에 기록된다 | verify: `human — 제품 소유자가 stack runners의 docker/native 의미를 확정해야 함 (AI가 결정할 수 없음)`
- [ ] 결정에 따라 TASK-010의 schema 범위가 확정된다 | verify: `human — 결정 기록 후 TASK-010 criteria 갱신`

## References

- [unified.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md) — G1/G2 맥락
- [009-fix-runners-plugin-resolution.md](../_archive/009-fix-runners-plugin-resolution.md)
