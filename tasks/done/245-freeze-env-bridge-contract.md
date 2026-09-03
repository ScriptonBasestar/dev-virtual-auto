---
id: TASK-245
title: "Freeze the public and filesystem contract for the config env bridge"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-01T19:21:00+09:00
source: "PLAN-002 env bridge decision gate"
scope: "env_file source-target model, exact CLI grammar, Git/path safety, output contract, cross-platform replace spike"
status: done
closed-at: 2026-09-03T00:53:27+09:00
needs-human: true
decision-status: decided
---

# Task 245: freeze the env bridge contract

## Summary

Decide the public CLI, configuration shape, filesystem safeguards, and output contract before any
security-sensitive bridge implementation begins.

## Decision required

D9 approved a bridge under the existing `config` group, not the exact `edit`/`unseal` argv or schema.
The current `env_file` accepts string, list, and object shapes and normalizes through `any`. A schema-only
addition can validate and then disappear at runtime. The command also needs a deterministic selector
when more than one env target exists.

No production implementation begins until this card is decided.

## Recommended direction

V1은 effective top-level `env_file`의 object entry에 source↔target 관계를 두고, write command가 선택한
entry의 선언 origin과 path anchor를 하나로 증명할 수 있을 때만 동작하는 방향을 권장한다. Selector는
configured target을 명시하고, encrypted entry가 정확히 하나일 때만 생략을 허용한다. `edit`은 sops가
encrypted source만 편집하게 하고 plaintext target 갱신은 별도 `unseal`에서만 수행한다.

Base/module/override가 합쳐져 provenance가 모호하거나 여러 origin이 같은 target을 주장하면 write를
거부한다. Interaction/subcommand metadata와 owner를 식별할 수 없는 subproject origin도 V1 write 대상에서
제외한다. 이는 load 호환성을 제거한다는 뜻이 아니라, secret-bearing mutation의 대상만 보수적으로
제한한다는 뜻이다. Provenance-preserving loader와 별도 fixture가 승인되면 지원 origin을 넓힐 수 있다.

## Decision (승인됨)

[런북 §5](../../docs/53-command-surface-agent-execution.md)에 따라 에이전트는 결정을 준비만 하고
확정하지 않는다. 아래 option, 근거, matrix, fixture, 기각 대안은 그 준비 단계에서 작성됐고,
`decision-status`는 사용자가 Option을 고르기 전까지 `pending`이었다.

**승인 이력.** 2026-09-03에 사용자가 §0에서 Option A를 선택하고 §13의 남은 불확실성 4개 항목을
전부 카드 권장안대로 확정했다. 런북 §5가 요구하는 사용자 선택은 이 시점에 충족됐고,
`decision-status: decided`와 아래 Completion Criteria 체크는 그 승인에 근거한다. 승인 이전에는
schema, command registration, loader 중 어떤 것도 바꾸지 않았다.

### 0. 상호 배타 option

| Option | 요약 | 판정 |
| --- | --- | --- |
| **A (확정)** | Entry-level `sops_source`, target으로 선택, `edit`/`unseal` 분리, top-level 3 origin만 write | **확정 2026-09-03** |
| B | Top-level `sops:` 별도 section에 source↔target map 선언 | 기각 §11 |
| C | `sops_source` 없이 `<target>.enc` naming convention 추론 | 기각 §11 |
| D | `env_file` shape 유지 + `dva config env unseal <source> <target>` 순수 argv 지정 | 기각 §11 |

Option A가 확정됐으므로 이하 §1~§10이 발효 중인 계약이다.

### 1. 확인된 현재 동작 (2026-09-02, 실제 코드)

- `Config.EnvFile`은 `any`이고 (`internal/config/config.go:21`) string / list / object 세 shape를
  `normalizeEnvFileConfig`가 `EnvFileConfig{Path, Required}`로 평탄화한다 (`internal/config/envfile.go`).
- `mergeFrom`은 `env_file`을 **whole replace**한다 (`internal/config/config.go:1075`,
  `internal/config/merge.go:400`). 따라서 root → modules(`.sb/dva/<mod>.yml`) → `dva.override.yml`
  순서로 마지막에 선언한 파일 하나가 effective `env_file` **전체**를 소유한다. 부분 병합된 리스트는
  만들어지지 않으므로 “여러 origin이 섞인 list”라는 상태는 존재하지 않는다. 다만 merge 후 어느 파일이
  이겼는지는 **기록되지 않아** 현재는 provenance를 복원할 수 없다.
- 상대경로 anchor는 origin과 무관하게 항상 root config 디렉터리다. `LoadEnvFile(..., c.FileDir(), ...)`가
  `filepath.Dir(cfg.filePath)`를 넘기며 module/override는 자기 디렉터리를 anchor로 쓰지 않는다.
- Schema는 `#/definitions/env_file` **하나**를 top-level(`internal/config/schema.json:948`)과
  `interaction_command`(`internal/config/schema.json:388`, 정의는 331행)가 공유한다. entry object는
  `additionalProperties: false`이므로 새 key는 명시 추가해야 하고, 공유 정의에 추가하면
  interaction에서도 자동 허용된다.
- `config show`는 `yaml.Marshal(c)` 후 `any`로 되읽는다 (`internal/cli/config_dump.go`). `EnvFile`이
  `any`인 동안은 선언한 key가 그대로 round-trip된다.
- 실패 JSON envelope은 `{"error":{"message":…,"exit_code":1}}` 하나뿐이다
  (`internal/cli/root.go:373`, `internal/cli/validate_json.go:29`). 이미 stdout에 document가 있으면
  envelope을 붙이지 않는다.
- `config` group 하위는 `show`, `docs`, `init`, `migrate`, `validate`다.

### 2. Source↔target 표현 (criterion 1)

Entry object에 **`sops_source`** 한 개를 추가한다. `path`는 오늘과 같은 plaintext target이고
`sops_source`는 그 target을 만들어내는 encrypted source다. 관계는 항상 **한 entry 안**에 갇힌다.

허용 위치는 top-level `env_file` **entry object** 뿐이다. bare string form에는 놓을 자리가 없고,
object wrapper(`{files:…, required:…}`) level에도 두지 않는다. wrapper level에 두면 source 하나가
여러 target을 주장하게 되어 §3의 fail-closed 선택 규칙과 충돌한다.

#### 2-1. 모든 기존 shape의 동작

