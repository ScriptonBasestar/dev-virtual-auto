---
id: TASK-281
title: "Freeze the gate-guarded seal and show contract for the config env bridge"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-03T18:10:00+09:00
source: "사용자 결정 2026-09-03 — 명령 표면을 모두 갖추되 기본 비활성, dva.yml opt-in"
decided-at: 2026-09-03T19:40:00+09:00
scope: "env_bridge gate schema and merge rules, seal contract, show contract, disabled-state behaviour, new error codes, superseded PLAN-002/TASK-245 rulings"
status: done
needs-human: true
decision-status: decided
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
| 이 카드 §3-1 (초안) | "구버전 DVA에서 config 전체가 검증 실패한다" | **오류였다.** 로더는 unknown key를 무시하고 스키마는 `dva validate`에서만 돈다 — §3-0이 정정 |
| TASK-245 §11 | "`dva config env show` / lifecycle auto-unseal — 복호값을 stdout에 노출하거나 암묵 write를 만든다" | `done` 카드이므로 편집하지 않는다. 이 카드가 supersede를 기록하고 PLAN-002가 링크한다 |

TASK-245의 나머지 판정 — origin provenance, path containment, atomic write, symlink/tracked 거부,
secret sentinel 금지, exit code 비전파 — 은 **전부 유지되며 `seal`/`show`에도 그대로 적용된다.**

## Recorded risk (게이트가 덮지 않는 부분)

게이트는 "실수로 켜지는 것"을 막지만 "켠 뒤의 사고"는 막지 못한다. 구현이 방어를 설계하도록
두 시나리오를 여기 남긴다.

1. **Lost update (`seal`)** — 어제 unseal한 평문 target을 오늘 seal하면, 그 사이 동료가 추가한
   키가 조용히 사라진다. 평문 파일에는 세대 정보가 없으므로 명령이 스스로 판단할 수 없다.
   `unseal`의 사고 반경은 내 로컬 파일 하나지만 `seal`의 사고 반경은 팀의 secret 진실 원천이고,
   커밋되어 전파된다.
   → **해소됨 (§3-3).** `seal`을 create-only로 동결해 기존 source를 덮어쓸 경로 자체를 없앴다.
   완화가 아니라 구조적 제거이므로 이 시나리오는 더 이상 발생할 수 없다.
2. **개인 값 유출 (`seal`)** — 로컬 평문 target에는 팀 공유 값과 개인 디버깅 토큰이 섞이기
   쉽다. 그대로 seal하면 개인 자격증명이 암호화된 채 저장소에 들어가 리뷰에서 보이지 않는다.
   → **부분 완화 (§3-3).** seal이 키 이름 집합을 제시하고 확인을 받는다(비대화형은 `--yes`).
   값은 비교에도 출력에도 쓰지 않는다. 사람이 목록을 잘못 승인하는 것은 여전히 막지 못한다.

## Decisions (frozen 2026-09-03)

아래 판정은 전부 확정이다. TASK-282는 이 절을 계약으로 읽고 구현하며, 여기서 벗어나려면
이 카드를 다시 연다.

### 3-0. 카드 전제 정정 — 구버전 DVA는 config 전체를 거부하지 않는다

이 카드는 `additionalProperties: false` 때문에 "구버전 DVA에서 config 전체가 검증 실패한다.
부분 무시가 아니다"라고 적었다. **틀렸다.** 소스를 확인한 결과:

- 로더는 `doc.Decode(cfg)`를 쓰고 **`KnownFields(true)`를 켜지 않는다** (`internal/config/config.go:926`,
  이유는 `migrate.go:92-103`에 기록됨). 모르는 최상위 key는 조용히 무시된다.
- JSON 스키마는 `validateYAMLSchema`가 강제하고, 그 유일한 호출 경로는 `Config.Validate()` →
  **`internal/cli/validate.go:151`** 하나다. 즉 `dva validate`에서만 실행된다.

따라서 `env_bridge`를 선언한 config를 구버전 DVA가 만나면 **두 갈래로 갈린다**:

| 경로 | 구버전 DVA의 실제 동작 |
| --- | --- |
| `dva up` 등 일반 명령 | **정상 로드.** key는 무시되고 게이트는 부재 — 두 명령은 꺼진 상태 |
| `dva validate` | **실패.** 최상위 `additionalProperties: false`가 `env_bridge`를 unknown property로 거부 |

