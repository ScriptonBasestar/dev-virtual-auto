---
id: TASK-083
title: "A provision/hook `step:` without `run:` prints the step and executes nothing, while `run:` without `step:` executes but fails schema validation"
type: fix
priority: P2
effort: M
status: done
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/runner — local.go, docker_compose.go executeSteps; internal/config — schema.json provision item oneOf; validate warnings"
---

# Task 083: A step that announces work it never does

## Problem

`ProvisionItem` takes its commands from `run:` (or, for the bare-string list form, from `Raw`).
`step:` is only a label. `internal/runner/local.go:56-70` read the label, collected
`step.RunCommands()`, and when that was empty it `continue`d — silently.

So a step with a label and no `run:` printed as if it worked. Measured on 0.1.44, with
`interaction.build.replace: [{step: "make build"}]` in a directory with **no Makefile**:

```
$ dva build
[hook:replace:build] [1/1] make build
$ echo $?
0
```

Nothing ran. `make build` is the label. The `replace:` hook also suppressed the default compose
build, so `dva build` became a no-op that reported success — and the line it printed is
indistinguishable from the executing form's first line. The only signal was the *absence* of the
`$` line that the executing form prints.

### The schema disagreed with the runner, in the opposite direction

| form | `dva validate` | executes? |
| --- | --- | --- |
| `- step: "make build"` | **0** (valid) | **no** — label only |
| `- run: "make build"` | **1** — `interaction.build.replace.0: step is required` | **yes** |
| `- step: "Building"` + `run: "make build"` | 0 | yes |
| `- "make build"` (bare string) | 0 | yes |

Both single-key forms were wrong, each in the way the other was right: the schema accepted the one
that did nothing and rejected the one that worked.

Found while fixing [TASK-076](../done/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md):
USAGE.md's hook example used `- run:` with no `step:`, the only two such lines in the repo.

## Shipped: one predicate, seven call sites, and a schema that stopped enforcing nothing

`ProvisionItem.IsInert()` (`internal/config/config.go:580-608`) reports "carries no payload at
all". Every payload field counts, not just `Run` — an item with `compose_up`, `echo` or `cmd` does
something, and reporting it would be a false positive on a working config. `Raw` needs no clause:
`RunCommands()` returns it. `parallel:` is a modifier, not a payload, so `{step, parallel}` is
still inert.

`InertStepMessage` lives beside it. The wording lived nowhere before, which is why the call sites
could not even agree on whether to print anything:

> nothing ran — this item is a label with no 'run:'. Add 'run:' to execute a command, or 'note:' if it is a message.

### Where it is consulted — seven sites, not the four the first survey claimed

| site | what it printed before |
| --- | --- |
| `internal/runner/local.go:64` | nothing at all — not even the label |
| `internal/runner/docker_compose.go:72` | nothing at all |
| `internal/cli/hooks.go:102` | the label only (to stderr) |
| `internal/cli/provision.go:120` | the label only, dry-run included |
| `internal/cli/provision.go:232` | the label only, parallel batch |
| `internal/cli/compose.go:464` | **nothing** — `build --mode` with `build=native` prints no labels |
| `internal/config/validate_warnings.go:235` | n/a — new `validate` warning |

`compose.go:464` and the two `provision.go` sites were absent from the initial code map; a
repo-wide `grep -rn RunCommands --include='*.go'` (22 hits) is what surfaced them. Taking the map
on trust would have shipped a fix that left `dva build --mode` exactly as silent as before.

The `validate` warning recurses through `Subcommands`, because
`interaction.db.subcommands.migrate.before` is as real a place to write an inert step as the top
level, and a check stopping at depth 1 would report the shallow mistake and stay silent on the
identical deep one. Results are sorted — both sources are maps, and unsorted output would reorder
between runs.

### Half 2: the schema now accepts `run:`-only

Chosen direction: **relax**. Branch 2 of `definitions.provision_item.oneOf` traded
`"required": ["step"]` for an `anyOf` over the six payload keys. Two measurements drove it:

- `- run: "echo X"` and `- "echo X"` are behaviourally identical — both print the synthesised
  label `step 1` and execute. Requiring `step:` enforced nothing the bare-string form did not
  already bypass.
- **Schema validation does not run on the execution path.** A config that fails `dva validate`
  (exit 1) still executes under `dva build` (exit 0). Only `validate` reads schema.json — which is
  why the runtime notice, not the warning, is the load-bearing half of this fix.

Widening only: no config that validated before fails now. Branch 3 (legacy `echo`/`cmd`) keeps
`additionalProperties: false` + `maxProperties: 1`, so the branches stay disjoint and `oneOf`
still matches exactly one.

### Measured after the fix

| probe | `validate` | `build` | inert notices | exec markers |
| --- | --- | --- | --- | --- |
| `step:` only | 0 | 0 | **1** | 0 |
| `run:` only | 0 (**was 1**) | 0 | 0 | 2 |
| `step:` + `run:` | 0 | 0 | 0 | 2 |
| bare string | 0 | 0 | 0 | 2 |
| `step:` + `note:` | 0 | 0 | **0** — no false positive | 0 |
| three steps, middle inert | 0 | 0 | **1** | 4 |

`validate` on the multi fixture emits both
`[warn] semantic: provision.default[1] "make build": nothing ran — …` and
`[warn] semantic: interaction.build.replace[1] "make build": nothing ran — …`.

## Non-goals (held)

- `step:` still means "label". Re-pointing it at execution would silently start running label
  prose like `Installing dependencies`.
- `note:` untouched; it returns before the command lookup, and an item with a note is not inert.

## Acceptance criteria

- [x] A `step:` with no `run:` is reported, not silently skipped | verify: `human — run the fixture below; the no-op must be visible in dva's own output, not inferable only from a missing '$' line` — the `⚠` line prints under the label at all six execution sites
- [x] The report distinguishes it from a step that ran | verify: `go test ./internal/runner/ -run TestStepWithoutRunIsReported` — 9 subtests pass
- [x] `provision` and both hook runners agree | verify: `/usr/bin/grep -c 'RunCommands' internal/runner/local.go internal/runner/docker_compose.go` — 1 and 1; both take the same branch, and the test drives the same table through both
- [x] Every `examples/*.yml` still validates | verify: `for f in examples/*.yml; ...` — **16 swept, 0 failures**, with a deliberately broken control config that did fail (exit 1), proving the sweep can detect failure
- [x] The schema/runner disagreement on `run:`-only is resolved in one direction and documented | verify: `human — state which was chosen` — **relaxed**; see "Half 2" above
- [x] Not vacuous | verify: `human — break the reporting branch and confirm the new test fails` — disabling the branch in `local.go` **alone** failed the `local` subtest while `docker_compose` still passed; dropping the `Note` clause from `IsInert` failed 4 cases across both packages
- [x] Full suite passes | verify: `make test` — all packages ok under `-race`

## Reproduction fixture

```yaml
version: "0.1.44"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
interaction:
  build:
    replace:
      - step: "make build"     # no run: — now prints the ⚠ notice
```

## Left open

Found while measuring, none of it in this task's scope:

- **The two runners and `hooks.go` disagree on note-vs-run ordering.** `runner/*.go` checks `note:`
  first and `continue`s, so an item with *both* `note:` and `run:` never runs its command;
  `cli/hooks.go` executes then prints. Silent divergence, unrelated to inertness.
- **`hooks.go` writes to stderr, `provision.go` writes to stdout** for the same class of message.
- **`executeProvisionStep` prints its label only `if step.Step != ""`**, so a wholly empty item
  yields the notice with no label above it. Unreachable from YAML in practice.
- **`dva validate --json` ignores the flag** and prints prose — filed under the `--json` follow-ups.

## Related

- [TASK-076](../done/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
  — where this surfaced; the doc half is fixed there.
- [TASK-073](../done/073-version-error-blames-the-config-for-a-build-defect.md) — same class: a
  message that reads as success while the underlying action did not happen.
- [TASK-079](../done/079-json-flag-does-not-cover-failures.md) — also fixed by finding every call
  site rather than the one the task named.
