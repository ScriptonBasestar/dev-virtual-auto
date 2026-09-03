---
id: PLAN-002
title: "Deliver the command-surface proposal through evidence-gated tasks"
type: plan
scope: "D6/D7 diagnostics, secure env bridge, required-env and interaction env-file policy, capability-driven init, and optional env promotion"
progress: 68
total-tasks: 16
completed-tasks: 11
children: [TASK-244, TASK-245, TASK-246, TASK-247, TASK-248, TASK-249, TASK-250, TASK-251, TASK-252, TASK-265, TASK-266, TASK-281, TASK-282, TASK-284, TASK-285, TASK-286]
target-date: "2026-12-31"
created: 2026-09-01
---

## Goal

사용자가 제공한 `reports/dva-command-surface/proposal.md`를 현재 코드와 실서비스 운영 조건에
대조한 실행 계획이다. `reports/`는 저장소에서 ignore되는 토론 작업물이므로 clean checkout의
구현자는 그것을 전제로 할 수 없다. 이 문서와 child card가 구현에 필요한 계약, 검토 판정, 작업
연결을 self-contained하게 소유한다. Ignore된 report는 역사적 입력이지 실행 의존성이 아니다.

목표는 제안의 합의된 부분은 즉시 구현하고, public contract·secret safety·외부 repository
compatibility가 미정인 부분은 evidence gate가 닫히기 전까지 구현 또는 승격되지 않게 만드는 것이다.
판정은 **부분 승인**이다. target date는 외부 evidence를 포함한 planning horizon이지 top-level
`env` 승격 약속이 아니다.

| Workstream | 판정 | 작업 |
| --- | --- | --- |
| lifecycle 7동사, plan 위치 인자 | 유지 | 변경 없음 |
| D6/D7 validate 경고 | 구현 가능 | [TASK-244](../todo/244-validate-plan-declaration-drift.md) |
| `config env` bridge | Option A 확정 | [TASK-245](../done/245-freeze-env-bridge-contract.md) → [TASK-246](../todo/246-implement-secure-config-env-bridge.md) |
| env bridge safe write가 target 디렉토리에 고정되지 않음 | 리뷰에서 확인, 재현됨 | [TASK-284](../todo/284-anchor-env-safewrite-to-target-directory.md) |
| required env 오류 전파 | owner 복구 완료·env_file 결정 필요 | [TASK-247](../_archive/done/247-freeze-required-env-command-policy.md) → [TASK-264](../_archive/done/264-restore-imported-command-ownership.md) → [TASK-248](../todo/248-enforce-required-env-command-policy.md) |
| interaction-level `env_file` | versioned rejection 결정됨 | [TASK-265](../done/265-decide-interaction-env-file-contract.md) → [TASK-266](../todo/266-deprecate-and-reject-interaction-env-file.md) |
| migrate/validate 커버리지 차이 + Stage A exit-code 서술 | 리뷰에서 확인, 재현됨 | [TASK-285](../todo/285-close-migrate-validate-coverage-gap.md) |
| 고정 3-plan `init` | 거부·재설계 | [TASK-249](../todo/249-redesign-capability-driven-init.md) → [TASK-250](../todo/250-implement-capability-driven-init.md) |
| migration gate | **N/A 종료** — TASK-252가 승격을 기각해 소비처가 사라짐 | [TASK-251](../_archive/251-build-env-migration-evidence-gate.md) |
| top-level `env` 예약 | **거부 확정 (2026-09-03)** — `config env`가 영구 canonical surface | [TASK-252](../done/252-decide-top-level-env-promotion.md) |

## Current status and recommended order (2026-09-03)

16장 중 11장이 닫혔다: TASK-244(`cb4d598`), TASK-245, TASK-246(`cccb310`), TASK-247,
TASK-248(`b23780e`), TASK-265, TASK-284(`bbe3db1`), TASK-285, TASK-252(결정), TASK-251(N/A
종료), TASK-281(결정 + 종결). 위 표의 링크가 `../todo/`를 가리키는 카드도 이미 `tasks/done/`에 있다 — doccheck가
basename으로 해소하므로(TASK-143) 링크는 유효하며, 상태 판정은 링크 경로가 아니라 카드
front-matter에서 읽는다.

