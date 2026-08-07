---
id: TASK-162
title: "A command inherited through a YAML merge key is dropped, and the interaction runs nothing and exits 0"
type: bug
priority: P2
effort: S
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-131 finalize verification — surfaced while checking its merge-key criterion"
scope: "dva repo — internal/config/config.go:437-461, InteractionCommand.UnmarshalYAML"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: polymorphicCommand merge; TestMergeKeyInheritsCommand PASS
verification-summary: |
  quality-review pass; re-checked deliverables. polymorphicCommand merge; TestMergeKeyInheritsCommand PASS. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 162: Make `command:` merge like every other field

## Problem

`InteractionCommand.UnmarshalYAML` decodes the node into a plain struct and then recovers
`command:` by hand, because the field is polymorphic (scalar or sequence). The manual pass walks
`node.Content` in key/value pairs and compares each key against the literal string `command`:

```go
// internal/config/config.go:437-461
for i := 0; i+1 < len(node.Content); i += 2 {
    if node.Content[i].Value != "command" {
        continue
    }
    ...
}
```

A merge key is not a literal key. It arrives as `<<` with an alias value, so this loop skips it
and `c.Command` stays empty — while `node.Decode(&p)` above it *does* honour `<<:`, so every
other field merges normally. The result is one field behaving differently from all its
neighbours, with no error.

## Measured

Fixture (`bin/dva` at HEAD, `1695f9d`):

```yaml
interaction:
  one: &base
    command: echo hello
    description: from-base
  two:
    <<: *base
  three: *base
```

| invocation | output | exit |
|---|---|---|
| `dva run three` — plain alias | `hello` | 0 |
| `dva run two` — merge key | *nothing* | **0** |
| `dva run two --explain` | `Command: ` (blank), `Description: from-base` | 0 |

`Description: from-base` is the control: it proves the merge itself worked and only `command:`
was lost. The run reports success having executed nothing, which is the part that makes this a
P2 rather than a formatting bug.

Introduced by `f2c3e95` (2026-04-02, "support polymorphic command execution"), which added the
manual scan. Predates TASK-131 by four months; TASK-131 neither caused nor touched it.

No shipped config uses `<<:`, so corpus impact today is zero: `grep -rl '<<:' examples` matches
**0 of 19** YAML files, and the repo-wide sweep excluding `tasks/` also matches 0. The cost is
to anyone who reads the merge key as working, which the other fields teach them.

## Acceptance criteria

- [x] `command:` inherited through `<<:` resolves, for both the scalar and the sequence form —
      `TestMergeKeyInheritsCommand` pins both (scalar → Command; sequence → CommandLines + first
      line as Command).
- [x] Local `command:` still overrides an inherited one — `override-after` (merge key before the
      local key) resolves to the local command. yaml.v3 makes the local key win regardless of
      order, which Decode honours.
- [x] Prove the gate fails on reverted code — see "Verify binding" below.
- [x] Hand-recovered field count — see "Field audit" below (1 → 0).
- [x] `make test` exits 0; the shipped corpus still validates (19/19, 0 use `<<:` so the parser
      change cannot affect any). `make lint` note below.

## Resolution

Removed the hand-written `command` scan entirely and gave the field its own type,
`polymorphicCommand`, with an `UnmarshalYAML` that handles scalar/sequence exactly as the old
scan did. `command` is now a field of the `plain` alias and is populated by `node.Decode`, which
is the call that already honoured `<<:` for every other field — so it now honours it for command
too. The class (a field recovered by hand rather than Decode) is gone, not just the instance.

## Verify binding

Ran `TestMergeKeyInheritsCommand` against the code BEFORE the fix (the literal-key scan still in
place) and AFTER:

```
BEFORE: --- FAIL: TestMergeKeyInheritsCommand
        via-merge-scalar.Command = "", want "echo hello" (the merge key dropped it)
        via-merge-list.CommandLines = [], want 2 lines inherited through the merge
        via-merge-list.Command = "", want first line "echo one"
AFTER:  --- PASS: TestMergeKeyInheritsCommand (0.00s)
```

The `Description: from-base` control passed in both runs (the merge itself always worked); only
`command` was lost before, which is the defect. Removing the fix reintroduces the three failures.

## Field audit

`InteractionCommand.UnmarshalYAML` recovered exactly ONE field by hand before this change:
`command` (the polymorphic scan). Every other field went through `node.Decode(&p)` and already
honoured merge keys. After this change ZERO fields are hand-recovered — `command` joined the
Decode path — so no field in this unmarshaler retains merge blindness. The cyclic-anchor
pre-decode guard (TASK-131) is a separate scan over `node.Content` that runs before Decode and is
unaffected.

## Lint note

`make lint`'s golangci-lint step is environmentally broken this session: `GOTOOLCHAIN=auto`
resolves a go1.26.5 tool against mise's go1.26.4 GOROOT, so golangci typecheck fails to compile
the stdlib — on `tools/libgen/main.go`, a file this task does not touch. The deterministic lint
parts pass: `go vet ./...` clean, `gofmt -s` 0 unformatted. Under `GOTOOLCHAIN=local`
golangci-lint runs and reports 0 issues on the changed packages. This is pre-existing
environment drift, not caused by the change.

## Notes

Distinct from [TASK-146](146-dry-run-shows-a-blank-command-for-a-steps-only-interaction.md),
which has the same visible symptom — a blank `Command:` under `--explain` — from a different
cause: there the interaction genuinely has no `command`, and Explain is blind to `steps:`. Here
the interaction has one and the parser loses it. Whoever fixes either should read the other,
because a fix to Explain's rendering would hide this one.

The obvious repair is to resolve merge keys before the manual scan, or to give `Command` its own
type with an `UnmarshalYAML` so `Decode` handles the polymorphism and the hand-written pass
disappears. The second removes the class rather than the instance.

## Related

- [TASK-131](../_archive/131-a-cyclic-anchor-kills-dva-before-any-check-runs.md) — the anchor
  work that surfaced this. Its criterion 3 measured the merge with `Service: web`, a field that
  goes through `Decode`; the one field handled by hand is the one that does not merge. Corrected
  inline there.