보안상으로는 무시가 fail-closed라 해롭지 않다 (게이트의 기본값이 `false`이고, 게이트는 무언가를
끄는 데 쓰이지 않는다). 문제는 진단 가능성이다 — `allow_show: true`를 적어 둔 사용자가 구버전
바이너리에서 `dva config env show`를 실행하면 "unknown command"만 보고 이유를 알 수 없다.

### 3-1. `env_bridge` 스키마, 선언 위치, 버전 정책 — 확정

**스키마.** 최상위 `env_bridge:` object. property는 `allow_seal`, `allow_show` 둘뿐이고 각각
boolean, 기본 `false`. 이 object 자체도 `additionalProperties: false`.

**섹션을 생략한 config는 한 글자도 바뀌지 않으며 두 명령이 모두 꺼져 있다.** 오늘 존재하는 모든
`dva.yml`이 여기 해당한다.

**버전 정책 — `version:` 동반 의무.** `env_bridge`를 선언한 config는 `version:`을 함께 선언해야
하며, 그 값은 **`env_bridge`를 도입한 릴리스 이상**이어야 한다. 위반은 `env_bridge_requires_version`.

근거는 §3-0의 두 갈래를 하나의 올바른 메시지로 접기 위해서다. `version:`은 이미 "config가 reader
에게 요구하는 최소 버전"이고 초과 시 거부된다(`checkConfigVersion`, `config.go:1158`). 이것을
붙이면 구버전 DVA의 두 경로가 모두 **`your dva version is 0.1.47, but config requires minimum
version 0.1.48. Please upgrade dva`** 로 수렴한다 — unknown property 오류보다도, 조용한 무시보다도
정확하다. 로더가 unknown key를 무시하는 이상 이것이 DVA가 쥘 수 있는 **유일한 레버**다.

도입 릴리스는 현재 `Version = "0.1.47"` 기준 **`0.1.48`** 으로 기록한다. TASK-282가 다른 번호로
릴리스되면 그 카드가 이 값을 조정하고 여기에 반영한다.

**`MinScaffoldVersion`은 `0.1.44`에서 움직이지 않는다.** `dva init`은 `env_bridge`를 절대 쓰지
않으므로(기본이 off이고 생략이 곧 off) 스캐폴딩된 config는 여전히 0.1.44에서 로드된다. 이 상수를
올리는 것은 init이 구버전이 파싱 못 하는 것을 내보내기 시작할 때뿐이라는 `version.go`의 규칙이
그대로 유지된다.

**선언 위치는 config root의 `dva.yml` 하나뿐이다.** `interaction`/`subcommand` 레벨은 물론이고
module·override·subproject도 선언할 수 없다. 상세는 §3-2.

### 3-2. 게이트의 origin — 확정: **root 전용, 병합 없음**

`env_file`은 whole-replace 병합이고 마지막 선언 파일이 이긴다 (root → module → override 순,
`config.go:738/766/781`). `Writable()`은 root/module/override를 참으로, subproject를 거짓으로 본다
(`envsource.go:46`). **게이트는 이 병합 모델을 따르지 않는다.**

| origin | 판정 | 코드 |
| --- | --- | --- |
| root `dva.yml` | **유일하게 유효** | — |
| module (`.sb/dva/*.yml`) | **거부** | `env_bridge_origin_not_root` |
| override (`dva.override.yml`) | **거부** | `env_bridge_origin_not_root` |
| subproject (import된 자식) | **무시** (오류 아님) | — |

**왜 병합이 아니라 root 전용인가.** whole-replace는 마지막 선언이 이기므로 module이 root의
`allow_show: false`를 `true`로 갈아엎을 수 있다. 이는 카드가 §3-2에서 경계한 "누군가 켰으면
켜진다"와 같은 결과다. 반면 root 전용은 **충돌을 해결하는 대신 표현 불가능하게 만든다** — 선언이
하나뿐이면 병합 규칙도, 충돌 판정도 필요 없다. 카드의 기존 권장(충돌 시 거부)보다 단순하고 강하다.

**module/override가 오류인 이유.** 둘 다 같은 저장소가 소유하고 같은 사람이 편집하는 파일이다.
거기 적힌 게이트는 의도가 아니라 착오이며, 조용히 무시하면 "켰는데 안 켜진다"가 된다. 오류가
사실을 말한다.

