---
id: TASK-259
title: "Discover qualified project addressing"
type: chore
priority: P0
effort: L
exec-tier: strong
created-at: 2026-09-02T10:11:00+09:00
source: "PLAN-003 cross-project discovery"
scope: "current routing grammar, namespace identity, reachability options, corpus evidence, and decision dossier"
status: todo
---

# Task 259: discover qualified project addressing

## Summary

Produce a self-contained evidence dossier for addressing a root project and imported projects without
changing routing or schema. The investigation must reconcile current direct project selection (`:` and
`--project`) with `/` canonical names used by imported items and literal user keys.

## Recommended direction

Treat `run --project` as the explicit machine route, retain `project:item` as direct-interaction shorthand,
and retain `project/item` only for explicitly imported parent-visible items. The discovery must test this
mixed grammar against real usage rather than assume one separator is inherently simpler. TASK-263, not this
card or TASK-260, owns the final address/exposure decision.

## Completion Criteria

- [x] Specify the current grammar and precedence for root routes, `project:item`, `--project`, imported `project/item` names, literal keys containing `:` or `/`, aliases, reserved prefixes, working-directory rebasing, lazy loading, and missing or ambiguous project errors; imported lifecycle canonical/alias routes must be inventoried as child-owned surfaces | verify: human — every claim must cite current tests and exact tracked symbols
- [x] Inventory how `run`, lifecycle verbs, provision, show, status, ls, manifest, help, and completion expose or omit project qualification, reachability, and owner identity; imported lifecycle routes report the owning child `env_file` and never substitute the parent stack or parent `env_file` | verify: `go test ./internal/config ./internal/cli -count=1`
- [x] Build a pinned, secret-free corpus of real qualified invocations and project layouts with canonical repository IDs, revisions, path inventory, collision findings, and dynamic-invocation limitations | verify: human — local absolute paths and unpinned repositories are not acceptable evidence
- [x] Compare retaining mixed grammar, making `/` canonical with an explicit compatibility period, and making `--project` the canonical explicit route; separately compare import/export reachability with automatic registration | verify: human — options must cover ambiguity, backwards compatibility, shell ergonomics, completion, machine discovery, and rollback
- [x] Append a canonical `## Evidence and Recommendation` section to this card with the decision-ready recommendation and rejected alternatives for TASK-263 without registering a new route, changing schema, or promising automatic reachability | verify: `make doc-check`

## Non-goals

- No `/` grammar, alias, project registry, or auto-import implementation.
- No cross-project plan composition implementation.
- No vocabulary rename.

## Evidence and Recommendation

Investigated against `ScriptonBasestar/dva` at revision `41995a870cbb48d8d608dc0f344e896f9791c06d`
(branch `master`), binary built from that revision with `make build` (`dva v0.1.47`). Every
invocation table below was executed against that binary; no result is inferred from source.

### 1. Current grammar and precedence

Four distinct addressing forms exist today. They are produced by three unrelated mechanisms,
which is why they behave differently even where they look alike.

| Form | Mechanism | Resolved by |
|---|---|---|
| `dva <item>` / `dva run <item>` | root interaction lookup | `runner.NewInteractionTree(c.Interaction).Find` (`internal/cli/run.go:45-46`) |
| `dva run --project <p> <item>` | explicit machine route | `runCmd` flag (`internal/cli/run.go:144`) → `runSubprojectCommand` (`internal/cli/run.go:85`) |
| `dva <p>:<item>` / `dva run <p>:<item>` | direct-selection shorthand | `strings.SplitN(cmdName, ":", 2)` (`internal/cli/run.go:32`) |
| `dva <p>/<item>` (and any declared alias) | parent-registered imported name | `resolveSubprojectImports` (`internal/config/subproject.go:78`); canonical name built at `:123`, `:150`, `:177` |

`/` is not a routing separator at all. `subprojectName + "/" + name` is written into the parent's
own `cfg.Plans`, `cfg.Interaction` and `cfg.Provision.Profiles` maps at config-load time, so
`engine/test` is looked up by exactly the same code path as `hello`. Nothing in `run.go`,
`plan_lifecycle.go` or `provision.go` ever splits on `/`.

**Precedence, in the order the code applies it** (`internal/cli/run.go:29-42`):

