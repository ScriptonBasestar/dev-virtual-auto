---
id: TASK-254
title: "Discover a command metadata registry boundary"
type: chore
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-02T10:06:00+09:00
source: "PLAN-003 command discovery ownership investigation"
scope: "command metadata inventory, derivation matrix, consistency gates, and implementation recommendation"
status: todo
---

# Task 254: discover command metadata ownership

## Summary

Map every owner of public command metadata and produce a decision-ready boundary for a possible registry.
The result is evidence for later architecture work; this task does not introduce a registry or change the
command surface.

## Recommended direction

Keep Cobra as the routing and flag owner, derive descriptions and registered flags from it, and introduce only
the smallest immutable descriptor for semantic fields Cobra cannot express: stable command identity, route role,
machine-safe explicit invocation and manifest type. Preserve existing coverage assertions instead of replacing
them with a large registry whose initialization order becomes a new source of truth.

## Completion Criteria

- [x] Inventory Cobra registration, names, aliases, groups, arguments, flags and descriptions; reserved names; hookable commands; manifest type/options/descriptions; direct-help wrappers; collision validation; completion and generated AI documentation | verify: human — the inventory must cite exact tracked paths and symbols and distinguish generated projections from canonical owners
- [x] For every metadata field, classify whether it can be derived, must remain hand-authored, or needs an explicit consistency assertion; document current duplication and the failure each existing guard prevents | verify: human — the matrix must cover runtime routing, help, manifest, completion, reserved-name validation, and generated documentation
- [x] Compare at least a minimal shared descriptor, Cobra-as-SSOT derivation, and the current split ownership; state initialization-cycle, dynamic-command, passthrough-option, testability, and migration tradeoffs | verify: human — no option may be selected solely because it removes line duplication
- [x] Append a canonical `## Evidence and Recommendation` section to this card with the smallest coherent ownership boundary and a staged migration seam, or record that current ownership should remain; if canonical/compatibility route identity requires new manifest fields or a shared descriptor, create a bounded decision/implementation child and update PLAN-003 children, count, graph, completion rule and affected TASK-256/TASK-258 dependencies in the same change | verify: `make doc-check`
- [x] Existing command and reserved-name tests pass without production changes | verify: `go test ./internal/cli ./internal/config -count=1`

## Non-goals

- No `CommandSpec` or equivalent production registry.
- No route, alias, help group, manifest schema, or generated-document change.
- No weakening of current registration and reserved-name consistency checks.

## Evidence and Recommendation

Every `file:line` below was opened in this worktree at `6d3f08d`. Every measured invocation was run against
`bin/dva` built from that commit by `make build` (version 0.1.47), in a scratch directory whose only content
was `dva.yml` holding `version: "0.1.47"`. Counts stated as "measured" come from `dva manifest -f json` and
`dva --help` on that binary, not from reading the tables.

### 1. Inventory of public command metadata owners

Canonical owners, and the projections that restate what they own.

