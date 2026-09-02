---
id: TASK-265
title: "Decide the interaction-level env_file compatibility contract"
type: chore
priority: P0
effort: M
exec-tier: strong
created-at: 2026-09-02T15:50:45+09:00
source: "TASK-247 full env-input audit"
scope: "interaction and subcommand env_file schema, inert runtime behavior, precedence and owner options, compatibility evidence, diagnostics, migration and implementation handoff"
status: todo
needs-human: true
decision-status: decided
depends-on: [TASK-247]
---

# Task 265: decide the interaction env_file contract

## Summary

Decide whether the schema-valid but currently inert `interaction.*.env_file` field becomes a supported
runtime input or is deprecated and rejected through a versioned migration. Do not let TASK-248 silently
choose either behavior while implementing top-level env-file failure policy.

## Problem

`InteractionCommand.EnvFile` is parsed, stored and merged at interaction/subcommand depth, and a tracked
example declares it, but no CLI or runner consumes it. A user can therefore write a valid configuration
that appears to require a file while the command runs without it. Starting to honor the field adds file I/O,
precedence, inheritance and child-owner semantics; removing it immediately is also a compatibility break.

## Recommended direction

Prefer versioned deprecation and rejection unless pinned usage evidence proves a distinct per-interaction
file is necessary beyond top-level `env_file` plus interaction `environment`. Silent inert acceptance is not
viable. If support is selected, freeze owner-relative path anchoring, parent/subcommand inheritance,
top-level-versus-command precedence, required/optional failure behavior and TASK-247 output parity first.

## Decision Record

Versioned deprecation and rejection을 선택한다. Interaction·subcommand `env_file`은 지원 field가 되지
않는다. 한 release에서 명시적으로 deprecate하고 다음 release에서 schema가 거부한다. Pinned corpus는
per-interaction file을 필요로 하는 사용을 하나도 제시하지 못했고, 지원을 선택하면 owner anchoring,
parent/subcommand inheritance, top-level 대비 precedence, required 실패라는 네 개의 새 public contract와
새 runtime file I/O를 수요 evidence 없이 추가하게 된다. Silent inert 유지는 카드 제약상 선택할 수 없다.

이 기록은 `needs-human: true` 카드의 결정문이다. 아래 §1·§2의 모든 주장은 tracked symbol 또는 재현
가능한 read-only scan으로 뒷받침한다. 정책상 독립 review는 이 결정을 작성하지 않은 주체가 수행하며,
카드를 done으로 옮기기 전에 완료한다.

### 1. 확인된 현재 동작

Producer와 merge는 존재하고 실행 consumer는 존재하지 않는다.

| 역할 | 위치 | 사실 |
| --- | --- | --- |
| Schema | `internal/config/schema.json` `definitions.interaction_command.properties.env_file` (L388-390) | 루트와 동일한 `#/definitions/env_file`을 참조한다. 같은 definition의 `additionalProperties`는 `false`다 |
| Decode | `internal/config/config.go:230`, `:367`, `:393` | `InteractionCommand.EnvFile any`를 `UnmarshalYAML`의 `plain` alias로 읽고 그대로 대입한다 |
| Subcommand decode | `internal/config/config.go:236` | `Subcommands map[string]*InteractionCommand` — 모든 깊이가 같은 field를 갖는다 |
| Module/override merge | `internal/config/merge.go:400-402` | `other.EnvFile != nil`이면 통째로 replace한다. subcommand는 `merge.go:412-425`에서 deep merge된다 |
| Subproject import | `internal/config/subproject.go:245`, `:270-277` | `clone := *command`가 field를 그대로 복사하므로 canonical·alias 등록과 중첩 subcommand까지 값이 실린다 |
| Tracked example | `examples/env-file-priority.yml:49`, `:56-58` | `interaction.rails.env_file`(scalar)과 `interaction.rspec.env_file`(list) 두 node |
| Tracked 문서 | `examples/README.md:113` | "Command-specific env_file" 한 줄. USAGE.md와 docs/30·31·41·42는 루트 section만 설명한다 |

실행 consumer는 다음과 같이 부재한다.

