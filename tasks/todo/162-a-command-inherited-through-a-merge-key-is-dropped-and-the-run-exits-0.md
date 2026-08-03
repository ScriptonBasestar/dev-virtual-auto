---
id: TASK-162
title: "A command inherited through a YAML merge key is dropped, and the interaction runs nothing and exits 0"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-03T15:40:00+09:00
source: "TASK-131 finalize verification — surfaced while checking its merge-key criterion"
scope: "dva repo — internal/config/config.go:437-461, InteractionCommand.UnmarshalYAML"
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

- [ ] `command:` inherited through `<<:` resolves, for both the scalar and the sequence form.
      A merge key that supplies `command` as a list must populate `CommandLines` too, not just
      `Command`.
- [ ] Local `command:` still overrides an inherited one, in both directions of key order.
- [ ] Prove the gate fails on reverted code | verify: restore the literal-key scan, run the new
      test, paste the failure. A criterion verified only in its passing state is what
      [TASK-116](../_archive/116-stack-override-warning-goes-to-stdout.md) warns about.
- [ ] Report how many other fields in this unmarshaler are recovered by hand rather than by
      `Decode`, and whether each has the same merge blindness. State the denominator — "only
      this one" is not a result until the count is printed.
- [ ] `make test` and `make lint` exit 0, and the shipped corpus still validates (19/19, the
      denominator TASK-131 established).

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