| Fact | Canonical owner | Projections |
| --- | --- | --- |
| Top-level route registration | 22 `rootCmd.AddCommand` calls: `internal/cli/root.go:99-115` (17), `doctor.go:99`, `status.go:111`, `config_dump.go:54`, `init.go:96`, `validate_alias.go:14`. Cobra adds `help` and `completion` lazily in `Execute()`; `manifest.go:260-261` calls `InitDefaultHelpCmd`/`InitDefaultCompletionCmd` to see them without executing. Measured top level: 24 | `manifest.go:308-395` table, `internal/config/reserved.go:13-21` |
| Nested route registration | `configCmd`: `validate.go:222`, `init.go:82`, `config_dump.go:53`, `config_docs.go:43`, `config_migrate.go:169` (5 children). `sshCmd`: `ssh.go:90` (3). `consoleCmd`: `console.go:89` (2). `skillCmd`: `skill.go:129-130` (4) | only `skill` reaches the manifest |
| Name and usage line | each command's `Use` literal — e.g. `compose.go:76,333,414,483,688,780`, `kubectl.go:13`, `run.go:19`, `status.go:16`, `provision.go:25`, `validate.go:130`, `init.go:22` | help output only |
| Aliases | one `Aliases` field in the whole tree: `version.go:14` `{"-v", "--version"}` | none — no surface publishes it |
| Compatibility routes | two, built two different ways (see below) | none marks either as compatibility |
| Help groups | 5 `cobra.Group` values and 17 `GroupID` assignments in `root.go:68-97`, plus `doctor.go:98` and `validate_alias.go:10` | help template only |
| Help rendering | hand-written `dvaUsageTemplate` `root.go:505-542`, three template funcs `root.go:150-158`, featured lifecycle pair hard-coded `root.go:475-499` | — |
| Arguments | no `Args` validator on any command; 8 commands set `DisableFlagParsing` (`compose.go:31,107,366,441,529,708,795`, `kubectl.go:19`) and parse their own argv | — |
| Registered flags | 3 root persistent `root.go:64-66`; cobra locals on 7 top-level commands (`doctor.go:96-97`, `init.go:92-95`, `list.go:45-46`, `manifest.go:36`, `provision.go:97`, `run.go:142-144`, `validate_alias.go:18-19`) and on 9 nested ones (`init.go:76-79`, `config_dump.go:52`, `config_migrate.go:167`, `validate.go:221`, `ssh.go:86-88`, `skill.go:119-128`) | `manifest.go:191-205` and `:207-237` derive both, but only for commands the manifest reaches |
| Hand-parsed flags | `parseDvaFlags` / `parsePlanFlags` read them out of raw args; no flag object exists | `manifest.go:153-161` const block, restated in prose in each `--help` Long |
| Direct-help wrappers | `manualFlagCommands` local slice `root.go:141-148` | copied by hand into `root_command_registration_test.go:152-156` |
| Hookability | `internal/config/reserved.go:30-34` (6 names) | `root.go:120-136` binds them to cobra values and panics in both directions; `reserved.go:49-56` derives the advice sentence |
| Reserved names | `internal/config/reserved.go:13-21` (24 names) | `root.go:27-29` dynamic routing, `reserved.go:91-99` shadowing, `:118-127` unroutable prefix, `:155-164` literal-key routing, `:195-221` validation, `:231-268` advice |
| Manifest document | `manifest.go:39-63`; `ManifestCmd` is 4 fields `manifest.go:105-110`; literal type table `manifest.go:308-395` | — |
| Interaction invocation form | `interactionUsage` `list.go:168-198` — one function feeding both `dva manifest` and `dva ls --json` | — |
| Completion | cobra's generated `completion` command; the only project-authored completion is `runCmd.ValidArgsFunction` `completion.go:11-24`. No plan-name, entry-name or subproject completion exists | — |
| Generated AI documentation | hand-authored embedded `internal/cli/dva_guide_template.txt`, written by `dva config docs` (`ai_docs.go:29-57`); the CLAUDE.md/AGENTS.md snippet `ai_docs.go:17-25`; `internal/cli/library_reference.txt`, concatenated by `make generate` from `agent-mesh-flows/shared/library/` | — |

The two compatibility routes, which matter to TASK-255 and TASK-257, are not aliases in cobra's sense and are
not built the same way:

- `dva validate` — `validate_alias.go:6-14` constructs a second `cobra.Command` copying `Use`, `Short`, `Long`
  and `RunE` from `validateCmd` (`validate.go:130-131`, registered under `config` at `validate.go:220-222`),
  and shares flag registration through `addValidateFlags` (`validate_alias.go:17-20`). It sets
  `GroupID: "advanced"`.
- `dva init` — `init.go:85-96` copies `Short`, `Long` and `RunE` from `initCmd` (`init.go:22-23`, registered
  under `config` at `init.go:82`) but re-registers all four flags literally (`init.go:91-94` against
  `init.go:75-78`) onto the same package-level variables. It sets no `GroupID`.

Measured consequence of that asymmetry: `dva validate` renders under "Advanced Utilities" while `dva init`
renders under "Additional Commands" beside `help` and `completion`. `dva init --help` and
`dva config init --help` currently differ only in their `Usage:` line.

### 2. Derivation matrix

