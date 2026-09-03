---
id: TASK-255
title: "Decide the kubectl canonical route and ktl compatibility"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-02T10:07:00+09:00
source: "PLAN-003 public route compatibility decision"
scope: "usage evidence, route naming, alias and reservation behavior, deprecation, rollback, and independent review"
status: todo
needs-human: true
decision-status: decided
depends-on: [TASK-254]
---

# Task 255: decide kubectl route compatibility

## Summary

Choose whether `ktl` remains canonical, `kubectl` becomes canonical with `ktl` compatibility, or the route
remains unchanged for lack of sufficient evidence. Do not register a new top-level name until this card is
approved.

## Recommended direction

현재 `ktl` 하나를 유지하는 것을 기본 권장안으로 둔다. 충돌 corpus green은 필요한 안전 조건일 뿐 새
top-level route의 사용자 가치를 증명하지 않는다. Pinned usage evidence가 반복되는 발견성 문제나 명확한
`kubectl` 수요를 보여주고 충돌도 없을 때만 `kubectl`을 canonical로 추가하고 `ktl`은 visible
compatibility route로 유지한다. 제거 날짜는 미리 약속하지 않고, evidence가 불완전하면 현행을 유지한다.

## Completion Criteria

- [ ] Build a secret-free invocation corpus across tracked DVA documentation, skills, scripts and pinned canonical consumer repositories; record repository IDs, revisions, scanned paths, literal matches, unresolved dynamic calls, and scanner limitations | verify: human — missing canonical repositories, unpinned revisions, or unexplained dynamic invocations stop a rename decision
- [x] Compare `ktl` canonical, `kubectl` canonical with compatibility, and no-change options for discoverability, typing cost, script compatibility, interaction collisions, completion, and support burden | verify: human — all three options and rejected reasons must be recorded
- [ ] If names coexist, freeze which name is canonical, whether the other is a hidden or visible compatibility route, how both names remain reserved, and parity across root flags, entry selection, passthrough argv, help, manifest, completion, debug output, exit status, signals, and process replacement | verify: human — no unspecified alias behavior may reach implementation
- [ ] Preserve the current collision matrix unless a separate approved contract changes it: config load warning, `config validate` error, bare-name built-in precedence, exact interaction reachability through `dva run <name>`, and reserved-prefix namespace rejection must be explicit for every coexisting name | verify: human — fail closed must not be interpreted as removing the explicit `run` escape route
- [ ] Decide whether manifest represents one canonical command with compatibility routes or coequal routes, including schema versioning and legacy-field meaning; if current schema cannot express the decision, require the bounded child produced from TASK-254 before implementation | verify: human — TASK-256 must not invent route-identity fields ad hoc
- [ ] Freeze deprecation warning channel, minimum compatibility releases, removal evidence gate, rollback route, and documentation migration; absence of sufficient evidence selects the current `ktl` route | verify: human — deprecation and removal must be separate decisions
- [ ] Obtain independent compatibility review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided` before TASK-256 begins | verify: `make doc-check`

## Non-goals

- No route registration or reserved-name change.
- No kubectl runner behavior change.
- No compatibility removal in the same release that introduces a new canonical name.

## Decision Record (2026-09-03)

**`kubectl`을 canonical route로 승격한다. `ktl`은 visible compatibility route로 유지한다.**

### 판단 권한과 근거

2026-09-03 사용자가 이 질문을 독립된 선택지로 직접 제시받고 승격을 선택했다. 제시 시점에
아래 비용이 전부 명시돼 있었다 — 25번째 예약어와 그 회수 불가능성, `kubectl`을 interaction
이름으로 쓰던 설정이 `config validate`에서 exit 1로 바뀌는 것, `docs/42-migration-and-
compatibility.md:157`이 스스로 무효 예시가 되는 것, "24"라는 숫자가 생성 블록을 포함해 여덟
곳에 박혀 있다는 것, 신규 parity 테스트가 필요하다는 것. 제시자의 권장안은 `ktl` 유지였고
사용자는 그것을 알고 승격을 선택했다.

이 카드의 `## Recommended direction`은 권장안이지 제약이 아니다. 권장안이 뒤집혔다는 사실
자체를 기록에 남긴다 — 나중에 "카드 권장안과 다르니 착오였을 것"이라는 근거로 재개할 수
없게 하기 위해서다.

### 이 판정이 하지 않는 것 — 완료기준 1은 면제되지 않는다

완료기준 1의 verify 조항은 "missing canonical repositories, unpinned revisions, or
unexplained dynamic invocations **stop a rename decision**"이다. 코퍼스는 아직 아무도
만들지 않았고, 이 판정은 그것을 만들어내지 않는다.

두 층을 구분한다.