**subproject가 무시인 이유.** 자식 config는 자기 저장소에서 단독 실행될 때 정당하게 자기 게이트를
가진다. 그것을 부모가 import했다는 이유로 오류로 만들면 부모가 자식을 import하지 못한다.
자식 디렉토리에서 `dva config env`를 실행하면 그때는 자식이 root이므로 자식의 게이트가 적용된다.
**부모 세션에서는 부모 root의 선언만이 유일한 진실이다** — 이것이 "subproject가 부모의 게이트를
켤 수 없다"는 요구를 만족시키는 방식이다.

**오류의 폭발 반경.** `env_bridge_origin_not_root`와 `env_bridge_requires_version`은
**`dva validate`와 `dva config env *`에서만** 보고한다. `dva up`을 벽돌로 만들지 않는다.
secret 표면에 대한 정책 선언이 무관한 lifecycle 명령을 중단시킬 이유가 없고, `config env` 경로
에서 거부되는 이상 fail-closed는 그대로 성립한다.

### 3-3. `seal` 계약 — 확정: **create-only**

**`seal`은 존재하지 않는 source만 만든다. 기존 source는 절대 덮어쓰지 않는다.**
기존 source가 있으면 `source_exists`로 거부하고 `edit`을 가리킨다. **이를 무효화하는 `--force`는
두지 않는다.**

**이 판정이 §2-1 lost update를 방어가 아니라 구조로 제거한다.** 어제 unseal한 평문을 오늘 seal해
팀의 SSOT를 되돌리는 시나리오는, seal이 기존 source를 건드릴 수 없으므로 **발생 자체가 불가능**
하다. 카드가 후보로 남긴 (a) `--force` 요구, (b) 복호 후 키 비교, (c) 키 삭제 fail-closed +
`--allow-key-removal` — **셋 다 채택하지 않는다.** 전부 "덮어쓸 수 있다"는 전제 위의 완화책이고,
그 전제를 제거하는 편이 결정적이고 TTY와 무관하며 새 플래그도 새 비교 로직도 필요 없다.

**두 명령의 역할이 상보적으로 닫힌다:**

| 하려는 일 | 명령 |
| --- | --- |
| 평문 `.env`를 암호화 관리로 처음 전환 | `seal` (bootstrap, 1회) |
| 이미 암호화된 source에 키 추가·수정·삭제 | `edit` (sops가 소유, 오늘 계약 그대로) |
| 암호화 source를 로컬 평문으로 펼치기 | `unseal` |
| 값을 눈으로 확인 | `show` (§3-4) |

TASK-245 §4의 "`edit`은 새 source를 만들지 않는다"와 정확히 맞물린다 — `seal`은 만들기만 하고,
`edit`은 고치기만 한다.

**키 인자 없음.** `--age`, `--kms`, `--pgp`, `--encrypted-regex`를 받지 않는다. recipient는
`.sops.yaml`의 `creation_rules`가 소유한다. PLAN-002 §7의 "age/provider/key management 재구현
안 함" 비목표는 이 명령으로도 깨지지 않는다.

**creation rule 부재는 fail-closed.** config root부터 containment 경계까지의 조상 디렉토리에
`.sops.yaml`이 하나도 없으면 sops를 호출하기 전에 `sops_creation_rule_missing`으로 거부한다.
`.sops.yaml`은 있으나 대상 경로에 매칭되는 rule이 없어 sops가 실패하면 **같은 코드로 매핑**한다
(sops의 exit code는 TASK-245대로 전파하지 않는다). DVA는 `.sops.yaml`의 rule 매칭을 재구현하지
않는다 — 존재 여부만 보고, 매칭 판단은 sops에게 맡긴 뒤 결과를 번역한다.

**metadata/recipient 보존 문제는 발생하지 않는다.** create-only이므로 갱신 경로가 없고,
신규 생성은 `sops encrypt`의 정상 경로다. 기존 파일의 per-file recipient 커스터마이즈를 DVA가
훼손할 방법이 없다.

**§2-2 개인 값 유출 방어 — 키 이름 확인.** seal은 암호화 대상 평문에서 읽은 **키 이름 집합만**
제시하고 확인을 받는다. **값은 화면에도 로그에도 오류 메시지에도 나타나지 않는다.**

- controlling terminal이 있으면 키 이름을 나열하고 확인을 받는다.
- 없으면 `--yes` 없이는 `confirmation_required`로 거부한다. `--yes`는 이 나열을 확인했다는 선언
  이며, TTY 유무와 무관하게 결정적이다.

