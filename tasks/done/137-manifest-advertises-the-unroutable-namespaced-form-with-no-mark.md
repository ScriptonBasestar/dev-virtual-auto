---
id: TASK-137
title: "manifest advertises the unroutable namespaced form, and its machine-readable surfaces have no state for it"
type: bug
priority: P2
status: done
effort: M
created-at: 2026-08-03T12:10:00+09:00
source: "TASK-076 finalize verification — the branch its non-goals excluded, untracked"
depends-on: [TASK-076]
scope: "dva repo — internal/cli/manifest.go, internal/config/reserved.go, internal/cli/list.go"
---

# Task 137: Give the manifest a state for "unroutable", not just "shadowed"

## Problem

TASK-076 fixed the case where `dva manifest` advertised an invocation that reaches a
built-in instead of the declared interaction: reserved names now carry
`shadowed_by_builtin` and a `usage_example` that actually works.

An interaction whose key uses a **reserved name as a namespace prefix** was excluded by
that task's non-goals, and it is still advertised as if it worked. Reproduced live with
`bin/dva` v0.1.44 against a fixture declaring `interaction: {"app:build": {run: ...}}`:

```
dva manifest → "app:build": { "runner": "Local", "usage_example": "dva app:build" }
dva app:build       → rc=1  ERROR: unknown command "app:build" for "dva"
dva run app:build   → rc=1  ERROR: subproject `app` not found. Available:
```

Both forms fail. `usage_example` names one of them.

`ShadowedByBuiltin` is correctly false — nothing shadows this key, it is simply
unreachable — so the fix is not to reuse that field. The surfaces need a third state.

## What already works

`dva ls` and `dva validate` diagnose it precisely (`internal/config/reserved.go`):

> interaction command namespace prefix 'app' is a reserved DVA command — no invocation
> reaches this key: the bare form is not a built-in, and the run form reads 'app:' as a
> subproject reference, so it fails with subproject 'app' not found. Use a different
> separator (e.g., 'app-build')

So the detection exists and the human-facing text is good. Only the machine-readable
surface — the one an AI agent reads to decide what to run — still advertises the dead
form.

## Acceptance criteria

- [x] `dva manifest` marks a namespace-prefixed reserved key with a state distinct from
      `shadowed_by_builtin` (e.g. `unroutable: "app"`), carrying the reason.
- [x] `usage_example` for such a key names no invocation that exits non-zero — either it
      is omitted, or it names the working alternative the warning already suggests.
- [x] `dva ls --json` exposes the same state as `manifest`.
- [x] A test proves both invocation forms fail for the marked key, so the mark cannot
      drift away from the behaviour it describes.
- [x] A non-reserved namespaced key (`build:fast` is reserved, `mytool:fast` is not) is
      left unmarked — the mark is not a blanket rule about colons.
      *(Implemented as written. The stated rationale does not hold — see Correction 2.)*
- [x] `make test` exits 0.

## Notes

TASK-076's own "Left open" section names this and calls it "this task's defect, in the
branch the non-goals excluded". A sweep on 2026-08-03 found no other task tracking it,
which is why this file exists.

Its two sibling left-opens are already handled elsewhere: USAGE.md's size is TASK-106,
and `validate` being fatal while `ls`/`manifest`/`run` proceed on the same config is
documented in USAGE.md as-is.

## Corrections to this task's own Problem section

Both found by running the reproduction rather than trusting it. Neither changes the
conclusion — every form still exits 1 — but each contradicts a premise the task argued
from, so they are recorded rather than quietly worked around.

**1. The bare form does not fail with `unknown command`.** The Problem section claims
`dva app:build` → `ERROR: unknown command "app:build" for "dva"`. Measured against
`bin/dva` with the fixture:

```
dva app:build     → rc=1  ERROR: subproject `app` not found. Available:
dva run app:build → rc=1  ERROR: subproject `app` not found. Available:
```

`app:build` is not a built-in, so `root.go:194` looks it up in `c.Interaction`, finds it,
and rewrites the invocation to `dva run app:build`. The two forms therefore converge on
one code path. That is *why* one assertion in the test can cover both — and it is also
why `unroutable` is the honest name: the key is not unknown to dva, it is known and
unreachable.

