---
id: TASK-281
title: "Freeze the gate-guarded seal and show contract for the config env bridge"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-03T18:10:00+09:00
source: "사용자 결정 2026-09-03 — 명령 표면을 모두 갖추되 기본 비활성, dva.yml opt-in"
scope: "env_bridge gate schema and merge rules, seal contract, show contract, disabled-state behaviour, new error codes, superseded PLAN-002/TASK-245 rulings"
status: todo
needs-human: true
decision-status: pending
depends-on: [TASK-246]
---

# Task 281: freeze the gated seal and show contract

## Summary

`dva config env`는 오늘 `edit`과 `unseal`만 갖는다. `seal`(평문 target → 암호화 source)과
`show`(복호값 stdout)는 TASK-245 §11에서 기각됐다. 사용자가 2026-09-03에 그 판정을 뒤집었다 —
**명령 표면은 전부 갖추되, 기본은 꺼져 있고 `dva.yml`이 켤 때만 켜진다.**

이 카드는 그 게이트와 두 명령의 계약을 동결한다. 구현은 TASK-282가 소유한다.

## Decision received (2026-09-03)

| 항목 | 확정 |
| --- | --- |
| 게이트 뒤 명령 집합 | `seal`, `show` 두 개. `rotate`와 sops 전체 pass-through는 채택하지 않음 |
| 스위치 위치 | 최상위 `env_bridge:` 섹션 |
| 스위치 입도 | 명령별 boolean (`allow_seal`, `allow_show`) |
| 기본값 | 둘 다 `false` |

`edit`과 `unseal`은 게이트 대상이 아니며 오늘 계약 그대로 항상 켜져 있다.

## What this supersedes

이 결정은 이미 동결된 판정 네 개를 바꾼다. 셋은 문서를 고쳐야 하고, 하나는 `done` 카드라
고치지 않고 supersede 사실만 기록한다.

| 위치 | 기존 문구 | 처리 |
| --- | --- | --- |
| PLAN-002 §1-1 | "복호값 stdout 금지, lifecycle auto-unseal 금지" | `show` 게이트가 **앞 절반만** 조건부로 연다. lifecycle auto-unseal 금지는 불변 |
| PLAN-002 §7 | "`dva env show` 또는 lifecycle auto-unseal" (비목표) | "게이트 없는 `show`"로 좁힌다 |
| PLAN-002 §7 | "DVA의 age/provider/key management 재구현" (비목표) | **불변.** `seal`도 키 인자를 받지 않는다 (§3-3) |
| TASK-245 §11 | "`dva config env show` / lifecycle auto-unseal — 복호값을 stdout에 노출하거나 암묵 write를 만든다" | `done` 카드이므로 편집하지 않는다. 이 카드가 supersede를 기록하고 PLAN-002가 링크한다 |

TASK-245의 나머지 판정 — origin provenance, path containment, atomic write, symlink/tracked 거부,
secret sentinel 금지, exit code 비전파 — 은 **전부 유지되며 `seal`/`show`에도 그대로 적용된다.**

## Recorded risk (게이트가 덮지 않는 부분)

게이트는 "실수로 켜지는 것"을 막지만 "켠 뒤의 사고"는 막지 못한다. 구현이 방어를 설계하도록
두 시나리오를 여기 남긴다.

1. **Lost update (`seal`)** — 어제 unseal한 평문 target을 오늘 seal하면, 그 사이 동료가 추가한
   키가 조용히 사라진다. 평문 파일에는 세대 정보가 없으므로 명령이 스스로 판단할 수 없다.
   `unseal`의 사고 반경은 내 로컬 파일 하나지만 `seal`의 사고 반경은 팀의 secret 진실 원천이고,
   커밋되어 전파된다. §3-3이 이 시나리오에 대한 방어를 반드시 판정해야 한다.
2. **개인 값 유출 (`seal`)** — 로컬 평문 target에는 팀 공유 값과 개인 디버깅 토큰이 섞이기
   쉽다. 그대로 seal하면 개인 자격증명이 암호화된 채 저장소에 들어가 리뷰에서 보이지 않는다.

## Decisions required

### 3-1. `env_bridge` 스키마와 선언 위치

- 최상위 `env_bridge:` object, key는 `allow_seal`/`allow_show`, 둘 다 boolean, 기본 `false`.
- `internal/config/schema.json`의 최상위는 `additionalProperties: false`다. 따라서 이 key를 쓴
  `dva.yml`은 **구버전 DVA에서 config 전체가 검증 실패**한다. 부분 무시가 아니다.
  `MinScaffoldVersion`/`version` 정책과의 관계를 이 카드가 판정한다.
- `interaction`/`subcommand` 레벨 선언은 거부한다 (`sops_source`와 같은 이유 — 명령 표면 정책이
  실행 단위 선언에 섞이면 안 된다).
- 섹션을 아예 쓰지 않은 오늘의 모든 `dva.yml`은 한 글자도 바뀌지 않고 두 명령이 꺼진 상태다.

### 3-2. 게이트의 origin과 merge 규칙

