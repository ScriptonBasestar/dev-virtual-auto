---
id: TASK-244
title: "Validate duplicate plan declarations and missing multi-plan defaults"
type: feature
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-01T19:20:00+09:00
source: "PLAN-002 self-contained D6/D7 contract"
scope: "semantic validation warnings, stable text/JSON output, strict behavior, tests, current reserved-command documentation"
status: done
---

# Task 244: detect plan declaration drift

## Summary

Add the two frozen plan diagnostics below without depending on ignored reports, claiming full runtime
equivalence, or changing default validation exit behavior.

## Problem

Repositories can carry two plan names whose agreed declaration fields are equal, or multiple plans
without an explicit default. The runtime catches the second case only when a bare lifecycle command
is attempted, and neither condition has a general semantic warning.

The fingerprint is deliberately narrower than a resolved execution plan. It compares plan
`environment`, `site`, `vars`, `endpoint_tags` and entry `name`, `runner`, `order`, `services`,
`depends_on`, `vars`. Map keys are sorted, list order is preserved, and nil/empty collections are
normalized as equal. The warning must describe equal declaration fields, not claim that site overrides
and every runtime input are identical.

## Completion Criteria

- [x] D6 compares exactly the plan and entry fields listed in this card, sorts map keys, preserves list order, treats nil/empty collections as equal, emits each unordered pair once, and never recommends a canonical name | verify: human — focused tests must name every compared field and pin deterministic pair ordering
- [x] D6 fixtures cover equal declarations, every one-field difference, map-order-only, list-order difference, nil/empty equality, and subproject namespaces; compare only within one owning config/`SubprojectPath` partition, exclude canonical/import-alias keys that reference the same plan pointer, and do not compare root↔child or child-A↔child-B lookalikes | verify: `go test ./internal/config -count=1`
- [x] D7 warns only for two-or-more plans without `default_plan`, excludes the single-plan implicit default contract, and does not duplicate the compose-split-specific remedy | verify: `go test ./internal/config -count=1`
- [x] Default text/JSON validation remains non-fatal while `--strict` promotes both new warnings to the existing non-zero contract; output order and category are stable | verify: `go test ./internal/cli -count=1`
- [x] Current-state sources report the 24-command set including `skill`: `docs/43` current status, `USAGE.md`, `skills/dva-config/references/schema-reference.md`, its generated library projection, and `docs/51-flowcheck-rules.md`; historical 27→23 transition text in `docs/43` and `CHANGELOG.md` remains historical and separately notes the later addition where needed | verify: `make generate && make check-generate && make doc-check`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make commit-check`

## Non-goals

- No automatic deletion, rename, alias, or canonical-plan recommendation.
- No cross-repository plan-name policy.
- No change to bare lifecycle runtime selection.

## Evidence

D6/D7은 `internal/config/validate_warnings.go`에 추가되고 `ValidateWarnings()`의
고정 순서 연결부(`:70-71`)에 편입됐다. 코드에는 D 번호 레지스트리가 없다 — D6/D7은
카드 문서상의 라벨이며 구현은 일반 semantic 경고 두 개다.

- **D6** `warnDuplicatePlanDeclarations()` — 이름을 정렬한 뒤 `names[i+1:]`로 순회해
  비순서쌍을 한 번만 방출한다. 비교는 `plansHaveEqualDeclaration()`이 담당하며 카드가
  명시한 필드만 본다: plan `Environment`/`Site`/`Vars`/`EndpointTags`, entry `Name`/
  `Runner`/`Order`/`Services`/`DependsOn`/`Vars`. 맵은 `maps.Equal`(키 순서 무관, nil==empty),
  슬라이스는 `slices.Equal`(순서 유의미). `Description`은 의도적으로 제외 — 선언 동등성이
  아니라 서술이다.
- **파티션/별칭 제외** — 두 축으로 걸러진다. (1) `SubprojectPath`가 다르면 비교하지 않아
  root↔child, child-A↔child-B 유사쌍이 절대 붙지 않는다. (2) `subproject.go:113-137`의
  import 별칭 루프는 canonical 키와 `as:` 별칭 키에 **같은 `*PlanConfig` 포인터**를 넣으므로
  `planA == planB` 포인터 비교로 배제된다. 값 비교가 아니라 포인터 동일성이라는 점이 핵심이고,
  `TestDuplicatePlanDeclarationsExcludesRealAliasImport`가 임시 디렉터리에 실제 부모/자식
  dva.yml을 쓰고 진짜 `Load()`를 통과시켜 이 동일성이 유지되는지 pin한다.
- **D7** `warnMultiplePlansWithoutDefault()` — 가드는 `len(c.Plans) < 2 || c.DefaultPlanName != ""`.
  조사 단계에서 제안된 `DefaultPlanSource() == "none"` 가드는 채택하지 않았다. 그 값은
  `default_plan`이 선언됐지만 존재하지 않는 plan을 가리킬 때도 "none"이 되는데, 그 경우는
  `validate.go:186-188`이 이미 하드 에러(`default_plan '%s' not found in plans`)로 거부한다.
  같은 조건에 경고를 겹치면 하드 에러를 중복 서술하게 되므로 `DefaultPlanName == ""`만 본다.
  구현부에 이 근거를 주석으로 남겼다.
- **compose-split 중복 회피** — `composeSplitWarning`(`:637`)은 이미
  "set default_plan to a minimal plan that selects one entry"라는 compose 특화 처방을 낸다.
  D7은 그 처방을 반복하지 않고 bare lifecycle 커맨드가 plan 이름을 요구한다는 일반 사실만
  서술한다.
- **메시지 접두사** — 저장소 관례상 특정 키 경로를 지목하는 경고만 `stack: `,
  `health_checks.%s: ` 같은 접두사를 쓴다. 설정 수준 경고(`warnMissingDefaultMode`,
  `warnNoPlansHint`)는 접두사가 없고, D7은 `default_mode` 경고와 문형까지 대응하므로
  동일하게 접두사를 두지 않았다.
- **strict 승격은 무상** — `validate.go:190`이 `len(semanticWarnings) > 0`을 그대로 보므로
  `ValidateWarnings()`에 들어간 순간 `--strict` 비영(非零) 종료 계약에 편입된다. 새 카테고리를
  만들지 않아 텍스트는 `[warn] semantic: `, JSON은 `"semantic"`으로 렌더된다.
- **criterion 5는 이미 in-tree 충족** — 예약어 SSOT(`internal/config/reserved.go`)는 24개이고
  `docs/43:12`("24개, 재구성 당시 23개, 이후 skill 추가"로 역사 서술 분리), `docs/51:77`,
  `USAGE.md:1045`, `skills/dva-config/references/schema-reference.md:17`이 모두 24개로 일치한다.
  이 브랜치에서 추가 수정 없이 게이트가 통과했다.
- **부수 정리** — `agent-mesh-flows/dva-improve.yaml`에 AUTOGEN 마커 **밖**으로 손으로 복제된
  예약 커맨드 목록이 있어 삭제했다(커밋 73f24b3). `make check-generate`는 마커 안쪽만 비교하므로
  마커 밖 사본은 구조적 사각지대다 — 갱신이 아니라 삭제가 옳다. 삭제 후 `make generate`가
  추가 변경을 만들지 않는 것으로 그 블록이 순수 수작업본이었음을 확인했다.

테스트: `internal/config/validate_warnings_plan_test.go`(14개 — 필드 10종 1필드차 매트릭스,
맵 순서 무관, 리스트 순서 유의미, nil/empty 동치, 파티션 격리/동일 파티션 내 경고, 실제 alias
import, 50회 순서 안정성, D7 단일 plan/유효 default/무효 default 케이스),
`internal/cli/validate_plan_warnings_test.go`(2개 — 텍스트 접두사·strict 승격, JSON 카테고리).

## Gate results

워크트리 `claude__mst__feat__task244-plan-drift`에서 실행:

| gate | 결과 |
| --- | --- |
| `make generate` | exit 0, 생성물 변경 없음 |
| `make check-generate` | exit 0 |
| `make doc-check` | exit 0 (flowcheck: 24 built-in command(s), 결함 없음) |
| `make lint` | exit 0 (go vet, gofmt -s 339 files, 0 issues) |
| `make test` | exit 0 (config 72.5%, cli 78.4%) |
| `make test-integration` | exit 0 |
| `make commit-check` | exit 2 — **기존 baseline** |

`commit-check`의 위반 3건(`a6666c1a`, `6ab9c643`, `095f525b`)은 모두 이 브랜치 이전에
master로 푸시된 커밋의 subject scope 누락이며 이미 재작성 불가능하다. TASK-271에서 기록한
baseline과 동일하고 이 작업이 악화시키지 않았다. 이 브랜치의 커밋은 모두 scope를 갖는다.

알려진 비차단 사항: `internal/config/validate_warnings.go`는 이 변경 **이전에** 이미 저장소
파일 크기 lint 훅 임계치를 넘어 있었고(703줄), 이번 추가로 ~784줄이 됐다. 파일 분할은 이
카드 범위 밖이라 손대지 않았다.