- Inheritance 자체가 없다. `mergeInteraction`(`internal/runner/interaction_tree.go:279-393`)은 병합할
  field를 하나씩 나열하며 `EnvFile`을 parent에서도 child에서도 옮기지 않는다. 즉 지금은 consumer를
  추가해도 parent 선언이 subcommand에 도달하지 않는다.
- Runtime node에 자리가 없다. `ResolvedCommand`(`internal/runner/interaction_tree.go:13-37`)는
  `Environment`만 갖고, `buildResolved`(`:216-244`)도 `entry.Environment`만 매핑한다.
- `internal/config` 밖에서 `.EnvFile`을 읽는 non-test 코드는 `internal/cli/root.go:426-428`의
  `loadEnv`와 `internal/cli/validate.go:390` 두 곳뿐이며, 둘 다 top-level `Config.EnvFile`이다.
  나머지 두 match는 `config.EnvFileKey`(`DVA_FILE`)를 쓰는 test다.
- `AllEnvFileConfigs`/`AllEnvFiles`는 `*Config` 메서드다(`internal/config/lifecycle_helpers.go:188-200`).
  `dva doctor`의 `checkEnvFiles`(`internal/cli/doctor.go:534-556`)도 이것만 읽으므로 interaction 선언은
  진단에도 나타나지 않는다.
- Generator corpus(`generatorCorpus()`, `internal/config/removed_keys_test.go:21-29`: agent-mesh-flows의
  library·guardrails, `skills/`, `internal/cli/library_reference.txt`)는 `interaction:` 아래에
  `env_file:`을 한 번도 쓰지 않는다. corpus의 모든 `env_file` 등장은 루트 section이거나 canonical
  section order 나열이다.

따라서 현재 실행 효과는 없다. 반례가 되는 code는 없다. 값이 비어서 무시되는 것이 아니라 값을 읽는
지점이 존재하지 않는다.

### 2. Pinned usage corpus

Secret-free다. 아래 scan은 YAML key 존재만 기록하며 value, file 내용, `.env` 파일을 읽지 않는다.

| Source | Pin | 결과 |
| --- | --- | --- |
| DVA 저장소 | `github.com/ScriptonBasestar/dva` @ `41995a870cbb`, 2026-09-02T19:02:11+09:00, tracked file 749개 | interaction 깊이 선언 2개, 모두 `examples/env-file-priority.yml`. runtime 설정이 아니라 teaching example이다. 저장소 자신의 `dva.yml`은 선언하지 않는다 |
| Workstation portfolio | `~/mydevbox` 하위 `dva.yml` 56개, 서로 다른 git repository 52개, untracked 0개 | top-level `env_file` 선언 28개. **interaction·subcommand 깊이 선언 0개** |
| Module corpus | 같은 트리의 `*/.sb/dva/*.yml` | 파일 0개. 이 corpus에는 module 분리 설정 자체가 없다 |

재현 방법: `/usr/bin/find ~/mydevbox -maxdepth 4 -name dva.yml`로 후보를 모으고, 각 파일을
`yaml.safe_load`한 뒤 `interaction`의 각 node와 중첩 `subcommands`를 재귀적으로 돌며 `env_file` key
존재만 센다. 같은 방식을 `*/.sb/dva/*.yml`에 반복한다.

Dynamic limitation은 결정을 바꾸지 않지만 명시적으로 남긴다.

- 이 corpus는 한 workstation의 checkout snapshot이다. revision은 audit 시점의 각 checkout `HEAD`에서
  읽었고 repository별로 이 카드에 개별 pin하지 않았다. 이 기기에 없는 repository, 다른 branch,
  push되지 않은 fork는 corpus 밖이다.
- Scan은 literal YAML key만 본다. `safe_load`가 anchor와 merge key는 이미 전개하므로 그 경로는
  포함되지만, `modules:`나 corpus 밖 `subprojects:` 조합으로만 나타나는 선언은 보이지 않는다. 다만
  이 corpus의 module 파일 수는 0이다.
- Runtime loading에 의존하는 선언은 정의상 0개다. 오늘 어떤 선언도 실행 효과가 없으므로 corpus는
  "선언 의도"를 측정할 뿐 "동작 의존"을 측정하지 않는다. 이는 support를 선택할 근거가 없다는 뜻이지,
  rejection이 무해하다는 증명은 아니다. 그래서 즉시 제거가 아니라 versioned rejection을 고른다.