1. `--project` wins outright. When it is set, `LiteralKeyWins` is not consulted and the argument
   is never split on `:`. Measured: `dva run --project engine engine:test` → rc 1,
   ``command `engine:test` not found in subproject `engine` `` — it searched the child for a key
   literally named `engine:test` rather than re-splitting.
2. `config.LiteralKeyWins(c, cmdName)` (`internal/config/reserved.go:155`) — a declared literal
   key containing `:` beats the subproject reading, unless its prefix is a reserved built-in.
3. `SplitN(":", 2)` — first colon only. `dva engine:mytool:fast` routes to project `engine`,
   command `mytool:fast`, and works (measured, rc 0).
4. Root interaction tree.

**Reserved prefixes** (`internal/config/reserved.go:118` `UnroutableNamespacePrefix`, `:195`
`ValidateReservedCommands`) apply to *interaction keys only*, never to subproject names. This
produces the sharpest asymmetry found:

- Interaction key `compose:ps` in the root → `dva config validate` rc 1, unroutable, `ls` marks
  it `(unreachable: 'compose' is a reserved DVA command; rename to 'compose-ps')`.
- Subproject named `compose` with a child interaction `ps` → `dva compose:ps`, `dva run
  compose:ps`, `dva run --project compose ps` and `dva compose/ps` all exit 0 and run the child.
  `dva config validate` exits 0. The schema pattern for subproject keys
  (`internal/config/schema.json`, `subprojects.patternProperties` `^[\w\-.]+$`) does not exclude
  reserved words.

Subproject names cannot contain `:` or `/` under that same pattern, so the grammar is bounded on
the project side — but only under `validate`. The loader does not enforce the schema: with
`subprojects: {"a/b": {...}}` declared, `dva config validate` exits 1 with *Additional property
a/b is not allowed* while `dva run --project a/b ps` exits 0 and runs. This is the already-known
validate-vs-runtime split recorded in `USAGE.md`, reconfirmed here on the addressing axis.

**Literal keys.** A literal `:` key that shadows a real subproject is legal, wins, and is warned
about — `warnLiteralKeyShadowsSubproject` (`internal/config/validate_warnings.go:973`) emits
``interaction.engine:test: `dva engine:test` runs this key, not subproject `engine`'s `test` — the
literal key takes precedence; use `dva run --project engine test` to reach the subproject``.
Measured: `dva engine:test` → `ROOT-LITERAL-COLON-WINS`; `dva run --project engine test` →
`child-test`. A literal `/` key has no such escape hatch: when it collides with an import, load
fails hard (`interaction name collision: "engine/test" already exists`) and every command
including `dva ls` and `dva config validate` exits 1.

Note a documentation defect this exposes. `internal/config/reserved.go:155`'s comment and
`USAGE.md` both assert that a real subproject is unaffected "because a parent declaring
`subprojects: {engine: ...}` has no literal `engine:test` key of its own". It can, it wins, and
`warnLiteralKeyShadowsSubproject` exists precisely because it does. Two tracked statements
contradict a third in the same repository.

**Aliases** are ordinary parent-namespace keys with no special casing, and this is consistent
across every rule checked. Measured: `as: build` → reserved conflict, validate rc 1, `ls` marks
``(built-in 'build' takes this name; run: dva run build)``, `dva run build` reaches the child's
interaction with the child's owner. `as: "compose:x"` → unroutable, validate rc 1, `ls` marks it.
`as: "other:thing"` where `other` is another declared subproject → `warnLiteralKeyShadowsSubproject`
fires on the alias. Canonical and alias always resolve to the same `*Config` owner
(`internal/config/subproject.go:192-194`; `internal/config/config.go:445` `ProfileOwner`).

**Working-directory rebasing** happens in the DVA environment only, not in the executed process.
`ownedRuntime` (`internal/cli/command_runtime.go:26`) roots the child at `c.FileDir()` via
`newOwnedConfigEnvironment` (`internal/cli/root.go:417`), which sets `Environment.workDir`
(`internal/config/environment.go:34`). That value feeds `DVA_WORK_DIR_REL_PATH`, health-check
directories and provision step cwd — but the local interaction runner calls
`dvaexec.ExecReplace`/`ExecSequential`, which never set `cmd.Dir`, so the shell inherits the
caller's cwd. Measured from `<root>/deep/er`:

