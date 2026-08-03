---
id: TASK-069
title: "`dva config migrate` applies to 0 of 31 live configs while 18 warn about migrating — validate and migrate give the user contradictory impressions"
type: fix
priority: P3
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/cli/config_migrate.go (the XS fix); a modes→plans converter would be internal/config/migrate.go and is deliberately out of scope"
verified-at: 2026-08-03T12:15:00+09:00
archived-at: 2026-08-03T12:15:00+09:00
follow-up: TASK-136
verification-summary: |
  Code matches the task's Resolution verbatim: internal/cli/config_migrate.go:60-77 prints
  the three-line hand-off message only on the true no-op path, with a comment explaining why
  the section names are hand-written rather than sourced from validate_warnings.go.
  Re-ran the full corpus sweep on the live ~/mydevbox (31 configs, bin/dva v0.1.44):
  migrate → would-migrate=0 nothing-to-do=31 other=0 (matches claim exactly).
  validate → modes=16 stack.*.order=17 no-plans=18 applications=8 env-fields=1,
  any-of-these-five=18/31 (matches the task's table exactly, matched on the literal warning
  strings at validate_warnings.go:142,153,175,197,208, not grep).
  TASK-066 cross-check: section-order warnings are independently 0/31 today (that fix holds),
  confirming TASK-066's own note that its 10-count and TASK-069's 18-count never overlapped —
  distinct literal strings, distinct checks.
---

# Task 069: Stop `config migrate` from implying a warning-heavy config has nothing to migrate

## Problem

`dva validate` tells 18 of 31 live configs to migrate. `dva config migrate` tells all 31 there
is nothing to migrate. In `~/mydevbox/matdosa-devbox`:

```
$ dva config migrate
dva.yml: no legacy compose declarations found

$ dva validate
[warn] semantic: ⚠ 'modes' section detected — consider migrating to 'plans' + 'environments' + 'sites'
[warn] semantic: ⚠ 'stack.*.order' detected — execution order should move to 'plans.*.entries[].order'
[warn] semantic: ℹ No 'plans' defined — consider adding execution plans for 'dva up <name>' support
```

Both messages are true. `migrate` says "no legacy **compose** declarations", and there are
none. But the command is called `config migrate`, and validate just said "consider migrating"
three times — so the user reasonably reads the reply as "validate is being fussy, there is
nothing to do here." The gap between the two commands is the defect; neither command is
individually wrong.

## Why migrate legitimately has nothing to do

`MigrateLegacyCompose` handles exactly one legacy shape — the compose declaration that
`LifecycleEntry.rejectLegacyComposeShape` **refuses outright** — in its three forms: a stack
entry named `compose` carrying compose keys flat, an explicit `plugin: compose`, and a nested
`compose:` sub-key (`internal/config/migrate.go:109-129`). That is a **load error**, not a
deprecation. So a config that still loads has, by definition, nothing for migrate to fix, and
measuring confirms it: **0 of 31 would migrate, 31 of 31 report nothing to do.**

Meanwhile the shapes validate warns about are deprecated-but-working, and migrate handles none
of them — nor does it claim to:

| shape | live configs | migrate handles it? |
| --- | --- | --- |
| legacy compose (flat / `plugin: compose` / nested) — a **load error** | **0** | yes, all three forms |
| `modes:` → `plans` + `environments` + `sites` | 16 | no |
| `stack.*.order` → `plans.*.entries[].order` | 17 | no |
| `applications:` → multi-runner stack entries | 8 | no |
| `environments.*.stack` / `stack_overrides` | 1 | no |
| no `plans:` block at all | 18 | no |

The only asset for the modes→plans path is
`skills/dva/assets/templates/migrate-modes-to-plans.yml`, which is a **target-shape example**,
not a transformation. So that migration is template-guided or AI-assisted, with no CLI support.

## The tool itself is not the problem

Recorded so it is not "improved" for no reason. `dva config migrate` is careful work:

- It rewrites **only** each migrated entry's line span and passes every other byte through
  untouched, precisely because yaml.v3 does not model blank lines and a whole-document
  round-trip would strip every separator (`migrate.go:20-24`). Comments ride along on the
  `yaml.Node` tree.
- It runs `VerifyMigrated` on the result **before** writing, so a file is never left in a state
  DVA cannot read (`config_migrate.go:65-70`).
- It previews by default and requires `--write` to apply.
- It copies rather than moves `tags`, because `LifecycleEntry.Tags` and
  `ComposePluginConfig.Tags` have two different live meanings and moving it would silently drop
  one (`migrate.go:11-15, 144-152`).
- After writing it says "Run 'dva validate' — issues the load failure was hiding may surface
  now", rather than implying the config is now clean.
- `--help` is accurate: one flag (`--write`) is registered and documented, the three covered
  forms are listed, and the byte-preservation claim is correctly scoped to lines outside the
  migrated entries.

