---
id: TASK-102
title: "Stack entries reached by name are never runner-resolved, so the `runners:` form loses its plugin type — `dva ktl` silently drops the declared namespace"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/config/lifecycle_helpers.go:109,211,225 — resolveRunnerPlugin runs only inside SortedStack, and only on a copy"
verified-at: 2026-08-03T13:40:00+09:00
archived-at: 2026-08-03T13:40:00+09:00
verification-summary: |
  All six criteria re-measured against the live tree, not the task text. Half 1 is
  internal/config/lifecycle.go:749-755 — a read-only DetectPlugin fallback to
  e.runnerPluginName(), with resolvePluginFromName's guard order swapped (lifecycle.go:688-694)
  so runners entries stop retaining rawNode. Half 2 is KubectlConfig()
  (lifecycle_helpers.go:30) wired into PrimaryKubectlConfig:208,216, KubectlEntries:239,
  cli/kubectl.go:55,72. End-to-end with shell shims on PATH: `stack log modernproc` reaches
  showStackEntryLog (no docker call), `ktl get pods` on runners.kubectl emits
  `--namespace my-namespace`, both legacy controls unchanged. Non-vacuity independently
  reproduced: each half reverted in a scratchpad copy produced exactly the failures the task
  claims, including the "succeeded; expected the missing-log-file error" shape and the
  `--namespace did not survive` message. All four FindStackEntry consumers now go through a
  runners-aware path (compose.go:57 ComposeConfig, stack.go:324 DetectPlugin, stack.go:454
  existence-only, kubectl.go:55 KubectlConfig).
---

# Task 102: name-lookup paths see unresolved stack entries

## Problem

`resolveRunnerPlugin()` (`lifecycle_helpers.go:64`) is what teaches an entry declared in the
modern `default_runner:` + `runners:` form which plugin it is — it backfills `e.Plugin` and the
typed config field (`e.Kubectl`, `e.Process`, …) from the runners map.

It is called from exactly one place:

```go
// lifecycle_helpers.go:104-111  — SortedStack
for name, e := range c.Stack {
    entry := *e                     // a COPY
    entry.Name = name
    entry.resolveRunnerPlugin()     // backfills the copy, not c.Stack[name]
    entries = append(entries, entry)
}
```

So the backfill lives only on the copies `SortedStack` returns. Entries reached any other way —
above all `FindStackEntry`, which is a bare map read:

```go
// lifecycle_helpers.go:225
func (c *Config) FindStackEntry(name string) *LifecycleEntry { return c.Stack[name] }
```

— are still unresolved: `Plugin` is `""` and every typed pointer is `nil`. `DetectPlugin()`
inspects exactly those fields (`lifecycle.go:703-738`), so it returns `""`, and callers fall
through to whatever their default branch is.

## Evidence (measured on 0.1.44, commit 8f8304d)

Two fixtures, each with entries that differ **only** in declaration form. Both validate
cleanly (`✅ dva.yml is valid`), and `dva show` — which goes through `SortedStack` — reports the
correct runner for every one of them. That is the control: DVA itself agrees the modern form is
a `process`/`kubectl` entry.

### `dva stack log` misroutes to docker

```yaml
stack:
  legacyproc:  { order: 1, process: { command: sleep 1 } }
  modernproc:  { order: 2, default_runner: process, runners: { process: { command: sleep 1 } } }
```

```
$ dva show
  legacyproc  [runner:process, order:1]
  modernproc  [runner:process, order:2]      <- both resolved here

$ dva stack log legacyproc
ERROR: no log file for stack entry "legacyproc": open …/.sb/dva/logs/legacyproc.log: no such file
                                                   ^ correct: treated as a process entry

$ dva stack log modernproc
DOCKER-SHIM-GOT: compose logs modernproc
                 ^ wrong: DetectPlugin() returned "", so stack.go:300's switch matched
                   neither "process" nor "compose" and it fell through to the compose default
```

### `dva ktl` silently drops the namespace — the reason this is P2

```yaml
# legacy control                     # modern
stack:                               stack:
  legacyk8s:                           modernk8s:
    kubectl: { namespace: my-ns }        default_runner: kubectl
                                         runners: { kubectl: { namespace: my-ns } }
```

```
$ dva ktl get pods       # legacy
KUBECTL-SHIM-GOT: --namespace my-namespace get pods

$ dva ktl get pods       # modern — dva show says [runner:kubectl]
KUBECTL-SHIM-GOT: get pods
                  ^ --namespace is gone
```

