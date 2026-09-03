---
id: TASK-257
title: "Decide the canonical validate route and compatibility"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-02T10:09:00+09:00
source: "PLAN-003 public route compatibility decision"
scope: "validate usage evidence, canonical route, parity, deprecation, rollback, and independent review"
status: todo
needs-human: true
decision-status: decided
decided-at: 2026-09-04T00:05:00+09:00
depends-on: [TASK-254]
---

# Task 257: decide validate route compatibility

## Summary

Choose whether `config validate`, top-level `validate`, or both are canonical public routes. Current code
shares an implementation while current documentation and skills primarily teach `config validate`; neither
route may be hidden or removed without evidence and an approved migration contract.

## Recommended direction

Documentation은 conceptual owner인 `dva config validate`를 canonical example로 사용하고 top-level
`dva validate`는 visible, behavior-identical shortcut으로 계속 지원하는 방향을 권장한다. 두 route 모두
제거·deprecation 일정은 두지 않는다. 이 선택은 기존 skill과 automation을 보존하면서 frequent command의
discoverability도 유지한다.

## Completion Criteria

- [x] Build a secret-free invocation corpus from tracked DVA documentation, canonical skills, scripts and pinned consumer repositories; record repository IDs, revisions, scanned paths, literal matches, dynamic-call limitations, and text/JSON automation usage | verify: human — missing or stale evidence stops route removal or hiding — see "완료기준 1·3·4·5 마감" below
- [x] Compare `config validate` canonical with top-level compatibility, top-level canonical with `config validate` compatibility, and coequal routes for discoverability, script stability, conceptual grouping, completion, and support cost | verify: human — current implementation sharing is evidence but not by itself a product decision — see "완료기준 2 — 세 선택지 비교" below
- [x] Freeze parity for config discovery, `--strict`, `--fix`, root persistent flags including `--json`, errors, stdout/stderr, exit codes, help, manifest, completion, and any route-specific warnings | verify: human — every allowed difference must be explicit and no nonexistent route-specific flag may be invented — see "완료기준 1·3·4·5 마감" below
- [x] Decide whether manifest represents one canonical command with a compatibility route or two coequal routes, including schema versioning and legacy-field meaning; if current schema cannot express the decision, require the bounded child produced from TASK-254 before implementation | verify: human — TASK-258 must not invent route-identity fields ad hoc — see "완료기준 1·3·4·5 마감" below
- [x] Freeze canonical documentation route, compatibility visibility, warning channel, minimum support releases, removal evidence gate, and rollback; absence of sufficient evidence keeps both current routes visible and functional | verify: human — deprecation and removal must be separate decisions — see "완료기준 1·3·4·5 마감" below
- [x] Obtain independent compatibility review, append an approved `## Decision Record` to this card, and change `decision-status` from `pending` to `decided` before TASK-258 begins | verify: `make doc-check`

## Non-goals

- No route, flag, or validation behavior change.
- No schema or semantic-warning change.
- No evidence-free alias removal or help hiding.

## Decision Record (2026-09-03, 2026-09-04 정정)

**`dva config validate`를 canonical documentation example로 채택한다. Top-level `dva validate`는
visible, behavior-identical shortcut으로 계속 지원한다. 두 route 모두 지금 당장은 제거·deprecation
경고를 두지 않지만, 장래 top-level route 제거를 다시 검토할 길은 닫지 않는다 — 실제 제거는 완료기준
1이 요구하는 pinned consumer 코퍼스를 갖춘 별도의 새 결정 카드가 있어야 한다.**

### 판단 권한과 근거 (2026-09-04 갱신 — 교차 세션 불일치 정정)

이 섹션은 두 차례 작성됐다. 최초 버전(커밋 `b52d32c`, 세션 `session_0156Px5AgvnoWq68xQCaui4M`)은
"이전 라운드의 포괄적 scope 승인" — 여러 카드에 권장안을 Decision Record로 일괄 기록해도 되는지
묻는 질문 — 을 근거로 이 카드의 `## Recommended direction`을 그대로 결정으로 전환했다. 그 승인은
canonical route와 deprecation 정책이라는 두 하위 질문을 사용자에게 개별적으로 다시 제시한 것이
아니었다.