`D` = already derived or derivable from cobra. `H` = must stay hand-authored. `A` = needs an explicit
consistency assertion because two owners must agree.

| Metadata field | Class | Current duplication | Guard, and the failure it prevents |
| --- | --- | --- | --- |
| Command description | D | none — `manifest.go:259-282` copies each `Short` | `TestStaticCommandDescriptionsMatchTheirShort` (`manifest_static_commands_test.go:265`) catches a command the derivation stops reaching, which would ship a blank description. Its own comment records that a re-added literal does **not** fail, because the derivation overwrites it |
| Registered flag name and usage | D | none — `manifest.go:207-237` copies each `pflag.Flag` | `TestStaticCommandOptionsCoverEveryRegisteredFlag` (`:153`) catches a flag the fill skips; the comment records the measured mutation (`f.Name == "images"`) that it caught |
| Root persistent flags | D | none — `manifest.go:191-205` walks `VisitAll` | `TestManifestPublishesEveryPersistentFlag` (`manifest_global_flags_test.go:12`) catches a fourth persistent flag never reaching agents |
| Hand-parsed flag name and description | H | 1 copy (`manifest.go:153-161`) plus prose in each `Long` | `TestHandParsedOptionsAreDocumented` (`:203`) is a hand-written `want` map. Its own comment records the direction it cannot check: `dev` and `docker` survived in it after the flags were removed |
| Command type (`static_commands[].type`) | H | 1 literal table `manifest.go:308-395` | `TestEveryStaticCommandCarriesAType` (`:112`) rejects an unlisted type and an empty description. `GroupID` cannot stand in: 5 coarse groups against 8 types |
| Top-level route existence | A | 3 owners: `rootCmd`, the manifest table, `reservedCommands` | `TestStaticCommandsCoverEveryRootCommand` (`:68`) and `TestStaticCommandsAgreeWithReservedCommands` (`:90`) close the triangle both ways. The failure: a command an agent reading the manifest concludes does not exist, or a name a user can declare as an interaction that then never runs |
| Nested route existence | A | **unguarded** | Both coverage tests walk `rootCmd.Commands()` only. Measured: 24 `static_commands`, `subcommands` populated for `skill` alone; `config`'s five children, `ssh`'s three and `console`'s two are absent |
| Help group membership | A | assignment sites vs template | `TestCommandHelpGroupsAndDiscoveryDescriptions` (`root_command_registration_test.go:55`) pins group order and three commands' placement; `TestLifecycleHelpSeparatesRecommendedFromOther` (`root_help_groups_test.go:27`) pins the lifecycle split |
| Hookable set | A | `config` set vs cobra binding | the two `panic`s in `root.go:126-136` fail the process on either half being forgotten — the strongest guard here, and it costs no test |
| Reserved-name collision behavior | A | one predicate per state, shared | `ShadowedByBuiltin` and `UnroutableNamespacePrefix` are consulted by validation, warnings, `ls` and the manifest alike, so the validator and the machine surfaces cannot disagree about which keys are conflicts (`reserved.go:199-218`, `list.go:168-198`) |
| Direct-help wrapping | A | production slice vs test slice | `TestDirectHelpDoesNotExecuteManualFlagCommands` (`root_command_registration_test.go:151`). Its comment states the blind spot honestly: a **new** `DisableFlagParsing` command added to `root.go` and not to the test is untested, not failing |
| Group-parent behavior | A | production call sites vs a hard-coded count | `TestGroupParentsCovered` (`command_group_test.go:58`) lists `config` and `ssh` and fails only if someone edits the list |
| Compatibility-route parity | A | see §1 | `TestRootValidateMatchesConfigValidate` (`root_command_registration_test.go:33`) compares `Use`, `Short` and every flag spec; `TestRootValidateMatchesConfigValidateBehavior` (`:175`) compares stdout, stderr and error for a valid and an invalid config. **No equivalent exists for `init`** |
| Aliases | A | — | **unguarded** |
| Route identity (which name is canonical) | H | — | no owner. `ManifestCmd` has no field for it |
| Generated AI guide | H | names commands in prose | `TestDVAGuideUsesNamedPlanLifecycle` (`ai_docs_test.go:23`) is a required-substring and forbidden-substring list; nothing checks that a command named in the guide exists or does what the line says |

