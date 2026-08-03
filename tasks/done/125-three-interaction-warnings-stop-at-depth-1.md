---
id: TASK-125
title: "Three interaction-tree warnings walk only the first level of a schema that is recursive by construction"
type: fix
priority: P2
effort: M
status: done
resolved-at: 2026-08-02T00:00:00+09:00
resolution: "Added a shared eachInteractionNode walker, made the three checks recurse and sort; depth-2 coverage went 0→3 on the reproduction fixture with no new warnings on any shipped example"
created-at: 2026-08-02T00:00:00+09:00
scope: "internal/config/validate_warnings.go — warnDuplicateParentSubcommand:303, warnChildOverridesParentCritical:660, warnUnreachableCommands:722 (pre-change line numbers; after the fix they sit at 332/690/753 alongside eachInteractionNode:309)"
---

# Task 125: the same mistake, one level down, is invisible

`interaction.*.subcommands` is `map[string]*InteractionCommand` — recursive by construction, no
depth limit. The runner honours that: `internal/runner/interaction_tree.go` `expandInto()` recurses
to unbounded depth, and `examples/full-stack.yml:157-179` already ships three levels
(`dva rails db migrate`).

Three of the four semantic warnings that walk that tree stopped at the first level.

## Measured

Two fixtures with the *same three mistakes*, differing only in depth. Real `dva config validate`
output, not a code reading:

```
$ (cd flat/ && dva config validate)          # depth 1
[warn] semantic: interaction.rails.subcommands.console: command "bundle exec rails" is identical to parent…
[warn] semantic: interaction.rails.subcommands.console: overrides parent runner (local → docker)…
[warn] semantic: interaction.grp: has subcommands but is not directly callable…
warn count: 3

$ (cd nest/ && dva config validate)          # depth 2, identical mistakes
✅ dva.yml is valid
warn count: 0
```

Three checks, three silent failures. `warnDuplicateParentSubcommand` carried a TODO admitting it
(`// TODO: consider checking nested subcommands recursively`); the other two did not admit anything.

## The standard was already in the file

`warnInertProvisionSteps` walks the same tree and *does* recurse, and its comment states the
argument this task applies to the other three verbatim:

> Recursive, because hooks nest: `interaction.db.subcommands.migrate.before` is as real a place to
> write an inert step as the top level, and a check that stopped at depth 1 would report the shallow
> mistake and stay silent on the identical deep one.

So this is not a new convention being introduced — it is three functions out of step with one the
repo already settled. `warnDeepSubcommandNesting` recurses too, via `calculateSubcommandDepth`.

Full survey of everything touching `.Subcommands` in the package:

| Function | Before |
|---|---|
| `warnInertProvisionSteps` | recursive ✓ |
| `warnDeepSubcommandNesting` | recursive ✓ (`calculateSubcommandDepth`) |
| `warnDuplicateParentSubcommand` | depth 1 only |
| `warnChildOverridesParentCritical` | depth 1 only |
| `warnUnreachableCommands` | depth 1 only |

## The second defect, which the first was hiding

Both the interaction map and each node's `Subcommands` are maps, and none of the three sorted.
With one warning per check that never showed; with recursion raising the per-check count, it does:

```
$ for i in $(seq 20); do dva config validate | grep '\[warn\] semantic' | tr '\n' '|'; done | sort -u | wc -l
3          # three different orderings of the same three warnings, same input
```

That is the defect [TASK-107](../_archive/107-command-suggestions-come-out-in-a-different-order-every-run.md) closed
for command suggestions, still live here. It is fixed in the same change rather than filed separately
because the recursion is what makes it bite — shipping the recursion alone would have made output
*less* stable than before.

## Acceptance criteria