**2. `mytool:fast` — criterion 5's control case — is equally unroutable.**

```
dva mytool:fast → rc=1  ERROR: subproject `mytool` not found. Available:
```

`run.go:33` splits *any* interaction key on `:` and reads the prefix as a subproject
reference. Whether the prefix is reserved is irrelevant to routing. So criterion 5's
stated rationale — "the mark is not a blanket rule about colons" — is contradicted by the
binary: colons really are the whole cause.

**3. `scope:` named a file that does not exist.** `internal/cli/ls.go` — the `ls` command
lives in `internal/cli/list.go`. Corrected in the frontmatter.

## Decision: the mark stays keyed to the reserved prefix

Correction 2 invites widening `unroutable` to every colon key. Rejected, for this task:

1. **The mark carries a reason, and the reason differs.** `unroutable_reason` is
   `ConflictAdvice`, which says *"namespace prefix 'app' is a reserved DVA command"*.
   Applied to `mytool:fast` that sentence is false — no reserved command is involved.
   Marking it would replace an unadvertised failure with a misreported one.
2. **The two have different fixes.** `app:build` must be renamed; the prefix is taken.
   `mytool:fast` is what the syntax at `run.go:31` was *built for* — it should route to a
   subproject and only fails because no subproject declares that name. Making the config
   valid is the fix there, not renaming the key.
3. **`unroutable` must agree with the validator.** `ValidateReservedCommands` rejects
   exactly the reserved-prefix keys. If the manifest marked more keys than validate
   rejects, the two surfaces would disagree about the same config — the drift TASK-076
   was about. `UnroutableNamespacePrefix` is shared by both so they cannot.

Filed **TASK-167** for the free-prefix case: a colon key whose prefix names no subproject
is silently unreachable and nothing warns. `reserved.go:93` and
`unroutable_namespace_test.go:87` already point a reader at it.

## Result

Third state added, distinct from `shadowed_by_builtin`. Measured on the fixture above:

| Key | manifest / ls --json | human `ls` |
|---|---|---|
| `app:build` | `"unroutable": "app"` + `unroutable_reason`, **no** `usage_example` | `(unreachable: 'app' is a reserved DVA command; rename to 'app-build')` |
| `mytool:fast` | no mark, `"usage_example": "dva mytool:fast"` | unmarked |
| `my-build` | no mark | unmarked |

Sites changed:

| File | Change |
|---|---|
| `internal/config/reserved.go` | new `UnroutableNamespacePrefix`; `ValidateReservedCommands` now calls it instead of repeating the colon/prefix logic |
| `internal/cli/list.go` | `interactionUsage` returns a third value; unroutable case returns empty `usage`; human mark; `buildCommandEntries` emits both JSON fields |
| `internal/cli/manifest.go` | `Unroutable`, `UnroutableReason` fields; `usage_example` gained `omitempty` |
| `internal/cli/unroutable_namespace_test.go` | new, 4 tests |

`usage_example` is **omitted**, not repointed at `dva app-build`: that command does not
exist until the author performs the rename, so naming it would be a second dead
invocation in the field whose implicit promise is that running it works. `omitempty` on
`UsageExample` is what makes omission possible — without it the field renders as `""`,
which reads as "run `dva `".

The tests pin the mark to behaviour, not to the implementation that produces it:

- `TestUnroutableKeyFailsBothInvocationForms` — asserts `app:build` is not a built-in,
  that the routing lookup at `root.go:194` finds it (so the bare form really does land on
  the run path), that the run path errors with ``subproject `app` not found``, and that
  `ConflictAdvice` names *that same* error. If any invocation ever starts working, the
  mark becomes the lie it exists to prevent, and this fails.
- `TestFreePrefixNamespacedKeyIsNotMarked` — criterion 5.
- `TestLsJSONExposesTheSameUnroutableState` — parity, with a `checked != 3` vacuity guard
  and an assertion that the marked key actually carried a mark (two empty maps agree).

`make test` exits 0.

## Review follow-up

`core:code-reviewer` reproduced three High findings against `bin/dva`. All three were real
and all three are fixed here; the measurements below are from the built binary, before and
after.