| 기존 shape | 예시 | Load / merge / show / validate | Write 대상 |
| --- | --- | --- | --- |
| string | `env_file: .env` | 무변화 | 아니오 (source 없음) |
| list of string | `env_file: [.env.defaults, .env]` | 무변화 | 아니오 |
| list of object | `- {path: .env, required: true}` | 무변화 | `sops_source` 선언 시 예 |
| object wrapper + string files | `{files: .env, required: true}` | 무변화 | 아니오 |
| object wrapper + list files | `{files: [...], required: true}` | 무변화, wrapper `required`는 종전대로 각 entry에 OR | list 안 entry object가 선언하면 예 |

`sops_source`는 **load 경로에 어떤 영향도 주지 않는다**. `LoadEnvFile`은 이 필드를 읽지 않고,
자동 복호도 하지 않으며, `required` 의미도 바꾸지 않는다. 선언 순서, 후속 파일 override, OS 우선순위,
`config show` 출력, `validate` exit는 모두 오늘과 동일하다. `EnvFileConfig`에 `SopsSource string`을
추가하되 `Config.EnvFile`은 **`any`로 유지**한다. 타입 struct로 바꾸면 `config show`의 round-trip
모양이 바뀌므로 금지한다.

#### 2-2. Top-level: 수용 예시

```yaml
# 1) 단일 encrypted entry — selector 생략 가능
env_file:
  - path: .env
    sops_source: secrets.env.enc
    required: true

# 2) plaintext entry와 공존
env_file:
  - .env.defaults
  - path: .env
    sops_source: secrets.env.enc

# 3) wrapper 안의 entry object
env_file:
  files:
    - path: .env
      sops_source: secrets.env.enc
  required: true

# 4) 복수 encrypted entry — selector 필수
env_file:
  - path: .env
    sops_source: secrets.env.enc
  - path: .env.local
    sops_source: secrets.local.env.enc
```

#### 2-3. Top-level: 거부 예시

```yaml
# R1 wrapper level source — 하나의 source가 여러 target을 주장
env_file:
  files: [.env, .env.local]
  sops_source: secrets.env.enc          # schema error: additionalProperties

# R2 같은 target을 두 entry가 주장
env_file:
  - {path: .env, sops_source: a.enc}
  - {path: .env, sops_source: b.enc}    # validation error: duplicate_env_target

# R3 하나의 source가 두 target을 주장
env_file:
  - {path: .env, sops_source: s.enc}
  - {path: .env.local, sops_source: s.enc}   # validation error: duplicate_env_source

# R4 다른 entry의 target을 source로 재사용 (chaining)
env_file:
  - {path: .env, sops_source: secrets.env.enc}
  - {path: .env.local, sops_source: .env}    # validation error: env_source_is_target

# R5 source == target
env_file:
  - {path: .env, sops_source: .env}          # validation error: source_is_target

# R6 절대경로 / config root 밖
env_file:
  - {path: /etc/app.env, sops_source: secrets.env.enc}   # write preflight error
  - {path: ../sibling/.env, sops_source: s.enc}          # write preflight error
```

R2~R5는 **선언 검증**(`Validate`) 단계 오류다. `sops_source`를 하나도 쓰지 않은 config에서
`path` 중복은 오늘의 관대한 동작을 그대로 유지한다 — 새 오류는 encrypted entry가 관여할 때만 생긴다.
R6은 load는 오늘처럼 계속 성공하고 **write에서만** 거부한다 (§5).

#### 2-4. Interaction / subcommand: 전부 거부

[TASK-265](265-decide-interaction-env-file-contract.md)는 `interaction.*.env_file`을 versioned
rejection으로 결정했다 — 0.1.48에서 deprecate, 0.1.49에서 schema가 field 자체를 거부한다
(구현은 [TASK-266](266-deprecate-and-reject-interaction-env-file.md) 소유). 현재 버전은 0.1.47이므로
`sops_source`가 도입되는 시점에 그 field가 아직 살아 있을 수 있다. 두 시점 모두를 고정한다.

**TASK-266 Stage B 이전 (field가 아직 존재하는 동안)** — 공유 정의를 둘로 나눈다.

- `#/definitions/env_file` — top-level 전용, entry object에 `sops_source` 허용
- `#/definitions/env_file_plain` — `sops_source` 없는 동일 구조. `interaction_command.env_file`
  (`internal/config/schema.json:388`)과 그 subcommand가 이것을 참조

```yaml
# 수용 (오늘과 동일, 0.1.48에서 deprecation warning 동반)
interaction:
  api:
    command: npm start
    env_file: .env

# 거부 — schema error, 조용한 무시가 아님
interaction:
  api:
    command: npm start
    env_file:
      - path: .env
        sops_source: secrets.env.enc
```

**TASK-266 Stage B 이후 (0.1.49~)** — `interaction_command.env_file`이 사라지므로 `env_file_plain`은
존재 이유가 없다. 그 정의를 **남겨두지 말고 삭제**한다. 죽은 정의를 남기면 다음 독자가 interaction
위치가 여전히 지원된다고 읽는다. TASK-246과 TASK-266 중 나중에 통합되는 쪽이 이 정리를 소유한다.

Schema 검증을 거치지 않는 경로가 있으므로, 두 시점 모두에서 normalizer도 같은 조건에서
**hard error**를 반환한다. 이 카드가 고정하는 것은 “encrypted-source metadata는 top-level entry
object 밖 어디에서도 수용되지 않는다” 하나이며, interaction `env_file` 자체의 수명은 TASK-265/266이
소유한다.

### 3. Command grammar와 selector (criterion 2)

새 top-level 예약 없이 기존 `config` group에 `dva <group> <noun> <verb>` 형태로 넣는다.

```text
dva config env unseal [target] [--force]
dva config env edit   [target]
```

`target`은 **선언된 `path` 문자열 그대로**다. source 이름도, index도, glob도 아니다. 해석·정규화·절대화한
경로와 비교하지 않는다. `--all`, `--yes`, 복수 target, 암묵 다중 write는 v1에 없다.

Encrypted entry = `sops_source`를 선언한 entry. 개수별 규칙:

| Encrypted entry 수 | argument | 결과 |
| --- | --- | --- |
| 0 | 있든 없든 | `no_encrypted_env_entry`, exit 1 |
| 1 | 없음 | 그 entry 선택 |
| 1 | 일치 | 그 entry 선택 |
| 1 | 불일치 | `unknown_env_target`, exit 1 |
| N>1 | 없음 | `ambiguous_env_selector`, exit 1, 선언된 target 목록 제시 |
| N>1 | 정확히 하나와 일치 | 그 entry 선택 |
| N>1 | 불일치 | `unknown_env_target`, exit 1 |
| 임의 | encrypted가 아닌 entry의 path와 일치 | `env_target_not_encrypted`, exit 1 |