### 3. Option 비교

| 축 | Support | Versioned deprecation/rejection (선택) | Inert 유지 |
| --- | --- | --- | --- |
| Owner/path anchoring | 선언한 owner의 config directory 기준을 새로 정의해야 한다. imported interaction은 TASK-264 child owner를 써야 하고 parent 기준 해석은 금지된다 | 새 anchoring 없음 | 정의되지 않은 채 schema만 green |
| Subcommand | `mergeInteraction`에 inheritance를 새로 추가해야 하고, replace/append/누적 중 하나를 골라야 한다 | inheritance를 추가하지 않는다 | parent 선언이 child에 도달하지 않는 것이 문서화되지 않은 사실로 남는다 |
| Precedence | TASK-247 §2 표에 layer가 하나 늘고, root `env_file`·interaction `environment`와의 상대 순서를 새로 고정해야 한다 | TASK-247 표가 그대로 유지된다 | 표와 schema가 계속 어긋난다 |
| Required 실패 | TASK-248의 atomic report를 command 깊이까지 확장해야 하고, `run` 실패가 새로 생긴다 | 새 실패 경로 없음 | 사용자가 `required: true`를 적어도 아무 것도 강제되지 않는다 |
| Text/JSON | doctor row, partial `status`, error envelope에 interaction owner 축을 추가해야 한다 | TASK-247 fixture가 그대로 유효하다 | doctor가 선언된 파일 누락을 영원히 보고하지 않는다 |
| Security | interaction마다 새 file read 경로가 생기고, imported child가 parent 파일을 읽을 위험을 새로 막아야 한다 | 새 read 경로 없음 | 없음 |
| Migration | 없음. 다만 지금까지 무시되던 파일이 갑자기 값을 주입하므로 조용한 동작 변경이 된다 | 두 release에 걸친 announce → reject | 없음 |
| Rollback | consumer·precedence·fixture를 되돌려야 한다 | schema property 1개, struct field 1개, merge branch 1개, guidance entry 1개 되돌리기 | 해당 없음 |

Support의 결정적 문제는 비용이 아니라 조용한 동작 변경이다. 오늘 무시되는 파일을 읽기 시작하면
같은 설정이 다음 release에서 다른 변수 집합으로 실행된다. 그 위험을 감수할 수요 evidence가 §2에
전혀 없다. Inert 유지는 카드 제약(“silent inert behavior may not be selected as the permanent
contract”)에 의해 배제된다.

### 4. Rejection contract (freeze)

Version gate는 release 경계뿐이다. `version:`은 최소 DVA 버전 선언이지 schema selector가 아니므로
schema를 선언 버전으로 분기할 수 없다. 그래서 announce와 reject는 서로 다른 release여야 한다.
아래 `0.1.48`·`0.1.49`는 최신 release `0.1.47` 다음의 연속된 두 minor를 가리킨다. 실제 tag 번호가
달라지면 TASK-266이 warning 문구와 CHANGELOG를 같은 변경에서 함께 맞춘다 — 사용자에게 보이는
release 이름과 실제 tag는 어긋나면 안 된다.

#### Stage A — 0.1.48 (announce)

- Schema는 그대로 두고 field도 그대로 inert하다. 어떤 file I/O도 추가하지 않는다.
- `Config.ValidateWarnings`에 선언 node마다 semantic warning 하나를 추가한다. Walker는 기존
  `eachInteractionNode`(`internal/config/validate_warnings.go:367-388`)를 재사용해 path를
  `interaction.<name>` / `interaction.<name>.subcommands.<sub>` 형태로 만든다. 고정 문구:

  ```text
  <path>: 'env_file' is inert and will be rejected in 0.1.49 — declare shared inputs in the top-level 'env_file:', or inline command-local values under this command's 'environment:'
  ```

- 출력 경로는 기존 것만 쓴다. `dva config validate`의 `[warn] semantic:` 채널과 JSON report의
  `semantic` category(`internal/cli/validate.go:170-174`)다. 새 category도 새 flag도 만들지 않는다.
