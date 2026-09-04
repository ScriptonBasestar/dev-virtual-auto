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
- [x] If names coexist, freeze which name is canonical, whether the other is a hidden or visible compatibility route, how both names remain reserved, and parity across root flags, entry selection, passthrough argv, help, manifest, completion, debug output, exit status, signals, and process replacement | verify: human — no unspecified alias behavior may reach implementation
- [x] Preserve the current collision matrix unless a separate approved contract changes it: config load warning, `config validate` error, bare-name built-in precedence, exact interaction reachability through `dva run <name>`, and reserved-prefix namespace rejection must be explicit for every coexisting name | verify: human — fail closed must not be interpreted as removing the explicit `run` escape route
- [x] Decide whether manifest represents one canonical command with compatibility routes or coequal routes, including schema versioning and legacy-field meaning; if current schema cannot express the decision, require the bounded child produced from TASK-254 before implementation | verify: human — TASK-256 must not invent route-identity fields ad hoc
- [x] Freeze deprecation warning channel, minimum compatibility releases, removal evidence gate, rollback route, and documentation migration; absence of sufficient evidence selects the current `ktl` route | verify: human — deprecation and removal must be separate decisions
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
없고, 남은 것은 목표가 고정된 엔지니어링 작업이다. **TASK-256은 완료기준 1·7이 닫히기
전에는 시작하지 않는다** — 3·4·5·6은 2026-09-04 상세 계약 동결로 닫혔다(아래
"완료기준 3·4·5·6" 절 참고).

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
3. **`reserved.go`가 24 → 25로 바뀌면 "24" 서술이 여러 곳에서 틀린다. 다만 대부분은 손으로
   고치면 안 된다.** 결정 제시 시점에 제시된 목록은 생성물과 원본을 구분하지 않아 부정확했고,
   생성 그래프를 추적해 아래로 교정한다.

   `tools/libgen/main.go:36-41,66-70`이 `config.ReservedCommands()`를 실제로 읽어
   `renderReserved`에서 `len(reserved)`로 개수를 산출하고 `reserved_commands` 블록을
   `agent-mesh-flows/shared/library/shared-guardrails.md`에 다시 쓴다. Makefile `generate`가
   그 library를 `internal/cli/library_reference.txt`로 합치고, flowgen이 flow YAML에
   인라인한다. 즉 **생성 경로의 개수는 자기 유지된다** — 손대면 `make check-generate`가 막는다.

   손으로 고쳐야 하는 곳은 5개 파일 6개 지점뿐이다.

   | 위치 | 비고 |
   | --- | --- |
   | `skills/dva-config/references/schema-reference.md:17` | canonical source. `agent-mesh-flows/shared/library/dva-schema.md`가 이 파일로의 **심볼릭 링크**라 한 번 고치면 `library_reference.txt:210`·`dva-improve.yaml:1039`·`dva-improve-guided/30-configure.yaml:332`로 전파된다 |
   | `skills/dva-config/references/schema-reference.md:147-152` | 같은 파일 안의 **두 번째 목록**. YAML 주석 형태로 `# (24 names.`로 끝난다. L17만 고치면 여기가 남는다 |
   | `skills/dva/references/commands.md:318` | 개수 없이 이름만 나열 — **grep할 숫자가 없다** |
   | `USAGE.md:1148` | 바로 아래 ASCII 표 `:1150-1153`도 함께 |
   | `docs/43-command-surface-restructure.md:12` | |
   | `docs/51-flowcheck-rules.md:77` | |

   **손대면 안 되는 함정 두 곳.** `CHANGELOG.md:244`는 "27개 → 23개; 같은 릴리스에서
   `dva skill`이 추가되어 이 릴리스의 예약어는 24개입니다"라는 릴리스 서술이고,
   `tasks/plan/002:178,197`은 "현재 구현을 24개 baseline으로 삼되"라는 결정 시점 사실이다.
   둘 다 24로 남아야 하며 일괄 치환은 이것들을 깬다.

   `make generate`와 `make check-generate`는 여전히 그 변경의 일부다 — 다만 나머지를
   갱신하기 위한 것이지, 위 6개 지점을 대신해 주지는 않는다.

   **부수 결함 — TASK-256에서 함께 처리할 후보.** `renderReserved`가 개수를 파생시키는데
   `schema-reference.md`는 같은 사실을 산문으로 두 번 하드코딩한다. 이 파일은 생성기의
   *입력*이면서 생성기가 이미 유도하는 값을 다시 적고 있어, 255의 결과와 무관하게 다음
   예약어 변경에서 조용히 어긋난다. 수동 갱신보다 libgen 마커를 씌우는 편이 맞다.