Three holes in that matrix were measured, not inferred. They are reported here as evidence and deliberately
left unfixed, per this card's non-goals.

1. **`console` is a third pure group parent that `setGroupParentBehavior` does not cover.** `consoleCmd`
   (`console.go:13-16`) has two children and no `Run`/`RunE`, exactly like `configCmd` and `sshCmd`, but
   `Execute()` wires only those two (`root.go:183-184`). Measured: `dva ssh statu` exits 1 with
   `unknown command "statu" for "dva ssh"` and a `status` suggestion; `dva console injec` **prints group help
   and exits 0** — the TASK-148 failure, still live for this one parent. `TestGroupParentsCovered` cannot see
   it: it asserts `len(groups) != 2` against its own hand-written list, so a production parent added without
   touching the test is invisible.
2. **`versionCmd.Aliases` is dead metadata.** Measured: `dva -v` exits 1 with `unknown shorthand flag: 'v' in
   -v`, `dva --version` exits 1 with `unknown flag: --version`. Root persistent-flag parsing claims both
   tokens before cobra's alias matching, which only runs on non-flag arguments; `isFlag` (`root.go:226-228`)
   also excludes them from dynamic routing. No surface publishes the field, so nothing contradicts it either.
3. **The generated agent guide teaches an invocation that does not do what it says.**
   `internal/cli/dva_guide_template.txt:47` reads `dva config # Show resolved configuration`. Measured:
   `dva config` prints group help and exits 0; `dva config show` is the route that prints the resolved
   configuration (`config_dump.go:18-36`). The substring test at `ai_docs_test.go:23-45` cannot catch this
   class.

Stale hand-written counts in comments are a fourth, milder instance of the same class: `manifest.go:296` says
"13 of 27", `manifest.go:393` says "finds 21 and this table lists 23", and
`manifest_static_commands_test.go:22-25` says `Commands()` returns 25 and the pair of `Init` calls "gives the
same 27". Measured today: 22 `rootCmd.AddCommand` calls, 24 top-level commands, 24 `static_commands` entries.
The assertions are correct; only the prose drifted. A registry would not have prevented this.

### 3. Options compared

**A — minimal shared descriptor.** Keep cobra as the router and flag owner. Add one immutable value per
command carrying only what cobra cannot express: stable identity, route role (canonical / compatibility),
machine-safe explicit invocation, manifest type.

- Initialization cycle: low risk. The descriptor is data with no dependency on `rootCmd`, so it can be a
  package-level literal read by both `init()` and `buildManifest`. It does not need to be *registered*, which
  is what makes registries order-sensitive.
- Dynamic commands: unaffected. Interaction keys never enter it; they keep `interactionUsage`
  (`list.go:168-198`), which already unifies the two machine surfaces.
- Passthrough options: unaffected. The 8 `DisableFlagParsing` commands keep hand-parsing, and their flag
  descriptions stay in `manifest.go:153-161`, which the descriptor does not try to absorb.
- Testability: the existing coverage tests keep working unchanged, because they are anchored on `rootCmd`,
  and gain one more table to close the triangle against.
- Migration: additive. Nothing existing has to move for the first command to carry a descriptor.

**B — cobra as sole SSOT, derive everything.** Delete the literal type table and walk `rootCmd.Commands()`.

- Blocked on facts cobra does not hold. `Type` is 8 values against `GroupID`'s 5 coarse groups
  (`manifest.go:299`), and the eight hand-parsed flags have no `pflag.Flag` to read. Both would have to be
  smuggled into `Annotations`, which is the descriptor of option A wearing a map.
- Also weaker as a guard, and this is measured rather than argued: `TestStaticCommandsCoverEveryRootCommand`
  fails today when a command is added to `rootCmd` and forgotten. Derive the table from `rootCmd` and that
  test becomes a tautology — the drift it catches stops being expressible.

**C — full `CommandSpec` registry as the single production source.** One table that owns names, groups, flags,
types and registration, with cobra built from it.