- [x] Nested mistakes are reported | verify: `go test ./internal/config/ -run InteractionWarningsRecurseIntoNestedSubcommands -v | grep -c '^--- PASS'` — **1, covering all three checks**
- [x] Warnings name the full YAML path | verify: `human — nested fixture prints interaction.rails.subcommands.db.subcommands.migrate, not just interaction.rails` — **measured; a top-level-only path is unactionable**
- [x] Depth-1 wording is byte-identical | verify: `go test ./internal/config/ -run 'InteractionWarningsDepth1WordingIsUnchanged' -v | grep -c '^--- PASS'` — **1, asserting all three full strings by equality**
- [x] Pre-existing depth-1 tests still pass unmodified | verify: `go test ./internal/config/ -run 'WarnDuplicateParentSubcommand|WarnChildOverridesParentCritical|WarnUnreachableCommands' -v | grep -c '^--- PASS'` — **3, source untouched**
- [x] Output order is stable | verify: `go test ./internal/config/ -run InteractionWarningsAreOrderStable -v | grep -c '^--- PASS'` — **1; 50 in-process repeats plus 20 real binary runs, 1 distinct ordering**
- [ ] No new warnings on shipped content | verify: `human — run config validate over examples/*.yml and dva.yml, count warnings matching 'subcommands\..*\.subcommands\.'` — **17 files swept, 20 semantic warnings emitted, 0 of them nested-path, including the depth-3 full-stack.yml. The 20 is quoted so the 0 reads as a measurement rather than a sweep that never ran.**
  - ⚠️ **This criterion failed and reported a pass.** The pattern requires *two* `subcommands.`
    segments, but the regression this commit actually shipped was
    `interaction.rails.subcommands.db` — one segment, on `examples/full-stack.yml`. The grep
    structurally could not match the thing it was written to catch, so the honest count was 1,
    not 0. Quoting the 20 guarded against a sweep that never ran; it did nothing about a sweep
    that ran the wrong query. Found and fixed in
    [TASK-128](128-the-recursion-was-right-the-nodes-it-walked-were-not.md).
- [x] No check iterates the interaction tree top-level-only | verify: `awk '/^func /{fn=$0} /range c\.Interaction/{print NR": "fn}' internal/config/validate_warnings.go` — **2 remain, both with their own recursion (`warnInertProvisionSteps`, `warnDeepSubcommandNesting`); the three fixed checks now reach the tree only through `eachInteractionNode`, 3 call sites**
- [x] Tests fail when reverted | verify: `human — de-recurse the walker; drop the sort` — **2 mutations, both caught; see below**
- [x] Full suite passes | verify: `make test` — exit 0
- [x] Lint clean | verify: `make lint` — `0 issues.`
- [x] Docs consistent | verify: `make doc-check` — `broken_links: 0`

## Resolution

A shared `eachInteractionNode(interaction, visit)` walker visits every node with its dotted config
path, so a warning can name the exact YAML location. The three checks call it; each sorts.

Message formats are unchanged at depth 1 — deliberately, so the three pre-existing tests keep working
as regression detectors rather than being rewritten alongside the code they guard.

### Mutation-tested

| Mutation | Result |
|---|---|
| Walker stops recursing | `RecurseIntoNestedSubcommands` fails for all 3 checks, 0 warnings each |
| `sort.Strings` removed from one check | `AreOrderStable` fails at repeat 3 of 50 |
| Walker drops the `interaction.` path prefix | `Depth1WordingIsUnchanged` fails for all 3 checks |

### Coverage was consistent with the gap

`TestWarnDuplicateParentSubcommand` and its two siblings only ever built depth-1 configs. The tests
did not miss a case the code handled; code and tests shared one assumption, which is why the TODO
sat unchallenged. The new tests state the depth contract explicitly.

They also assert less than their names suggest: each checks a *phrase* and a count
(`strings.Contains(w, "identical to parent")`), never the config path. Running the third mutation
above proves it — with the `interaction.` prefix stripped, `TestWarnDuplicateParentSubcommand` and
`TestWarnChildOverridesParentCritical` both still passed. So "depth-1 output is unchanged" could not
have rested on them; `TestInteractionWarningsDepth1WordingIsUnchanged` supplies the equality check
they never made, and they stay as-is so this change does not rewrite the tests that guard it.