| Invocation | process cwd | `DVA_WORK_DIR_REL_PATH` |
|---|---|---|
| `dva rootwhere` | `<root>/deep/er` | `deep/er` |
| `dva engine:where` | `<root>/deep/er` | `.` |
| `dva run --project engine where` | `<root>/deep/er` | `.` |
| `dva engine/where` | `<root>/deep/er` | `.` |

All three child routes agree exactly. Root config discovery walks up from cwd, so every form
above works unchanged from any descendant directory.

**Lazy loading** differs by mechanism, and the difference is load-bearing.
`resolveSubprojectImports` skips any subproject with no import entries
(`internal/config/subproject.go:83-89`), so a declaration-only subproject is loaded on demand by
`runSubprojectCommand` (`internal/cli/run.go:95`) and a broken child costs nothing until it is
addressed. A subproject **with** imports is loaded on every single command. Measured with one
import pointing at a missing directory:

| Invocation | rc | message |
|---|---|---|
| `dva ls` | 1 | `resolving subprojects: loading subproject "ghost" (…): no such file or directory` |
| `dva run hello` | 1 | same, correct cause |
| `dva hello` (bare shorthand, local root command) | 1 | `unknown command "hello" for "dva"` |
| `dva --json hello` | 1 | `unknown command "hello" for "dva"` |

The bare form loses the cause because `cli.Execute` discards the load error
(`internal/cli/root.go:195`, `if c, err := loadConfig(); err == nil`) and lets cobra report an
unknown command. One unreachable child therefore makes the parent's own unrelated local commands
fail with a message that names the wrong thing — the silent-relocation shape TASK-137 and
TASK-167 exist to prevent, reappearing on the addressing axis.

**Missing and ambiguous project errors** are three different messages for one condition:

| Invocation | rc | message |
|---|---|---|
| `dva run --project nosuch test` | 1 | ``subproject `nosuch` not found. Available: engine`` |
| `dva run nosuch:test` | 1 | same |
| `dva nosuch:test` | 1 | `unknown command "nosuch:test" for "dva"` |

The bare form diverges because `dynamicRunArgs` (`internal/cli/root.go:236-252`) only prepends
`run` when the prefix names a declared subproject; otherwise the token reaches cobra unrouted.

**Imported lifecycle routes are child-owned surfaces, and this holds.** Measured with root
`env_file: .env.root` + `vars: {SHARED: root-vars}` and child `env_file: .env.child` +
`vars: {SHARED: child-vars}`:

| Invocation | observed |
|---|---|
| `dva rootwhere` | `ROOT_MARK=from-root CHILD_MARK= SHARED=root-vars` |
| `dva engine:where` | `ROOT_MARK= CHILD_MARK=from-child SHARED=child-vars` |
| `dva run --project engine where` | `ROOT_MARK= CHILD_MARK=from-child SHARED=child-vars` |
| `dva engine/where` | `ROOT_MARK= CHILD_MARK=from-child SHARED=child-vars` |
| `dva provision engine/seed` | `ROOT_MARK= CHILD_MARK=from-child SHARED=child-vars` |

`dva up engine/dev --dry-run` and `dva up engine-dev --dry-run` both print
`vars: env_file — declared [.env.child]`. No route substitutes the parent stack or the parent
`env_file`. TASK-262/TASK-264's repair is intact and needs no further work from TASK-263.

**Tracked tests that already pin this grammar** (all in the revision above):

- `internal/cli/literal_colon_key_test.go` — `TestFreePrefixColonKeyRunsTheLiteralKey`,
  `TestReservedPrefixColonKeyStaysUnroutable`, `TestRemovedBuiltinPrefixBecomesRoutable`,
  `TestSubprojectNamespaceStillRoutes`, `TestLiteralKeyWins`.
- `internal/cli/unroutable_namespace_test.go` — `TestUnroutableKeyFailsBothInvocationForms`,
  `TestManifestMarksUnroutableNamespacedKey`, `TestSubcommandsOfAnUnroutableKeyAreMarkedToo`,
  `TestFreePrefixNamespacedKeyIsNotMarked`, `TestLsJSONExposesTheSameUnroutableState`.
