---
id: TASK-124
title: "`ResolutionTrace` is built on every plan run, printed nowhere, and one of its lines states the opposite of what DVA does"
type: fix
priority: P3
effort: M
status: done
resolved-at: 2026-08-02T00:00:00+09:00
resolution: "Deleted the false env_file TODO, rewrote the trace to report all 7 documented precedence layers including absent ones, surfaced it on the existing --dry-run for up/down/stop/restart; 13 tests, both directions mutation-tested"
created-at: 2026-08-01T00:00:00+09:00
scope: "internal/lifecycle/resolver.go ResolvePlan — 17 append sites with no reader; internal/cli/plan_lifecycle.go — the --dry-run path of up/down/stop/restart"
---

# Task 124: the trace that was written for nobody

`ExecutionPlan.ResolutionTrace` is in the design document
([docs/31 §9](../../docs/31-execution-plan-resolution.md#9-executionplan-초안)) and `ResolvePlan`
faithfully appends to it on every plan resolution. Nothing ever read it.

That is two defects, and the second is the one that matters.

## Problem

**Dead.** `grep -rn ResolutionTrace internal/ --include="*.go"` returned only the struct field, its
allocation, and 17 `append` calls — all writers, no readers. Every `dva up <plan>` built a
ten-element string slice and dropped it.

**False.** Because nothing read it, one line was free to be wrong, and was:

```go
// TODO: Add dedicated config.LoadEnvFileVars helper and merge here directly.
if cfg.EnvFile != nil {
	resolved.ResolutionTrace = append(resolved.ResolutionTrace, "vars: env_file merge skipped (TODO)")
}
```

Read on its own, this says DVA does not apply `env_file` on the plan path — contradicting
[docs/31 §4-3](../../docs/31-execution-plan-resolution.md#4-3-vars-병합),
[docs/41 §10-1](../../docs/41-execution-plans-and-cli.md#10-1-vars-우선순위) and USAGE.md, all of
which put `env_file` at the bottom of a seven-layer chain. Either the code was broken or the docs
were.

A second, quieter falsehood: `"vars: merged global vars"` was appended unconditionally, whether or
not `cfg.Vars` had a single key. The trace claimed work it had not done.

## Measured

Neither doc nor code was trusted; the built binary was asked. Fixture with `.env` (`FROM_ENV_FILE=1`,
`SHARED=env_file`), global `vars.SHARED=global`, and a `script` entry echoing both:

```
$ dva up p1
FROM_ENV_FILE=1        # env_file IS applied on the plan path
SHARED=global          # and global vars correctly beat it

$ SHARED=os dva up p1
SHARED=os              # OS still wins over every layer
```

So the docs were right and the code was right. **Only the trace string was wrong** — a false
statement about working behaviour, which is exactly the defect class TASK-074/076/079/111/112/116/118
have each closed one instance of.

`env_file` reaches the plan through `loadEnv` ([internal/cli/root.go](../../internal/cli/root.go),
`vars:` → `environment:` → `env_file:`), and `plan.EnvVars` is merged on top of that via
`e.MergeVars(plan.EnvVars)`. Two stages compose into the one documented order; `ResolvePlan` only
ever owned layers 3-6 and was apologising for not owning layers it does not need.

## Decision: delete the TODO rather than implement it

The TODO proposed a `config.LoadEnvFileVars` helper called from `ResolvePlan`. Rejected, with
reasons rather than taste:

- `loadEnv` is invoked from 20+ CLI sites and the legacy non-plan path needs `env_file` regardless,
  so the helper would not replace anything — it would duplicate it.
- Every plan invocation would then read the same files **twice**, and the second read could disagree
  with the first if the file changed between them.
- `ResolvePlan` is otherwise pure: config in, plan out. Adding file I/O to it to restate a merge that
  already happened upstream buys nothing.

The correct fix for "the trace does not mention env_file" is for the trace to mention env_file
truthfully, not for the resolver to redo the work. That reasoning now lives in the
`appendUpstreamVarTrace` doc comment so it is not re-derived into a re-opened TODO.

## Surface: fold into `--dry-run`, do not add `--explain`

`--dry-run` already means *tell me what would happen instead of doing it*. The resolution is that
answer. A new `--explain` would be a second flag for the same question — and TASK-082/123 settled
that this repo prefers fewer surfaces that each cover more. Off the dry-run path the trace stays
silent: ten extra lines on every `dva up` is noise, not diagnostics.

It writes to **stderr**, for the reason TASK-116 established: `--json` output must stay parseable.

## Absent layers are reported, not skipped

Every layer prints whether or not it contributed, because *"why is my variable not set"* is answered
by the layer that is missing, not the one that is present. Each layer phrases its own empty case —
a section is `not declared`, a selected profile `declares no vars`, and a command-line flag is
`none passed`, because a flag is never "declared".

`env_file` says **declared**, not applied: `config.LoadEnvFile` silently skips an optional file that
does not exist, so naming the declared set is the strongest claim this code can make without opening
the files itself. Writing "applied" here would have reintroduced the same defect one line over.

## Acceptance criteria

- [x] No trace line claims `env_file` is skipped | verify: `grep -c 'skipped (TODO)' internal/lifecycle/resolver.go` — must be 0 with a non-zero total trace-site count printed beside it via `grep -cE 'resolved\.trace|resolved\.traceLayer|p\.trace\(' internal/lifecycle/resolver.go` — **0 skip claims, 18 trace sites**
- [x] Trace has a reader | verify: `grep -rn 'ResolutionTrace' internal/ --include="*.go" | grep -v _test` — **5 sites: 3 in resolver (field/alloc/append), 2 readers in `plan_lifecycle.go:200,204`**
- [x] All 7 documented layers appear, in merge order | verify: `go test ./internal/lifecycle/ -run 'ResolutionTraceReportsEveryPrecedenceLayer|ResolutionTraceOrderMatchesMergeOrder' -v | grep -c '^--- PASS'` — **2**
- [x] Absent layers stated, not omitted | verify: `go test ./internal/lifecycle/ -run ResolutionTraceReportsAbsentLayers -v | grep -c '^--- PASS'` — **1**
- [x] Reaches every plan verb that takes `--dry-run` | verify: `go test ./internal/cli/ -run 'Plan(Up|Down|Stop|Restart)DryRunPrintsResolution' -v | grep -c '^--- PASS'` — **4, one per verb**
- [x] `--json` stdout stays parseable | verify: `go test ./internal/cli/ -run PlanResolutionGoesToStderr -v | grep -c '^--- PASS'` — **1; asserts stdout begins `{`**
- [x] Silent off the dry-run path | verify: `go test ./internal/cli/ -run PlanUpWithoutDryRunStaysQuiet -v | grep -c '^--- PASS'` — **1**
- [x] Tests fail when the fix is reverted | verify: `human — restore the TODO string, drop the print call, then make it unconditional` — **3 mutations, all caught; counts in Resolution**
- [x] `--dry-run` documented where it is accepted | verify: `grep -c 'Print the variable resolution and the actions, without executing' internal/cli/compose.go` — **4 plan-usage blocks (`compose.go:83,306,364,426` = up/down/stop/restart); `up` accepted the flag with no help line at all before this**
- [x] Full suite passes | verify: `make test` — exit 0, 5 packages ok, cli 64.8%→65.4%, lifecycle 61.6%→61.9%
- [x] Lint clean | verify: `make lint` — `0 issues.`, gofmt 235 files 0 unformatted
- [x] Docs consistent | verify: `make doc-check` — `links_checked: 428, broken_links: 0, oversized_docs: 0`

## Resolution

### What the trace says now

```
Resolution:
  plan: resolved "full"
  vars: env_file — declared [.env, .env.local], applied at config load below every layer here
  vars: environment: — applied at config load, below every layer here (1 key)
  vars: global vars — merged (2 keys)
  vars: environments."dev" — merged (1 key)
  vars: sites."local".vars — merged (1 key)
  vars: plans."full".vars — merged (1 key)
  vars: cli --var — merged (1 key)
  vars: OS environment overrides every layer above
  entry: probe -> runner=native order=10 deps=0
  waves: calculated and entries sorted
```

The last vars line is not decoration. It is the reason the chain cannot be read as "last writer
wins": `Environment.MergeVars` gives any key already in the OS environment priority over everything
above it, so a user staring at `--var FOO=x` having no effect gets the answer in the output.

### Mutation-tested, both directions

A criterion only counts if it fails when the fix is reverted (the standard TASK-116 set):

| Mutation | Failing tests |
|---|---|
| Restore `"vars: env_file merge skipped (TODO)"` | 5 of 7 lifecycle trace tests |
| Delete the 4 `printPlanResolution(plan)` calls | 5 of 6 cli tests |
| Print unconditionally (drop the `effectiveDryRun` guard) | `PlanUpWithoutDryRunStaysQuiet` |

The third matters as much as the first two: without it, "surface the trace" could be satisfied by
dumping it on every run, which is the failure mode that made `--explain` unattractive.

### 13 tests, the trace's first contract

- `internal/lifecycle/resolver_trace_test.go` (7) — layer coverage, merge order, the anti-regression
  pair forbidding `skip`/`TODO`, declared-vs-applied wording, absent layers, flag-vs-section phrasing,
  key counts.
- `internal/cli/plan_resolution_test.go` (6) — the four verbs, the silence off dry-run, and the
  stream split. Needed its own `captureStreams` helper: the shared `captureOutput` merges stdout and
  stderr, which would have made the stream assertion vacuous.

### Also corrected in passing

`"vars: merged global vars"` fired even with zero global vars; `"lookup: resolved plan"` did not name
the plan; `"entries: empty"` did not say whose. All three now state what actually happened.

### Docs

[USAGE.md](../../USAGE.md#실제-적용-결과-확인) gained the worked example;
[docs/31 §4-3](../../docs/31-execution-plan-resolution.md#4-3-vars-병합) now explains that layers 1-2
come from `loadEnv` rather than `ResolvePlan`, which is what made the original TODO look like a bug;
[docs/41 §10-1](../../docs/41-execution-plans-and-cli.md#10-1-vars-우선순위) points at the flag.

## Related

- [TASK-118](118-a-health-check-that-never-passes-is-still-exit-0.md) — nearest sibling: a gate that
  reported success it had not verified. Here, a trace that reported a skip that never happened.
- [TASK-116](116-stack-override-warning-goes-to-stdout.md) — source of the stderr rule this feature
  had to obey from its first line rather than be corrected into later.
- [TASK-082](../decision/082-the-dogfood-loop-cannot-score-an-absent-section.md) and
  [TASK-123](../decision/123-dogfood-loop-cannot-score-a-reserved-name-collision.md) — where "an
  absence is information" and "fewer surfaces, each covering more" were settled. Both are decisions
  about the dogfood loop; this task is the first time they were applied to a runtime output path.
