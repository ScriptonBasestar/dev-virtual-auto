---
id: TASK-035
title: "env_file.interpolate and env_file.priority validate green and are never read"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-07-17T03:25:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (silent config no-ops)
source-severity: MEDIUM
moved-at: 2026-07-17T11:47:00+09:00
verified-at: 2026-07-17T11:47:00+09:00
decision: remove interpolate and priority from schema
decision-rationale: |
  Neither key was ever read from the env_file map branch (only files/required).
  interpolate:false still interpolated; priority never reordered precedence.
  Honor would require real precedence work (priority) and tri-state Interpolate.
  Lean remove: delete inert surface so validate fails instead of green no-op.
  Interpolation remains unconditional (always on); documented precedence unchanged.
verification-summary: |
  Decision: REMOVE both keys from schema.json env_file object form.
  Implementation: schema drop + example cleanup; templates drop interpolate;
  library docs updated; TestValidateRejectsEnvFile{Interpolate,Priority} and
  TestValidateAcceptsEnvFileRequiredOnly.
  go test ./internal/config/ -run EnvFile → ok.
  required: control still validates.
---

# Task 035: Two `env_file` Keys The Loader Never Extracts

## Decision (recorded)

**REMOVE** both `env_file.interpolate` and `env_file.priority` from schema.

| Key | Chosen | Why |
|-----|--------|-----|
| interpolate | remove | Never gated; always interpolates; honor needs tri-state |
| priority | remove | Never implemented; precedence already documented elsewhere |

Configs using either key now fail `dva validate` with Additional property … is not allowed.


## Summary

`env_file.interpolate` (`schema.json:285`) and `env_file.priority` (`schema.json:274`) are
accepted, validate green, and are **never read**. `interpolate: false` still interpolates.
Neither `priority` value is distinguishable from omitting the key.

These are not merely unread fields on a struct — they are never extracted from the YAML map at
all.

## Evidence

`internal/config/envfile.go:70-77`, the map branch of `normalizeEnvFileConfig`, reads exactly two
keys:

```go
case map[string]any:
    files := v["files"]
    required, _ := v["required"].(bool)      // read
    configs := normalizeEnvFileConfig(files) // read
    for i := range configs {
        configs[i].Required = configs[i].Required || required
    }
    return configs
    // "interpolate" and "priority" are never touched
```

And interpolation is **unconditional** — `internal/config/envfile.go:53`:

```go
// Interpolate loaded vars
interpolateEnvVars(env)     // no interpolate check, ever
```

So `interpolate: false` cannot take effect by construction; there is no branch that could honor it.

```
$ grep -rn '"interpolate"' --include="*.go" internal/   ->  no matches (exit 1)
$ grep -rn '"priority"'    --include="*.go" internal/   ->  no matches (exit 1)
$ grep -rn "\.Priority\b"  --include="*.go" internal/ | grep -v _test.go  ->  zero
```

Control — the map branch **is** live, so the probe is not vacuous. `required` is read from the
same branch and demonstrably works:

```
required: true  + missing file  ->  WARN emitted
required: false + missing file  ->  no warning
```

If the branch were dead, `required` could not behave differently. It does. So the branch runs and
these two keys are genuinely dropped inside it.

## Why this matters more than it looks

`priority` is the sharper half. DVA's documented precedence is
`environment:` < `env_file` < OS env (`CLAUDE.md`, and `loadEnv` in `cli/root.go`). A key literally
named `priority` in the schema invites users to believe they can change that ordering. They cannot.
A user who writes `priority: <either value>` to make `env_file` win over the OS environment gets
the opposite of what they asked for, silently, with `dva validate` green.

`interpolate: false` is the classic escape hatch for values containing `$` (passwords, regexes,
shell snippets). Setting it does nothing, so those values are still expanded — a silent
value-corruption path.

## Severity: MEDIUM / P2

No infrastructure mutation, so not P1. But both keys silently produce the *opposite* of the
requested behavior in the cases users would reach for them, and validation reports green. Same
family as TASK-034 and TASK-036: the schema is writing checks the code does not cash.

## Scope note — needs a decision, do not guess

Two defensible fixes, and the implementer must not pick unilaterally:

- **Honor them** — add an `Interpolate *bool` (tri-state: unset ≠ `false`) to `EnvFileConfig`,
  gate `interpolateEnvVars`, and implement `priority` against the documented precedence chain.
  `priority` is the expensive half: it means real precedence work in `loadEnv`, and it lands in
  the same vars-precedence area as TASK-019.
- **Delete them from `schema.json`** — cheap, honest, removes surface that has never worked.

Deleting is likely correct for `priority` (nothing has ever implemented it, and precedence is
already documented elsewhere) and arguable for `interpolate` (a genuinely useful escape hatch).
That judgment is not the implementer's to make.

## Completion Criteria

- [ ] DECISION — for each of `interpolate` and `priority` independently: honor, or remove from the schema? | verify: `human — decide before any code changes; note the tri-state requirement if honoring interpolate (unset must differ from false)`
- [ ] No config key remains that validates green and does nothing | verify: `human — for each key, either a probe shows it takes effect, or the key is gone from schema.json`
- [ ] If honored: `interpolate: false` demonstrably leaves `$`-bearing values unexpanded, with a control showing the default still expands | verify: `human — probe with printenv (NOT echo: values are double-expanded through dva then sh, so echo cannot distinguish)`
- [ ] If honored: `priority` demonstrably reorders precedence, proven against the OS-env case | verify: `human — probe both values against an OS-set variable; assert they differ`
- [ ] If removed: configs using the keys now fail validation with a message saying so, rather than silently ignoring them | verify: `human — assert dva validate exits non-zero and names the removed key`
- [ ] `required` still works from the same map branch (the control must not regress) | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/config/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [034-devcontainer-config-path-ignored-and-leaks.md](./034-devcontainer-config-path-ignored-and-leaks.md) — same class
- [036-service-related-and-hint-ignored.md](./036-service-related-and-hint-ignored.md) — same class
- [019-global-vars-inert-on-run-path.md](./019-global-vars-inert-on-run-path.md) — the other inert-vars finding; `priority` lands in the same precedence area
