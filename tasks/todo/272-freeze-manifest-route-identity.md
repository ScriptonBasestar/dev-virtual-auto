---
id: TASK-272
title: "Freeze the manifest command route-identity representation"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-03T09:00:00+09:00
source: "TASK-254 evidence — the manifest schema cannot express a canonical/compatibility route pair"
scope: "manifest consumers, static_commands subcommand coverage, route-identity representation, schema versioning, and the implementation split across TASK-256 and TASK-258"
status: todo
needs-human: true
decision-status: decided
depends-on: [TASK-254]
---

# Task 272: freeze manifest route identity

## Summary

TASK-254 measured that `dva manifest` cannot describe a command reachable under two names. `static_commands`
is a flat map keyed by one name whose entry carries only `description`, `type`, `options` and `subcommands`
(`internal/cli/manifest.go:105-110`), and only `skill` populates `subcommands`, so the five `config` children
— including the `config validate` route TASK-257 is choosing between — are absent from the document
altogether. Decide how the manifest represents route identity, so TASK-256 and TASK-258 implement an approved
representation instead of inventing one.

## Recommended direction

Separate the coverage defect from the identity question and prefer the smaller answer for each. Publishing
`config`, `ssh` and `console` children through the existing `subcommands` field needs no new schema and fixes
a document that today omits the routes DVA's own guidance teaches. Route identity is the part that needs a
field, and the recommendation is one optional marker naming the canonical invocation on the compatibility
entry — not a parallel route table — so a consumer that ignores it still reads a correct document.

## Completion Criteria

- [x] Record every tracked consumer of the command manifest and what each reads from `static_commands`; state exactly which facts about a two-name route the current schema can and cannot carry | verify: human — the account must cite tracked paths and the measured manifest, and must distinguish a missing field from a missing entry
- [x] Compare subcommand-coverage-only, canonical/compatibility fields on the static command entry, an invocation-keyed route list, and no change; state schema-version, legacy-consumer, completion and help consequences for each | verify: human — no option may be selected only because it is the smallest diff
- [x] Freeze the representation, the `schema_version` policy for it, the meaning legacy fields keep, and which of TASK-256 and TASK-258 may implement which part | verify: human — an implementation task may not extend the representation beyond what is frozen here
- [x] Append an approved `## Decision Record` to this card and change `decision-status` from `pending` to `decided` before TASK-256 or TASK-258 touches the manifest | verify: `make doc-check`

## Non-goals

- No route, alias, help group, or reserved-name change.
- No decision about which of `ktl`/`kubectl` or `validate`/`config validate` is canonical — that stays with TASK-255 and TASK-257.
- No command registry refactor; TASK-254 recommends keeping current ownership.

## Evidence and Options (prepared 2026-09-03)

This section supplies measurement and option comparison for a human to decide against. It does not
select an option, does not append a Decision Record, and does not change `decision-status`.

### 1. Current schema shape (measured)

`ManifestCmd` (`internal/cli/manifest.go:113-118`) is:

```go
type ManifestCmd struct {
    Description string
    Type        string
    Options     map[string]string
    Subcommands map[string]ManifestCmd
}
```

`static_commands` (`Manifest.StaticCommands`, keyed by one name) has no field that can name a second
invocation for the same command, and no field that marks one entry as a compatibility route for another.
`Subcommands` is populated only when the literal table in `buildManifest` (`internal/cli/manifest.go:325-451`)
declares a `Subcommands` map for that entry ahead of time — `fillCommandOptions`/`fillCommandDescriptions`
(`internal/cli/manifest.go:257-260`, `:311-314`) only fill in a child that is already present as a key;
a real cobra child with no matching literal key is silently skipped, not added. Today only `"skill"`
declares a `Subcommands` map (`internal/cli/manifest.go:419-430`); `"config"` (`:434`), `"ssh"` (`:403`) and
`"console"` (`:416`) do not.

Verified against a real build (`make build`, then `./bin/dva manifest -f json`, `schema_version: "1.4"`):

- `static_commands.config` = `{"description": "...", "type": "config"}` — no `subcommands` key at all.
- `static_commands.ssh` = `{"description": "...", "type": "lifecycle"}` — no `subcommands` key.
- `static_commands.console` = `{"description": "...", "type": "passthrough"}` — no `subcommands` key.
- `static_commands.validate` and `static_commands.init` each exist as independent top-level entries with
  no field linking them to their `config` counterpart.