#### 3-1. Argv 표

`E` = 기존 실패 envelope (§7), `S` = 성공 document (§7).

| Argv | 조건 | stdout (text) | stdout (`--json`) | exit |
| --- | --- | --- | --- | --- |
| `dva config env unseal` | encrypted 1개, target 없음 | `unsealed .env from secrets.env.enc (created)` | `S` result=`created` | 0 |
| `dva config env unseal` | encrypted 1개, target 있음, force 없음 | (비움) | `E` code=`target_exists` | 1 |
| `dva config env unseal --force` | encrypted 1개, target regular | `unsealed .env from secrets.env.enc (replaced)` | `S` result=`replaced` | 0 |
| `dva config env unseal` | encrypted 0개 | (비움) | `E` code=`no_encrypted_env_entry` | 1 |
| `dva config env unseal` | encrypted 2개 | (비움) | `E` code=`ambiguous_env_selector` | 1 |
| `dva config env unseal .env` | encrypted 2개 중 일치 | `unsealed .env from secrets.env.enc (created)` | `S` | 0 |
| `dva config env unseal .env.x` | 일치 없음 | (비움) | `E` code=`unknown_env_target` | 1 |
| `dva config env unseal .env.defaults` | plaintext entry와 일치 | (비움) | `E` code=`env_target_not_encrypted` | 1 |
| `dva config env unseal --force` | target이 symlink | (비움) | `E` code=`target_not_regular` | 1 |
| `dva config env unseal` | target이 tracked | (비움) | `E` code=`target_tracked` | 1 |
| `dva config env unseal` | Windows | (비움) | `E` code=`unsupported_platform` | 1 |
| `dva config env edit` | encrypted 1개 | sops/$EDITOR passthrough (hint는 stderr) | 거부 | 0 |
| `dva config env edit --json` | 임의 | (비움) | `E` code=`json_unsupported_for_edit` | 1 |
| `dva config env edit --force` | 임의 | (비움) | `E` code=`force_unsupported_for_edit` | 1 |

`--force`는 `unseal` 전용 flag이며 `edit`에 등록하지 않는다.

### 4. `edit` 소유권과 `unseal` state matrix (criterion 3)

`edit`은 **encrypted source만** 다룬다. `sops edit <source>`를 argv로 실행하고 stdin/stdout/stderr를
그대로 넘긴다. plaintext target을 읽지도 쓰지도 만들지도 않는다. 성공 후 stderr에 그 release에 실제로
존재하는 명령만 가리키는 한 줄을 남긴다.

```text
note: .env is now stale; run `dva config env unseal .env` to refresh it
```

`edit`은 새 source를 만들지 않는다. source 생성·키 회전·provider 설정은 sops/age 소유이며 DVA는
그 표면을 재구현하지 않는다.

#### 4-1. Unseal 전수 matrix

Source 상태 × target 상태 × `required` × `--force`. `required`는 **모든 행에서 결과에 영향이 없다**
(load 전용 필드이므로 write 경로에 배선하지 않는다). 그래서 아래 표는 `required`를 열로 두고 값이
같음을 명시한다.

| # | Source | Target | `required` | `--force` | 결과 | Code |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 없음 | 없음 | true/false | no/yes | 실패 | `source_missing` |
| 2 | 없음 | regular | true/false | no/yes | 실패, target 무변경 | `source_missing` |
| 3 | 없음 | non-regular | true/false | no/yes | 실패 | `source_missing` |
| 4 | symlink/non-regular | 임의 | true/false | no/yes | 실패 | `source_not_regular` |
| 5 | 읽기 불가 | 임의 | true/false | no/yes | 실패 | `source_unreadable` |
| 6 | regular | 없음 | true/false | no | **생성** | — |
| 7 | regular | 없음 | true/false | yes | **생성** (force 무의미, 오류 아님) | — |
| 8 | regular | regular | true/false | no | 실패, target 무변경 | `target_exists` |
| 9 | regular | regular | true/false | yes | **교체** | — |
| 10 | regular | symlink | true/false | no/yes | 실패 | `target_not_regular` |
| 11 | regular | directory | true/false | no/yes | 실패 | `target_not_regular` |
| 12 | regular | fifo/socket/device | true/false | no/yes | 실패 | `target_not_regular` |
| 13 | regular | source와 동일 경로 | true/false | no/yes | 실패 | `source_is_target` |
| 14 | regular | Git tracked | true/false | no/yes | 실패 | `target_tracked` |
| 15 | regular | Git repo 안·untracked·not ignored | true/false | no/yes | 실패 | `target_not_ignored` |
| 16 | regular | Git repo 밖 | true/false | no/yes | 6~9 규칙 적용 + notice | — |
| 17 | regular | 부모 디렉터리 없음 | true/false | no/yes | 실패 (디렉터리를 만들지 않음) | `target_parent_missing` |
| 18 | regular | 부모 디렉터리 write 불가 | true/false | no/yes | 실패 | `permission_denied` |
| 19 | regular | 임의 | true/false | no/yes | sops 실패 시 실패, target 무변경 | `decrypt_failed` |
| 20 | regular | 임의 | true/false | no/yes | sops exit 0 + 0 byte면 실패 | `empty_decrypted_output` |
| 21 | regular | 임의 | true/false | no/yes | dotenv 파싱 실패면 실패 | `invalid_dotenv_output` |
| 22 | regular | 임의 | true/false | no/yes | sops 바이너리 없음 | `sops_not_found` |
| 23 | regular | 임의 | true/false | no/yes | 미검증 OS | `unsupported_platform` |

행 4·10~15·23은 **`--force`로 뚫리지 않는다 (non-overridable)**. 행 8만이 force의 유일한 용도다.
행 17에서 디렉터리를 자동 생성하지 않는 이유는, 존재하지 않는 부모를 만들면 containment 검사가
검증한 트리 밖에 새 경로를 만들어낼 수 있기 때문이다.

#### 4-2. 검사 순서 (고정)

여러 조건이 동시에 참일 때 어떤 code가 나오는지가 fixture로 고정되도록 순서를 못박는다. 첫 실패에서
즉시 중단하며, 이 순서 앞쪽 단계는 **뒤쪽 단계가 필요로 하는 정보를 만들지 않고도 판정 가능**하도록
배치했다. sops는 9단계를 모두 통과한 뒤에만 실행된다 — 거부될 write를 위해 복호하지 않는다.