**TASK-252가 이 계획의 최대 분기를 닫았다.** top-level `env` 승격을 거부하고 `config env`를
영구 canonical surface로 확정했다. 되돌리기 비대칭성이 근거다 — `config env`는 나중에 승격할
수 있지만 예약한 reserved name은 남의 config를 깨지 않고 회수할 수 없다. 이 결정 하나로
TASK-251(P0/L)이 코드 한 줄 없이 N/A로 닫혔다. 승격 재개는 이 카드들을 되살리는 것이 아니라
새 카드로 한다.

남은 5장과 각각의 착수 조건 (TASK-281은 2026-09-03 완료기준 9개 전부 충족으로 종결):

| Task | 상태 | 착수 조건 |
| --- | --- | --- |
| TASK-249 | `decision-status: pending` | 사람 결정 필요. TASK-244가 닫혀 입력은 갖춰졌다 |
| TASK-250 | todo | TASK-249 결정 이후 |
| TASK-266 | todo, Stage A 완료 | Stage B는 0.1.48 릴리스 대기 |
| TASK-282 | todo | **착수 가능** — TASK-281이 계약을 동결했다 |
| TASK-286 | todo | **착수 가능** — TASK-281 §3-6이 게이트 대상과 pty 구멍을 넘겼다 |

다음 순서를 권장한다.

1. **TASK-282 구현.** TASK-281이 16개 오류 코드, 29행 `seal` 상태 매트릭스, `show` 순서,
   비활성 상태 argv 표까지 동결했으므로 남은 판단이 없다. 이 계획에서 에이전트가 지금 바로
   진행할 수 있는 유일한 순수 구현 카드다.
2. **TASK-286.** TASK-281 §3-6이 advisory agent detection의 한계(pty 우회)를 명시적으로
   넘겼다. 프로젝트 수준 deny rule은 그 구멍을 코드가 아닌 정책으로 막는다. TASK-282와
   파일이 겹치지 않아 병렬 진행할 수 있다.
3. **TASK-249 결정 → TASK-250 구현.** 남은 유일한 사람 결정이다. TASK-244가 D6/D7 경고를
   구현하며 닫혔으므로 init 재설계가 기다리던 입력은 존재한다.
4. **TASK-266 Stage B는 릴리스가 연다.** Stage A는 `c6aa64b`으로 통합됐고, 카드의 Constraint가
   "Stage B must not start before 0.1.48 has shipped"를 못박는다. 현재 태그는 `v0.1.47`이므로
   착수 조건은 코드가 아니라 릴리스다.

이 계획의 출시 blocker는 남아 있지 않다.

## 1. 비판적 검토 결과

### 1-1. 즉시 구현 가능한 범위

다음은 `consent.md` D1·D3~D7·D10과 현재 코드가 함께 뒷받침한다.

- `up|down|stop|restart|status|logs|build <plan>` 형태 유지
- plan은 위치 인자이며 닫힌 이름 어휘를 도입하지 않음
- D6: 합의된 plan 선언 필드의 equality-only non-fatal warning
- D7: 다중 plan + `default_plan` 부재의 non-fatal warning, 단일 plan 제외
- sops/age 소유권 유지, DVA는 호출만 함
- 복호값 stdout 금지, lifecycle auto-unseal 금지 — 앞 절반은 [TASK-281](../done/281-freeze-gated-env-bridge-commands.md)의
  `env_bridge.allow_show` 게이트가 켜졌을 때 `show`의 stdout 하나에 대해서만 열린다. 게이트가 없거나
  꺼진 상태, 그리고 log/error/JSON/temp filename은 예외 없이 금지다. lifecycle auto-unseal 금지는 불변
- sops 미발견·복호 실패 exit 1