`--yes`는 §3-6이 금지한 "우회 플래그"가 아니다. 그 금지는 `show`의 advisory 거부를 무력화하는
플래그를 겨냥한 것이고, `--yes`는 아무 값도 드러내지 않으며 없는 권한을 열지도 않는다.

**평문 target의 preflight.** target은 이제 read 대상이므로 unseal의 write 규칙을 그대로 쓰지
않는다. 적용: 경로 shape·component symlink 거부·containment는 그대로(경로 신뢰가 방향과 무관),
target 자체는 **regular file이어야 하고**(`target_not_regular`) 읽을 수 있어야 한다
(`permission_denied`). **tracked/ignored 검사는 하지 않는다** — 평문 target이 git에 추적되고
있다면 그것은 이미 벌어진 사고이고, seal은 그 사고를 되돌리는 방향의 명령이므로 여기서 막으면
복구 경로를 막는 셈이 된다.

**source write는 unseal의 target write와 동일한 규약**을 따른다: 같은 디렉토리 0600 `O_EXCL`
temp fd → sops stdout 수용 → rename → 부모 fsync. create-only이므로 rename 대상이 이미 있으면
그 자체가 `source_exists` 조건 위반이다. 실패 시 새 파일이 남지 않는다.

#### 3-3-1. `seal` 상태 매트릭스

순서는 위에서 아래. 첫 번째로 참이 되는 행에서 종료하며, 그 아래는 평가하지 않는다.

| # | 조건 | 결과 | 코드 | exit |
| --- | --- | --- | --- | --- |
| 1 | 게이트 off | 거부 | `seal_not_enabled` | 1 |
| 2 | `--json` 지정 | 거부 | `json_unsupported_for_seal` | 1 |
| 3 | 플랫폼 미지원 | 거부 | `unsupported_platform` | 1 |
| 4 | `env_bridge` origin이 root 아님 | 거부 | `env_bridge_origin_not_root` | 1 |
| 5 | `version:` 누락/미달 | 거부 | `env_bridge_requires_version` | 1 |
| 6 | env_file origin 불명 | 거부 | `unknown_env_origin` | 1 |
| 7 | env_file origin이 쓸 수 없음 | 거부 | `unsupported_env_origin` | 1 |
| 8 | selector 모호 | 거부 | `ambiguous_env_selector` | 1 |
| 9 | 암호화 엔트리 없음 | 거부 | `no_encrypted_env_entry` | 1 |
| 10 | target 미선언 | 거부 | `unknown_env_target` | 1 |
| 11 | target에 `sops_source` 없음 | 거부 | `env_target_not_encrypted` | 1 |
| 12 | 경로가 절대경로 | 거부 | `absolute_path` | 1 |
| 13 | 경로가 config root 밖 | 거부 | `path_escapes_config_root` | 1 |
| 14 | 경로 component가 symlink | 거부 | `path_component_symlink` | 1 |
| 15 | source == target | 거부 | `source_is_target` | 1 |
| 16 | **source가 이미 존재** | 거부 | `source_exists` | 1 |
| 17 | source의 부모 디렉토리 없음 | 거부 | `source_parent_missing` | 1 |
| 18 | 평문 target 없음 | 거부 | `target_missing` | 1 |
| 19 | 평문 target이 regular file 아님 | 거부 | `target_not_regular` | 1 |
| 20 | 평문 target 읽기 불가 | 거부 | `permission_denied` | 1 |
| 21 | 평문 target이 dotenv로 파싱 불가 | 거부 | `invalid_dotenv_input` | 1 |
| 22 | 평문 target이 비어 있음 | 거부 | `empty_plaintext_input` | 1 |
| 23 | `.sops.yaml` 부재 | 거부 | `sops_creation_rule_missing` | 1 |
| 24 | `sops` 바이너리 없음 | 거부 | `sops_not_found` | 1 |
| 25 | TTY 없고 `--yes` 없음 | 거부 | `confirmation_required` | 1 |
| 26 | TTY 있고 사용자가 거절 | 취소, source 미생성 | — | 1 |
| 27 | sops 암호화 실패 (rule 미매칭 포함) | 거부, source 미생성 | `sops_creation_rule_missing` 또는 `encrypt_failed` | 1 |
| 28 | temp write/rename/fsync 실패 | 거부, source 미생성 | `permission_denied` | 1 |
| 29 | 위 전부 통과 | source 생성, 평문 target **삭제하지 않음** | — | 0 |