- `internal/cli/imported_command_owner_test.go` — `TestImportedInteractionAndProvisionOwnerIsolation`,
  `TestImportedCommandOwnerSharedAcrossRegistrations`.
- `internal/cli/imported_plan_lifecycle_test.go` — `TestImportedPlanLifecycleParity`,
  `TestImportedPlanUsesOwnerHooksAndEndpoints`, `TestImportedPlanInvalidOwnerFailsBeforeHook`,
  `TestImportedPlanManifestPreservesSchema`.
- `internal/cli/shadowed_builtin_test.go`, `internal/cli/interaction_key_space_test.go` —
  shadowing marks and the ls/manifest agreement.
- `internal/config/subproject_test.go` — loader contract and import resolution.

`go test ./internal/config ./internal/cli -count=1` passes at this revision with no production
change (`ok internal/config 0.856s`, `ok internal/cli 15.376s`).

### 2. Surface inventory: qualification, reachability, owner identity

`--project` is registered on `runCmd` alone. Measured by reading `--help` for all fifteen
commands: `run` is the only one that accepts it; `up`, `down`, `stop`, `restart`, `build`,
`logs`, `status`, `provision`, `show`, `ls`, `manifest`, `validate`, `doctor`, `console` reject
it. `dva up --project engine dev` → rc 1, `unknown flag "--project" for "dva up"`.

| Surface | `p:item` | `--project` | `p/item` | owner identity shown | canonical vs alias distinguished |
|---|---|---|---|---|---|
| `run` | yes | yes | yes (as a plain key) | no | no |
| `up`/`down`/`stop`/`restart`/`build`/`logs` | no | no | yes | no | no |
| `provision` | no | no | yes | no | no |
| `status` | no | no | plan arg only | prints `Subprojects: N` + `name -> path` only | n/a |
| `show` | no | no | listed as plain names | `Subprojects: name -> path` only | no |
| `ls` (table and `--json`) | no | no | listed as plain names | no | no |
| `manifest` | n/a | lists the `project` option | listed in `dynamic_commands`/`plans` | separate `subprojects` block, unlinked | no |
| `help` (`dva run --help`) | not mentioned | flag listed, no example | not mentioned | n/a | n/a |
| completion | not offered | no value completion | offered under `run` only | no | no |
| `doctor` | n/a | n/a | n/a | one check names the subproject key (`internal/cli/doctor.go:434` `checkSubprojectComposeProjectNames`) | n/a |

**Owner identity exists in the model and is exposed by nothing.** `PlanConfig.owner`
(`internal/config/config.go:77`), `InteractionCommand.owner` (`:263`) and
`ProvisionConfig.profileOwners` (`:439`) are populated for every import and correctly drive
execution — but no output surface reads them. Measured on a root importing one interaction, one
plan and one provision profile from `engine`, each with an alias:

```
$ dva ls
engine-test
engine/test
hello
...
Plans (dva up <name>):
  engine-dev  # childsvc
  engine/dev  # childsvc
  local       # rootsvc

$ dva show
Provision Profiles: engine-seed, engine/seed
```

Three imported items appear as six independent names. Nothing says `engine-dev` and `engine/dev`
are one plan, that either is child-owned, or that `local` is not. `ls --json` and `manifest`
carry the same shape with the same omissions: `interaction_commands` and `plans` entries have no
`project`, `owner`, `imported` or `canonical_name` field.

**The manifest advertises a wrong invocation under collision.** `buildManifestSubprojectCommands`
(`internal/cli/manifest.go:536`) writes `UsageExample: fmt.Sprintf("dva %s:%s", name, k)`
(`:545`) unconditionally — it consults neither `LiteralKeyWins` nor the parent's interaction map.
Measured with a parent that declares a literal `engine:test`:

```
$ dva manifest --json | jq '.subprojects.engine.commands.test.usage_example'
"dva engine:test"
$ dva engine:test
ROOT-LITERAL-COLON-WINS
```

The manifest's stated audience is machines. `dva config validate` warns a human about exactly
this collision and names the working form; the manifest neither carries the warning nor offers
the collision-safe `dva run --project engine test`. This is the same defect class that
`ConflictAdvice` (`internal/config/reserved.go:231`) was written to close for the root namespace
— "advice that names a command which refuses is worse than no advice" — reappearing on the
subproject surface it never covered.