D6 fingerprint는 plan의 `environment`, `site`, `vars`, `endpoint_tags`와 각 entry의 `name`,
`runner`, `order`, `services`, `depends_on`, `vars`만 비교한다. Map key는 정렬하고 list order는
보존하며 nil collection과 empty collection은 같은 runtime 의미로 정규화한다. 같은 imported plan
pointer를 canonical name과 `as` alias로 등록한 pair는 의도된 alias이므로 제외한다. 문구는 “실행이
동일하다”고 단정하면 안 된다. Site override와 resolved execution 전체를 비교한 것이 아니기 때문이다.
비교 pair는 같은 owning config/`SubprojectPath` partition 안으로 제한한다. Root와 imported child,
서로 다른 child repository 사이에서 모양만 같은 plan은 cross-repository duplicate로 취급하지 않는다.
기본 validate는 exit 0을 유지하지만
`config validate --strict`은 semantic warning을 기존 계약대로 exit 1로 승격한다. “exit 변화 없음”은
default mode에만 참이다.

### 1-2. 합의보다 앞서간 env 명세

D9가 확정한 것은 N release에서 `env`를 예약하지 않고 기존 `config` group 아래 bridge로 시작한다는
**순서**다. 정확한 `edit`/`unseal` argv, source↔target schema, selector, `--force`, text/JSON contract는
Q7/Q8에 남아 있다. 이를 구현 단계의 사소한 선택으로 두면 public CLI와 secret safety를 구현자가
임의 결정하게 된다.

따라서 TASK-245·247을 release-blocking decision으로 승격한다. 두 카드가 닫히기 전에는 command 등록,
schema 변경, `loadEnv` 반환 계약 변경을 시작하지 않는다.

### 1-3. 고정 3-plan scaffold는 D8과 직접 충돌

D8은 검증되지 않은 분류와 그 label을 `init` template에 내리는 것도 제품 계약의 부활이라고 기각했다. 생성기는
반복 배포되는 기본값이므로 “강제가 아니라 시작점”이라는 설명만으로 해소되지 않는다.

제안 YAML도 현재 schema에 맞지 않는다.

- top-level `environment`는 profile 이름이 아니라 환경변수 map이다.
- `site`는 top-level field가 아니라 plan 안의 reference다.
- 각 plan에는 실제 `entries`가 필요하다.
- 현재 `init`은 Compose file이 없으면 거부하고 단일 compose stack을 생성하므로 native app plan을
  증명할 정보가 없다.

고정 `local-infra`/`local-dev`/`full-stack` 세 plan과 그 이름의 generator 배포는 폐기한다. 새 evidence가
D8을 명시적으로 재판정하기 전에는 capability가 plan 존재를 정당화해도 이 세 label을 generator가
발명하지 않는다. 기존 사용자 선언 이름은 보존한다. 새 이름은 entry/provider identity에서 기계적으로
도출하거나 사용자가 명시적으로 선택하며, evidence 없는 plan은 생략한다.

### 1-4. `--force`가 safety boundary를 우회해서는 안 됨

제안은 `--force`가 existing target 거부와 Git 검사를 함께 우회하도록 했다. 이 의미는 tracked plaintext,
not-ignored target, symlink/non-regular file, source=target, path escape까지 한 flag로 덮을 수 있다.

기본 판정은 다음과 같다.

- `--force`는 existing regular target overwrite만 허용한다.
- tracked/not-ignored, symlink/non-regular, source=target, path escape 보호는 우회하지 않는다.
- source/target 선택이 모호하면 fail closed한다. v1에 암묵 `--all`을 두지 않는다.
- 다른 동작은 TASK-245에서 명시적으로 재판정하지 않는 한 구현하지 않는다.

### 1-5. required env는 lifecycle/조회 이분법으로 닫히지 않음

현재 `loadEnv`는 오류를 warning으로 삼키고 계속한다. Optional file도 존재하면 unreadable/malformed
오류를 반환하고, multi-file load는 앞 file을 merge한 뒤 뒤 file에서 실패할 수 있다. 제안은 lifecycle을 exit 1, doctor·조회를 advisory로
나눴지만 `status`, `logs`, `down`/`stop`, JSON 경로를 어디에 둘지 정하지 않았다. 관측을 완전히 막으면
장애 대응이 어려워지고, 필요한 interpolation 없이 계속하면 잘못된 resource를 관측하거나 실행할 수 있다.