- Initialization cycle is the real cost. Registration currently happens across 12 `init()` functions in
  alphabetical file order, and `root.go:180-184` already documents one ordering constraint that had to be
  moved into `Execute()` because `init()` order was wrong for it. A registry makes that ordering load-bearing
  for the whole surface instead of one call.
- Dynamic commands and passthrough get worse, not better: interaction keys are config-derived and cannot live
  in a compile-time registry, so the surface would be described by two sources again — the thing the change
  was for.
- Testability regresses for the same reason as option B: three independent owners agreeing is what the
  coverage tests check, and collapsing them to one deletes the check rather than satisfying it.
- Migration is a single large change touching every command file, with no intermediate state that ships.

None of the three is chosen for removing duplicated lines. Option A is chosen because it adds an owner for a
fact that currently has none (route role), and leaves every existing guard intact; B and C are rejected
because each converts a live consistency check into a tautology.

### 4. Recommended boundary and staged seam

Keep current ownership. Cobra stays the router, flag owner and description owner; `reservedCommands` stays the
collision owner; the manifest type table stays hand-authored. Do not build a registry.

The one gap worth closing is route identity, and it is closed by TASK-272 rather than here. The staged seam,
in dependency order, is:

1. **TASK-272 freezes the representation** — subcommand coverage, and whether a compatibility entry names its
   canonical invocation. No code moves.
2. **TASK-256 / TASK-258 implement only what TASK-272 froze**, under their existing criteria, which already
   say the manifest must use the approved representation or wait.
3. Anything beyond that — the three measured holes in §2, the stale comment counts — is a separate small
   repair, not part of this boundary. They are recorded here so they are not rediscovered as arguments for a
   registry, because none of them is a duplication problem: `console`'s parent behavior is a missing call
   site, the `version` aliases are a dead field, and the guide line is unverified prose.

### 5. Route-identity child: condition triggered

TASK-255 criterion 5 and TASK-257 criterion 4 both say that if the current schema cannot express the chosen
representation, the bounded child produced from this card is required before implementation. That condition is
met, and it is met independently of which way either decision goes.

- `ManifestCmd` (`manifest.go:105-110`) has exactly four fields: `description`, `type`, `options`,
  `subcommands`. None of them can say that two entries are one command, or which name is preferred.
- `static_commands` is keyed by a single name, so `validate` and `config validate` cannot be linked even in
  principle.
- Measured: the manifest publishes 24 `static_commands` and populates `subcommands` for `skill` only.
  `config validate` — one of the two routes TASK-257 is choosing between, and the form both
  `internal/cli/dva_guide_template.txt:44` and the `dva init` next-steps text (`init.go:69`) teach — **is not
  in the manifest at all**. So no TASK-257 outcome, including "keep both routes exactly as they are", is
  expressible in the document today.
- The same holds for TASK-255: if `kubectl` is ever added beside `ktl`, both appear as unrelated coequal
  entries with identical descriptions and no way to mark either.

[TASK-272](272-freeze-manifest-route-identity.md) is created for this, `TASK-256` and `TASK-258` gain it as a
dependency, and [PLAN-003](../plan/003-command-surface-renewal-discovery.md) is updated in the same change.

## Troubleshooting Log

- 2026-09-03 (~10m) 증상: 존재하지 않는 하위 route를 확인하려고 zsh에서 `for c in "config bogus"; do $B $c; done`을 돌렸더니 `unknown command "config bogus" for "dva"`가 나와 `config`가 하위 명령을 전혀 라우팅하지 않는 것처럼 보였다. 원인: zsh는 인용 없는 변수 확장을 단어 분할하지 않아 `"config bogus"` 한 덩어리가 단일 argv로 전달됐고, 측정 대상이 아니라 측정 도구가 만든 결과였다. 해결: 인자를 리터럴로 분리해 재측정했고 실제 동작은 `dva config statu` → rc 1(제안 없음), `dva ssh statu` → rc 1 + `status` 제안, `dva console injec` → rc 0 그룹 help였다. 마지막 것이 §2에 기록한 `console` 결함이다.