**A second broken advice string.** `internal/cli/run.go:115` tells the user
``Run 'dva ls --project engine'``. That flag does not exist: `lsCmd` registers only `--format`
and `--detailed` (`internal/cli/list.go:43-46`). Measured, `dva ls --project engine` → rc 1,
`unknown flag: --project`. `USAGE.md` documents `ls` without the flag, so the error message is
the only place in the repository claiming it.

**Child-namespace rules do not apply to qualified routes.** A child whose own
`dva config validate` exits 1 is reported valid by the parent, and the parent's manifest
advertises examples for the keys the child rejects. Measured with a child declaring `build` and
`compose:ps`:

| Invocation | rc | observed |
|---|---|---|
| `dva config validate` (in child) | 1 | two reserved-command conflicts |
| `dva config validate` (in parent) | 0 | valid |
| `dva engine:build` | 0 | `CHILD-BUILD-INTERACTION` |
| `dva engine:compose:ps` | 0 | `CHILD-UNROUTABLE-KEY` |
| parent `manifest` `subprojects.engine.commands` | — | both listed, no `shadowed_by_builtin`, no `unroutable`, `usage_example` set for both |

Qualification is therefore an escape hatch out of the reserved namespace. Importing the same keys
behaves correctly instead (the alias lands in the parent namespace and is checked there), so the
gap is specific to the direct `:`/`--project` routes and to the manifest's subproject block.

**Completion is the thinnest surface.** `internal/cli/completion.go:11` registers exactly one
`ValidArgsFunction`, on `runCmd`, returning `c.Interaction` keys. Measured:

| Request | result |
|---|---|
| `dva __complete run ''` | `engine-test`, `hello`, `mytool:fast`, `weird/slash`, `engine/test` |
| `dva __complete up ''` | nothing (`:0`, falls back to filenames) |
| `dva __complete ''` | built-in command names only — no interaction, no shorthand |
| `dva __complete run --project ''` | nothing |

So imported `/` names are completable, plan names are not, the `:` shorthand is not, and
`--project` has no value completion. Whichever grammar TASK-263 selects, completion must be
built; it cannot be inherited.

### 3. Pinned corpus

Pinned to `ScriptonBasestar/dva` at `41995a870cbb48d8d608dc0f344e896f9791c06d`. The corpus is
stated as repository-relative tracked paths plus fixtures reproduced inline, so no local checkout
path is load-bearing and nothing here depends on an external corpus. All fixtures are synthetic
and secret-free: the only values are the sentinels `from-root`, `from-child`, `root-vars`,
`child-vars` and `echo` payloads.

**Tracked layout evidence in the repository** (real qualified layouts, no invented paths):

| Path | What it pins |
|---|---|
| `internal/config/subproject.go` | canonical `p/name` construction, alias registration, collision errors |
| `internal/config/subproject_test.go` | loader contract, `import:` resolution, module/override merge per child |
| `internal/cli/literal_colon_key_test.go` | `:` split vs literal key, reserved-prefix exception, live-reserved-set dependence |
| `internal/cli/imported_command_owner_test.go` | interaction and provision owner isolation across canonical and alias |
| `internal/cli/imported_plan_lifecycle_test.go` | imported-plan lifecycle parity, owner hooks/endpoints, manifest schema |
| `internal/cli/doctor_subproject_test.go` | the one diagnostic that names a subproject |
| `internal/config/schema.json` (`subprojects`) | `^[\w\-.]+$` project-name pattern, `import.plans/interactions/provision` shapes |
| `USAGE.md` (`#### run`, `### subprojects`) | the documented grammar the surfaces are measured against |

**Fixture A — mixed grammar, one child, three import kinds with aliases.** Parent `dva.yml`:

```yaml
subprojects:
  engine:
    path: ./child
    import:
      interactions:  [{name: test,  as: engine-test}]
      plans:         [{name: dev,   as: engine-dev}]
      provision:     [{name: seed,  as: engine-seed}]
interaction:
  hello:         {command: echo root-hello}
  "mytool:fast": {command: echo literal-colon-key}
  "weird/slash": {command: echo literal-slash-key}
stack:
  rootsvc: {default_runner: native, runners: {native: {run: echo root-service}}}
plans:
  local: {entries: [{name: rootsvc}]}
```