행 29의 단서가 중요하다: **`seal`은 평문 target을 지우지 않는다.** 지우는 것은 별개의 파괴적
행위이고, `unseal`이 만든 파일을 `seal`이 치우는 대칭은 매력적이지만 사용자가 요청하지 않은
삭제다. 정리는 사용자 몫이다.

### 3-4. `show` 계약 — 확정

- **출력은 controlling terminal(`/dev/tty`) 전용.** 열 수 없으면 `no_controlling_terminal`로
  fail-closed. stdout으로 내보내지 않으므로 `>`, `|`, `$(...)`로 포획되지 않는다. 파일이 필요한
  사용처는 `unseal`이 이미 소유한다.
- **`--json` 거부** — `json_unsupported_for_show`.
- **부분 조회 `--key NAME` 도입하지 않는다.** 표면만 늘고 전체 조회를 막지도 못해 보안 이득이 없다.
- **게이트가 여는 것은 `show`의 terminal 출력 하나뿐이다.** TASK-245 §7의 나머지 — debug log,
  오류 메시지, temp 파일명, 다른 명령의 출력·JSON envelope에 복호값이 나타나지 않는다 — 는
  **예외 없이 그대로다.**
- **preflight 부분집합.** 적용: 플랫폼, `env_bridge` origin/version, env_file origin provenance,
  selector, 경로 shape·symlink·containment, source==target, source 상태(존재·regular·읽기 가능).
  **적용하지 않음**: target 부모 디렉토리, target kind, git tracked/ignored — `show`는 파일을
  만들지 않으므로 target 쪽 검사에 대상이 없다.

#### 3-4-1. `show` 판정 순서

`show`만의 검사가 공용 preflight보다 앞에 온다. 이유는 §3-6.

1. 게이트 off → `show_not_enabled`
2. `--json` → `json_unsupported_for_show`
3. 에이전트 환경 감지 → `agent_environment_detected`
4. `/dev/tty` 열기 실패 → `no_controlling_terminal`
5. 위 preflight 부분집합
6. 복호 → terminal에 출력, exit 0

### 3-5. 꺼져 있을 때의 동작 — 확정

**명령은 항상 등록하고 `--help`에 노출한다.** 실행하면 전용 코드로 거부한다. 숨기면 사용자가
명령의 존재와 켜는 방법을 알 수 없다.

| argv | stdout | stderr | exit |
| --- | --- | --- | --- |
| `dva config env seal` | (없음) | `error: seal is disabled; set env_bridge.allow_seal: true in dva.yml to enable it` | 1 |
| `dva config env show` | (없음) | `error: show is disabled; set env_bridge.allow_show: true in dva.yml to enable it` | 1 |

**게이트 검사는 preflight 1단계(platform)보다 앞에 온다.** 꺼진 명령이 플랫폼이나 config 상태에
따라 다른 오류를 내면 게이트가 무슨 일을 하는지 설명할 수 없다. `--help`는 게이트와 무관하게
항상 동작한다.

### 3-6. 에이전트 노출 통제 — 확정

**어떤 호출자 판정도 보안 경계로 선언하지 않는다.** CLI에는 인증된 호출자 신원이 없고, 감지
신호는 전부 호출자 자신의 환경변수이며 `env -u`로 사라진다. 이 문단은 문서에 그대로 남는다.

**Advisory 감지를 채택한다 — `show`에만 적용, 거부, 우회 플래그 없음.**

- 신호 목록(동결): `CLAUDECODE`, `CLAUDE_CODE_ENTRYPOINT`, `AI_AGENT`. 값과 무관하게 **설정되어
  있으면** 참. 목록은 좁게 유지한다 — `TERM_PROGRAM` 같은 에디터 신호는 사람을 오탐하므로 넣지
  않는다.
- 감지 시 `agent_environment_detected`로 거부한다. 오류 문구는 **거부 사실만 말하고 해제 방법을
  적지 않는다.** `--i-am-human` 류 플래그를 두면 오류 메시지가 우회 설명서가 되고 에이전트가
  그것을 읽고 재시도한다.
- 문서에는 **advisory라고 적고 보안 경계라고 적지 않는다.**