Real cobra registration (`dva config --help`, run against the build above) shows `config` actually has six
children: `docs`, `env`, `init`, `migrate`, `show`, `validate` — none of which appear under
`static_commands.config.subcommands` because the field is absent, not because it is present-but-empty. This
is the "missing field vs. missing entry" distinction the card asks for: `config`, `ssh`, and `console` are
each a **missing entry** case (the child commands are not represented at all, regardless of route identity);
the **missing field** case is narrower — only `init` and `validate`, which already have a live second
invocation and no field to say so.

### 2. Two of the six `config` children are live two-name routes today (not hypothetical)

- **`init`**: `internal/cli/init.go:86` registers `initCmd` under `configCmd` (`dva config init`).
  `internal/cli/init.go:88-101` then builds a second `*cobra.Command` (`initAliasCmd`) with the same `Use`,
  `Short`, `Long`, `Example`, `RunE`, and flag set, and registers it directly on `rootCmd` (`dva init`) —
  comment at `:87`: "Keep a top-level alias for backward compatibility: dva init → dva config init".
- **`validate`**: `internal/cli/validate.go:237` registers `validateCmd` under `configCmd`
  (`dva config validate`). A second file, `internal/cli/validate_alias.go:6-14`, builds `rootValidateCmd`
  copying `Use`/`Short`/`Long`/`RunE` and registers it on `rootCmd` (`dva validate`).
  `internal/cli/root_command_registration_test.go:33-49`
  (`TestRootValidateMatchesConfigValidate`) and `:175` (`TestRootValidateMatchesConfigValidateBehavior`)
  assert the two stay behaviorally identical — but they compare the two live `*cobra.Command` objects
  directly; they say nothing about the manifest, which has no way to express that comparison's subject
  (two names, one command) at all.

So the manifest problem TASK-254/TASK-255 raised for `ktl`/`kubectl` is not a future case being designed for
in the abstract — `init`/`config init` and `validate`/`config validate` are two working instances of the
same shape already shipping today, and the manifest currently represents each pair as two unrelated,
disconnected entries (`static_commands.init` and `static_commands.validate`, with `config`'s own entry not
even listing them as children).

`internal/config/reserved.go:13-21` (`reservedCommands`) is flat and single-keyed the same way: `init` and
`validate` are each one entry, with no concept of a route pair either. Any option below that adds identity
to the manifest does not, by itself, change this registry — see `Non-goals`.

### 3. Tracked consumers and what each reads from `static_commands`

**Contract-encoding Go tests** (the closest thing this repo has to a consumer spec):

- `internal/cli/manifest_static_commands_test.go`
  - `TestStaticCommandsCoverEveryRootCommand` (`:68-82`) — asserts every **top-level** `rootCmd.Commands()`
    name has a `static_commands` entry and vice versa. It walks `rootCmd.Commands()`, not each command's
    children, so it does not check `config`/`ssh`/`console` subcommand coverage at all — this is exactly
    the gap that lets `config`'s six children stay undocumented today without failing any test.
  - `TestStaticCommandsAgreeWithReservedCommands` (`:90-108`) — cross-checks against
    `config.ReservedCommands()`; also top-level names only.
  - `TestEveryStaticCommandCarriesAType` (`:112+`), `TestStaticCommandOptionsCoverEveryRegisteredFlag`
    (`:153+`), `TestHandParsedOptionsAreDocumented` (`:203+`), `TestStaticCommandDescriptionsMatchTheirShort`
    (`:272+`) — all consume `Type`/`Options`/`Description`, none consume or assert anything about
    `Subcommands` contents for `config`/`ssh`/`console`.
- `internal/cli/root_command_registration_test.go` — `TestRootCommandRegistersValidate` (`:23-31`),
  `TestRootValidateMatchesConfigValidate` (`:33-49`), `TestRootValidateMatchesConfigValidateBehavior`
  (`:175+`) assert route parity directly on cobra commands, bypassing the manifest entirely — they are
  evidence the parity fact exists and is tested, but not evidence the manifest can state it.
- `internal/cli/shadowed_builtin_test.go`, `internal/cli/manifest_global_flags_test.go` — cover
  `shadowed_by_builtin`/`unroutable`/global flags; unrelated to route identity between two `static_commands`
  entries.

**Documentation consumers:**