Child `child/dva.yml`:

```yaml
env_file: .env.child
interaction:
  test: {command: sh -c 'echo child-test CHILD_MARK=$CHILD_MARK'}
stack:
  childsvc: {default_runner: native, runners: {native: {run: echo child-service}}}
plans:
  dev: {entries: [{name: childsvc}]}
provision:
  seed: [{step: seed, run: echo seeding}]
```

Observed invocations (rc, output):

| Invocation | rc | output |
|---|---|---|
| `dva engine:test` | 0 | `child-test CHILD_MARK=from-child` |
| `dva run engine:test` | 0 | same |
| `dva run --project engine test` | 0 | same |
| `dva engine/test` | 0 | same |
| `dva run engine/test` | 0 | same |
| `dva engine-test` | 0 | same |
| `dva mytool:fast` | 0 | `literal-colon-key` |
| `dva weird/slash` | 0 | `literal-slash-key` |
| `dva up engine/dev --dry-run` | 0 | resolves child `env_file: .env.child` |
| `dva up engine-dev --dry-run` | 0 | identical resolution |
| `dva provision engine/seed --dry-run` | 0 | `[1/1] seed` |
| `dva run engine:test extra-arg` | 0 | extra args pass through |
| `dva run --project engine engine:test` | 1 | key searched literally in child |
| `dva --project engine test` | 1 | `unknown command "test" for "dva"` |
| `dva up --project engine dev` | 1 | `unknown flag "--project" for "dva up"` |
| `dva ls --project engine` | 1 | `unknown flag: --project` |
| `dva provision --project engine seed` | 1 | `unknown flag: --project` |
| `dva run --project nosuch test` | 1 | ``subproject `nosuch` not found. Available: engine`` |
| `dva run nosuch:test` | 1 | same |
| `dva nosuch:test` | 1 | `unknown command "nosuch:test" for "dva"` |
| `dva run engine/nosuch` | 1 | ``command `engine/nosuch` not recognized!`` |

**Fixture B — literal `:` key shadowing a real subproject.** Parent adds
`interaction: {"engine:test": {command: echo ROOT-LITERAL-COLON-WINS}}` alongside
`import.interactions: [test]`. `dva config validate` → rc 0 with the shadowing warning;
`dva engine:test` → `ROOT-LITERAL-COLON-WINS`; `dva run --project engine test` → `child-test`;
`manifest.subprojects.engine.commands.test.usage_example` → `dva engine:test` (wrong).

**Fixture C — literal `/` key colliding with an import.** Adding
`interaction: {"engine/test": {...}}` to Fixture B makes every command fail:
`dva config validate`, `dva ls`, `dva run engine:test` and `dva run --project engine test` all
exit 1 with `resolving subprojects: interaction name collision: "engine/test" already exists`.
Fail-closed, but the message does not distinguish "your literal key collides with an import"
from "two imports collide", and it takes down routes that never touch the name.

**Fixture D — reserved word as a project name.** `subprojects: {compose: {path: ./child}}` with
child interaction `ps`. `dva config validate` → rc 0; `dva compose:ps`, `dva run compose:ps`,
`dva run --project compose ps`, `dva compose/ps` → all rc 0, `CHILD-PS`. The identical token as a
*root interaction key* is a hard validation error.

**Fixture E — lazy vs eager child loading.** Two subprojects, one pointing at a missing
directory. With no `import:` on either: `dva ls`, `dva hello`, `dva engine:test` all rc 0, and
only `dva ghost:test` / `dva run --project ghost test` fail with the true cause. Adding
`import.interactions` to both: `dva ls` rc 1 with the true cause, `dva run hello` rc 1 with the
true cause, `dva hello` and `dva --json hello` rc 1 with `unknown command "hello" for "dva"`.

**Fixture F — invalid subproject names.** `subprojects: {"a/b": {...}, "c:d": {...}}`.
`dva validate` and `dva config validate` → rc 1, *Additional property a/b is not allowed*.
`dva run --project a/b ps` → rc 0, runs.

**Collision findings, consolidated.**