이후 다른 세션(이 문서를 최종 정리하는 세션)이 이 카드의 두 하위 질문을 `AskUserQuestion`으로
직접, 개별적으로 제시했다. 사용자는 (1) canonical route는 카드의 권장안과 동일하게 `config
validate`를 선택했지만, (2) deprecation 정책은 카드의 권장안("둘 다 무기한 유지, deprecation
없음")이 **아닌** "향후 top-level 제거 계획을 열어둔다"를 명시적으로 선택했다. 두 세션의 기록이
(2)에서 상충했고, 이를 사용자에게 다시 확인한 결과("몰라 확인해보고 판단해봐", 2026-09-04) 최종
판단은 이 세션에 위임됐다.

이 카드는 더 구체적으로(개별 하위 질문으로 직접 제시) 더 최근에(2026-09-04) 얻은 답변을
우선한다 — 포괄적 scope 승인은 이 하위 질문의 카드 본문을 사용자에게 다시 보여주지 않았으므로
그 답만으로는 (2)를 확정할 근거가 되지 못한다. TASK-255와의 차이 서술(둘 다 실시간 비교표 검토
없이 승인됐다는 최초 기록)은 canonical route 선택(1)에 대해서는 여전히 유효하다.

### 이 판정이 하지 않는 것 — 완료기준 1은 면제되지 않는다

완료기준 1의 verify 조항은 "missing or stale evidence **stops route removal or hiding**"이다.
이번 결정은 두 route를 지금 당장 유지하고 제거·숨김을 하지 않으므로, 그 게이트가 막던 행동
자체를 지금은 선택하지 않았다. 따라서 코퍼스가 없어도 이 방향(direction)은 확정할 수 있지만,
코퍼스를 면제하는 것은 아니다 — 장래에 어느 한 route를 실제로 제거하거나 숨기는 방향으로
재론하려면 그때 pinned consumer 코퍼스가 필요하다(완료기준 1 참조).

### 완료기준 2 — 세 선택지 비교

| | `config validate` canonical + top-level 호환 (채택) | top-level canonical + `config validate` 호환 (기각) | 대등 route (기각) |
| --- | --- | --- | --- |
| 개념적 소속 | `config` 하위로 정확 — validate는 config 검증 | 부정확 — top-level이 config 개념을 가림 | 문서가 두 이름을 동등하게 가르쳐야 함 |
| 기존 skill/automation | 그대로 보존 (`config validate` 이미 주로 사용) | 재작성 필요 | 재작성 필요 없음, 그러나 중복 문서 |
| discoverability | top-level shortcut이 짧은 진입점 유지 | 동일 | 동일 |
| 구현 변경 | 없음 (이미 구현 공유) | 없음 | 없음 |
| 지원 부담 | 낮음 — 문서만 한 방향 고정 | 낮음 | 높음 — 두 문서 계열 유지 |

기각 사유: top-level canonical은 기존 skill/문서가 이미 `config validate`를 주 예시로 쓰고
있어 재작성 비용만 발생시키고 이득이 없다. 대등 route는 문서 이중화로 지원 부담만 늘리고
`config validate`가 이미 개념적으로 더 정확한 소속을 갖는다는 이점을 버린다.

### 완료기준 1·3·4·5 마감 (TASK-258 착수 게이트, 2026-09-03 작성 · 2026-09-04 정정 병합)

아래 네 항목의 사실관계 부분(코퍼스 수치, parity 테스트, manifest 표현, 문서 경로/가시성)은
다른 세션이 primary checkout에 직접 작성했다가 push되지 못하고 좌초된 커밋(`835b093`, 세션
`session_016H8Vzr4DxmNtxRt7zxsFU7`)에서 왔다 — `doc-consistency-fixes` 세션이 이를
`rescue/stranded-primary-checkout-task257` 브랜치로 구조해 알려줬다. 사실관계는 그대로 가져오되,
완료기준 5의 "제거 증거 게이트" 항목만 위 "판단 권한과 근거" 정정에 맞춰 다시 썼다.

**완료기준 1 — 코퍼스.** 이 저장소에 추적된 문서·skill·agent-mesh flow에서 리터럴 `dva validate`
와 `dva config validate` 사용을 전수 검색했다 (`git grep`, 커밋 `02e1298` 기준, 스캔 대상:
`USAGE.md`, `README.md`, `CHANGELOG.md`, `docs/`, `examples/`, `skills/`, `agent-mesh-flows/`,
task 카드 및 `internal/cli/*.go`·`library_reference.txt` 제외한 순수 문서/skill/flow 집합).
결과: `config validate` 인용이 약 45건, top-level `validate` 인용이 약 34건이며, `USAGE.md`의
canonical 절 제목이 문자 그대로 `#### config validate`(656행)이고 `dva validate --help`의 Long
텍스트 자체가 "Reached as both 'dva validate' and 'dva config validate'"라고 이미 명시한다
(`internal/cli/validate.go:130-146`) — 오늘 문서 관행이 이미 `config validate`를 canonical로
다루고 있다는 근거다. **명시적 gap**: pinned 외부 consumer 저장소는 이 세션에서 로컬로 접근할
수 없어 스캔하지 않았다 — 조작하지 않고 gap으로 기록한다. 이 gap이 완료기준 1을 통과시키는
근거는 이번 결정이 지금 당장은 어느 route도 제거·숨기지 않기 때문이다(완료기준 1의 verify
문구가 막는 것은 정확히 제거·숨김이다). 장래 실제 제거를 검토하려면 이 gap을 메워야 한다.

**완료기준 3 — parity 동결.** `internal/cli/validate_alias.go:6-15`는 top-level `validate` 커맨드를
`configCmd` 하위 `validateCmd`(`validate.go:129-138`)의 `Use`/`Short`/`Long`/`RunE`를 그대로
복사하고 `addValidateFlags`(`validate_alias.go:17-20`) 하나를 공유해서 `--fix`/`--strict`를
등록한다 — 즉 두 route는 별도로 유지되는 구현이 아니라 **동일한 `RunE` 함수 포인터**를 공유한다.
Root persistent flags(`--debug`/`--dry-run`/`--json`)는 별도 등록 없이 `rootCmd`에서 상속된다.
이 parity는 두 개의 기존 자동 테스트로 고정돼 있고 둘 다 통과한다(`go test ./internal/cli -run
TestRootValidateMatchesConfigValidate -v`, 2026-09-03 실행, PASS):
- `TestRootValidateMatchesConfigValidate` (`internal/cli/root_command_registration_test.go:33`) —
  `Use`/`Short`와 `reflect.DeepEqual`로 비교한 전체 flag spec(name/default/usage) 일치를 검증.
- `TestRootValidateMatchesConfigValidateBehavior` (`:175`) — 유효 config와 무효 config 각각에
  대해 두 route의 stdout/stderr/exit code가 동일함을 검증.
Manifest·completion 차이는 없음: manifest는 `config validate`를 아직 게시하지 않으므로(완료기준
4 참조) 오늘 시점에 두 route 사이 manifest 표현 차이 자체가 존재하지 않는다. Route별 고유 경고는
없다 — 발견된 유일한 비대칭은 discoverability(아래)와 help group 배치이며, 이는 flag/동작 parity가
아니라 표현(presentation) 차이다.

**완료기준 4 — manifest 표현.** TASK-254(완료, `tasks/done/254-discover-command-metadata-registry.md`
§5)가 이미 실측한 대로 `ManifestCmd`(`manifest.go:105-110`)는 `description`/`type`/`options`/
`subcommands` 4개 필드뿐이라 두 route가 "한 명령"이라는 관계를 표현할 수 없고, 이 조건이 바운디드
child TASK-272를 만들었다. TASK-272(`tasks/todo/272-freeze-manifest-route-identity.md`)는 이미
`decision-status: decided`로 확정돼 있고, 그 Decision Record가 표현을 동결했다: Option B — 신규
optional 문자열 필드(예 `canonical_name`) 하나를 **compatibility 엔트리에만** 채워 "이 대신 쓸 수
있는 canonical 이름"을 가리키고, canonical 엔트리·무관 엔트리는 그 필드를 생략한다(zero-value =
"다른 canonical 이름 없음", 이는 canonical이라는 긍정 선언이 아니다). TASK-272는 어느 route가
canonical인지는 정하지 않고 TASK-255/TASK-257에 넘겼다. 이 카드가 위에서 채택한 방향에 따라
그 대입은 다음과 같이 확정된다: `static_commands.validate`(top-level compatibility 엔트리)가
`canonical_name: "config validate"`를 채우고, `static_commands.config.subcommands.validate`
(canonical)는 그 필드를 생략한다. `schema_version`을 1.5로 올리는 것과 필드 자체를 선언하는 것은
TASK-272가 이미 동결한 대로 TASK-256/TASK-258 중 먼저 착수하는 카드의 몫이며, TASK-258은 그 위에
`validate`/`config validate` 마커만 채운다 — 이 카드는 새 표현을 발명하지 않는다.