**경고: 오탐은 실재하며 이것이 채택의 대가다.** 에이전트 터미널에서 사람이 직접 `show`를 치는
경우가 거부된다. 그러나 그 터미널의 출력은 트랜스크립트에 기록되므로, 그 상황에서의 거부는
오탐이 아니라 정탐에 가깝다. 해제 수단은 플래그가 아니라 환경 자체를 바꾸는 것이다.

**남는 구멍 — pty (TASK-286으로 이관).** 에이전트가 `script -q /dev/null …` 등으로 pty를
할당하면 `/dev/tty`가 열리고 출력이 포획된다. 이는 이미 우회 의도이며 CLI 계층이 막을 수 있는
종류가 아니다. 이 지점부터는 런타임 deny 규칙(TASK-286)의 영역이다.

**층위별 강도** (약함 → 강함): DVA advisory 감지 < `/dev/tty` 전용 출력 < 런타임 deny 규칙
(TASK-286) < `allow_show: false` 유지. **마지막이 가장 강하고, 그것이 기본값이다.**

### 3-7. 신규 error code 목록 (동결)

기존 28개에 더한다. 이름은 TASK-282가 바꾸지 않는다.

| 코드 | 명령 | 조건 |
| --- | --- | --- |
| `seal_not_enabled` | seal | 게이트 off |
| `show_not_enabled` | show | 게이트 off |
| `env_bridge_origin_not_root` | seal, show, validate | module/override가 선언 |
| `env_bridge_requires_version` | seal, show, validate | `version:` 누락 또는 도입 릴리스 미만 |
| `json_unsupported_for_seal` | seal | `--json` |
| `json_unsupported_for_show` | show | `--json` |
| `source_exists` | seal | 기존 source 덮어쓰기 시도 |
| `source_parent_missing` | seal | source 부모 디렉토리 부재 |
| `target_missing` | seal | 평문 target 부재 |
| `invalid_dotenv_input` | seal | 평문 target 파싱 실패 |
| `empty_plaintext_input` | seal | 평문 target이 비어 있음 |
| `sops_creation_rule_missing` | seal | `.sops.yaml` 부재 또는 rule 미매칭 |
| `encrypt_failed` | seal | sops 암호화 실패 (rule 문제 외) |
| `confirmation_required` | seal | TTY 없고 `--yes` 없음 |
| `no_controlling_terminal` | show | `/dev/tty` 열기 실패 |
| `agent_environment_detected` | show | advisory 신호 감지 |

## Completion Criteria

- [x] Freeze the `env_bridge` schema, its accepted declaration locations, and the exact behaviour of a config that omits it | verify: human — accepted and rejected YAML examples must be recorded for every declaration location
- [x] Freeze the gate's origin provenance and multi-origin merge rule, including the subproject case | verify: human — a subproject must not be able to enable the parent's gate, and the conflicting-declaration outcome must be named
- [x] Freeze the `seal` contract with no key or provider arguments, an explicit missing-creation-rule failure, and a decided defense for the lost-update scenario recorded in this card | verify: human — the chosen defense must be deterministic without a TTY, and key names must never be compared by value
- [x] Freeze the full `seal` state matrix across source existence, target state, key-set delta, and flags, reusing TASK-245's atomic-write and containment guarantees | verify: human — the matrix must cover every Cartesian branch and name one code per branch
- [x] Freeze the `show` contract, its output stream, and precisely which TASK-245 redaction rule the gate opens and which remain absolute | verify: human — only `show`'s own human-facing output may be excepted; log, error, JSON and filename rules must be restated as unchanged
- [x] Decide the agent-exposure controls, recording that no caller-identity test is claimed as a security boundary and that any advisory refusal ships without a bypass flag | verify: human — the residual pty hole must be recorded and handed to TASK-286 rather than left implied
- [x] Freeze disabled-state behaviour, help visibility, and the new error codes, placing the gate check before every other preflight step | verify: human — the argv table must show text and exit for both disabled commands
- [x] Record the compatibility consequence of adding a top-level key under `additionalProperties: false`, including which DVA versions reject such a config outright | verify: human — the version boundary and any scaffold/version policy change must be named
- [x] Update PLAN-002 §1-1 and §7 to the narrowed wording, record that TASK-245 §11 is superseded without editing that done card, and create the implementation child before closing | verify: `make doc-check`

## Fail-closed default

판정되지 않은 항목이 하나라도 남으면 그 명령은 게이트가 켜져도 **등록되지 않는다.**
계약 없이 구현된 secret write 표면을 내보내지 않는다.
