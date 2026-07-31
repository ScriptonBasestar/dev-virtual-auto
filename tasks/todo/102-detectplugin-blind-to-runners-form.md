---
id: TASK-102
title: "Stack entries reached by name are never runner-resolved, so the `runners:` form loses its plugin type — `dva ktl` silently drops the declared namespace"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/config/lifecycle_helpers.go:109,211,225 — resolveRunnerPlugin runs only inside SortedStack, and only on a copy"
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

**Option B — resolve once at load.** `ResolvePluginFromName()` (`lifecycle.go:654`) is already
described as "the only hook both load paths (`Config.Load` and `Config.Merge`) run per entry
once Name is known". Calling `resolveRunnerPlugin()` there makes `c.Stack` hold resolved
entries, so every access path agrees by construction and `SortedStack`'s call becomes redundant
(harmless — it is guarded by `if e.Plugin != ""`). Check ordering against
`rejectLegacyComposeShape()`, which runs in the same hook and reasons about `e.Compose` being
nil in the modern form — backfilling before it runs would change what it sees.

Alternatives considered and why they are worse:

- **A — resolve inside `FindStackEntry`**: it returns the shared `*LifecycleEntry` from the map,
  so this mutates shared state lazily on read. Idempotent and backfill-only, so probably benign,
  but it makes "has this entry been resolved?" depend on access history.
- **C — add `KubectlConfig()` etc. mirroring `ComposeConfig()`**: fixes the measured symptoms but
  leaves `DetectPlugin()` blind, so `stack.go:299` still needs its own fix and every future
  plugin type needs another accessor.

## Acceptance criteria

- [ ] `stack log` routes by declared runner, not declaration form | verify: the two-entry fixture above — `dva stack log modernproc` must look for `.sb/dva/logs/modernproc.log`, not reach docker
- [ ] `ktl` honours a `runners.kubectl` namespace | verify: `dva ktl get pods` on the modern fixture must send `--namespace my-namespace`
- [ ] Legacy form is unchanged | verify: both legacy fixtures produce byte-identical argv to the pre-fix run — this fix must not regress the deprecated shape
- [ ] `DetectPlugin()` non-empty for a runners-form entry reached by name | verify: unit test on `FindStackEntry(...).DetectPlugin()`
- [ ] Tests are non-vacuous | verify: reverting the fix makes each FAIL, and the kubectl one names the missing `--namespace`
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-103](103-ktl-forwards-root-flags-to-kubectl.md) — a *separate* defect in the same
  command; 103 is about DVA's flags reaching kubectl, this is about the entry's config not
  reaching it. Fixing either does not fix the other.
- [TASK-092](../done/092-stack-log-forwards-root-flags-to-docker.md) — found while tracing
  `stack log`'s passthrough; `stack.go:299` is the branch immediately after 092's fix.