`KubectlEntries()` (`lifecycle_helpers.go:211-217`) iterates the raw map testing
`e.Kubectl != nil`, finds nothing, and `ktl` takes its zero-entries fallback
(`kubectl.go:28-36`), which asks `PrimaryKubectlConfig()` — blind the same way — and also gets
nil. So kubectl runs against the **default namespace** with no error and no warning.

That is the severity: `dva ktl delete …` or `dva ktl apply …` against the wrong namespace,
silently, from a config DVA has just told the user is valid and `runner:kubectl`.

## Blast radius

The distinguishing factor is whether a helper reads the typed field directly or goes through a
runners-aware accessor. Compose already has one — `ComposeConfig()` (`lifecycle_helpers.go:5-21`)
checks `e.Compose` **and then** `e.Runners`. No other plugin type has an equivalent.

| site | reads | verdict |
|---|---|---|
| `lifecycle/orchestrator.go:99,185,240,298` | `SortedStack()` (orchestrator.go:63) | **safe** — resolved copies |
| `PrimaryComposeEntry` :124, `AllComposeFiles` :155, `ComposeEntries` :200 | `ComposeConfig()` | **safe** — accessor is runners-aware |
| `cli/compose.go:56` | `entry.ComposeConfig()` | **safe** — same accessor |
| `cli/stack.go:388` | `FindStackEntry(n) == nil` | **safe** — existence check only |
| `cli/stack.go:299` | `FindStackEntry` → `DetectPlugin()` | **BROKEN** — measured above |
| `KubectlEntries` :214, `PrimaryKubectlConfig` :183 | `e.Kubectl` directly | **BROKEN** — measured above |
| `cli/kubectl.go:46` | `FindStackEntry` → `found.Kubectl != nil` | **broken by the same mechanism**, not separately measured (needs a 2-kubectl-entry fixture to reach the multi-entry branch) |

Every non-compose, non-kubectl plugin type (`script`, `helm`, `docker`, …) has no accessor at
all, so any future name-lookup site will inherit the same blindness by default.

## Proposed fix

> **Superseded during implementation.** Option B below was the original recommendation and it is
> wrong — it would have made every valid modern compose config fail to load. Kept here because
> the reason is the useful part; see **Resolution** for what was actually done.

**~~Option B — resolve once at load.~~** `ResolvePluginFromName()` (`lifecycle.go:654`) is already
described as "the only hook both load paths (`Config.Load` and `Config.Merge`) run per entry
once Name is known". Calling `resolveRunnerPlugin()` there makes `c.Stack` hold resolved
entries, so every access path agrees by construction and `SortedStack`'s call becomes redundant
(harmless — it is guarded by `if e.Plugin != ""`). Check ordering against
`rejectLegacyComposeShape()`, which runs in the same hook and reasons about `e.Compose` being
nil in the modern form — backfilling before it runs would change what it sees.

That last sentence understated the problem. `resolveRunnerPlugin` → `applyRunnerConfig`
(`lifecycle_helpers.go:50`) assigns `e.Compose = c` for `runners.compose`, and a non-nil
`e.Compose` is not merely *seen differently* by `rejectLegacyComposeShape` — it is the **sole
signal** that identifies the deprecated shape (`lifecycle.go:661-681`). The two run in the same
function. Backfilling first therefore makes every `runners.compose` entry indistinguishable from
a legacy one, and the loader rejects it with *"compose must be declared under runners.compose"* —
advice the config was already following. Option B trades a silent misroute for a total load
failure on the most common modern config there is.

The general lesson: **backfill is not free when a field carries a second meaning.** `e.Compose`
is doing double duty as "the compose config" and "this entry used the old syntax", and any fix
that writes it destroys the second meaning.

Alternatives considered:

- **A — resolve inside `FindStackEntry`**: it returns the shared `*LifecycleEntry` from the map,
  so this mutates shared state lazily on read. Idempotent and backfill-only, so probably benign,
  but it makes "has this entry been resolved?" depend on access history. Rejected — and it shares
  Option B's `e.Compose` hazard, just on a later schedule.
- **C — add `KubectlConfig()` etc. mirroring `ComposeConfig()`**: fixes the measured symptoms but
  leaves `DetectPlugin()` blind, so `stack.go:299` still needs its own fix and every future
  plugin type needs another accessor. Adopted **in part** — see Resolution.

## Acceptance criteria