1. platform (`unsupported_platform`)
2. origin provenance (`unknown_env_origin`, `unsupported_env_origin`)
3. selector (`no_encrypted_env_entry`, `ambiguous_env_selector`, `unknown_env_target`,
   `env_target_not_encrypted`)
4. 경로 형태 (`absolute_path`, `path_escapes_config_root`, `path_component_symlink`)
5. `source_is_target`
6. source 상태 (`source_missing`, `source_not_regular`, `source_unreadable`)
7. target 부모 (`target_parent_missing`)
8. target 종류 (`target_not_regular`)
9. Git (`git_unavailable` → `target_tracked` → `target_not_ignored`)
10. `target_exists` 대 `--force`
11. `sops_not_found`
12. 복호 (`decrypt_failed`, `empty_decrypted_output`, `invalid_dotenv_output`)

`permission_denied`는 순서가 아니라 위치다 — 어느 I/O 단계에서든 EACCES/EPERM이 나오면 그 시점에
보고하고, 그때까지 target은 변경되지 않은 상태다.

### 5. Origin, provenance, path anchor, 안전 상태 (criterion 4)

#### 5-1. 허용 origin

| Origin | Load | V1 write |
| --- | --- | --- |
| root `dva.yml` top-level `env_file` | 예 | **예** |
| `dva.override.yml` top-level `env_file` | 예 | **예** |
| module `.sb/dva/<mod>.yml` top-level `env_file` | 예 | **예** |
| subproject / imported child config | 예 (child owner) | 아니오 — `unsupported_env_origin` |
| interaction / subcommand `env_file` | TASK-265/266 소유 (0.1.49에 거부) | 아니오 — schema error (§2-4) |

Subproject를 제외하는 이유는 owner·anchor·Git repo가 parent와 달라 §5-3의 보증을 parent 세션에서
증명할 수 없기 때문이다. 사용자는 child 디렉터리에서 같은 명령을 실행하면 된다 — 기능 손실이 아니라
mutation 대상의 축소다.

#### 5-2. Provenance

`env_file`은 whole-replace이므로 effective 선언의 origin은 **정확히 하나**다. TASK-246은 load 시
승자 origin 파일 경로를 `Config`의 **unexported** 필드에 기록한다 (`filePath`와 같은 방식이라
`config show` 출력에 새 key가 생기지 않는다). Write는 이 값을 요구한다.

- origin이 §5-1의 허용 3종 중 하나가 아니면 → `unsupported_env_origin`, exit 1
- origin이 비어 있거나 복원 불가면 → `unknown_env_origin`, exit 1 (조용한 추정 금지)

“여러 origin이 같은 target을 주장”하는 상태는 현재 loader에서 만들어지지 않는다. 대신 **한 선언 안의**
중복(§2-3 R2~R5)을 선언 검증에서 닫는다. 나중에 provenance-preserving partial merge를 도입하면 그때
같은 target을 주장하는 복수 origin을 `ambiguous_env_provenance`로 거부하도록 이 절을 다시 연다.

#### 5-3. Anchor와 containment

- **Anchor는 origin과 무관하게 root config 디렉터리**(`Config.FileDir()`)다. 이것은 오늘 loader가
  실제로 하는 동작이고, module/override에 자기 디렉터리 anchor를 도입하면 기존 선언의 load 위치가
  조용히 바뀐다. 그 호환성 파괴가 “module이 자기 옆 파일을 가리킨다”는 직관보다 비싸다.
- `path`와 `sops_source`는 write 대상일 때 **둘 다 상대경로**여야 한다. 절대경로 → `absolute_path`.
  Load는 절대경로를 계속 지원한다 (기존 호환).
- 두 경로 모두 anchor 트리 **안**이어야 한다. `..` escape → `path_escapes_config_root`.
- 강제 수단은 `os.OpenRoot(anchor)` 하나의 handle이다. 이 handle을 preflight부터 replace까지
  **계속 잡고** 모든 open/lstat/rename/remove를 handle-relative로 수행한다.
- `os.Root`는 **root 안의 symlink는 따라간다** (§8 spike로 확인). 따라서 containment만으로 부족하다.
  두 경로의 **모든 component를 `Root.Lstat`으로 검사**해 symlink component가 하나라도 있으면 거부한다
  (`path_component_symlink`). Target의 마지막 component는 부재이거나 regular여야 한다.
- 위 항목은 전부 **non-overridable**이다.

#### 5-4. Git 상태

`git` plumbing exit code는 §8 spike에서 고정했다.

| 상태 | 판정 도구 | 결과 |
| --- | --- | --- |
| Repo 안 · tracked | `git ls-files --cached --error-unmatch -- <target>` exit 0 | 거부 `target_tracked` |
| Repo 안 · ignored | `git check-ignore --quiet -- <target>` exit 0 | 허용 |
| Repo 안 · untracked · not ignored | 위 둘 다 exit 1 | 거부 `target_not_ignored` |
| Repo 밖 | 조상에 `.git` 없음 | 허용 + notice |
| Repo 안 · `git` 미설치 | 조상에 `.git` 있음, `git` PATH 없음 | 거부 `git_unavailable` |

“repo 안인가”는 `git` 없이도 판정해야 하므로 anchor에서 위로 올라가며 `.git` (디렉터리 또는 파일)을
찾는 자체 탐색으로 결정한다. repo 안이라고 판정했는데 `git`이 없으면 tracked 여부를 **증명할 수 없으므로**
fail closed한다. Repo 밖 허용은 tracked plaintext 위험 자체가 존재하지 않기 때문이며, 이때도 §4 행 8의
force 규칙은 그대로 적용된다.

### 6. `--force`의 범위 (criterion 5)

`--force`는 **행 8 하나** — 존재하는 regular target의 덮어쓰기 — 만 허용한다. tracked, not-ignored,
symlink/non-regular, source=target, absolute, path escape, component symlink, unsupported origin,
unsupported platform 중 어느 것도 우회하지 않는다. `edit`에는 존재하지 않는다.

기각한 대안과 마이그레이션 안내는 §11에 있다.

### 7. 출력 계약 (criterion 7)

#### 7-1. 실패

기존 root envelope **하나만** 쓴다. 새 최상위 envelope을 만들지 않는다. 기계 코드는 같은 `error`
객체의 **선택적 `code` key**로 싣는다 — 이것은 두 번째 envelope이 아니라 기존 envelope의 가산 확장이며,
`.error.message` / `.error.exit_code`를 읽는 기존 소비자는 영향을 받지 않는다.