TASK-247은 `loadEnv` 호출 위치를 단순히 세지 않고, 한 command의 중복 branch와 hook wrapper를 합쳐
사용자에게 보이는 route별로 아래 matrix를 전수 판정한다. 호출처 수는 조사 결과로 기록하며 계획의
고정 전제로 두지 않는다.

```text
caller × required(true/false) × missing/unreadable/malformed × multi-file partial merge
       × text/JSON × exit × child-started × diagnostic-completeness
```

TASK-248은 그 판정을 code로 옮긴다. `loadEnv` 내부의 단순 hard-fail은 금지한다.

### 1-6. N+1 gate는 재현 가능한 release gate가 아님

“scan 0, corpus validate 0, 네 repository report”에는 repository revision, candidate DVA commit,
scanner digest, report schema, freshness, 실행 주체가 없다. 예약 전 binary의 validate exit 0은 예약 후
interaction 충돌을 증명하지 못한다.

TASK-251은 machine-readable evidence와 versioned scanner를 만들고 현재 conflict 판정 함수를 재사용하는
virtual reserved-set mode로 `env` 충돌을 계산한다. 이것은 promotion eligibility evidence일 뿐 실제
routing candidate의 release evidence가 아니다. TASK-252가 promotion을 선택하면 새 implementation child가
실제 reservation candidate를 만들고 같은 pinned corpus를 다시 검증한 뒤에만 source branch 통합을 검토한다.

## 2. 현재 코드 기준 정정

- 예약어는 현재 24개다. 다만 “24개 영구 동결”은 D1~D10의 합의가 아니다.
- `docs/43-command-surface-restructure.md`의 current-status 문장, `USAGE.md`, canonical
  `skills/dva-config/references/schema-reference.md`, generated `internal/cli/library_reference.txt`,
  `docs/51-flowcheck-rules.md`의 현재형 “23개”는 drift다.
- `CHANGELOG.md`의 27→23은 `skill` 추가 전 제거 시점의 역사 기록이므로 24로 고치지 않는다.
- `docs/43`의 27→23 transition도 역사 기록으로 보존하고, 이후 `skill` 추가로 현재 24가 됐음을 분리한다.
- 현재 `config` 하위는 `show`, `docs`, `init`, `migrate`, `validate`다. 제안의 `dump`는 현행 명령이 아니다.
- doctor의 required env present/inaccessible check는 이미 구현돼 있다. 후속은 source-aware hint와 read
  semantics 보강이지 새 check 신설이 아니다.
- `env_file`은 `Config.EnvFile any`이고 string/list/object 세 shape를 normalize한다. schema만 열면
  `sops_source`가 runtime에서 사라질 수 있으므로 data model/normalizer/show round-trip을 함께 바꿔야 한다.
- 같은 `#/definitions/env_file`을 top-level과 interaction이 공유하고 `InteractionCommand.EnvFile`도
  `any`다. Encrypted source metadata는 별도 use case가 승인되지 않는 한 top-level에서만 허용하고
  interaction/subcommand 위치에서는 schema error로 거부한다.
- 현재 CI는 Ubuntu만 실행하지만 release artifact는 Windows도 포함한다. atomic replace를 Windows에서
  증명하지 못하면 해당 명령은 그 OS에서 fail closed해야 한다.

## 3. 보존할 command placement 원칙

현재 구현을 24개 baseline으로 삼되 영구 동결로 과장하지 않는다.

1. 25번째 top-level 예약은 별도 compatibility decision이다.
2. 새 built-in은 먼저 기존 group, doctor check, `ls`/`show`/`manifest` format으로 표현 가능한지 본다.
3. group 내부는 가능하면 `dva <group> <noun> <verb>` 순서를 쓴다.
4. built-in이 interaction 이름을 예약하면 `config validate`를 깨뜨리므로 migration evidence 없이 예약하지 않는다.
5. `env`는 자동 예외가 아니다. D9는 조건부 검토를 허용했을 뿐 승격을 명령하지 않았다.

