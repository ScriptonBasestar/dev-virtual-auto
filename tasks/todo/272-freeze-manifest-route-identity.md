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
decision-status: pending
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

- [ ] Record every tracked consumer of the command manifest and what each reads from `static_commands`; state exactly which facts about a two-name route the current schema can and cannot carry | verify: human — the account must cite tracked paths and the measured manifest, and must distinguish a missing field from a missing entry
- [ ] Compare subcommand-coverage-only, canonical/compatibility fields on the static command entry, an invocation-keyed route list, and no change; state schema-version, legacy-consumer, completion and help consequences for each | verify: human — no option may be selected only because it is the smallest diff
- [ ] Freeze the representation, the `schema_version` policy for it, the meaning legacy fields keep, and which of TASK-256 and TASK-258 may implement which part | verify: human — an implementation task may not extend the representation beyond what is frozen here
- [ ] Append an approved `## Decision Record` to this card and change `decision-status` from `pending` to `decided` before TASK-256 or TASK-258 touches the manifest | verify: `make doc-check`

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