```json
{"error":{"code":"target_exists","message":"env bridge: .env already exists; pass --force to replace it","exit_code":1}}
```

Human mode는 stdout을 비우고 기존 root 경로가 stderr에 `ERROR: <message>`를 출력한다.

Code 집합은 **닫혀 있다**. 아래가 전부이며 구현자가 새 code를 발명하지 않는다. `<t>`는 선언된 target
문자열, `<s>`는 선언된 source 문자열이다. 모든 message는 `env bridge: ` prefix를 갖는다.

| Code | Message (prefix 뒤) | 발생 |
| --- | --- | --- |
| `no_encrypted_env_entry` | `no env_file entry declares sops_source` | §3 |
| `ambiguous_env_selector` | `multiple encrypted env_file entries; name one: <t>, <t>` | §3 |
| `unknown_env_target` | `unknown env target "<t>"; declared encrypted targets: <t>, <t>` | §3 |
| `env_target_not_encrypted` | `env target "<t>" declares no sops_source` | §3 |
| `json_unsupported_for_edit` | `edit is interactive and has no --json output` | §3-1 |
| `force_unsupported_for_edit` | `edit never writes a target, so --force does not apply` | §3-1 |
| `unsupported_env_origin` | `env_file is declared by an origin that cannot be written: <origin>` | §5-2 |
| `unknown_env_origin` | `cannot determine which file declares env_file` | §5-2 |
| `absolute_path` | `"<p>" is absolute; env bridge writes only config-relative paths` | §5-3 |
| `path_escapes_config_root` | `"<p>" resolves outside the config directory` | §5-3 |
| `path_component_symlink` | `"<p>" contains a symlinked path component` | §5-3 |
| `source_missing` | `encrypted source <s> does not exist` | §4 행 1~3 |
| `source_not_regular` | `encrypted source <s> is not a regular file` | §4 행 4 |
| `source_unreadable` | `cannot read encrypted source <s>` | §4 행 5 |
| `source_is_target` | `<t> is both the source and the target` | §4 행 13 |
| `target_exists` | `<t> already exists; pass --force to replace it` | §4 행 8 |
| `target_not_regular` | `<t> is not a regular file` | §4 행 10~12 |
| `target_tracked` | `<t> is tracked by git; a decrypted file must never be tracked` | §4 행 14 |
| `target_not_ignored` | `<t> is not ignored by git; add it to .gitignore first` | §4 행 15 |
| `target_parent_missing` | `the directory for <t> does not exist` | §4 행 17 |
| `permission_denied` | `permission denied writing <t>` | §4 행 18 |
| `git_unavailable` | `<t> is inside a git repository but git is not on PATH` | §5-4 |
| `sops_not_found` | `sops is not installed or not on PATH` | §4 행 22 |
| `decrypt_failed` | `decryption failed for <s>` | §4 행 19 |
| `empty_decrypted_output` | `sops produced no output for <s>` | §4 행 20 |
| `invalid_dotenv_output` | `decrypted output is not valid dotenv at line N` | §4 행 21 |
| `unsupported_platform` | `this platform is not supported for env bridge writes` | §4 행 23, §8-3 |

§2-3의 R2~R5는 **bridge code가 아니다**. 선언 검증 오류이므로 기존 config-load 실패 경로로 나가며,
`dva config validate`와 모든 명령이 동일하게 거부한다. bridge를 실행하지 않아도 발생한다.

#### 7-2. 성공

`--json` 성공은 stdout에 document 하나를 쓴다. 이미 document가 있으면 envelope이 붙지 않는 기존 규칙
(`internal/cli/root.go`의 `StdoutHasDocument`)과 정합한다.

```json
{"action":"unseal","target":".env","source":"secrets.env.enc","result":"created"}
```

`result`는 `created` 또는 `replaced`다. Human mode는 stdout 한 줄이다.

```text
unsealed .env from secrets.env.enc (created)
```

§4 행 16(Git repo 밖)에서는 위 성공 출력에 더해 **stderr**에 한 줄을 낸다. stdout document는 바뀌지
않고, JSON 성공 document에 새 key도 만들지 않는다.

```text
note: .env is not inside a git repository; nothing verifies that it stays untracked
```

`edit` 성공도 stdout이 아니라 stderr에 §4의 stale hint 한 줄을 낸다. `edit`의 stdout은 sops/$EDITOR
passthrough 전용이다.

경로는 **선언된 문자열 그대로** 출력한다. 로컬 절대경로로 확장하지 않는다 (TASK-247 §2와 동일 규칙).

#### 7-3. Exit code

모든 실패는 **exit 1**이다. 새 exit code를 만들지 않고 **child exit code를 전파하지 않는다**. §8에서
확인했듯 sops는 키 실패에서 **exit 128**을 낸다. 이 값이 새어 나가면 DVA의 exit 계약이 sops 구현에
묶인다.

#### 7-4. Redaction

- 복호 plaintext는 DVA의 buffer를 거치지 않는다. **DVA가 만든 0600 O_EXCL temp의 fd를 sops child의
  stdout으로 직접 준다.** 성공 시에도 값이 문자열로 존재하지 않는다.
- 유일한 예외는 replace 직전의 dotenv 검증이다. 이 parser는 temp를 다시 읽되 **count와 line number만**
  반환하고 key도 value도 반환·기록하지 않는다.
- sops stderr는 상한 있는 buffer로 받고 **그대로 출력하지 않는다**. §7-1의 고정 문구만 낸다.
  (spike에서 sops stderr에 sentinel이 0회 나타났지만, 그것을 계약으로 삼지 않는다.)
- temp 이름은 `.dva-env-<pid>-<nanos>.tmp` — 파일 내용에서 유도한 성분이 없다.
- debug log는 argv(경로만)를 남길 수 있고 값·key·file 내용은 남기지 않는다.

### 8. Cross-platform replace / concurrency spike (criterion 6)

#### 8-1. 실행한 것

동일한 Go 프로그램(`os.Root` handle-relative safe writer)을 두 OS에서 실행했다. 13개 시나리오:
replace-over-existing, temp 0600, O_EXCL 충돌, file+dir fsync, 4 프로세스 × 300 replace 동시성,
root escape, symlink target, preflight 후 symlink swap, handle 보유 후 부모 디렉터리 symlink 교체,
non-regular target, 실패 후 target 불변, owned stale-temp 회수, SIGINT 취소 정리.