- [x] `stack log` routes by declared runner, not declaration form | verify: the two-entry fixture above — `dva stack log modernproc` must look for `.sb/dva/logs/modernproc.log`, not reach docker
- [x] `ktl` honours a `runners.kubectl` namespace | verify: `dva ktl get pods` on the modern fixture must send `--namespace my-namespace`
- [x] Legacy form is unchanged | verify: both legacy fixtures produce byte-identical argv to the pre-fix run — this fix must not regress the deprecated shape
- [x] `DetectPlugin()` non-empty for a runners-form entry reached by name | verify: unit test on `FindStackEntry(...).DetectPlugin()`
- [x] Tests are non-vacuous | verify: reverting the fix makes each FAIL, and the kubectl one names the missing `--namespace`
- [x] Full suite passes | verify: `make test`

## Resolution

Fixed read-only, in two independent halves. Neither writes to the shared `*LifecycleEntry` in
`c.Stack`, which is what makes it safe where Option B was not.

**1. `DetectPlugin()` falls back to the runners map** (`lifecycle.go`). After the existing switch
over the typed pointers finds nothing, it consults `e.runnerPluginName()` and returns that,
mapping `native` → `process` exactly as `applyRunnerConfig` does. This is a *derivation*, not a
backfill: `e.Compose` stays nil for modern entries, so `rejectLegacyComposeShape` keeps working.

Doing so required reordering `resolvePluginFromName`: its first guard is
`if e.Plugin != "" || e.DetectPlugin() != "" || e.rawNode == nil { return nil }`, and now that
`DetectPlugin()` answers for runners entries, they exited there with `rawNode` — a parsed YAML
node — retained for the lifetime of the config. The `len(e.Runners) > 0` check now runs first and
clears it. Same return value, no retention.

**2. `KubectlConfig()` accessor** (`lifecycle_helpers.go`), mirroring `ComposeConfig()`, wired
into `PrimaryKubectlConfig`, `KubectlEntries`, and both `cli/kubectl.go` sites. Alternative C in
full for kubectl only — the other plugin types have no name-lookup consumer today, and adding
fourteen unused accessors is the over-engineering this repo's rules forbid. Half 1 means a future
one starts from a correct `DetectPlugin()` rather than from nothing.

### Measured after (same fixtures as Evidence)

```
stack log legacyproc  -> ERROR: no log file … legacyproc.log                  (control, unchanged)
stack log modernproc  -> ERROR: no log file … modernproc.log                  (was: DOCKER-SHIM-GOT compose logs modernproc)
ktl get pods (modern) -> KUBECTL-SHIM-GOT: --namespace my-namespace get pods  (was: get pods)
ktl get pods (legacy) -> KUBECTL-SHIM-GOT: --namespace my-namespace get pods  (control, unchanged)
```

### Tests

- `internal/config/runners_form_detection_test.go` (new) — 4 tests. Every fixture declares the
  same plugin in both shapes so the legacy rows act as controls;
  `TestNameLookupAgreesWithSortedStack` states the invariant rather than a case list, so a shape
  added later is covered without editing the file.
- `internal/cli/stack_log_routing_test.go` (new) — the command-level symptom, with a compose
  positive control so "docker was never invoked" cannot pass vacuously.
- `internal/cli/ktl_flag_passthrough_test.go` — gained the `runners.kubectl` case TASK-103's
  "Left open" promised once this landed; `composePassthroughFixture` was generalised to
  `composePassthroughFixtureWith(t, body)` so a test can vary the stack shape.

### Non-vacuity probe

Each half reverted independently in a scratch copy of the tree:

| reverted | fails | still passes |
|---|---|---|
| half 1 (`DetectPlugin` fallback) | `…RunnersFormOnNameLookup` (modernproc/modernnative/modernk8s), `…AgreesWithSortedStack`, `TestRunnersOnlyEntryStillLoads`, `stack log modernproc` | every legacy control, and all kubectl tests |
| half 2 (accessor call sites) | `KubectlEntries finds both shapes` → `[legacyk8s]`, `ktl … runners.kubectl` → `--namespace did not survive into "get pods"` | every `DetectPlugin` test |

Under half 1 reverted, `stack log modernproc` fails as **"succeeded; expected the missing-log-file
error"** — the original defect's real danger. The docker shim exits 0, so DVA reported success
while shelling out to compose for an entry that has no container.

`make test` green under `-race`; `internal/config` 65.9% → 66.5%, `internal/cli` 62.6% → 62.7%.

## Related

- [TASK-103](103-ktl-forwards-root-flags-to-kubectl.md) — a *separate* defect in the same
  command; 103 is about DVA's flags reaching kubectl, this is about the entry's config not
  reaching it. Fixing either does not fix the other.
- [TASK-092](../_archive/092-stack-log-forwards-root-flags-to-docker.md) — found while tracing
  `stack log`'s passthrough; `stack.go:299` is the branch immediately after 092's fix.