- `skills/dva/references/commands.md:277` — lists the manifest's top-level field names (including
  `static_commands`) for an AI agent reading the skill; it names the field but says nothing about route
  identity, so it inherits whatever the schema does or does not carry.
- `USAGE.md` (`manifest` section starting `:262`) — documents `manifest -f json/yaml` as the automation
  surface companion to `ls` (`:269-270`) and separately documents `shadowed_by_builtin` (`:1194-1219`) as an
  example of a field a consumer must know to look up in `static_commands` — precedent that consumers are
  expected to read specific optional fields off a `static_commands` entry, not just its presence.

**Programmatic/flow consumers:**

- `agent-mesh-flows/dva-improve-guided/00-analyze.yaml:110` — the `dva_manifest` step runs
  `dva manifest -f json` and feeds the raw output into an LLM prompt as project context. This is a **soft**
  consumer: no hardcoded field-path parsing, so it degrades gracefully under any of the options below, but
  an LLM asked to reason about "is `dva init` the same as `dva config init`?" from this context today has
  no signal to use except two coincidentally-identical `description` strings.
- The `<!-- AUTOGEN:reserved_commands:start -->` blocks in `agent-mesh-flows/dva-improve-guided/00-analyze.yaml:307`
  and `agent-mesh-flows/dva-improve-guided/30-configure.yaml:230` are **not** manifest consumers — they are
  generated from `config.ReservedCommands()` (flat, single-keyed, see above), not from `static_commands`.
  Noted here only to rule it out as a hidden fifth consumer.

No tracked consumer today parses `static_commands[*].subcommands` or reads any per-entry identity field,
because no such field exists yet to read.

### 4. Option comparison

