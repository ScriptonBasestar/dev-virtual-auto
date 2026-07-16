---
id: TASK-044
title: "Provision's legacy structured format implements echo/cmd but silently ignores shell/sleep/docker"
type: bug
priority: P2
status: todo
needs-human: true
effort: M
created-at: 2026-07-16T23:05:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (zero-reader audit of every yaml-tagged config field)
source-severity: MEDIUM
---

# Task 044: `- sleep: 4` Waits Zero Seconds And Reports "✅ Provision complete!"

## Summary

`schema.json` defines a "Legacy structured command" provision item with exactly five allowed keys —
`echo`, `cmd`, `shell`, `sleep`, `docker` — under `additionalProperties: false`, so all five are
deliberate, enumerated, and accepted.

**Two of the five are implemented.** `echo` prints and `cmd` executes. `shell`, `sleep`, and `docker`
are parsed into the struct and read by nothing. A provision step written with them validates green,
runs, prints `✅ Provision complete!`, and exits 0 — having done nothing.

The sharpest instance: **`schema.json` ships `{"sleep": 10}` in its own `provision` examples block**,
so the schema recommends a no-op.

## Evidence — by construction

`ShellC` and `Sleep` each appear **exactly once in the entire repository** — their own field
declaration (`internal/config/config.go:527-528`):

```go
// Legacy structured format
Echo   string `yaml:"echo"`
Cmd    string `yaml:"cmd"`
ShellC string `yaml:"shell"`     // <-- declared here, referenced nowhere else, ever
Sleep  any    `yaml:"sleep"`     // <-- same
Docker any    `yaml:"docker"`    // <-- same
```

```
$ grep -rn "\bShellC\b" --include="*.go" .
internal/config/config.go:527:	ShellC string `yaml:"shell"`

$ grep -rn "\bSleep\b" --include="*.go" .
internal/config/config.go:528:	Sleep  any    `yaml:"sleep"`
```

One hit each: the declaration. No reader, no test, no mention. A field that is never read cannot have
an effect — the same by-construction argument as TASK-035/040, and it needs no probe to be conclusive.

`ProvisionItem.Docker` likewise has zero readers. The only `p.Docker` in the tree is
`AppExecPaths.Docker` at `config.go:302` (`HasDocker()`), an unrelated struct — recorded because that
name collision is exactly the trap TASK-040 documented, where a careless grep reports a field as
"consumed" when the consumer is something else entirely.

### Where the two implemented keys are read (the in-band control)

`internal/cli/provision.go:179-193` handles precisely two of the five:

```go
// Legacy format: echo
if step.Echo != "" { fmt.Printf("    %s\n", step.Echo) }

// Legacy format: cmd
if step.Cmd != "" { ... runShellCommand(e, step.Cmd) ... }
```

`RunCommands()` (`config.go:547`) reads only `Raw` and `Run`. Nothing anywhere reads the other three.

### Control — the search shape and the audit both find real readers

The same zero-reader audit that surfaced this **re-discovered TASK-036's already-filed finding**
(`Related`/`Hint`, `config.go:226-227`, zero readers) — a known-true positive. And every other
`ProvisionItem` field checks out as genuinely consumed, so the audit is not simply reporting
everything as dead:

| Field | Readers | |
|---|---|---|
| `Step` / `Raw` / `Run` / `Note` | read | step-based format, fully implemented |
| `ComposeUp` / `ComposeExec` / `ComposeRun` | read | |
| `Parallel` | read (`provision.go:56`, `:95`) | |
| `Echo` / `Cmd` | read (`provision.go:179`, `:184`) | legacy format, implemented |
| **`ShellC` / `Sleep` / `Docker`** | **0** | legacy format, **inert** |

The step-based branch is complete. The legacy branch is half-built.

## Evidence — measured, and the control pair is the finding

Probe copied from **`schema.json`'s own `provision.examples`** — both forms, verbatim in shape.
`dva validate` EXIT=0 on this config, so the liveness gate holds:

```yaml
provision:
  legacy_string:            # schema example: raw strings
    - echo 'Setting up...'
    - sleep 4
  legacy_structured:        # schema example: structured keys
    - echo: "Starting..."
    - cmd: echo CMD_RAN
    - sleep: 4
```

```
$ dva provision legacy_string
    $ echo 'Setting up...'
Setting up...
    $ sleep 4
✅ Provision complete!
EXIT=0  ELAPSED=4s          # <-- honored

$ dva provision legacy_structured
    Starting...
    $ echo CMD_RAN
CMD_RAN
✅ Provision complete!
EXIT=0  ELAPSED=0s          # <-- NOT honored. Same intent, same schema, same green output.
```

**Why this pair is decisive.** The raw-string run is the positive control: it proves `sleep` is a
meaningful operation the tool performs, that the profile loads, and that the wait is observable when
it happens. The structured run prints `echo`/`cmd` output, proving the item list *was* processed and
those steps *were* in the profile — so the silence on `sleep:` is specific to that key, not "the
profile didn't load". The two forms differ only in shape, come from the same schema example, both
validate green, both report success, and only one waits.

A third probe confirmed the same for the other two keys — `- shell: echo SHELL_SHOULD_HAVE_RUN` and
`- docker: {service: ..., command: ...}` produced no output, no execution, no warning, EXIT=0.

## Why it matters

`provision:` is the setup path — its steps exist so that everything after them has an environment to
run in. A silently skipped provision step does not fail where it was skipped; it fails later, in
whatever assumed the step had run, with an error that does not mention provisioning.

- `- shell: ./scripts/migrate.sh` — the migration never runs. `✅ Provision complete!`, exit 0.
- `- sleep: 10` between "start database" and "run migrations" — no wait, so the race the author was
  explicitly guarding against is reintroduced, intermittently.

This is the harmless direction in the narrow sense (DVA does *less*; nothing is mutated or destroyed)
but it is the run's organizing theme at its sharpest: **a green surface certifying work that never
happened**. `dva validate` says valid, `dva provision` says complete, exit 0 says success, and the
user's own instruction was discarded in silence.

The schema example makes it worse than a vestigial field. TASK-015 (P2) fixed a schema `version`
example that could never load — that example failed *loudly*, and was still worth P2 on the reasoning
that **editors surface schema examples as completions**. The same reasoning applies here with the
opposite failure mode: `{"sleep": 10}` is copied from the schema, loads, validates, reports success,
and does nothing. A wrong example that errors teaches the user immediately; one that succeeds silently
does not.

## Severity: MEDIUM / P2

Not P1: the legacy structured format is not recommended by any doc or example — `docs/`, `examples/`,
`README.md`, and `USAGE.md` contain no provision `shell:`/`sleep:`/`docker:` items (every `shell:` hit
in those files is an interaction *named* `shell`, or `InteractionCommand.Shell *bool` at
`config.go:315` — different fields, checked). So real-world exposure is limited to old configs and to
anyone following the schema.

Not P3, because the schema does not merely permit these keys — it **enumerates them** under
`additionalProperties: false` (a deliberate allowlist, not an oversight) and **ships `{"sleep": 10}`
as an example**. The tool's own machine-readable contract recommends a no-op, and the failure is
silent success on the setup path.

## Scope note — needs a decision

Same honor-vs-remove shape as TASK-035/036/037/040, and not the implementer's call:

- **Honor them** — implement all three in `provision.go` alongside `echo`/`cmd`. `shell` is
  straightforward (`runShellCommand`, mirroring `Cmd` at `:184`). `sleep` needs a type decision:
  `Sleep` is `any`, and the schema example uses a bare number (`{"sleep": 10}`) while YAML could also
  give `"10s"` — so seconds-vs-duration-string must be settled, and it must respect `--dry-run`
  (printing rather than actually sleeping) the way `Cmd` does at `:185`. `docker` is the largest:
  `Docker` is `any` with no defined shape anywhere in the Go code, so honoring it means **designing**
  it, not restoring it. Note `docker` has no schema sub-shape either — the legacy branch types it as a
  bare allowed key.