`env_file`은 이미 `EnvFileOrigin()`/`Writable()`로 어느 파일이 선언했는지를 추적하고,
`checkWritableOrigin`이 쓸 수 없는 origin을 거부한다 (`internal/cli/config_env_select.go:54`).
게이트도 같은 수준의 provenance를 요구한다.

- root / module / override 중 어디까지가 게이트를 켤 수 있는가.
- **subproject가 부모의 게이트를 켤 수 없어야 한다.** import된 자식 설정이 부모 저장소의 secret
  write 표면을 여는 경로는 fail-closed다.
- 여러 origin이 서로 다른 값을 선언했을 때: whole-replace인가, OR인가, **충돌이면 거부**인가.
  권장은 거부다 — boolean OR는 "누군가 켰으면 켜진다"가 되어 게이트의 존재 의의를 깎는다.

### 3-3. `seal` 계약

방향이 반대이므로 `unseal` 매트릭스를 그대로 뒤집을 수 없다. 최소한 다음을 판정한다.

- **키 인자 없음.** `--age`, `--kms`, `--pgp`, `--encrypted-regex`를 DVA가 받지 않는다.
  recipient는 `.sops.yaml`의 `creation_rules`가 소유한다. 규칙이 없으면 fail-closed
  (`sops_creation_rule_missing`) — DVA가 기본 키를 고르지 않는다.
- **기존 source가 있을 때 metadata/recipient 보존**을 어떻게 보장하는가.
  `sops encrypt` 신규 생성과 기존 파일 갱신은 다른 경로다.
- **Lost update 방어 (§2-1).** 후보:
  - (a) 기존 source가 있으면 `--force` 요구 — 사고를 못 막는다. 삭제도 "덮어쓰기"로 보이기 때문.
  - (b) seal 전에 source를 복호해 target과 key set을 비교하고, 사라지는 키가 있으면 stderr에
        보여준 뒤 확인 — 비대화형/CI에서 결정적이지 않다.
  - (c) **키 삭제가 발생하면 fail-closed**, 별도 `--allow-key-removal`로만 통과.
        추가·값 변경은 통과, 삭제만 막는다.
  - 권장은 (c)다. 추가는 안전하고 삭제만 사고이며, TTY 여부와 무관하게 결정적이다.
    비교는 **키 이름 집합만** 사용하고 값은 비교 결과에도 로그에도 남기지 않는다.
- **평문 target 자체의 preflight** — symlink/non-regular/tracked 거부가 여기서도 필요한가.
  target은 이제 read 대상이므로 `unseal`과 같은 규칙이 자동으로 옳지는 않다.
- source write는 `unseal`의 target write와 같은 atomic 0600/O_EXCL same-directory temp 규약을
  따른다. 실패 뒤 기존 source는 byte-identical이어야 한다.
- 성공/실패/취소 stdout·JSON·exit 전수 매트릭스.

### 3-4. `show` 계약

`show`는 복호값을 사람에게 보여준다. 이 저장소가 지금까지 절대 금지로 다뤄온 유일한 동작이므로
예외의 경계를 좁게 판정한다.

- **출력 경로.** 권장은 **stdout이 아니라 controlling terminal(`/dev/tty`) 전용**이다. 열 수
  없으면 fail-closed(`no_controlling_terminal`). 이유는 §3-6이 소유한다 — 요약하면, 캡처
  가능한 스트림으로 secret을 내보내지 않는 것은 호출자가 누구인지 판정하지 않고도 성립하는
  유일한 방어다. `isTerminal`은 이미 `internal/cli/root.go:562`에 있다.
  파일이 필요한 사용처는 `unseal`이 이미 소유하므로 `show`가 리다이렉션을 지원할 이유가 없다.
- **`--json` 지원 여부.** 권장은 거부(`json_unsupported_for_show`)다. envelope에 secret을 넣으면
  기계 소비 파이프라인과 로그로 흘러드는 경로가 기본값이 된다.
- 게이트가 여는 것은 **`show`의 stdout 하나뿐**이다. debug log, error message, temp filename,
  다른 명령의 출력에 복호값이 나타나지 않는다는 TASK-245 §7 규칙은 예외 없이 유지된다.
- 부분 조회(`--key NAME`) 도입 여부. 도입하지 않는 쪽을 권장한다 — 표면이 늘고, 전체를 못 보게
  막는 것도 아니라 보안 이득이 없다.
- `show`가 `unseal`과 다른 점은 **파일을 만들지 않는다**는 것뿐이므로, write preflight(9단계)
  중 어디까지가 여전히 필요한지 판정한다 (platform whitelist는? tracked 검사는 무의미하다).

### 3-5. 꺼져 있을 때의 동작

권장: **명령은 항상 등록하고 `--help`에 노출하되, 실행하면 전용 error code로 거부**한다.

```text
$ dva config env seal
error: seal is disabled; set env_bridge.allow_seal: true in dva.yml to enable it
```

숨기면 사용자가 명령의 존재와 켜는 방법을 알 수 없고, 이 저장소는 이미 모든 거부를 동결된
error code로 표현한다. 새 code 후보: `seal_not_enabled`, `show_not_enabled`.