**1. The mark stopped at the declared key.** `interactionUsage` guarded the unroutable
branch with `len(path) == 1`, so a declared `app:build` carrying a `subcommands:` block
flattened into two entries and only the parent was marked:

```
'app:build'        usage='dva app:build'        unroutable='app'
'app:build fast'   usage='dva app:build fast'   unroutable=''      ← advertised, exits 1
```

Both forms fail identically — `run.go` splits `args[0]` and never reaches the rest — so the
child was publishing the exact promise this task exists to retract, to the audience
(`manifest`'s consumers are agents) it exists to protect. The guard is gone; the branch now
keys off `path[0]` at any depth. After: both rows carry `unroutable='app'` and neither
carries a `usage_example`.

**2. The rename advice named a command that still fails.** `ConflictAdvice` and the `ls`
mark both computed the suggestion with `strings.Replace(name, ":", "-", 1)` — first colon
only. For `app:sub:cmd` that suggested `app-sub:cmd`, which `run.go` still splits, which
still exits 1 with ``subproject `app-sub` not found`` — and because `app-sub` is not a
reserved command, `UnroutableNamespacePrefix` no longer covers it, so `validate` reports the
config clean and `ls` shows no mark at all. Following the tool's own advice converted a
caught error into TASK-167's silent one. This is precisely what `ConflictAdvice`'s own doc
comment warns against: *"advice that names a command which refuses is worse than no advice
— the reader trusts it and stops."*

Fixed by `config.RenameSuggestion`, one function replacing every colon, called from both
sites — the two copies of that expression were how they drifted. Verified end to end:
`app:sub:cmd` → advised `app-sub-cmd` → renamed → `dva app-sub-cmd` prints `deep`, rc 0.

Broadening finding 1 then exposed a defect in the fix itself: the child row inherited the
rename and advised `rename to 'app-build fast'`, a key containing a space that appears
nowhere in the file. The suggestion now resolves through `cmd.Path[0]`, so both rows point
at the single declared key the author can actually edit.

**3. The "both invocation forms" test proved only one form.**
`TestUnroutableKeyFailsBothInvocationForms` called `runSubprojectCommand(c, nil, "app",
"build", nil)` — with the two halves hardcoded. It never reached `run.go:33`, so it would
have passed unchanged no matter what the split does. That matters beyond this task: both
this file and TASK-167 cite the test as the tripwire that catches TASK-167's "route it"
option, and that option edits exactly the split being bypassed. The citation was worthless
as written. The test now writes a fixture to a temp dir, `t.Chdir`s into it, and calls
`runCmd.RunE(runCmd, []string{"app:build"})` with the key unsplit.

One thing that fell out of writing it: `version:` in `dva.yml` declares the *minimum dva
version*, not a schema version, and a failed load calls `os.Exit(1)` inside
`mustLoadConfig` — which takes the whole test binary down with no output. Noted in the
fixture comment for the next person.

**Also done:** `USAGE.md` documented the sibling `shadowed_by_builtin` field but not
`unroutable` / `unroutable_reason`; it now covers both, the subcommand case, and the
all-colons rename rule.

**Not done here:** the reviewer's Low finding — a leading-colon key (`:build`) is permitted
by the schema, is unreachable, and is covered by neither the mark nor the validator, because
`UnroutableNamespacePrefix` guards with `idx <= 0`. It is the same family as TASK-167 rather
than a regression from this change, and its error text differs (the empty prefix mangles
`cmdName` instead of producing a subproject lookup), so it went into TASK-167's fixture
table rather than widening this task.

All four original tests still pass, plus three new ones. Both new tests were falsified
against the pre-fix code:

```
--- FAIL: TestSubcommandsOfAnUnroutableKeyAreMarkedToo
    subcommand unroutable = "", want "app" — the prefix is dead for the child too
    usage_example = "dva app:build fast" for a subcommand nothing reaches
--- FAIL: TestRenameSuggestionLeavesNoColon
    RenameSuggestion("app:sub:cmd") = "app-sub:cmd", which run.go still splits
    RenameSuggestion("a:b:c:d") = "a-b:c:d", which run.go still splits
```
