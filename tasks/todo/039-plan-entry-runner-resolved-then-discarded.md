---
id: TASK-039
title: "plans.<p>.entries[].runner is resolved and validated, then discarded before execution"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-07-17T05:35:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (examples runnability)
source-severity: MEDIUM
---

# Task 039: The Plan Computes The Right Answer, Validates It, And Throws It Away

## Summary

A plan entry's `runner:` is read, normalized, and **validated** by `ResolvePlan` — naming an
undeclared runner is a hard error. The resolved value is then dropped: `runPlanUp` passes only
`Names:` to the orchestrator, which re-derives the plugin on its own via `DetectPlugin()`. So
`runner:` decides whether your config is *accepted*, and has no say in what actually *runs*.

When the plan's runner and the entry's default disagree, DVA silently runs the other one.

## Evidence

Measured against `bin/dva` built via `make build` from a clean worktree at HEAD (`b0a9a35`).
`dva validate` exits 0 on every probe below, so no result here is the vacuous consequence of a
config that failed to parse.

### Probe 1 — the plan's runner is ignored, silently, exit 0

```yaml
stack:
  s1:
    default_runner: script
    runners:
      script:  { up: echo RAN_SCRIPT_RUNNER }
      process: { command: echo RAN_PROCESS_RUNNER }
plans:
  p1:
    entries: [ { name: s1, runner: process, order: 10 } ]
```

```
$ dva validate   ->  EXIT=0  "✅ dva.yml is valid"
$ dva up p1      ->  EXIT=0
                     [plan: p1] environment=dev site= entries=1
                     $ echo RAN_SCRIPT_RUNNER
                     RAN_SCRIPT_RUNNER          # <-- the plan said runner: process
```

The plan asked for `process`. DVA ran `script`, reported success, and said nothing.

### Probe 2 — CONTROL: the resolver really does read `runner:`

This is the control that makes Probe 1 mean something. If `runner:` were simply an unparsed key,
this would be a different (and lesser) bug — dead schema surface, like TASK-035/036. It is not:

```yaml
plans: { p1: { entries: [ { name: s1, runner: helm, order: 10 } ] } }   # helm not declared on s1
```

```
$ dva up p1  ->  EXIT=1
                 ERROR: plan "p1": runner: entry "s1" runner "helm" is not declared in stack.runners
```

So the value is genuinely consumed, normalized, and validated against `stack.runners`
(`resolver.go`, the `runnerDeclared` check). It is then discarded. That is the whole defect.

### Probe 3 — the sharpest form: the answer was right there

Entry declares two runners and **no** `default_runner`; the plan names one explicitly:

```yaml
stack: { s1: { runners: { script: {...}, process: {...} } } }        # no default_runner
plans: { p1: { entries: [ { name: s1, runner: script, order: 10 } ] } }
```

```
$ dva validate  ->  EXIT=0
$ dva up p1     ->  EXIT=1
                    ERROR: entry "s1": unknown lifecycle plugin ""
```

The plan said `script`. The resolver confirmed `script` is declared. Execution then failed because
it did not know which runner to use. The information was computed and verified one call earlier and
thrown away in transit — this is not a missing feature, it is a dropped value.

## Root cause

`ResolvePlan` (`internal/lifecycle/resolver.go`) computes `finalRunner` per entry — plan entry's
`runner:` wins, else the sole declared runner, else `DetectPlugin()` — validates it against
`stack.runners`, loads its config via `GetRunnerConfig`, and stores it on `ResolvedEntry.Runner`.

`runPlanUp` (`internal/cli/plan_lifecycle.go`) then throws `ResolvedEntry.Runner` away:

```go
plan, err := lifecycle.ResolvePlan(c, planName, flags.cliVars)
...
orch.Up(context.Background(), lifecycle.UpOptions{
    DryRun: dryRun || flags.dryRun,
    Force:  flags.force,
    Wait:   flags.wait,
    Names:  planEntryNames(plan),   // <-- only the NAMES survive the plan
    Env:    plan.EnvironmentName,
})
```

`UpOptions` has no field capable of carrying a per-entry runner, so `Orchestrator.Up` re-resolves
from scratch at `orchestrator.go:98` with `entry.DetectPlugin()`, which reads `e.Plugin` as
backfilled by `resolveRunnerPlugin` from `default_runner` or a sole runner. The plan's decision is
structurally unable to reach execution. `runPlanDown`/`runPlanStop` pass `Names:` the same way.

This is the same shape as TASK-033 (`Restart` dropping `Names` across the Stop/Up seam): a value
correctly computed on one side of a call boundary that the boundary cannot carry.

## Blast radius, measured honestly

`grep -rn "runner:" examples/` → **84** lines. But the defect is only *observable* where the plan's
runner can disagree with what `DetectPlugin` picks — i.e. entries declaring more than one runner:

- 25 example stack entries declare `runners:`; **7** declare more than one; all 7 set `default_runner`.
- For the other 18, the plan's `runner:` must name the sole declared runner anyway, so it agrees by
  construction and the bug is invisible. Those 18 work **by coincidence**, not because `runner:` is
  honored.

All 7 multi-runner entries are `default_runner: native` (`examples/applications.yml`,
`full-stack.yml`, `service-orchestration.yml`), naming `docker`/`compose`/`kubectl` in their plans.
Stated plainly: those 7 are **already broken for a different reason** — `native` is not in
`knownPluginNames` (`internal/config/lifecycle.go`) and not in the plugin registry, so they fail
`NewPlugin` regardless of this bug. That is TASK-017/TASK-026's territory, not this task's. So this
defect's practical impact on the shipped examples today is *masked* by a larger one, and I am not
claiming those 7 as this task's damage.

The defect is proven independently of that mess by Probes 1 and 3, which use `script` and `process`
— two fully implemented, registered plugins.

## Severity: MEDIUM / P2

No infrastructure is destroyed, so not P1. But `dva up <plan>` can run a different runner than the
plan specifies, with exit 0 and no warning (Probe 1) — the wrong process starts and reports success.
Same organizing theme as the rest of this run: validate-green, runtime-wrong.

## Scope note — needs a decision

The fix direction is *not* settled by in-repo precedent the way TASK-032/033 were, so do not pick
unilaterally:

- **Honor it** — the orchestrator must accept a per-entry runner from the plan. `UpOptions` carries
  only stack-wide fields today, so this means a new carrier (e.g. `Runners map[string]string`, or
  passing `[]ResolvedEntry` instead of `Names []string`) threaded through `Up`/`Down`/`Stop`. It
  overlaps TASK-017's question of what `native`/`docker` even mean, and `ResolvedEntry` already
  carries the resolved runner *and* its config, so the plumbing exists on the producing side.
- **Remove it** — delete `runner:` from the plan-entry schema and let the stack entry decide via
  `default_runner`. Cheap and honest, but it deletes what looks like the central idea of plans
  (`plans` are documented as combining "stack entries, environment, site, and vars into an
  executable configuration"), and the migration hint `dva validate` prints to every plan-less config
  explicitly advertises `runner: <runner-name>` in its example. Removing it means changing that
  advice too.

Lean **honor**, weakly: the resolver already does the hard part, and a plan that cannot pick a
runner has trouble justifying `default_runner`'s existence alongside it. But this is a real product
question about what a plan *is*, and it should be answered before any code is written.

## Completion Criteria

- [ ] DECISION recorded: honor the plan's `runner:` at execution, or remove it from the schema | verify: `human — maintainer picks one and records why; note the TASK-017 overlap if honoring`
- [ ] If HONOR: Probe 1 runs the process runner, not the script runner | verify: `human — build Probe 1's config; assert RAN_PROCESS_RUNNER is emitted and RAN_SCRIPT_RUNNER is NOT`
- [ ] If HONOR: Probe 3 succeeds instead of failing on `unknown lifecycle plugin ""` | verify: `human — build Probe 3's config; assert dva up p1 exits 0 and runs the script runner`
- [ ] If HONOR: `down`/`stop` on the plan path honor it too — not just `up` | verify: `human — the seam is in runPlanDown/runPlanStop as well; assert both, or record explicitly why not`
- [ ] If HONOR: a config where plan runner == the only declared runner still behaves identically (the 18 coincidental cases must not regress) | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/lifecycle/`
- [ ] If REMOVE: `runner:` is gone from the plan-entry schema, a config using it fails validation naming the key, and the migration hint that advertises `runner: <runner-name>` is updated to match | verify: `human — probe a config with entries[].runner; assert non-zero exit naming it; then run dva validate on a plan-less config and confirm the printed example no longer advertises the key`
- [ ] Probe 2's control still holds — an undeclared runner is still rejected | verify: `human — assert 'runner: helm' on an entry not declaring helm still errors`
- [ ] A regression test asserts the chosen behavior and is proven to fail without the fix | verify: `human — revert the fix, confirm the new test FAILS for the right reason, restore, confirm it passes`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [017-runners-docker-native-semantics.md](./017-runners-docker-native-semantics.md) — what `native`/`docker` mean; blocks the HONOR direction and explains why the 7 multi-runner examples are already broken
- [026-shipped-examples-validate-green-runtime-red.md](./026-shipped-examples-validate-green-runtime-red.md) — the examples' runtime failures, which mask this defect's impact
- [033-restart-discards-service-names.md](../_archive/033-restart-discards-service-names.md) — same shape: a resolved value dropped at a call boundary
- [038-stack-status-silently-hides-unconstructible-entries.md](./038-stack-status-silently-hides-unconstructible-entries.md) — `dva stack status` is what hides Probe 3's broken entry