| Collision | Detection | Runtime behavior |
|---|---|---|
| literal `p:item` vs subproject `p` | semantic warning, rc 0 | literal key wins; `--project` reaches the child |
| literal `p/item` vs imported `p/item` | load error, rc 1 everywhere | nothing runs, including unrelated routes |
| import canonical vs import canonical | load error, rc 1 | nothing runs |
| import alias vs any existing name | load error, rc 1 | nothing runs |
| alias equal to a reserved built-in | reserved conflict, rc 1 on `validate`; `ls`/`manifest` mark it | built-in wins the bare form, `dva run <alias>` reaches the child |
| alias with a reserved prefix | reserved conflict, rc 1 on `validate`; `ls` marks unreachable | unreachable by any form |
| subproject named after a built-in | none | qualified forms reach the child; bare `dva compose` runs the built-in |
| child key that is reserved in the child | child-local `validate` only | reachable from the parent via every qualified form; parent `validate` rc 0 |

**Dynamic-invocation limitations — unresolved, not green.**

1. The bare shorthand is resolved before cobra runs, from `os.Args`
   (`internal/cli/root.go:186-198`). It cannot see flags, so `dva --project engine test` can never
   work, and a config load failure degrades every dynamic name into `unknown command`.
2. There is no way to enumerate the `:` shorthand. `dva ls`, `dva show` and `dva manifest`
   present only the root namespace plus a separate `subprojects` block whose entries are keyed by
   *child-local* names; joining the two into an invocation is left to the consumer, and the one
   place that does the join (`manifest.go:545`) gets it wrong under collision.
3. Provision profiles are absent from `manifest` entirely, so imported provision names are
   discoverable only through `dva show` text or a failing `dva provision`.
4. Because `--project` exists on `run` only, there is no measured evidence for how a project
   qualifier would interact with the lifecycle verbs' flag sets (`--force`, `--purge`,
   `--volumes`, tag selectors). Any option that adds `--project` to those verbs is unmeasured
   here and TASK-263 must not treat it as validated.
5. Nothing was measured on Windows or under a non-POSIX shell; `/` and `:` in shell words were
   exercised on `darwin` with `zsh` and `sh` only.

### 4. Options

**Addressing.**

*Option A — retain the mixed grammar (`:` shorthand, `--project` explicit, `/` for imports).*
Ambiguity is bounded and already has a fail-closed answer at every point: `--project` is
unambiguous by construction, `:` has one documented tie-break with a warning and a working escape
hatch, `/` cannot be ambiguous because it is never parsed. Backwards compatibility is total.
Shell ergonomics are good — neither separator needs quoting in `zsh`/`bash`. Completion is
buildable but must be written from scratch for all three forms. Machine discovery is the weak
point: an agent cannot currently learn which form applies without reading the config itself.
Rollback is a no-op. Cost: three forms to document and three surfaces to keep honest.

*Option B — make `/` canonical, with a compatibility period for `:`.* Attractive for uniformity,
but it collapses two different operations onto one token. `/` today means "a parent-registered
imported item"; direct selection means "a child item the parent never registered". Unifying them
either (i) makes every child item reachable as `p/item`, which is automatic reachability under a
different name and changes the meaning of an existing valid config, or (ii) leaves `p/item`
meaning both things depending on whether an import happens to exist, which is worse than the
present split. It also collides with literal `/` keys, which are legal today (`weird/slash`
measured working) and currently have *no* precedence rule — the `:` grammar at least has
`LiteralKeyWins`. Compatibility would require a deprecation period on a form (`:`) that is
documented in `USAGE.md`, printed by `warnLiteralKeyShadowsSubproject`, emitted by every
`manifest` subproject entry, and pinned by five tracked tests. Rollback after adoption is hard,
because configs written against the new meaning do not un-write.

*Option C — make `--project` the canonical explicit route and deprecate `:`.* Removes the one
genuinely ambiguous form and gives machines a single collision-safe shape. But `--project` is
registered on `runCmd`, so making it canonical for lifecycle verbs and `provision` means adding
a flag to eight commands and deciding its interaction with every destructive flag — none of
which is measured here. And it does not remove `/`, because imported names are map keys, not
routes. So Option C on its own does not reduce the number of forms; it changes which one is
blessed while adding flag surface.

**Reachability.**

*Explicit import (current).* A child is loaded eagerly only when it is imported; its items enter
the parent namespace only by name; collisions are caught at load. Blast radius measured: one
broken import fails every parent command (Fixture E), so eager loading is already the cost of
import — but that cost is opt-in and proportional to what the author asked for.