### Why the walker needs no cycle guard

> **The conclusion below holds; its evidence does not.**
> [TASK-131](131-a-cyclic-anchor-kills-dva-before-any-check-runs.md) measured all three
> claims in this section: it is a `fatal error`, not a panic; the trace has **18**
> `internal/config` frames, one of which — `(*InteractionCommand).UnmarshalYAML` — is what drives
> the recursion; and it is not upstream, because the same document decoded into a struct without a
> custom unmarshaler returns a clean `anchor 'loop' value contains itself` on the same yaml.v3
> version. The walker really is safe today, for the reason stated in the last sentence rather than
> the ones stated first. Kept as written — the wrong evidence is why the crash was filed as
> upstream and left alone.

`eachInteractionNode` recurses without a depth limit, which is only safe if the tree cannot
contain a cycle. It cannot: the sole way to express one in YAML is a self-referencing anchor
(`loop: &loop { subcommands: { self: *loop } }`), and `yaml.v3` dies decoding it — the panic
trace is entirely `yaml.v3` decode frames (`getStructInfo`, `mappingStruct`, `unmarshal`), with
no `internal/config` frame, so the decoder never hands validation a cyclic `Config` to walk.
Measured against a binary that does contain the new walker, confirming the crash is a decode-time
property and not something this change introduced. `calculateSubcommandDepth` has recursed
unguarded on the same tree since before this task for the same reason.

That yaml.v3 stack-overflows instead of returning an error is a real robustness gap, but it is
upstream of every check in this file and predates this change; it is not folded in here.

### Observed but deliberately not fixed

> **Both items below are closed as of [TASK-128](128-the-recursion-was-right-the-nodes-it-walked-were-not.md)**
> (`86539b0`). They are kept as written rather than deleted, because the reasoning for deferring
> them is what that task had to overturn. Read them as history, not as open work — and note that
> the first one's "maintenance trap" is exactly what let the same inheritance bug reach two path
> conventions.

`warnInertProvisionSteps` keeps its own walker rather than adopting `eachInteractionNode`, because
the two build paths differently: it joins with `path+"."+subName`, producing
`interaction.db.migrate.steps[0]`, while the shared walker joins with `path+".subcommands."+subName`
to match the three checks' existing depth-1 format and the actual YAML location. Its form is the
less addressable of the two — a reader cannot paste `interaction.db.migrate` into a config — but
unifying them changes a user-visible message, which belongs in a task about that message, not in
one whose whole premise is that depth-1 output stays byte-identical. Two walkers with two path
conventions is a maintenance trap; it is named here so the next change to either one sees it.

`warnHealthCheckRedundancy` (`validate_warnings.go:276`) ranges two maps (`c.HealthChecks`,
`c.Stack[].HealthChecks`) and does not sort either, so it carries the same nondeterminism. It is
left alone because it walks flat structures, not the interaction tree — this change neither causes
nor worsens it, and folding an unrelated surface into a depth-recursion task would make the
mutation evidence above cover less than it appears to. Recorded here so the next person finds it
already known rather than rediscovering it.

## Related

- [TASK-107](../_archive/107-command-suggestions-come-out-in-a-different-order-every-run.md) — same
  nondeterminism, different surface.
- [TASK-124](124-resolution-trace-is-built-never-printed-and-partly-false.md) — immediate
  predecessor and the same defect class: output that claims a coverage it does not have. There a
  trace line described a merge that never happened; here a clean `✅ dva.yml is valid` described a
  tree that was never fully walked.
- [TASK-118](118-a-health-check-that-never-passes-is-still-exit-0.md) — the recurring shape these
  all belong to: a check whose silence is read as a pass.