| OS | 버전 | Go | FS | 결과 |
| --- | --- | --- | --- | --- |
| darwin/arm64 | macOS 26.6.2 (25G83) | go1.26.5 | APFS | 13/13 PASS, 417,957 reads, torn 0, residue 0 |
| linux/arm64 | Debian 13, kernel 6.8.0-100 (container) | go1.26.8 | overlay | 13/13 PASS, 130,448 reads, torn 0, residue 0 |
| windows | — | — | — | **미실행** |

명령:

```text
go vet ./... && go build -o envspike . && ./envspike
tar cf - go.mod main.go | docker run --rm -i golang:1.26 \
  sh -c 'mkdir -p /src && cd /src && tar xf - && go run .'
```

sops 계약 spike (macOS, sops 3.13.3, age 1.3.1):

```text
sops encrypt --age <pub> --input-type dotenv --output-type dotenv plain.env > secrets.env.enc   # exit 0
sops decrypt --input-type dotenv --output-type dotenv secrets.env.enc > owned.env               # exit 0
SOPS_AGE_KEY_FILE=/nonexistent sops decrypt … secrets.env.enc                                   # exit 128, stdout 0 byte
sops decrypt … plain.env                                                                         # exit 1, stdout 0 byte, "sops metadata not found"
```

Git plumbing spike:

```text
git check-ignore --quiet -- <p>           # ignored=0, 그 외=1
git ls-files --cached --error-unmatch -- <p>   # tracked=0, 그 외=1
git rev-parse --show-toplevel             # repo 안=0, 밖=128
```

#### 8-2. 확인된 사실

- `Root.Rename`은 존재하는 regular target을 교체하고, 동시 writer 4개 × 300회 동안 reader가
  **잘린 내용을 한 번도 보지 못했다**. 마지막 내용은 항상 어느 한 writer의 완결된 payload다
  (last-writer-wins, lost update는 “동시에 서로 다른 두 사용자가 같은 target을 unseal” 시나리오에만
  해당하며 §8-4에서 다룬다).
- temp는 0600으로 생성되고 rename 후에도 0600을 유지한다.
- 두 OS 모두 **부모 디렉터리 fsync가 수용**된다.
- `os.Root`는 `..`와 절대경로를 차단한다 (`path escapes from parent`).
- **`os.Root`는 root 안의 symlink를 차단하지 않는다.** 그래서 §5-3의 명시적 `Lstat` component 검사가
  필수다. 이것이 spike가 바꾼 설계 결정이다.
- preflight 뒤 target을 symlink로 바꿔치기해도 `rename(2)`는 **마지막 component를 따라가지 않고**
  링크 자체를 교체했다. victim 파일은 byte-identical이었다. 즉 swap은 write 유출로 이어지지 않는다.
- handle을 잡은 뒤 config 디렉터리 symlink를 다른 디렉터리로 repoint해도 write는 **원래 해석된
  디렉터리**로 갔다. 이것이 handle-relative TOCTOU 방어의 실증이다.
- 취소(SIGINT)는 temp를 지우고 target을 건드리지 않는다.

#### 8-3. Windows: 미검증, fail closed

Windows를 v1 지원에서 제외하고 `unsupported_platform`으로 fail closed한다. 추정이 아니라 확인한 근거:

- Go는 Windows에서 file mode를 **`FILE_ATTRIBUTE_READONLY` 하나로만** 매핑한다
  (`os/root_windows.go:323-325`, `os/types_windows.go:178`). 따라서 `0600`이 **다른 사용자 접근을
  제한하지 않는다.** plaintext secret 파일을 쓰는 명령에 이것은 치명적이다.
- `os.Rename`은 `MoveFileEx(..., MOVEFILE_REPLACE_EXISTING)`이다
  (`os/file_windows.go:259`, `internal/syscall/windows/syscall_windows.go:366`). target을 다른
  프로세스가 `FILE_SHARE_DELETE` 없이 열고 있으면 실패하므로 replace 성공이 조건부다.
- 디렉터리 handle에 대한 `FlushFileBuffers` 동작을 검증하지 못했다.

이 셋 중 하나라도 별도 evidence로 해소되면 그때 지원 OS를 넓히는 결정을 다시 연다.

#### 8-4. 남은 미해결 보증

- **SIGKILL / 전원 손실**: temp가 남을 수 있다. cleanup을 보장한다고 주장하지 않는다. 대신 §8-5의
  owned stale-temp 회수가 다음 실행에서 정리한다.
- **Durability**: file fsync + parent dir fsync까지 보장한다. 디스크 write cache 뒤(비 F_FULLFSYNC
  macOS 경로 포함)는 보장하지 않는다.
- **Lost update**: 두 `unseal`이 같은 target에 동시 실행되면 한쪽이 조용히 이긴다. v1은 두 writer가
  **같은 source에서 같은 내용**을 만들기 때문에 실질 피해가 없다. 서로 다른 source가 같은 target을
  주장하는 것은 §2-3 R2에서 선언 시점에 막힌다. 추가 lock은 v1 범위 밖이며, 필요해지면 anchor 안의
  O_EXCL lock 파일로 도입한다.
- **파일시스템 범위**: APFS와 overlay만 실측했다. ext4/xfs/zfs는 미실측(POSIX rename 계약 동일 가정),
  **네트워크 파일시스템(NFS/SMB)과 VM bind mount는 rename 원자성 보증이 약하며 DVA는 이를 이식성 있게
  탐지할 수 없다.** 그러므로 그 위에서의 durability를 주장하지 않는다.
- **아키텍처**: Linux 근거는 aarch64다. `ubuntu-latest`는 amd64이므로 CI가 그 델타를 덮는다.

#### 8-5. Owned stale-temp 회수

`unseal` 시작 시 anchor 디렉터리에서 다음을 **모두** 만족하는 항목만 제거한다: `.dva-env-` prefix,
`.tmp` suffix, `Root.Lstat` 기준 regular file, mtime이 1시간 이상 경과. spike에서 이 규칙이 owned temp
1개를 지우고 이름이 다른 파일을 보존했다. 다른 사용자 소유 파일이나 prefix 밖 파일은 절대 지우지 않는다.

#### 8-6. 지원 OS와 CI matrix

**지원 선언: `linux`, `darwin`. 그 외는 `unsupported_platform`으로 fail closed.**

TASK-246이 `.github/workflows/ci.yml`에 추가하는 `config-env-platform` job을 다음으로 고정한다.

