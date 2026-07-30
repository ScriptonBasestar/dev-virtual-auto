---
id: TASK-083
title: "A provision/hook `step:` without `run:` prints the step and executes nothing, while `run:` without `step:` executes but fails schema validation"
type: fix
priority: P2
effort: M
status: todo
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/runner — local.go, docker_compose.go executeSteps; internal/config — schema.json provision item oneOf; validate warnings"
---

# Task 083: A step that announces work it never does

## Problem

`ProvisionItem` takes its commands from `run:` (or, for the bare-string list form, from `Raw`).
`step:` is only a label. `internal/runner/local.go:56-70` reads the label, collects
`step.RunCommands()`, and when that is empty it `continue`s — silently.

So a step with a label and no `run:` prints as if it worked. Measured on 0.1.44, with
`interaction.build.replace: [{step: "make build"}]` in a directory with **no Makefile**:

```
$ dva build
[hook:replace:build] [1/1] make build
$ echo $?
0
```

Nothing ran. `make build` is the label. The `replace:` hook also suppressed the default compose
build, so `dva build` became a no-op that reported success — and the line it printed is
indistinguishable from the executing form's first line.

The compare-and-contrast, same fixture shape with `step:` + `run:`:

```
$ dva build
[hook:replace:build] [1/1] Building
  $ echo BUILT-VIA-STEP-RUN
BUILT-VIA-STEP-RUN
```

The only signal that the first one did nothing is the *absence* of the `$` line.

### The schema disagrees with the runner, in the opposite direction

| form | `dva validate` | executes? |
| --- | --- | --- |
| `- step: "make build"` | **0** (valid) | **no** — label only |
| `- run: "make build"` | **1** — `interaction.build.replace.0: step is required` | **yes** |
| `- step: "Building"` + `run: "make build"` | 0 | yes |
| `- "make build"` (bare string) | 0 | yes |

Both single-key forms are wrong, each in the way the other is right: the schema accepts the one
that does nothing and rejects the one that works. `schema.json`'s own examples (line ~944) use the
`step:`+`run:` pair, so the pair is the intended shape and the `oneOf` requiring `step:` is
deliberate — but nothing tells the author who wrote `step:` alone that their step is inert.

Found while fixing [TASK-076](../done/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md):
USAGE.md's hook example used `- run:` with no `step:`, the only two such lines in the repo. Those
two lines are already fixed to `step:`/`run:` pairs. This task is the code behind them.

## Proposed fix

Two independent halves; the first is the defect, the second is a consistency question.

1. **A step with a label and no commands is a warning, not a silent skip.** It is a config
   mistake with no legitimate reading — an author who wants to print a note has `note:`, which
   the runner already handles on the line above. Options: reject in `Validate` (breaks configs
   that currently "work"), or emit a `validate_warnings.go` warning plus a runtime line that
   makes the no-op visible (`[skipped: no run:]`). The runtime line matters most: `validate` is
   not what the author runs when the hook misbehaves.
2. **Decide whether `run:`-without-`step:` should validate.** The runner executes it, so today
   the schema is the thing refusing a working config. Either relax the `oneOf` to accept a bare
   `run:`, or keep the requirement and make the message say what to add rather than only what is
   missing.

Pick per half; they can ship separately.

## Non-goals

- Not changing what `step:` means. It is a label in every `examples/*.yml`, and re-pointing it at
  execution would silently start running label prose like `Installing dependencies`.
- Not touching `note:`, which already returns before the command lookup.

## Acceptance criteria

- [ ] A `step:` with no `run:` is reported, not silently skipped | verify: `human — run the fixture below; the no-op must be visible in dva's own output, not inferable only from a missing '$' line`
- [ ] The report distinguishes it from a step that ran | verify: `go test ./internal/runner/ -run TestStepWithoutRunIsReported`
- [ ] `provision` and both hook runners agree | verify: `/usr/bin/grep -c 'RunCommands' internal/runner/local.go internal/runner/docker_compose.go` — print the counts; both call sites must take the same branch
- [ ] Every `examples/*.yml` still validates | verify: `for f in examples/*.yml; do d=$(mktemp -d); cp "$f" "$d/dva.yml"; (cd "$d" && dva validate >/dev/null) || echo "FAIL $f"; done` — print a count of failures AND a count of files swept, expect 0 and 16
- [ ] The schema/runner disagreement on `run:`-only is resolved in one direction and documented | verify: `human — either validate accepts it or the error names the fix; state which was chosen`
- [ ] Not vacuous | verify: `human — break the reporting branch and confirm the new test fails`
- [ ] Full suite passes | verify: `make test`

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
      - step: "make build"     # no run: — prints, executes nothing, exit 0
```

## Related

- [TASK-076](../done/076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
  — where this surfaced; the doc half is fixed there.
- [TASK-073](../done/073-version-error-blames-the-config-for-a-build-defect.md) — same class: a
  message that reads as success while the underlying action did not happen.