- Exit code는 기존 규칙 그대로다. default `dva config validate`는 0을 유지하고, `--strict`는 이미
  semantic warning이 있으면 1이므로(`internal/cli/validate.go:190-192`) strict 사용자만 한 release
  먼저 실패를 본다. 이것이 announce의 전부이며 Stage A에서 다른 exit code 변화는 없다.
- 다른 route는 침묵한다. `dva run`, lifecycle 동사, `doctor`, `show`는 경고하지 않는다. 실행 효과가
  없는 field를 hot path마다 알리는 것은 validate route 대비 얻는 것이 없다.
- `dva config migrate`는 rewrite하지 않고 report한다. `Migrate`(`internal/config/migrate_report.go:35-62`)
  에 `MigrateStackOrder` 다음 단계를 추가하고 결과를 `Changes`가 아니라 `Blocked`에 넣는다. 목적지가
  공유 입력(top-level `env_file`)인지 command-local 값(`environment:`)인지는 저자의 의도이고 DVA가
  추측하면 안 되기 때문이다. 고정 문구:

  ```text
  <path>.env_file: remove it — declare shared inputs in the top-level 'env_file:', or inline command-local values under 'environment:'
  ```

- Tracked example을 같은 release에서 정리한다. `examples/env-file-priority.yml:49`와 `:56-58`의 두
  선언을 삭제하고 `examples/README.md:113`의 "Command-specific env_file" 항목을 제거한다. deprecate한
  field를 계속 가르치는 example은 announce를 무효로 만든다.
- USAGE.md의 interaction 레퍼런스에 `env_file`이 interaction field가 아니라는 한 줄을 넣고,
  CHANGELOG에 `### Deprecated` 항목으로 field, 거부 release, 두 대체 수단을 적는다.

#### Stage B — 0.1.49 (reject)

- `definitions.interaction_command.properties.env_file`을 삭제한다. 같은 definition의
  `additionalProperties`가 이미 `false`이므로 schema error는 `interaction.<name>` field에서
  `Additional property env_file is not allowed`가 된다.
- `InteractionCommand.EnvFile`, `UnmarshalYAML`의 `plain` twin과 대입, `merge.go`의 `EnvFile` branch를
  삭제한다. `mergeInteraction`은 애초에 이 field를 모르므로 변경 없다.
- Guidance는 **path-scoped여야 한다**. `removedSchemaKeys`(`internal/config/validate.go:31-46`)는 property
  이름만으로 keying하므로 여기에 `env_file`을 넣으면 (a) 여전히 유효한 루트 `env_file` 오류에 제거
  안내가 붙고, (b) `TestRemovedKeysAbsentFromGeneratorCorpus`(`internal/config/removed_keys_test.go:35`)가
  루트 section을 올바르게 가르치는 corpus 줄마다 실패한다. 따라서 `removedRootKeys` 옆에 "이 parent
  아래에서 제거됨"을 표현하는 세 번째 map을 추가하고, `desc.Field()`가 interaction node를 가리킬 때만
  참조한다. 고정 안내:

  ```text
  removed from interaction: declare shared inputs in the top-level 'env_file:', or inline command-local values under this command's 'environment:'
  ```

- Stage A의 semantic warning은 제거한다. `Config.Validate`가 `validateYAMLSchema`를 먼저 실행하므로
  거부된 설정은 `ValidateWarnings`에 도달하지 않고, 도달할 수 없는 경고는 존재하지 않는 상태를
  설명하게 된다.
- `dva config migrate`의 blocked 문구는 **유지한다**. `Migrate`는 schema 검증 없이 YAML node를 읽으므로,
  거부당한 사용자가 실행할 수 있는 유일한 안내 경로다.
- Precedence, owner, required 실패는 두 stage 모두 변하지 않는다. TASK-247 §2의 precedence 표에 layer가
  추가되지 않고, `internal/lifecycle/resolver.go:394-400`의 `vars: env_file` trace 줄은 계속 루트 선언만
  설명한다.

#### Rollback

두 stage 모두 revert-only다. DVA는 사용자의 파일을 다시 쓰지 않는다(`migrate`는 report만 한다). Stage A는
경고와 문서만 되돌리면 되고, Stage B는 schema property 1개, struct field 1개와 그 decode/merge 참조,
guidance map entry 1개를 되돌리면 이전의 accept-and-inert 상태로 정확히 복귀한다. 되돌릴 사용자 데이터가
없다.