**(a) Subcommand-coverage-only — no schema change.**
Declare `Subcommands` maps in the `buildManifest` literal table for `config`, `ssh`, and `console` (mirroring
the existing `skill` entry), so all nine currently-missing children (`config`'s six, `ssh`'s three via
`internal/cli/ssh.go:105` — `up`/`down`/`status`, `console`'s two via `internal/cli/console.go:95` —
`start`/`inject`) appear. This fixes every **missing-entry** case. It does not touch `ManifestCmd` and
`schema_version` would not need to change. It does **not** fix the **missing-field** case: after this
change, `static_commands.config.subcommands.init` and `static_commands.init` (top-level) still exist as two
disconnected entries with matching `description` text and no field linking them — a consumer still cannot
tell `dva init` and `dva config init` are the same command rather than two commands that happen to do the
same thing. Same for `validate`. This option is necessary regardless of what else is chosen (every other
option below still needs it to reach `ssh`/`console`/`config`'s four non-aliased children), but it does not
by itself answer the card's route-identity question.

**(b) Canonical/compatibility fields added to `ManifestCmd`.**
Add one or two optional fields to `ManifestCmd`, e.g. `CanonicalOf string` (set on the compatibility entry,
naming the canonical invocation path — mirrors the existing `ShadowedByBuiltin`/`Unroutable` pattern on
`ManifestDynCmd` at `internal/cli/manifest.go:141-159`, which is exactly this repo's precedent for "optional
marker on the non-primary entry, absent everywhere else"). Applies to both a top-level compatibility entry
naming its `config`-child canonical form (`static_commands.init.canonical_of: "config init"`) and, symmetrically,
a `config`-subcommand entry that is itself the compatibility side of some other pair.
- `schema_version` consequence: additive, `omitempty` field — existing minor-version bump norm applies
  (current is `"1.4"`; this repo's existing fields like `ShadowedByBuiltin`/`Unroutable` were each added
  this way without a major-version-style break). A consumer ignoring the new field reads exactly what it
  reads today, satisfying the card's recommendation that "a consumer that ignores it still reads a correct
  document."
- Legacy-consumer consequence: none — old consumers see the same `description`/`type`/`options`/`subcommands`
  shape, plus one field they don't look at.
- Completion consequence: TASK-256 and TASK-258 each set the field on exactly the one pair their approved
  decision concerns (`ktl`/`kubectl`, `validate`/`config validate`); neither needs to touch the other's pair
  or design the field itself, since it is frozen here.
- Help consequence: none directly — this is a machine field; `--help` text is a separate, already-parity-tested
  surface (`TestRootValidateMatchesConfigValidate`). A generated-docs pass could optionally surface it in
  prose, but nothing requires that.

**(c) An invocation-keyed route list (new top-level `Manifest.Routes` or similar, independent of
`static_commands`).**
E.g. `Routes: []ManifestRoute{{Canonical: "config validate", Compatibility: ["validate"]}}`.
- `schema_version` consequence: additive new top-level field, same low-risk shape as (b) — but now there are
  two places a consumer must cross-reference (`static_commands` for the entry, `routes` for its identity)
  instead of one. A consumer that wants "is `X` a compatibility name" must join two collections by name,
  which is exactly the kind of derived-fact-in-two-places drift this repo's own comments elsewhere warn
  about (e.g. the `Description` derivation rationale at `internal/cli/manifest.go:277-289` — two
  hand-maintained copies of one fact drifted for 12 of 13 commands before that field was derived instead).
- Legacy-consumer consequence: none broken, but a legacy consumer reading only `static_commands` (as every
  current consumer does — see §2) gets no benefit without an additional code change on the consumer side to
  look at the new top-level key.
- Completion consequence: larger implementation surface for TASK-256/TASK-258 — each must add a `Routes`
  entry rather than set one field on an existing entry it is already touching.
- Help consequence: same as (b), none directly, but harder for a doc-generation pass to join against
  `static_commands` without extra lookup code.

**(d) No change.**
`static_commands.config` (and `ssh`/`console`) stay without `subcommands`; `init`/`validate` stay as two
disconnected top-level entries. `TASK-256`/`TASK-258` proceed to implement `kubectl` and `config validate`
routes in cobra/help/completion/docs but the manifest continues to describe neither pair as related, and
`config`'s six real children continue to be undocumented in the one surface whose own doc comment
(`internal/cli/manifest.go:...` "This is the machine-readable twin of 'dva ls'") claims to be the complete
machine-readable command surface.
- `schema_version` consequence: none, stays `"1.4"`.
- Legacy-consumer consequence: none change, but the gap TASK-254 measured (config's children absent from the
  document) persists, and the LLM-context consumer (`00-analyze.yaml:110`) keeps receiving a manifest that
  can't state the identity fact even informally.
- Completion consequence: TASK-256/TASK-258 each still have a completion criterion requiring manifest
  consistency ("manifest uses the approved existing representation or waits for the bounded route-identity
  child rather than changing schema inside this task" — `tasks/todo/256-implement-kubectl-route-decision.md`
  and `tasks/todo/258-implement-validate-route-decision.md`, both identical wording); with no representation
  approved, both would have to satisfy that criterion by leaving the manifest exactly as inconsistent as it
  is today, indefinitely.
- Help consequence: none — `--help` output is unaffected either way, since it derives from cobra directly,
  not from the manifest (confirmed: `dva config --help` already lists all six real children correctly today,
  because cobra's own `--help` walks the live command tree rather than the hand-maintained
  `StaticCommands` literal).

No option above is presented as smallest-diff-wins: (a) is necessary but insufficient by itself; (d) is the
smallest diff and is included only because the card requires stating it, not as a recommendation.

### 5. `schema_version` policy and legacy-field meaning, by option

- **(a) alone**: no `schema_version` change needed; `Subcommands` already exists and is `omitempty`, so
  populating more of it is not a schema change, only a data change.
- **(b)**: bump `schema_version`'s existing convention for additive fields (the repo's own recent precedent:
  `ShadowedByLiteralKey`, `Unroutable`, `StartUnreachable` were each added as new `omitempty` fields on
  existing structs without restructuring older fields or forcing a version bump beyond the routine minor
  increments already visible in the `"1.4"` value). No legacy field changes meaning; the new field is purely
  additive and absent-by-default.
- **(c)**: also additive at the top level, but because it duplicates identity information that could
  otherwise live beside the entry it describes, a future consumer wanting both facts together must be
  written against both `static_commands` and `routes` from day one — there is no "legacy" field whose
  meaning shifts, but the join requirement itself is a new contract a consumer must adopt, unlike (b) where
  an existing per-entry reader needs zero new code to keep working and only new code to gain the fact.
- **(d)**: no `schema_version` change; no field exists to define a policy for.

### 6. Which parts TASK-256 and TASK-258 could each implement, once a representation is chosen

Both cards already gate on this: `tasks/todo/256-implement-kubectl-route-decision.md`'s and
`tasks/todo/258-implement-validate-route-decision.md`'s manifest completion criteria are worded identically
("manifest uses the approved existing representation or waits for the bounded route-identity child rather
than changing schema inside this task") — neither may invent a representation; each may only apply one that
this card freezes. Under option (a)+(b) (the recommended-direction shape the card already sketches):
TASK-256 would set the compatibility marker on exactly the `ktl`/`kubectl` pair (whichever direction TASK-255
decides is canonical) and add `config`'s six children to `static_commands.config.subcommands` if that
coverage fix is not already done by this task or a shared prerequisite; TASK-258 would set the same marker
on exactly the `validate`/`config validate` pair. Neither implementation task should need to touch the
`ManifestCmd` struct itself if the struct change is made once, here or in a shared follow-up, before either
starts — the struct change is a single shared edit, not something either route decision should carry
individually.

> **참고 (2026-09-03):** 위 §1–6 실측은 doc-consistency-fixes 세션이 이 카드에 병렬로 준비한 증거이며 판정을 내리지 않은 상태로 남겨져 있었다. 사용자가 이 아래 Decision Record의 세 질문에 답하는 시점에는 이 증거를 갖고 있지 않았지만, 답변 근거로 제시된 kubectl/ktl·validate/config validate 비교와 위 §4 옵션 비교가 방향(옵션 (a)+(b))에서 일치한다 — 서로 다른 증거 경로가 같은 결론에 도달했다. 위 §2가 추가로 밝힌 `init`/`config init` 쌍(같은 문제가 오늘 이미 살아있는 두 번째 사례)은 아래 결정의 §3 표현 정의에 그대로 적용된다: `canonical_of` 마커는 `kubectl`/`ktl`, `validate`/`config validate` 두 쌍에만 적용하기로 좁혔지만, 같은 필드로 `init`/`config init`도 표현 가능하다 — 다만 그 세 번째 쌍에 마커를 실제로 채우는 것은 이 카드의 범위가 아니며 별도 카드가 필요하다 (TASK-256/258 어느 쪽 범위도 아니다).

## Decision Record (2026-09-03)

**Coverage 보강과 canonical 마커 1개를 함께 채택한다. `schema_version`은 1.4→1.5로
올린다. 구현은 TASK-256이 kubectl/ktl에, TASK-258이 config 계열에 각자 자기 route만
적용하는 방식으로 분담한다.**

### 1. 완료기준 1 — 추적된 소비자와 오늘 표현 가능한/불가능한 사실

Manifest(`dva manifest -f json`) 를 참조하는 추적 경로를 실측했다:

- **agent-mesh flow 3건** (`agent-mesh-flows/dva-improve.yaml:450-451`,
  `dva-improve-guided/00-analyze.yaml:110`, `30-configure.yaml`) — 런타임에
  `dva manifest -f json`을 실행해 파싱한다. 그러나 이들이 실제로 읽는 것은
  `dynamic_commands`(프로젝트별 interaction/plan 탐지)이지 `static_commands`가 아니다.
  예약어 24개 목록은 이 파일들에 **하드코딩된 prose**로 존재하며(`reserved.go`를
  직접 읽지 않는다) manifest의 `static_commands`를 통해 유도되지 않는다.
- **skills/dva/SKILL.md:57** — "Read the `dynamic_commands` section ... to identify
  project-specific commands" — 명시적으로 `static_commands`가 아니라
  `dynamic_commands`를 읽으라고 가르친다.
- **skills/dva/references/commands.md:277** — manifest 최상위 필드 이름을 나열하는
  문서 문장 하나. `static_commands`의 내부 구조(엔트리 필드)는 언급하지 않는다.
- **docs/43, docs/53, docs/54, USAGE.md** — 전부 "`dva manifest`로 탐색하라"는
  안내 수준이고, route identity(두 이름이 한 명령)를 다루는 문장은 없다.

**핵심 사실: 오늘 어떤 추적 소비자도 `static_commands`의 route-identity를 실제로
파싱하지 않는다.** TASK-254가 측정한 대로 `ManifestCmd`(`manifest.go:105-110`)는
`description`/`type`/`options`/`subcommands` 4개 필드뿐이고, `subcommands`는 `skill`
외에는 채워지지 않으며, `config validate`는 manifest에 아예 없다. 표현을 바꿔도
기존 소비자를 깨뜨릴 근거 있는 사례가 없다 — coverage 결손과 identity 결손을 나눠
가장 작은 답을 택할 수 있다는 카드의 recommended direction이 실측으로 확인됐다.

### 2. 완료기준 2 — 옵션 비교

- **A. Coverage-only (기각, 단독으로는 불충분)** — `config`/`ssh`/`console`
  자식을 `skill`처럼 `subcommands`에 채운다. 스키마 변경 없음. 노출 결손은
  고치지만 kubectl/ktl 같은 **같은 레벨 두 엔트리 간** 관계는 여전히 표현
  못 한다 — `subcommands`는 부모-자식 포함관계이지 별칭 관계가 아니다.
- **B. Coverage + canonical 마커 1개 (채택)** — A를 포함하고, compatibility
  엔트리에 선택적 필드(`canonical_name` 형태, 예: `ktl` 엔트리가
  `canonical_name: "kubectl"`을 가짐)를 추가한다. 이 필드를 모르는 소비자는
  여전히 두 개의 독립된 유효 명령으로 읽는다 — 그냥 관계 정보 하나를 놓칠 뿐,
  잘못 읽지 않는다(fail-open이 안전한 방향).
- **C. Invocation-keyed route 목록 (기각)** — `static_commands`와 별개로
  `routes: [{name, canonical, aliases}]` 최상위 배열을 신설한다. §1 실측대로
  오늘 어떤 소비자도 이런 관계를 요구하지 않는데, 가장 큰 스키마 표면을 새로
  만드는 것은 필요보다 큰 답을 미리 확정하는 것이다. 소비자가 두 구조(정적
  엔트리 + 라우트 목록)를 대조해야 해서 완결성 위험(둘이 어긋나는 상태)도
  새로 생긴다.
- **D. 현행 유지 (기각)** — TASK-254가 이 카드를 만든 이유 자체가 "스키마가
  표현 못 한다"는 실측이었다. 변경 없음을 택하면 TASK-256/258이 각자 표현을
  발명해야 하고, 그것이 TASK-272를 만든 이유를 해소하지 못한다.

### 3. 완료기준 3 — 표현·schema_version·legacy 필드·분담 동결

**표현**: Option B. `ManifestCmd`에 선택적 문자열 필드(예: `canonical_name`,
compatibility 엔트리에만 채움, canonical 엔트리·관계 없는 엔트리는 생략 또는 빈
문자열)를 추가하고, `config`/`ssh`/`console`은 `subcommands`를 채운다. 정확한
필드명·JSON 태그는 구현 카드(TASK-256/258)가 코드 컨벤션에 맞춰 정하되, 의미는
"이 엔트리를 대신 쓸 수 있는 canonical 이름"으로 고정한다 — 별도 route 테이블이나
양방향 별칭 그래프를 만들지 않는다.

**schema_version**: 1.4 → 1.5로 올린다. 이 저장소의 실측 이력(`685344f` subproject
도입 1.2→1.3, `17a74b9` lifecycle 마이그레이션 1.3→1.4)은 필드가 backward-compatible한
추가였는지와 무관하게 매 구조 변경마다 minor를 올려왔다 — 이 판정은 그 선례를 따른다.

**Legacy 필드 의미**: 기존 4개 필드(`description`/`type`/`options`/`subcommands`)는
의미가 바뀌지 않는다. 새 마커 필드가 없는(zero-value) 엔트리는 "다른 canonical
이름이 없다"는 뜻이지 "canonical이다"라는 긍정 선언이 아니다 — 오늘 마커를 모르는
소비자와 동일하게 읽힌다.

**TASK-256/258 분담**: 각자 자기 route에만 적용한다 — TASK-256이 kubectl/ktl 두
엔트리에 마커를 적용하고, TASK-258이 `config` 자식 `subcommands` coverage와
validate/config validate 마커를 적용한다. 구조체 필드 추가(`ManifestCmd`에 신규
필드 선언, `manifest.go`의 `SchemaVersion` 상수 1.5로 갱신) 자체는 먼저 착수하는
카드가 하고, 두 카드 모두 그 위에 자기 route만 채운다 — 이 카드는 그 분담 원칙만
얼린다.

### 4. 완료기준 4

`decision-status`를 `pending` → `decided`로 변경한다(아래). Non-goals(route/alias/
help group/reserved-name 변경 없음, ktl-vs-kubectl·validate-vs-config-validate 자체
선택 없음, 명령 레지스트리 리팩터 없음)는 그대로 유지된다 — 이 판정은 표현 방식만
정했다.
