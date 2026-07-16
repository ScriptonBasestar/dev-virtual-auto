---
id: TASK-040
title: "--force on 'up'/'stack up' is documented, parsed, threaded through two structs, and read by nobody"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-07-17T05:50:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-030 follow-up (help surfaces vs actual behavior)
source-severity: LOW
---

# Task 040: A Flag That Travels Two Structs To Reach Nobody

## Summary

`dva up --force` advertises "Force restart even if already running". The flag is parsed, stored in
`UpOptions.Force`, copied into `PluginContext.Force` — and **no plugin ever reads it**. It is a
documented no-op.

Its neighbour `--no-wait` takes the identical path and *is* read. That symmetry is what makes this a
bug rather than an unused struct field.

## Evidence

`--force` is written at 5 sites and read at **0**:

```
$ grep -rn "\.Force\b\|Force:" --include="*.go" internal/ | grep -v _test.go
internal/lifecycle/orchestrator.go:109:   Force:           opts.Force,     # UpOptions -> PluginContext
internal/cli/compose.go:140:              Force:       force,             # up
internal/cli/compose.go:362:              Force:       true,              # restart, hardcoded
internal/cli/plan_lifecycle.go:156:       Force:  flags.force,            # plan path
internal/cli/stack.go:91:                 Force:       force,             # stack up
```

Every one is an assignment. Field definitions: `internal/lifecycle/plugin.go:26`
(`PluginContext.Force`) and `internal/lifecycle/orchestrator.go:20` (`UpOptions.Force`). A repo-wide
search for any *read* — `pctx.Force`, or `.Force` in any position other than a struct literal key,
across all Go files including tests — returns **nothing**.

A field that is never read cannot have an effect. This is the same by-construction argument as
TASK-035's unconditional `interpolateEnvVars`, and it needs no probe to be conclusive.

### Control 1 — the grep shape finds real readers

The identical search finds `Volumes` genuinely consumed:

```
internal/lifecycle/compose.go:68:   if pctx.Volumes {
```

So the zero for `Force` is a real absence, not a broken search.

### Control 2 — the decisive one: `--no-wait` takes the same path and works

`--force` and `--no-wait` are siblings in every respect. Both are hand-documented in the same help
block (`compose.go:69-70`), both hand-parsed in the same `switch` (`compose.go:103-106`), both
threaded through `UpOptions` → `PluginContext` in the same struct literal. Only one is read:

```go
// internal/lifecycle/compose.go:30 — Wait IS consumed
if !pctx.Wait {
    // Remove --wait for immediate return
```

`Wait` is also read at `orchestrator.go:134` (health checks) and `:453`. `Force`: nowhere. Two flags,
one path, one arrives. That rules out "plugins simply don't take options" as an explanation.

### Control 3 — don't confuse it with `clean --force`, which works

`internal/cli/compose.go:432` does read a `force`:

```go
if !force && (volumes || images) {     // cleanCmd — "Skip confirmation prompt"
```

That is a **different flag** on a different command, registered properly via
`cleanCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")` (`compose.go:632`), and it
functions. This task makes no claim about `dva clean --force`. Recorded because the name collision
makes a careless grep report `--force` as "consumed" when the consumer is another command entirely.

### Empirical (supporting, not decisive)

Against a HEAD binary with a script-runner config, `dva validate` exiting 0 first:

```
$ dva stack up            \
$ dva stack up --force    /  -> byte-identical output
```

Recorded honestly as weak: a script runner would not act on `--force` even if it were implemented.
The zero-readers proof is what carries this finding; the probe merely fails to contradict it.

## Why it is not zero-impact

The help text makes a specific promise — "Force restart even if already running". A user whose
container is already up runs `dva up --force`, gets exit 0, and believes it was recreated. Nothing
was. The flag is the natural thing to reach for when a stale container needs replacing, which is
exactly when silently doing nothing is most misleading.

`restart` hardcodes `Force: true` (`compose.go:362`), so `dva restart` does not force anything
either, despite that being the plausible intent of the hardcode.

This is TASK-030's mirror image and belongs to it: there, help *understated* the binary (working
flags were undocumented); here, help *overstates* it (a documented flag does nothing).

## Severity: LOW / P3

Nothing is mutated or destroyed; the flag's absence means DVA does *less*, not something harmful —
the harmless direction. It is a documentation-vs-behavior gap, not a correctness failure of the
stack itself. P3 on its own evidence, filed because it is a live instance of the run's organizing
theme (a green surface that certifies nothing).

## Scope note — needs a decision

Same honor-vs-remove shape as TASK-035/036/037, and not the implementer's call:

- **Honor it** — plugins act on `pctx.Force`. For compose that plausibly means adding
  `--force-recreate` to the up options, the way `pctx.Wait` already toggles `--wait` at
  `compose.go:30` (a directly adjacent precedent, so this is a small, well-shaped change for the
  compose plugin). What `--force` should mean for `script`/`process`/`helm`/`kubectl` is genuinely
  unclear and must be answered per plugin, or scoped explicitly to compose.
- **Remove it** — drop `--force` from the help block, `UpOptions`, `PluginContext`, and the five
  assignment sites. Cheap and honest. Also decide what `restart`'s hardcoded `Force: true` becomes.

Lean **honor for compose only, and say so in the help text**, weakly: `--force-recreate` is a real
need the flag's wording already describes, and `pctx.Wait` shows exactly how. But a flag documented
generically on `up` while working for only one plugin is its own trap, so the wording matters as
much as the code here.

## Completion Criteria

- [ ] DECISION recorded: honor `--force` (and for which plugins) or remove it | verify: `human — maintainer picks one and records why`
- [ ] No flag remains that `--help` advertises and no code reads | verify: `human — for --force, either a probe shows it changing real behavior, or it is gone from the help block AND from UpOptions/PluginContext`
- [ ] If HONOR: a probe shows `--force` changing observable behavior against a real backend, with a control showing the default does NOT | verify: `human — requires a compose-backed probe (docker); assert the recreate happens with the flag and not without it`
- [ ] If HONOR: the help text says which plugins honor it, if not all | verify: `human — read 'dva up --help'; a generic promise that only compose keeps is not acceptable`
- [ ] If HONOR: a regression test asserts Force reaches the plugin, proven to fail without the fix | verify: `human — revert the plugin read, confirm the new test FAILS, restore, confirm it passes`
- [ ] `restart`'s hardcoded `Force: true` (`compose.go:362`) is resolved consistently with the decision | verify: `human — assert it is either meaningful or removed, not left as decoration`
- [ ] `--no-wait` still works — the control must not regress | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/lifecycle/`
- [ ] `dva clean --force` is untouched | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/cli/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [030-help-surfaces-understate-working-flags.md](../_archive/030-help-surfaces-understate-working-flags.md) — the mirror image: help understating the binary. This finding surfaced while verifying that task's outcome
- [035-env-file-interpolate-and-priority-ignored.md](./035-env-file-interpolate-and-priority-ignored.md) — same class, same honor-vs-remove decision shape
- [036-service-related-and-hint-ignored.md](./036-service-related-and-hint-ignored.md) — same class
- [039-plan-entry-runner-resolved-then-discarded.md](./039-plan-entry-runner-resolved-then-discarded.md) — same class, but the dropped value is validated first