```yaml
config-env-platform:
  strategy:
    matrix:
      os: [ubuntu-latest, macos-latest]
  runs-on: ${{ matrix.os }}
  # 1) go test ./internal/cli ./internal/config
  # 2) pinned sops(3.13.x) + age(1.3.x) 설치 후 make test-integration
```

여기에 더해 **미지원 platform의 fail-closed 경로**를 OS와 무관하게 실행되는 table test로 검증한다
(`runtime.GOOS` 게이트를 주입 가능하게 만들고 `windows` 값에서 `unsupported_platform`을 확인).

**Conditional platform rule 판정: 별도 CI enablement child를 만들지 않는다.** 이유는
[TASK-246](246-implement-secure-config-env-bridge.md)이 이미 `config-env-platform` job을 `ci.yml`에
추가하는 것을 자기 acceptance criterion으로 소유하고 있어, 그 job을 1-OS가 아닌 2-OS matrix로 쓰는 것이
범위 확장이 아니기 때문이다. 단 하나의 조건을 붙인다 — `macos-latest`에서 pinned sops/age 설치가
재현 가능하게 되지 않으면 **범위를 넓히지 말고 darwin을 fail-closed로 내리고** 그 사실을 이 카드에
기록한다.

### 9. TASK-246 acceptance fixture

| TASK-246 criterion | 이 결정이 요구하는 fixture |
| --- | --- |
| `TestConfigEnvLegacyShapeRoundTrip` | §2-1 다섯 shape의 load 순서·`required`·merge 결과·`config show` round-trip 불변 |
| `TestConfigEnvSourceMetadataScope` | §2-4 interaction/subcommand 거부(무시 아님), §2-3 R1~R5, §5-1 subproject origin 거부 |
| `TestConfigEnvRejectsPathSwap` | §8-2의 세 swap 시나리오 + component symlink 거부 + absolute/`..` 거부 |
| `TestConfigEnvAtomicWriteFaultMatrix` | §4 행 1~23 전부 + create/write/sync/close/rename 주입 실패 + §8-5 회수 |
| `TestConfigEnvConcurrentWriters` | §8-2 동시성(프로세스 다중, torn 0, residue 0, 실패 후 byte-identical) |
| `TestConfigEnvNeverEmitsSecretSentinel` | §7-4 — stdout/stderr/JSON/debug log/error/temp 이름 전부 |
| `TestConfigEnvRealSOPS` | §8-1 argv, exit 128 비전파, 0 byte 거부, dotenv in/out 고정 |

추가로 §3-1 argv 표의 모든 행을 text/JSON/exit snapshot으로 고정한다.

**Interop 주의**: spike에서 `API_TOKEN=tok-123 # inline comment`를 sops가 값 전체로 보존한 반면,
DVA의 `parseEnvFile`은 unquoted 값에서 ` #` 이후를 주석으로 잘라낸다 (`internal/config/envfile.go`).
즉 sops가 쓴 값과 DVA가 읽는 값이 달라질 수 있다. 이것은 이 카드가 만드는 결함이 아니라 기존 parser
동작이지만, unseal은 **DVA 자신의 parser**로 검증해야 하고 (§7-4) 사용자 문서는 ` #`를 포함한 값에
따옴표를 쓰라고 안내해야 한다.

### 10. 비협상 baseline 준수 확인

Sops는 shell 없이 argv로 실행(§8-1), dotenv in/out 명시(§8-1), secret은 출력에 도달하지 않음(§7-4),
모호한 selector는 write 전에 실패(§3), DVA는 age key/provider를 소유하지 않음(§4).

### 11. 기각한 대안

- **Option B (top-level `sops:` section)**: `env_file`과 별개의 두 번째 진실 원천이 생겨 두 선언이
  어긋날 수 있고, `env_file`의 whole-replace merge와 새 section의 merge 규칙이 달라지면 provenance가
  더 나빠진다.
- **Option C (naming convention 추론)**: 선언 없이 파일 이름으로 secret 관계를 추정하는 것은 사용자가
  의도하지 않은 파일을 write 대상으로 만든다. 명시 선언이 없으면 write도 없다.
- **Option D (argv로 source/target 지정)**: config에 없는 임의 경로 쌍을 CLI가 write할 수 있게 되어
  §5의 origin·anchor·containment 보증이 전부 무력해진다.
- **`--force`가 Git/symlink/path 검사까지 우회**: 제안 원안이지만 tracked plaintext와 symlink write를
  flag 하나로 여는 것은 이 카드의 존재 이유와 정면 충돌한다. PLAN-002 §1-4 판정을 유지한다.
- **`--all` 또는 암묵 다중 unseal**: 모호한 선택을 성공으로 만든다.
- **`dva env` top-level 예약**: D9는 순서만 확정했다. [TASK-252](252-decide-top-level-env-promotion.md)
  소유.
- **`dva config env show` / lifecycle auto-unseal**: 복호값을 stdout에 노출하거나 암묵 write를 만든다.
- **sops `--output <file>`로 sops가 직접 target을 쓰게 하기**: O_EXCL/0600/atomic replace/검증을 전부
  포기하게 된다. temp fd를 child stdout으로 주는 방식이 같은 결과를 안전하게 낸다.
- **child exit code 전파**: sops가 128을 내므로 DVA exit 계약이 외부 구현에 묶인다.
- **`.error` 대신 새 최상위 error envelope**: criterion 7이 금지하며, 기존 소비자를 깨뜨린다.
- **message 문자열에 `code:` prefix**: 기계 소비자가 사람이 읽는 문장을 파싱하게 만든다.
- **module/override에 자기 디렉터리 anchor 도입**: 기존 선언의 load 위치를 조용히 바꾼다.
- **행 17에서 부모 디렉터리 자동 생성**: 검증한 트리 밖에 새 경로를 만든다.

### 12. Migration 영향

- 기존 `env_file` 선언은 **한 글자도 바꿀 필요가 없다**. load/merge/show/validate 모두 불변이다.
- bridge를 쓰려면 해당 entry를 string에서 object로 바꾸고 `sops_source`를 추가한다 (opt-in).
- interaction/subcommand에 `sops_source`를 넣은 config는 **없다** (오늘 schema가 그 key 자체를 모른다).
  따라서 §2-4의 거부는 기존 config를 깨뜨리지 않는다. 그 위치의 `env_file` 자체는 TASK-265 결정에 따라
  0.1.49에 사라지며, 이 카드는 그 일정을 앞당기지도 늦추지도 않는다.