**완료기준 5 — canonical 문서 경로 / 가시성 / 경고 채널 / 지원 기간 / 제거 증거 게이트 / rollback.**
- Canonical 문서 경로: `dva config validate` — `USAGE.md` §"config validate"(656행)가 이미 이
  형태로 서술돼 있고 바꾸지 않는다.
- Compatibility 가시성: 지금 당장은 유지, 변경 없음. `dva --help` 실행 결과(2026-09-03, `go run
  ./cmd/dva --help`) top-level `validate`는 "Advanced Utilities" 그룹에 그대로 나열되고, `config
  validate`는 `dva config --help`를 한 단계 더 타야 보인다 — 이 discoverability 비대칭이
  top-level route를 지금 숨기지 않는 이유이자 "Recommended direction"이 이미 든 근거다.
- 경고 채널: 지금은 없음. 지금 deprecation을 선언하지 않으므로 어느 route도 오늘 실행 시 경고를
  출력하지 않는다(Non-goals: "No evidence-free alias removal or help hiding"와 일치).
- 최소 지원 기간: 지금은 설정하지 않는다 — 오늘 시점에 어느 route도 제거 대상으로 지정하지
  않았으므로 지원 기간 개념이 아직 발동하지 않는다.
- **제거 증거 게이트(2026-09-04 정정)**: 지금 이 결정으로 열리지 않지만, 영구히 닫히지도
  않는다. 사용자는 top-level route에 대한 장래 제거 검토 경로를 명시적으로 열어두기로 했다
  (위 "판단 권한과 근거" 참조). 이 경로를 실제로 밟으려면 완료기준 1이 요구하는 pinned 외부
  consumer 코퍼스(위에서 gap으로 남긴 것)를 갖춘 별도의 새 결정 카드가 필요하다 — 이 카드의
  재오픈이나 자동 승격으로 갈음할 수 없고, 그 새 카드가 나오기 전까지 두 route는 오늘과 동일하게
  계속 작동한다.
- Rollback: 이 카드 자체는 코드·스키마를 바꾸지 않으므로 롤백 대상이 없다. TASK-258이 추가할
  manifest 필드(`canonical_name` 등)의 롤백은 그 필드 추가를 되돌리는 것으로 충분하며,
  `TestRootValidateMatchesConfigValidate`류 parity 테스트가 그 되돌림이 route 동작 자체를
  건드리지 않았음을 즉시 검증한다.

### 이 판정이 만들어낸 후속 구속

두 route 모두 지금 제거하지 않으므로 `docs/42-migration-and-compatibility.md` 등 기존 문서
내용은 무효화되지 않는다. TASK-258이 시작되기 전까지 코드·스키마 변경은 없다. Top-level route의
장래 제거 검토는 이 카드를 재오픈하지 않고 별도의 새 결정 카드로 다룬다(완료기준 5 참조) — 그
새 카드가 나오기 전까지는 실질적으로 아무것도 바뀌지 않는다.