- **Remove them** — drop `shell`/`sleep`/`docker` from the schema's legacy branch and from
  `ProvisionItem`, and fix the `legacy_structured` schema example that ships `{"sleep": 10}`. Honest
  and cheap, and the raw-string form already covers every one of these needs today (`- sleep 4` and
  `- ./scripts/migrate.sh` both work, as the control proves). **Back-compat caveat:** with
  `additionalProperties: false`, removing the keys from the schema turns configs that validate today
  into hard validation failures. They are already no-ops, so nothing *working* breaks — but a config
  that passes `dva validate` today would start failing, which is a visible change.

Lean **remove**, weakly, and note this is the opposite lean from TASK-040. The reason for the
difference: `--force` has no working alternative, so removing it deletes a capability. Here the
raw-string form already does all three jobs and is *proven* to (`sleep 4` → 4s), so removal deletes
nothing but a lie — and `docker:` in particular has no defined shape to honor, so "honor" means
inventing a feature rather than fixing a bug. But the back-compat break and the fate of the schema
example are a maintainer's call.

Whichever way it goes, the `{"sleep": 10}` schema example is wrong **under both options** and must
change: honored, it should demonstrate the settled type; removed, it must go.

## Completion Criteria

- [ ] DECISION recorded: honor `shell`/`sleep`/`docker` in the legacy structured format, or remove them | verify: `human — maintainer picks one and records why; may be decided per-key (e.g. honor shell, remove docker)`
- [ ] The `legacy_structured` example in schema.json no longer recommends a no-op | verify: `human — it currently ships {"sleep": 10}; assert the example matches the decision. This is wrong under BOTH options`
- [ ] No key remains that schema.json enumerates and no code reads | verify: `human — for each of shell/sleep/docker, either a probe shows it changing observable behavior, or it is gone from schema.json AND ProvisionItem`
- [ ] If HONOR sleep: the type is settled and documented (bare seconds vs duration string) | verify: `human — Sleep is 'any'; the schema example uses a bare number; assert one shape is chosen and rejected inputs error rather than silently no-op`
- [ ] If HONOR sleep: --dry-run prints the sleep instead of performing it | verify: `human — mirror Cmd at provision.go:185; a dry run must not actually wait`
- [ ] If HONOR docker: its shape is designed, since none exists today | verify: `human — ProvisionItem.Docker is 'any' with no schema sub-shape; this is a design task, not a restoration`
- [ ] If HONOR: a probe shows each honored key changing observable behavior, with the raw-string form as the control | verify: `human — reproduce the pair: '- sleep 4' (raw, 4s) vs '- sleep: 4' (structured); assert both wait`
- [ ] If HONOR: a regression test asserts each honored key reaches execution, proven to fail without the fix | verify: `human — revert the read, confirm the test FAILS for the right reason, restore, confirm it passes`
- [ ] If REMOVE: the back-compat break is acknowledged — configs that validate today will fail | verify: `human — additionalProperties:false means removing the keys makes them hard errors; confirm intended, and note they are no-ops today so no working behavior is lost`
- [ ] `echo` and `cmd` still work — the in-band controls must not regress | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/cli/`
- [ ] The raw-string form still works | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./internal/config/`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [015-fix-schema-version-example.md](../_archive/015-fix-schema-version-example.md) — the precedent: a wrong schema example is P2 because editors surface examples as completions. That one failed loudly; this one succeeds silently
- [036-service-related-and-hint-ignored.md](./036-service-related-and-hint-ignored.md) — same class; its `Related`/`Hint` fields were independently re-discovered by the same zero-reader audit that found this, which is the audit's positive control
- [040-up-force-flag-documented-and-inert.md](./040-up-force-flag-documented-and-inert.md) — same class, opposite lean (no working alternative there; here the raw-string form works). Also the source of the name-collision warning applied to `Docker`
- [035-env-file-interpolate-and-priority-ignored.md](./035-env-file-interpolate-and-priority-ignored.md) — same class, same honor-vs-remove decision shape