게이트 검사는 preflight **1단계보다 앞**에 온다 — 꺼진 명령이 platform이나 config 상태에 따라
다른 오류를 내면 게이트가 무슨 일을 하는지 설명할 수 없게 된다.

### 3-6. 에이전트 노출 통제

`env_bridge.allow_show`를 켠 저장소에서 LLM 에이전트가 `show`를 실행하면 복호값이 그대로
대화 트랜스크립트와 공급자 로그로 들어간다. 이 카드는 그 노출을 줄이는 수단을 판정한다.

**전제: 호출자가 LLM인지 판정하는 것은 불가능하다.** CLI에는 인증된 호출자 신원이 없다.
`CLAUDECODE`, `AI_AGENT`, `CLAUDE_CODE_ENTRYPOINT` 같은 신호는 전부 호출자 자신의 환경변수이고
`env -u`로 사라진다. 부모 프로세스 검사도 셸이 겹치면 무너진다. HTTP `User-Agent`로 접근제어를
하지 않는 것과 같은 이유이므로, **어떤 탐지도 보안 경계로 선언하지 않는다.** 선언하면 막히지
않은 것을 막혔다고 믿게 된다.

따라서 문제를 바꾼다 — "호출자가 LLM인지 알아낸다"가 아니라 **"LLM이 값을 가져갈 수 없게 한다"**.
앞은 탐지 문제라 불가능하고 뒤는 배관 문제라 성립한다.

| 층 | 수단 | 성격 | 소유 |
| --- | --- | --- | --- |
| DVA 배관 | §3-4의 `/dev/tty` 전용 출력 | **구조적** — 탐지하지 않음 | 이 카드 |
| DVA advisory | 에이전트 환경변수 감지 시 거부 | 사고 방지 | 이 카드 |
| 에이전트 런타임 | 저장소가 배포하는 deny 규칙 | 런타임이 강제 | TASK-286 |
| 팀 정책 | `allow_show: false` 유지 | 가장 강함 | 사용자 |

판정할 것:

- **Advisory 감지 채택 여부와 신호 목록.** 채택한다면 **우회 플래그를 만들지 않는다.**
  `--i-am-human` 류를 두는 순간 오류 메시지가 우회 설명서가 되고 에이전트가 그것을 읽고
  재시도한다. 문서에는 반드시 advisory라고 적고 보안 경계라고 적지 않는다.
- `/dev/tty` 전용 출력이 남기는 구멍의 기록. 에이전트가 pty를 할당하면(`script -q /dev/null …`)
  뚫린다. 이것은 이미 우회 의도이며 CLI 계층이 막을 수 있는 종류가 아니라는 사실을 카드에
  남긴다 — 그 지점부터는 런타임 deny 규칙(TASK-286)의 영역이다.
- Advisory 거부와 `no_controlling_terminal`이 **같은 상황에서 서로 다른 code를 내지 않도록**
  순서를 고정한다. 에이전트는 TTY도 없는 것이 보통이므로 두 조건이 거의 항상 함께 참이다.

## Completion Criteria

- [ ] Freeze the `env_bridge` schema, its accepted declaration locations, and the exact behaviour of a config that omits it | verify: human — accepted and rejected YAML examples must be recorded for every declaration location
- [ ] Freeze the gate's origin provenance and multi-origin merge rule, including the subproject case | verify: human — a subproject must not be able to enable the parent's gate, and the conflicting-declaration outcome must be named
- [ ] Freeze the `seal` contract with no key or provider arguments, an explicit missing-creation-rule failure, and a decided defense for the lost-update scenario recorded in this card | verify: human — the chosen defense must be deterministic without a TTY, and key names must never be compared by value
- [ ] Freeze the full `seal` state matrix across source existence, target state, key-set delta, and flags, reusing TASK-245's atomic-write and containment guarantees | verify: human — the matrix must cover every Cartesian branch and name one code per branch
- [ ] Freeze the `show` contract, its output stream, and precisely which TASK-245 redaction rule the gate opens and which remain absolute | verify: human — only `show`'s own human-facing output may be excepted; log, error, JSON and filename rules must be restated as unchanged
- [ ] Decide the agent-exposure controls, recording that no caller-identity test is claimed as a security boundary and that any advisory refusal ships without a bypass flag | verify: human — the residual pty hole must be recorded and handed to TASK-286 rather than left implied
- [ ] Freeze disabled-state behaviour, help visibility, and the new error codes, placing the gate check before every other preflight step | verify: human — the argv table must show text and exit for both disabled commands
- [ ] Record the compatibility consequence of adding a top-level key under `additionalProperties: false`, including which DVA versions reject such a config outright | verify: human — the version boundary and any scaffold/version policy change must be named
- [ ] Update PLAN-002 §1-1 and §7 to the narrowed wording, record that TASK-245 §11 is superseded without editing that done card, and create the implementation child before closing | verify: `make doc-check`

## Fail-closed default

판정되지 않은 항목이 하나라도 남으면 그 명령은 게이트가 켜져도 **등록되지 않는다.**
계약 없이 구현된 secret write 표면을 내보내지 않는다.