- `sops_source`를 쓰는 entry에서만 `path`/`sops_source` 중복 검증이 새로 생긴다.
- Windows 사용자는 이 명령을 쓸 수 없고 명시적 오류를 받는다. direnv `use sops`나 기존 Makefile 경로가
  그대로 남으므로 기능 회귀는 아니다.

### 13. 남은 불확실성 (승인 완료)

1. **darwin 지원 여부** — `macos-latest`에서 pinned sops/age를 재현 가능하게 설치할 수 있는지는
   TASK-246 착수 시 확인된다. 실패하면 darwin도 fail-closed로 내린다 (§8-6).
2. **`edit`의 `--json` 거부** — 대안은 편집 후 envelope을 붙이는 것이지만 editor와 stdout document가
   섞인다. 거부를 권장한다.
3. **Git repo 밖 허용** — fail-closed로 바꿀 수도 있으나 repo 없는 devbox를 막는다. 허용 + notice를
   권장한다.
4. **`.error.code` 가산** — 기존 envelope의 확장이므로 안전하다고 판단하지만, 공개 계약 변경이므로
   명시 승인을 요청한다.

**확정 (2026-09-03, 사용자 승인).** 네 항목 모두 위 권장안대로 확정됐다.

| 항목 | 확정 내용 |
| --- | --- |
| 1 darwin | `macos-latest`에서 pinned sops/age 재현 설치가 되지 않으면 범위를 넓히지 말고 darwin을 fail-closed로 내리고 그 사실을 이 카드에 기록한다 (§8-6) |
| 2 `edit --json` | 거부한다 — editor를 실행하지 않고 `E` code=`json_unsupported_for_edit`, exit 1 (§3-1). `--json`은 root persistent flag(`internal/cli/root.go:66`)라 모든 하위 명령이 상속하므로 `edit`에서 등록을 빼는 선택지가 없고, 넘겼을 때의 동작을 계약이 정의해야 한다 |
| 3 Git repo 밖 | 허용하고 stderr에 notice 한 줄을 낸다 (§7-2). fail-closed는 repo 없는 devbox를 막으므로 채택하지 않는다 |
| 4 `.error.code` | 기존 envelope에 가산한다. 두 번째 root error envelope은 만들지 않는다 (§7-1, §7-3) |

### 14. 승인 게이트 (충족됨)

**충족 2026-09-03.** 사용자가 Option A를 고르고 §13의 4개 항목을 확정했으므로, 같은 세션이
`decision-status: decided`와 Completion Criteria 체크를 기록하고 `make doc-check`를 실행한 뒤
통합했다. TASK-246은 이제 착수 가능하다.

PLAN-002 §6의 조건부 platform rule은 §8-6이 판정했다 — [TASK-246](246-implement-secure-config-env-bridge.md)이
이미 `ci.yml`의 `config-env-platform` job 추가를 자기 acceptance criterion으로 소유하므로, 이 카드를
닫는 변경에서 별도 CI enablement child를 만들지 않는다.

## Completion Criteria

- [x] Choose one source↔target representation and show its behavior in every existing top-level `env_file` shape, including load, merge, show, and validation round-trip compatibility; interaction/subcommand `env_file` must reject encrypted-source metadata unless a separate runtime use case is approved | verify: human — the decision must include accepted and rejected YAML examples for both schema locations
- [x] Freeze the exact command grammar and the zero/one/many encrypted-entry selection rule; ambiguous selection and implicit multi-target writes must fail closed | verify: human — the decision must include an argv table with text and JSON outcomes
- [x] Define `edit` ownership and the full unseal state matrix across source/target existence, required/optional, and force | verify: human — the matrix must cover every Cartesian branch
- [x] Define the allowed effective top-level declaration origins and preserved provenance, then define the resolution anchor for root/module/override/subproject declarations plus path containment, absolute paths, Git-outside behavior, tracked/not-ignored targets, symlink/non-regular files, source=target, and permission failures | verify: human — every origin, ambiguous merge, location, and unsafe state must name its exact resolution or fail-closed rejection and whether it is non-overridable
- [x] Limit `--force` to existing regular-target overwrite unless a separately justified security decision says otherwise; it must not silently bypass tracked, ignore, symlink, type, or path guards | verify: human — rejected alternatives and migration advice must be recorded
- [x] Run a Linux/macOS/Windows replacement and concurrency spike; specify handle-relative or equivalent TOCTOU defense, file and parent-directory sync, atomicity, durability, cancellation cleanup, SIGKILL/power-loss limits, owned stale-temp recovery, and fail-closed behavior on an unverified platform | verify: human — evidence must include commands, OS/version, results, unresolved guarantees, and the exact supported-OS CI matrix that will keep those guarantees live
- [x] Freeze success/error text, JSON envelope, exit codes, secret redaction, and stable machine-code policy without inventing a second root error envelope | verify: human — fixture-ready expected documents must contain no decrypted value or raw child output
- [x] Record the selected option and why alternatives were rejected in this card before changing its status | verify: `make doc-check`

## Non-negotiable baseline

Sops is invoked without a shell, dotenv input/output is explicit, secret material never reaches DVA
output, and an ambiguous selector fails before any write. DVA does not adopt age key/provider ownership.

## Conditional platform rule

지원한다고 결정한 OS는 TASK-246의 safe-writer와 command integration CI에서 계속 검증해야 한다. 현재
CI가 그 matrix를 제공하지 못하고 이를 TASK-246 범위에서 안전하게 추가할 수 없다면, 이 결정 카드가
별도의 bounded CI enablement child를 만들고 PLAN-002의 children·count·graph와 TASK-246 dependency를
같은 변경에서 갱신한다. 그 child가 통합되기 전에는 해당 OS 지원을 선언하지 않으며, 검증되지 않은
platform에서는 mutation을 fail closed한다. 이 조건은 spike를 지속 보증으로 오인하지 않기 위한 것이다.

## Troubleshooting Log

- 2026-09-02 — 증상: `os.Root` 컨테인먼트만으로 symlink 공격이 막힌다고 가정한 초안이 spike에서 뚫림 / 원인: `os.Root`는 root **밖으로 나가는** 경로만 거부하고 root 안에 머무는 symlink는 그대로 따라가므로 `.env → 같은 디렉터리의 tracked 파일` 링크가 통과함 / 해결: §5-3에 경로 component 단위 `Root.Lstat` symlink 게이트를 필수 단계로 추가하고 §8-2에 설계를 바꾼 근거로 기록 / 걸린시간: 약 40분