4. **`docs/42-migration-and-compatibility.md:157`은 오늘 옳고 이 판정으로 틀려진다.**
   `dva kubectl`을 interaction 예시로 가르치고 있다. 완료기준 6의 "documentation migration"에
   포함된다. 저장소 내 `examples/`는 영향받지 않는다 — `kubernetes.yml:49`와
   `full-stack.yml:209` 모두 interaction 키로 `k8s`를 쓴다.
5. **완료기준 4의 충돌 매트릭스는 그대로 보존된다.** 두 이름 모두에 대해 config load 경고,
   `config validate` 오류, bare-name 우선, `dva run <name>` 정확 도달 경로, reserved-prefix
   거부가 명시돼야 한다. 특히 `dva run kubectl`이라는 escape route는 제거되지 않는다.
6. **제거 날짜는 약속하지 않는다.** `ktl`은 visible compatibility route이며, deprecation과
   removal은 완료기준 6이 규정한 대로 별개 결정이다. 이 판정은 removal을 승인하지 않는다.

### 완료기준 3·4·5·6 — 상세 계약 동결 (2026-09-04)

위 "후속 구속" 절의 산문을 각 완료기준의 verify 조항에 맞춰 formal하게 얼린다. 새로운
product 판단은 없다 — 이미 존재하는 `ktlCmd` 구현(`internal/cli/kubectl.go`)과 TASK-257이
세운 `CanonicalName` 선례(`internal/cli/manifest.go:407-409`, `validate`/`config validate`
쌍)를 그대로 두 번째 이름 쌍에 적용한 것뿐이다.

**완료기준 3 — parity 명세.** `kubectl`과 `ktl`은 같은 기저 명령의 두 이름이므로, 아래 표에
없는 차이는 전부 금지된다(= "unspecified alias behavior"는 존재하지 않는다).

| 표면 | 명세 |
| --- | --- |
| root 플래그 | 동일. 둘 다 `consumeRootPersistentFlags`를 거친다(`kubectl.go`가 이미 그렇다) |
| entry 선택 | 동일 로직 재사용 — 단일 kubectl entry면 생략 가능, 복수면 첫 인자가 entry 이름 |
| passthrough argv | 동일 — entry 이후 인자를 그대로 하위 `kubectl` 바이너리에 전달 |
| help | `kubectl --help`가 canonical 설명을 보여주고, `ktl --help`는 같은 본문에 "compatibility name for `kubectl`" 한 줄을 덧붙인다 |
| manifest | 아래 "완료기준 5" 절 참고 |
| completion | 코드 변경 없이 cobra 등록만으로 두 이름 모두 자동 생성(TASK-254 §5 실측과 일치) |
| debug 출력 | 동일 — 어느 이름으로 불렀든 실행되는 하위 프로세스는 문자열 그대로 `"kubectl"`이다(`ExecReplace(e, "kubectl", ...)`, 이름 인자가 아니라 상수) |
| exit status | 동일 — `ExecReplace`가 프로세스를 대체하므로 종료 코드는 실행된 `kubectl` 바이너리의 것이다 |
| signals | 동일 — 대체된 프로세스가 직접 시그널을 받는다(`dva`는 관여하지 않는다) |
| process replacement | 동일 — 두 이름 모두 `dvaexec.ExecReplace`를 그대로 호출한다. 이 사실 자체가 "두 이름이 같은 명령"이라는 이 판정의 핵심 전제를 코드 수준에서 보증한다 |

**완료기준 4 — 충돌 매트릭스 보존.** "후속 구속" 5번 그대로 확정한다: config load 경고,
`config validate` 오류, bare-name 우선, `dva run <name>` 도달 경로, reserved-prefix 거부를
`kubectl`·`ktl` 둘 다에 대해 변경 없이 유지한다. `dva run kubectl` escape route는 제거되지
않는다. 이 완료기준은 "바꾸지 않는다"가 결정 내용이므로 별도 신규 명세가 필요 없다.

**완료기준 5 — manifest 표현.** TASK-272(2026-09-04 종결, `canonical_name` 마커 채택)가
정확히 이 자리에 쓰라고 얼린 필드를 그대로 적용한다.