Three premises this task was originally scoped around turned out to be false, and are recorded
so they are not re-investigated: migrate does **not** re-serialize the whole document; there is
**no** root-level `compose:` key in the config model (the `yaml:"compose"` at
`config.go:314` is `InteractionCommand.Compose`, and `lifecycle.go:155` is the stack entry's
legacy nested form); and `--help` does not overstate what the command does.

## Fix shape — XS

When migrate finds no legacy compose declarations, say what it checked and point at the
migrations it does not perform, instead of a bare negative. Something like:

```
dva.yml: no legacy compose declarations found (this command only converts the
compose shape DVA cannot load). Run 'dva validate' for deprecation warnings —
'modes', 'stack.*.order' and 'applications' are migrated by hand.
```

Keep it to the no-op branch (`config_migrate.go:60-63`); the success path already tells the
user to run validate. Do not print it when there was work to do.

Prefer deriving the named sections from the same source validate uses rather than
hand-listing them in a second place — this repo has been bitten repeatedly by a second copy of
a rule that nothing compiles (TASK-057, TASK-060, TASK-065,
[TASK-067](../todo/067-version-field-rule-stated-three-incompatible-ways.md)). If no such shared list
exists, a hand-written string is acceptable here because it is a hint rather than a rule, but
say so in a comment.

## Out of scope: a modes→plans converter

Tempting and much larger. `modes:` → `plans:` cannot be mechanical the way the compose
rewrite is: it has to invent plan names, decide which mode maps to an `environment` versus a
`site`, and synthesize an `entries[]` list with orders. `stack.*.order` → `plans.*.entries[].order`
is only meaningful once a plans block exists, so it depends on that judgment. This is why the
repo ships a template instead of a converter, and that looks like the right call. If it is ever
built, it belongs behind its own decision, not as an extension of a load-error repair tool.

## Non-goals

- Do not extend `MigrateLegacyCompose` to cover `modes`/`order`/`applications`.
- Do not change the byte-preservation strategy, the pre-write `VerifyMigrated` gate, or the
  `tags` copy-not-move behaviour. Each is deliberate and commented.
- Do not run `dva config migrate --write` against `~/mydevbox`. Nothing there needs it (0 of 31),
  and the user's tree is not this task's to edit.
- Do not raise the deprecation warnings to errors.

## Acceptance criteria

- [x] The no-op message names what it checked and points at `dva validate` | verify: `go test ./internal/cli/ -run TestConfigMigrateNoOpMessage`
- [x] The message does not appear when entries were migrated | verify: `go test ./internal/cli/ -run TestConfigMigrateNoOpMessage`
- [x] A config with legacy compose still migrates unchanged | verify: `go test ./internal/config/ -run TestMigrate` <!-- was -run TestMigrateLegacyCompose, a name no test ever had: the command exited 0 having run nothing. Corrected under TASK-136, which added the doccheck guard that now catches this class. -->
- [x] Full suite green | verify: `make test`
- [x] Still a no-op across the corpus | verify: `human — re-run the Evidence sweep, expect would-migrate=0, nothing-to-do=31`

## Resolution

The no-op branch now names the shape it converts and hands the user off to `dva validate`:

```
dva.yml: no legacy compose declarations found (this command only converts the
compose shape DVA cannot load). Run 'dva validate' for deprecation warnings —
'modes', 'stack.*.order' and 'applications' are migrated by hand.
```

The Fix shape preferred deriving those three section names from validate's own source.
That was **not** done, deliberately: `warnLegacyModes`/`warnLegacyStackOrder`/
`warnLegacyApplications` have no exported list of the sections they cover, and exposing
one means editing `internal/config/` for a hint string — more blast radius than an XS
message fix should carry. The string is hand-written with a comment saying it is a hint
rather than a rule, and naming what to update if validate's set changes.

Re-verified after the change, preview only, `--write` passed to nothing:
total=31 nothing-to-do=31 would-migrate=0 other=0.

## Evidence

Measured 2026-07-30 with `bin/dva`, preview mode only — `--write` was never passed to any
config in `~/mydevbox`.

```
for f in $(find . -maxdepth 2 -name dva.yml); do (cd $(dirname $f) && dva config migrate); done
→ would-migrate=0  nothing-to-do=31  other=0
```

Migration-warning counts come from DVA's own `validate` output matched on the literal strings
emitted at `internal/config/validate_warnings.go:142,153,175,197,208`, not from hand-rolled
greps of the YAML. An earlier attempt at the latter reported `stack.*.order` in 30 of 31
configs because the pattern `^ +order:` also matches the **modern** `stack.<entry>.order`
nesting; DVA's own check reports 17 because it knows which `order:` is deprecated.

The set of configs emitting any migration warning is exactly the set with no `plans:` key —
18 of 31, with the other 13 clean. No config is partially migrated, so `plans:` alone
discriminates migrated from not.
