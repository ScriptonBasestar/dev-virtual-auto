---
id: TASK-119
title: "`dva doctor`'s compose check hardcodes `docker` and ignores `command:`, so it validates a tool the user is not running"
type: fix
priority: P3
effort: S
status: todo
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/doctor.go:531 checkComposeConfigResolves"
---

# Task 119: the fifth copy of the compose argv builder

TASK-115 consolidated four copies of the compose argv builder into `exec.ComposeArgv`. There is a
fifth, found while mapping the callers and left in place on purpose — folding it in changes more
than argv construction, so it is its own decision.

## What it does

`checkComposeConfigResolves` (`internal/cli/doctor.go:531`) builds its own argv:

```go
if _, err := exec.LookPath("docker"); err != nil {
	return nil
}
args := []string{"compose"}
…
cmd := exec.CommandContext(ctx, "docker", args...)
```

It never reads `cc.Command`. Three consequences, in increasing order of severity:

1. **It checks the wrong binary.** A user whose `dva.yml` says `command: podman-compose` gets
   "Compose config resolves ✓" from `docker compose config`. The check passes or fails on a tool
   they are not running.
2. **It skips silently when docker is absent.** `LookPath("docker")` failing returns `nil` — no
   result at all. A podman-only machine gets no compose check and no note that one was skipped. The
   comment says the daemon check reports the absence, which is true of *docker's* absence, not of
   this check having been dropped.
3. **It does not interpolate.** `cc.Files` and `cc.ProjectName` go in raw, while every other
   consumer passes them through `env.Interpolate`. A `files: [compose.${STAGE}.yml]` entry is
   checked as the literal `compose.${STAGE}.yml`, which does not exist — so this check reports a
   failure the real run would not have.

Point 3 is the one that misleads in the direction of noise rather than silence, and it is
independent of the other two.

## Why it was not folded into TASK-115

Replacing the argv construction is three lines. Deciding what to do about the `LookPath` skip is
not: the check needs to resolve *the configured* binary, decide whether "configured binary missing"
is a skip or a failure, and say so either way. That is a behaviour change to `dva doctor`'s output
contract, which TASK-115 had no mandate to make.

## Proposed fix

1. Build the argv with `exec.ComposeArgv(env, cc, c.FileDir())`, which also fixes the interpolation
   gap. This requires a `*config.Environment` at the call site; check what `doctor.go` already has.
2. `LookPath` the command `ComposeArgv` returns, not the literal `"docker"`.
3. When that binary is missing, emit a result saying the check was skipped and naming the binary —
   not `nil`. A check that silently does not run is the defect shape this repo keeps producing.
4. Propagate the error `ComposeArgv` now returns; a `command:` that splits to nothing should surface
   here as a failed check, since `dva doctor` is the command people run to find out what is wrong.

## Acceptance criteria

- [ ] The wrong-binary case is reproduced first | verify: `human — a fixture with 'command: podman-compose'; record which binary doctor actually executes (dtruss/strace, or a PATH shim named docker that logs its argv)`
- [ ] doctor runs the configured binary | verify: `go test ./internal/cli/ -run 'Doctor.*Compose' -v`
- [ ] Interpolation is applied | verify: `go test ./internal/cli/ -run 'Doctor.*Compose' -v` — a `files:` entry containing `${VAR}` must be checked expanded
- [ ] A missing binary is reported, not skipped | verify: `go test ./internal/cli/ -run 'Doctor.*Compose' -v` — the result set must be non-empty and name the binary
- [ ] No sixth copy | verify: `grep -n '"compose"' internal/cli/doctor.go | wc -l` — must be 0
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-115](../done/115-four-compose-argv-builders-share-two-bugs.md) — the four copies this one
  escaped, and the builder it should now call.