- **방향(direction)**은 이 판정으로 확정됐다. 더 이상 사람 결정 대기 항목이 아니며,
  `ktl` 유지나 무변경은 재검토 대상이 아니다.
- **착수(implementation)**는 여전히 닫혀 있다. 완료기준 1이 rename에 건 게이트는 방향이
  정해졌다고 열리지 않는다 — 오히려 이제서야 구속력을 가진다. `ktl` 유지를 골랐다면 이
  코퍼스는 영영 불필요했다. 승격을 고른 결과로 필수가 됐다.

따라서 `decision-status`는 `decided`로 바꾸되 카드는 `todo/`에 남는다. 사람이 답할 것은
없고, 남은 것은 목표가 고정된 엔지니어링 작업이다. **TASK-256은 완료기준 1·3·4·5·6·7이
닫히기 전에는 시작하지 않는다.**

### 완료기준 2 — 세 선택지 비교 (이 절로 충족)

| | `ktl` 유지 (기각) | **`kubectl` canonical + `ktl` 호환 (채택)** | 무변경/보류 (기각) |
| --- | --- | --- | --- |
| 발견성 | kubectl 사용자가 이름을 추측할 수 없음 | 도구 이름 그대로 | `ktl` 유지와 동일 |
| 타이핑 비용 | 3자 | 7자 (호환 경로로 3자 유지) | 3자 |
| 스크립트 호환 | 영향 없음 | `ktl` 유지로 기존 스크립트 보존 | 영향 없음 |
| interaction 충돌 | 없음 | `kubectl` interaction이 exit 1로 전환 | 없음 |
| 예약어 | 24 | **25 — 회수 불가** | 24 |
| completion | 변경 없음 | 거의 무료 (TASK-254 실측) | 변경 없음 |
| 지원 부담 | 없음 | 두 이름의 parity를 영구 유지 | 없음 |
| 되돌리기 | N/A | **불가** | 질문이 다시 올라옴 |

기각 사유를 명시한다. `ktl` 유지는 비용이 0이고 되돌릴 수 없는 것이 없다는 점에서 가장
안전했으나, 사용자가 발견성 이득을 그 안전성보다 높게 평가했다. 무변경/보류는 코드 상태가
`ktl` 유지와 같으면서 판정만 남기지 않아, TASK-256 → TASK-261(P0) 사슬을 계속 막고 같은
질문을 재발생시킨다는 이유로 기각했다.

### 이 판정이 만들어낸 후속 구속

1. **완료기준 1의 코퍼스가 필수 선행 작업이 됐다.** rename에 걸린 hard stop이다.
2. **TASK-272가 선택적 상류가 아니라 하중을 받는 구조가 됐다.** TASK-254 §5의 측정이
   그대로 발동한다 — `ManifestCmd`는 필드가 4개뿐이라(`internal/cli/manifest.go:105-110`)
   두 이름이 *설명이 동일한 무관한 대등 항목*으로 나열되고, 어느 쪽이 호환 경로인지 표시할
   수단이 없다. 완료기준 5는 이 경우 "TASK-254에서 산출된 bounded child를 구현 전에
   요구한다"고 이미 규정하고 있으며, 그 child가 TASK-272다. 즉 완료기준 5는 TASK-272의
   판정으로 닫힌다.
3. **`reserved.go`가 24 → 25로 바뀌면 여덟 곳의 "24" 서술이 전부 틀린다** — `USAGE.md:1148`,
   `internal/cli/library_reference.txt:40,210`, `skills/dva-config/references/schema-
   reference.md:17`, `skills/dva/references/commands.md:318`, `agent-mesh-flows/shared/
   library/shared-guardrails.md:38`, `docs/43:12`, `docs/51:77`. 일부는 생성 대상이므로
   `make generate`와 `make check-generate`가 그 변경의 일부이지 후속 정리가 아니다.
4. **`docs/42-migration-and-compatibility.md:157`은 오늘 옳고 이 판정으로 틀려진다.**
   `dva kubectl`을 interaction 예시로 가르치고 있다. 완료기준 6의 "documentation migration"에
   포함된다. 저장소 내 `examples/`는 영향받지 않는다 — `kubernetes.yml:49`와
   `full-stack.yml:209` 모두 interaction 키로 `k8s`를 쓴다.
5. **완료기준 4의 충돌 매트릭스는 그대로 보존된다.** 두 이름 모두에 대해 config load 경고,
   `config validate` 오류, bare-name 우선, `dva run <name>` 정확 도달 경로, reserved-prefix
   거부가 명시돼야 한다. 특히 `dva run kubectl`이라는 escape route는 제거되지 않는다.
6. **제거 날짜는 약속하지 않는다.** `ktl`은 visible compatibility route이며, deprecation과
   removal은 완료기준 6이 규정한 대로 별개 결정이다. 이 판정은 removal을 승인하지 않는다.