```go
"kubectl": {Type: "passthrough"},
"ktl":     {Type: "passthrough", CanonicalName: "kubectl"},
```

`validate`/`config validate` 쌍(`internal/cli/manifest.go:407-409`)과 정확히 같은 형태 —
호환 경로(`ktl`)가 자신의 canonical 이름을 가리키고, canonical 쪽(`kubectl`)은 이 필드를
비운다. schema_version은 이미 1.5로 올라 있어(TASK-258) 추가 마이그레이션이 없다.

**완료기준 6 — deprecation/rollback 동결.** 완료기준 자체의 fallback 조항("absence of
sufficient evidence selects the current `ktl` route")을 문자 그대로 적용한다 — 아직
deprecation을 정당화할 usage evidence가 없으므로(완료기준 1이 아직 코퍼스를 만들지
않았다), **이번 라운드에서는 `ktl`을 deprecate하지 않는다.**

- deprecation 경고 채널: 없음(이번 릴리스에서 deprecate하지 않으므로 발동시킬 경고 자체가 없다)
- 최소 호환 유지 릴리스 수: 무기한 — 제거를 계획하지 않는다
- removal evidence gate: 이 판정으로 열리지 않는다. `ktl` 제거는 이 카드가 만든 게이트가
  아니라 별도의 새 승인된 결정을 요구한다
- rollback route: `ktl`이 이미 살아있는 visible compatibility route이므로 "롤백"은 곧
  현재 상태 유지다 — 되돌릴 별도 기제가 필요 없다
- documentation migration: `docs/42-migration-and-compatibility.md:157`(“후속 구속” 4번)이
  유일한 갱신 대상이며, 실제 문구 수정은 TASK-256 구현 범위다 — 이 카드는 어디를 고칠지만
  얼린다

### 완료기준 1 — invocation corpus (2026-09-04)

스캔 범위는 사용자 권장 작업 순서 승인으로 아래처럼 고정했다. DVA CLI 저장소 자체는
포트폴리오 카탈로그에 ID가 없다. canonical consumer는 `portfolio/catalog.yaml`에서
`manifests[].path == dva.yml`인 항목이다.

| Source | Pin | Note |
| --- | --- | --- |
| workbook `ce-workbook` | `870d89792e6622f537106be74989a33c88595474` (`master`) | `uv run python scripts/portfolio-catalog check` PASS |
| producer `ScriptonBasestar/dva` | `40a35f8f79031f4ddb02c7317f8c8c461684901b` | uncatalogued; scanned this checkout |
| 18 catalog consumers | local `HEAD` at scan time (table below) | not origin-fetched |

**포함 경로.** producer: `docs/`, `skills/`, `examples/`, `agent-mesh-flows/`,
`workflows/`, `assets/`, `USAGE.md`, `README.md`, `AGENTS.md`, `CLAUDE.md`,
`PRODUCT.md`, `ARCHITECTURE.md`, `CHANGELOG.md`, `Makefile`. consumers: 같은
텍스트 확장자 + `dva.yml`.

**포함 패턴.** `dva ktl`, `dva kubectl`, `dva run ktl`, `dva run kubectl`만
invocation으로 센다. `runners.kubectl`(plugin 타입)과 bare `kubectl` 바이너리는
제외한다.

**제외.** `.env*`(`.env.example` 제외), `*.sops`, `ENC[` ciphertext, `token=`/
`password=` 줄, 400KB 초과 파일, `node_modules`/`vendor`/`dist`/`tmp`.

#### Producer invocations (`dva ktl` / `dva kubectl`)

| File | Line | Match |
| --- | --- | --- |
| `docs/43-command-surface-restructure.md` | 106 | `dva ktl ...` escape hatch |
| `docs/42-migration-and-compatibility.md` | 157 | `` `dva kubectl` `` as an interaction example — TASK-256 must rewrite this |
| `skills/dva/SKILL.md` | 163 | `dva ktl <args>` |
| `skills/dva/references/commands.md` | 245, 250–252 | `dva ktl` help and examples |
| `USAGE.md` | 604 | `` `dva ktl ARGS` `` |

`dva kubectl` as a CLI invocation appears once, and only as that now-invalid
interaction example. Live passthrough docs all say `dva ktl`.

#### Catalog consumers

`portfolio-catalog check` 통과 시점의 18개 `dva.yml` 소비자. SHA는 이 워크스테이션
checkout의 `HEAD`이며 origin을 fetch하지 않았다.

| ID | catalog branch | observed HEAD | files | `dva ktl` | `dva kubectl` |
| --- | --- | --- | --- | --- | --- |
| resume-devbox | master | *checkout missing* | — | — | — |
| flow-task-automator-devbox | develop | *checkout missing* | — | — | — |
| flow-taskchain-devbox | develop | `6eed51487cd323095fa8017074061da9d2404b2a` | 1672 | 0 | 0 |
| flow-agent-mesh-devbox | master | `81053099aad4de4fe0d953714cbea66a96923a65` | 827 | 0 | 0 |
| flow-pipechain-devbox | develop | `9341e9c12f94c8671f07ce2a67388457d261aab0` | 805 | 0 | 0 |
| flow-knowchain-devbox | develop | `f45c8e6aef71e2f54e003585c6600d96ce84c40e` | 985 | 1 | 0 |
| flow-observechain-devbox | develop | `56a2aa0ce454ae5701092d7970cda198a6c6a1d4` | 299 | 0 | 0 |
| gorisa-devbox | *(undeclared)* | `e10fe3c2af736a40de781c5f5ef8f130bac94619` | 682 | 2 | 0 |
| gorisa-rails | master | `c38539845fa7988fafe5b4778e122ad868fdb86b` | 99 | 0 | 0 |
| gizzahub-devbox | develop | `ab995ecfb88a07337f38c0faef7e297b21b7c058` | 1542 | 0 | 0 |
| cwrapper-devbox | develop | `5cd5c6f9fba56f6cf911560560854b09e547df31` | 1125 | 0 | 0 |
| cwrapper-engine-py | develop | `8d025e19baa8e7461b2d90387d777bc1311ac837` | 207 | 0 | 0 |
| cwrapper-ranch-workers | develop | `07367cf175bc0c65f16ff77cccbc402d39e9df5d` | 293 | 0 | 0 |
| cloud-script-transformer | develop | `2962f582995e7ca3e7d6a7b85c80c5595128ca94` | 61 | 0 | 0 |
| dripter-devbox | develop | `c6352f9ea6003078fbc05b9633a864d35ee1e64a` | 565 | 0 | 0 |
| dripter-engine-ktor | develop | `1f9f7a985b87a7eb2ba22db26ffe4f058408151f` | 152 | 0 | 0 |
| dripter-frontend-astro | develop | `6cb87cb3a83a93942e925484f86f42555c46720f` | 114 | 0 | 0 |
| matdosa-devbox | master | `1cc592857978c10e55f698276639d99395f637d7` | 114 | 0 | 0 |

Consumer literal matches (no secrets; kubeconfig path is a local filename only):

- `flow-knowchain-devbox` `docs/dva-guide.md:128` — `dva ktl <args>` (guide copy)
- `gorisa-devbox` `deploy/kustomize/operations/forum-data-backfill/README.md:88` —
  `dva ktl -n data get services ...` (read-only evidence command)
- `gorisa-devbox` `docs/60-deployment/forum-data-rehearsal.md:85` — same
  `dva ktl -n data get services ...` invocation in rehearsal notes

**스캔된 소비자에서 `dva kubectl` invocation은 0건이다.** 살아있는 소비자 호출은 전부
`dva ktl`이다.

#### Scanner limitations (unresolved, not guessed)

- Two catalog consumers had no local checkout: `resume-devbox`,
  `flow-task-automator-devbox`. Their invocations are unknown.
- Revisions are workstation `HEAD`, not `git fetch` + `origin/<source_branch>`.
- `gorisa-devbox` has no catalog `source_branch`.
- No expansion of `dva $cmd`, shell aliases, or ignored files.
- Producer Go sources and tests were not scanned; this corpus is invocation
  docs/scripts, not command registration.

완료기준 1 체크박스는 이 코퍼스만으로 닫지 않는다. 누락 checkout 2곳과 unfetched
pin은 카드 verify 문장("missing canonical repositories, unpinned revisions, or
unexplained dynamic invocations stop a rename decision")이 가리키는 공백이다.
닫는 판정은 완료기준 7 독립 리뷰가 한다. TASK-256은 1·7이 둘 다 체크되기 전에는
시작하지 않는다.