현재 code baseline:

```text
dva
├── up|down|stop|restart|status|logs|build <plan>
├── run <interaction>
├── config
│   ├── show
│   ├── docs
│   ├── init
│   ├── migrate
│   └── validate
├── validate / doctor / ls / show / manifest / init
├── compose / ktl / ssh / provision
└── skill / console / completion / version / help
```

이 tree는 새 동결 계약이 아니라 구현 drift를 막는 관측 baseline이다.

## 4. Env bridge의 비협상 수용 조건

TASK-245가 exact schema/argv를 고르더라도 다음은 완화하지 않는다.

- 기존 string/list/object `env_file` shape의 load/show/validate 의미 보존
- source/target selection이 0개 또는 여러 개로 모호하면 fail closed
- root/module/override/subproject에서 source와 target 상대경로가 어느 config root에 묶이는지 명시
- shell을 거치지 않는 sops argv와 dotenv input/output type 고정
- secret sentinel이 stdout, stderr, JSON, debug log, error, temp filename에 나타나지 않음
- same-directory 0600/O_EXCL temp, success/failure/cancel cleanup
- symlink/non-regular/source=target 거부
- failure 뒤 existing target byte-identical, success 뒤 temp residue 0
- concurrent writer는 serialize되거나 한쪽이 명시적으로 실패; lost update 금지
- path preflight 뒤 component/symlink swap을 허용하지 않는 platform-safe handle-relative operation 또는 동등한 방어
- file sync와 parent-directory sync 범위, SIGKILL/power-loss 한계, owned stale-temp 탐지·복구 명시
- verified OS에서 atomic replace; 미검증 OS에서 fail closed
- 지원한다고 선언한 OS는 safe-writer와 command integration CI matrix에서 지속 검증; matrix 밖 OS는
  unsupported fail-closed 동작을 테스트
- fake-sops fault injection과 pinned real-sops integration
- 오류 안내는 해당 release에 실제 존재하는 command만 가리킴

이 조건은 게이트 뒤에 추가되는 `seal`/`show`에도 그대로 적용된다. 게이트는 명령이 켜지는 조건을
바꿀 뿐 위 보증을 완화하지 않는다.

## 5. Promotion evidence contract

TASK-251의 gate artifact는 최소 다음을 고정한다.

- canonical repository ID, commit SHA, 검사 path inventory
- base DVA version/commit, virtual reserved set, scanner version/digest, 검사 시각
- secret-free manifest와 report 원문을 canonical tracked path에 보관하고 promotion·rollback 지원 기간까지
  retention; 외부 immutable artifact를 택하면 URI, content digest, retention policy를 함께 추적
- recursive interaction/subcommand body
- shell/script/Make/workflow/document의 literal `dva env` invocation
- stdout decrypt surface와 D4 위반
- direnv `use sops`, env.mk, DVA bridge의 현재 owner 상태
- before/after invocation과 machine-readable migration result
- dynamic invocation을 완전히 증명할 수 없는 한계의 명시적 finding

artifact에는 env 값, decrypted output, credential, local absolute path를 기록하지 않는다. Gate와 예약
commit 사이 external SHA drift 허용값은 0이다. missing/stale/ambiguous evidence 또는 외부
SSOT 미확인은 fail closed한다. gate 실패 시 `config env`가 지원 표면으로 잔류하며 일정을 자동 연장하지
않는다.

TASK-252가 promotion을 선택하면 exact route/compatibility/rollback contract를 가진 구현·release child
card를 새로 만들고 PLAN-002의 children/total을 갱신한다. 그 child는 실제 reservation candidate로
pinned corpus를 다시 검증한다. 그 child가 통합되기 전에는 PLAN-002를 완료로
닫지 않는다. Permanent `config env`를 선택하면 추가 promotion 구현은 없다.

## 6. 작업 graph