*Automatic registration.* Every declared subproject's items become parent-visible. This makes
every child a load-time dependency of every parent command, multiplying Fixture E's failure mode
by the number of subprojects; it silently reinterprets existing configs (a declaration-only
subproject currently contributes zero names); and it turns every child key into a potential
parent-namespace collision, including against reserved built-ins, with no author decision behind
it. No measured evidence supports it.

### 5. Recommendation

**Keep the current mixed grammar and explicit import (Option A + explicit import).** The
measurements support the direction the card and TASK-263 already record, and add the specific
reason: the three forms are not three spellings of one operation. `--project` selects a project
before any name resolution; `:` is a shorthand for that same selection with one documented
tie-break; `/` is not a route at all but a parent-owned name for a parent-registered item. A
change that unifies the separators has to first make those three operations one operation, and
nothing measured here suggests that is desirable.

What is actually broken is **exposure, not grammar**. Owner identity already exists in the model
and is correct at execution time; every surface simply declines to print it. TASK-263 should
freeze representation, not routing:

1. Machine surfaces must name the owning project and the collision-safe invocation. `ls --json`
   and `manifest` entries for imported items need an owner field and a canonical-vs-alias marker,
   so an agent stops seeing six names for three items.
2. `manifest.subprojects.*.commands.*.usage_example` must be computed against the parent's
   namespace, exactly as `ConflictAdvice` and the root `dynamic_commands` marks already are —
   falling back to `dva run --project <p> <item>` when the `:` form is shadowed. The current
   unconditional `dva %s:%s` is measurably wrong.
3. `internal/cli/run.go:115` must stop advertising `dva ls --project`, or `ls` must gain the flag.
   The two cannot continue to disagree.
4. Completion must be specified for all three forms plus `--project` values and plan names; today
   only root interaction keys are completed.
5. Two questions are genuinely open and should be decided rather than inherited: whether a
   subproject may be named after a reserved built-in (today it may, silently), and whether a
   child's reserved-name and unroutable-prefix keys should stay reachable through a qualified
   route (today they do, while the child's own validator rejects them).
6. `USAGE.md` and the `LiteralKeyWins` comment must drop the claim that a parent has no literal
   `p:item` key of its own; the warning that exists proves otherwise.

**Rejected alternatives, with the reason each fails.**

- *`/` as the single canonical separator* — rejected on compatibility and meaning. It either
  introduces automatic reachability by another name or leaves `p/item` ambiguous between two
  meanings; it collides with legal literal `/` keys that have no precedence rule; and rollback
  after configs adopt it is not clean.
- *`--project` as the sole canonical route with `:` deprecated* — rejected as premature. It
  requires adding the flag to eight commands and settling its interaction with destructive flags,
  none of which is measured. It also cannot remove `/`, so it does not reduce form count.
- *Automatic registration of every subproject's items* — rejected on blast radius, silent
  reinterpretation of existing configs, and unbounded collision surface. Fixture E measures the
  failure mode at one subproject; automatic registration applies it to all of them.
- *Flattening child items into the parent namespace without a prefix* — not evaluated as a live
  option: it is a strict superset of automatic registration's problems and PLAN-003 already
  excludes child-stack flattening.

**Fail-closed default.** If TASK-263 cannot obtain the compatibility evidence a grammar change
needs, the exact current grammar and explicit import policy stand, and the exposure repairs above
are separable from any addressing decision — they are correct under every option.

## Troubleshooting Log

- 증상: 한 subproject의 `path`가 존재하지 않을 때 부모 자신의 로컬 interaction까지 `unknown command "hello" for "dva"`로 실패 / 원인: `import:`가 선언된 subproject는 매 명령마다 eager 로드되는데, `cli.Execute`(`internal/cli/root.go:195`)가 `loadConfig()` 에러를 버리고 cobra에 넘겨 동적 라우팅이 통째로 사라짐 — `dva run hello`는 진짜 원인을 출력하므로 bare 형태에서만 마스킹됨 / 해결: 진단 코드 변경 없이 lazy-vs-eager 차이와 마스킹 지점을 corpus fixture E로 고정해 TASK-263 결정 근거로 기록 / 약 40분