### 5. Implementation handoff

- [TASK-248](248-enforce-required-env-command-policy.md)은 이 결정의 어떤 부분도 구현하지 않는다.
  TASK-247 §4가 고정한 대로 field를 inert하게 유지하며, 읽지도 경고하지도 제거하지도 않는다. 이
  결정이 기록된 시점에 TASK-248의 이 카드에 대한 의존은 해소된다.
- [TASK-266](266-deprecate-and-reject-interaction-env-file.md)이 Stage A와 Stage B를 모두 소유한다.
  두 stage 사이에 0.1.48 release 경계가 있으므로 한 카드 안에서 순서화하되 Stage B는 release 이후에만
  착수한다.
- 새 runtime file I/O는 어느 stage에서도 추가되지 않는다. Support가 선택되지 않았으므로 unspecified
  contract에서 file을 읽는 코드가 생길 여지 자체가 없다.

### 6. 기각한 대안

- Support: §2 corpus에 수요가 0이고, 오늘 무시되는 파일을 읽기 시작하는 것은 같은 설정의 조용한
  동작 변경이라 기각한다.
- 즉시 schema 제거: announce 없는 거부는 카드 제약이 금지하며, tracked example이 아직 그 field를
  가르치고 있는 상태에서의 거부는 사용자에게 원인을 설명하지 못한다.
- Inert 유지: schema가 green인데 `required: true`가 아무 것도 강제하지 않는 상태를 영구 계약으로
  삼는 것이며, 카드 제약이 명시적으로 금지한다.
- Stage B를 `removedSchemaKeys`로 처리: 이름 keying이 유효한 루트 `env_file`을 오염시키고 corpus
  test를 깨뜨린다. path-scoped map이 필요하다.
- 두 stage를 TASK-248에 합치기: Stage B가 release 경계 뒤에 있어 TASK-248이 완료될 수 없게 만든다.

## Completion Criteria

- [x] Inventory schema, decode, module/override merge, interaction-tree inheritance, tracked examples, documentation and every runtime consumer; record that current execution effect is absent or cite contrary code | verify: human — every claimed producer and consumer must cite a tracked symbol or fixture
- [x] Build a pinned, secret-free usage corpus from tracked DVA files and canonical consumer repositories, recording repository IDs, revisions, paths, dynamic limitations and whether declarations rely on runtime loading | verify: human — unpinned or unavailable evidence remains an explicit finding
- [x] Compare support, versioned deprecation/rejection and inert compatibility for owner/path anchoring, subcommands, precedence, required failures, text/JSON, security, migration and rollback | verify: human — silent inert behavior may not be selected as the permanent contract
- [x] If support is selected, freeze exact precedence and inheritance plus root/direct-child/imported canonical/alias fixtures and assign implementation to TASK-248 or a bounded child before TASK-248 starts | verify: human — no runtime file I/O may be added from an unspecified contract
- [x] If rejection is selected, freeze warning/error releases, config validate behavior, migration command or message, schema timing, rollback and the disposition of tracked examples | verify: human — immediate unannounced schema rejection is not allowed
- [x] Record an independently reviewed `## Decision Record`, change `decision-status` to `decided`, and update TASK-248 dependencies if a new implementation child is required | verify: `make doc-check`

## Non-goals

- No field support, deprecation warning or schema removal in this decision card.
- No top-level `env_file` or encrypted-source contract change.
- No imported command ownership implementation; TASK-264 owns it.

## Troubleshooting Log

- 2026-09-02 (~15m) 증상: rejection stage의 removed-key 안내를 기존 `removedSchemaKeys`에 넣으면 루트
  `env_file`까지 오염되고 `TestRemovedKeysAbsentFromGeneratorCorpus`가 corpus 전체에서 실패한다.
  원인: 그 map은 property 이름만으로 keying되고 `env_file`은 루트에서 여전히 유효하다. 해결: §4 Stage B에
  path-scoped map을 요구 사항으로 고정했다. 부수 확인: `docs/53`은 10115/10240 byte로 doc-check size
  한계에 붙어 있어 wave 표에 TASK-266 행을 추가하지 않았다.