```text
TASK-244  D6/D7 warnings

TASK-247 policy ──┬─> TASK-264 imported command owner ──┐
                  └─> TASK-265 interaction env decision ┴─> TASK-248 propagation ──┐
TASK-245  env bridge contract ───────────────────────────┴─> TASK-246 secure bridge
TASK-246 + TASK-248 ─────────────> TASK-252 initial decision
                                      ├─ permanent config env ─> TASK-251 N/A disposition
                                      └─ promotion evidence ───> TASK-251 gate ─> resume TASK-252

TASK-265 decision ──> TASK-266 deprecate then reject interaction env_file

TASK-244 + TASK-249  init redesign ──> TASK-250 init implementation
```

위 그래프의 간선 중 TASK-244→TASK-250, TASK-245→TASK-246, TASK-247→TASK-248,
TASK-265→TASK-266은 좌변이 모두 닫히면서 해소됐다. 남은 미해소 간선은 TASK-249→TASK-250
(사람 결정)과 TASK-252→TASK-251(조건부 실행)뿐이다. 구현과 independent review는 계속 분리한다.

TASK-245가 여러 OS를 지원한다고 결정했다면 TASK-246이 그 OS를 지속 검증하는 CI matrix까지 소유해야
한다는 조건이 걸려 있었다. TASK-246은 `cccb310`으로 닫혔으므로, 이 조건은 그 카드의 완료 근거에서
확인할 문제이지 남은 작업의 착수 조건이 아니다. 검증되지 않은 OS를 fail closed로 두는 원칙은
그대로 유효하다.

세션 경계, 모델 라우팅, 서브에이전트 역할과 재사용 시작 프롬프트는
[Command Surface 작업의 에이전트 실행 런북](../../docs/53-command-surface-agent-execution.md)이 소유한다.

## 7. 하지 않는 것

- lifecycle 동사 재배치 또는 plan flag화
- 닫힌 plan vocabulary와 plan key 강제
- D6 warning의 canonical name/삭제 대상 권고
- 게이트 없는 `config env show`, top-level `dva env show`, lifecycle auto-unseal
  (기본 비활성 + `env_bridge` opt-in은 [TASK-281](../done/281-freeze-gated-env-bridge-commands.md)이 소유하며,
  [TASK-245](../done/245-freeze-env-bridge-contract.md) §11의 무조건 기각을 그만큼 supersede한다)
- DVA의 age/provider/key management 재구현
- fixed archetype/3-plan scaffold
- `--force`의 tracked/symlink/path safety 우회
- evidence 없는 top-level `env` 예약 또는 예약 일정 약속
- 기존 devbox의 강제 plan-name migration

## 8. 완료 정의

각 task의 acceptance criteria와 repository mechanical gate가 모두 exit 0이어야 한다. env bridge가 든
release candidate는 최소 `make lint`, `make test`, `make test-integration`, `make doc-check`,
`make check-generate`, `make release-check`를 통과한다. 외부 corpus 수치는 revision 없는 숫자로 재사용하지
않고 TASK-251 manifest에서 다시 측정한다.

## Children

- TASK-244 — validate duplicate plan declarations and missing multi-plan defaults
- TASK-245 — freeze the public and filesystem contract for the config env bridge
- TASK-246 — implement the decided config env bridge without exposing plaintext
- TASK-247 — freeze required env-file behavior for every command path
- TASK-248 — enforce the decided required env policy without breaking diagnostics
- TASK-249 — redesign init around verified capabilities instead of a fixed three-plan template
- TASK-250 — implement evidence-based init for people and agents
- TASK-251 — build a versioned cross-repository env migration evidence gate
- TASK-252 — decide whether top-level env promotion is safer than keeping config env
- TASK-265 — decide the interaction-level env_file compatibility contract
- TASK-266 — deprecate then reject the inert interaction env_file field
- TASK-281 — freeze the gate-guarded seal and show contract for the config env bridge
- TASK-282 — implement the gated seal and show commands behind the env_bridge switch
- TASK-284 — anchor the env safewrite to the target's directory
- TASK-285 — close the migrate/validate coverage gap for the env bridge
- TASK-286 — project agent-runtime deny rules for the commands agents must not run

Cross-plan prerequisite: PLAN-003의 TASK-264가 TASK-248보다 먼저 imported interaction/provision owner를 복구한다.
