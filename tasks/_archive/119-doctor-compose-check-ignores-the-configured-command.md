---
id: TASK-119
title: "`dva doctor`'s compose check hardcodes `docker` and ignores `command:`, so it validates a tool the user is not running"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/doctor.go:531 checkComposeConfigResolves"
verified-at: 2026-08-03T14:45:00+09:00
archived-at: 2026-08-03T14:45:00+09:00
verification-summary: |
  The fix is real and end-to-end verifiable with the shipped binary, not just with unit tests.
  Driving ./bin/dva doctor inside a copy of tmp/task-119 with PATH replaced by shims for both
  `docker` and `podman-compose`: the compose check executed `podman-compose -f …/compose.dev.yml
  --project-name task119 config --quiet` and printed `4 passed, 1 failed` (the remaining FAIL is
  the unrelated .gitignore advice) — matching the after column of the task's table line for line.
  With podman-compose removed from PATH the check reports `[pass] Compose config resolves
  (skipped: podman-compose is not on PATH)` and runs nothing, instead of vanishing.
  The tests are not vacuous: doctor_compose_test.go replaces PATH entirely, installs both
  binaries so docker *could* answer, and asserts on the argv the shim recorded (doctor_compose_test.go:150,163)
  rather than on the DoctorResult — which is what kills the "LookPath the right binary, spawn the
  wrong one" mutant. resetDoctorGlobals (:25) clears the loadEnv cache so subtests cannot pass on
  a previous subtest's environment.
  The task's own count of "7 PASS lines" is now 8 (a subtest was added or the count went stale);
  the binding still passes. The deliberately-left-open exit-code item was filed and closed as
  TASK-122, so no follow-up is missing there.
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

- [x] The wrong-binary case is reproduced first | verify: `human — a fixture with 'command: podman-compose'; record which binary doctor actually executes (dtruss/strace, or a PATH shim named docker that logs its argv)`
- [x] doctor runs the configured binary | verify: `go test ./internal/cli/ -run 'Doctor.*Compose' -v`
- [x] Interpolation is applied | verify: `go test ./internal/cli/ -run 'Doctor.*Compose' -v` — a `files:` entry containing `${VAR}` must be checked expanded
- [x] A missing binary is reported, not skipped | verify: `go test ./internal/cli/ -run 'Doctor.*Compose' -v` — the result set must be non-empty and name the binary
- [x] No sixth copy | verify: `f=internal/cli/doctor.go; [ -f "$f" ] || { echo "$f does not exist — nothing was measured"; exit 2; }; n=$(/usr/bin/grep -c '"compose"' "$f" || true); echo "occurrences=$n in $f ($(wc -l < "$f" | tr -d ' ') lines)"; [ "$n" -eq 0 ]` — **`occurrences=0 in internal/cli/doctor.go (811 lines)`, exit 0 (TASK-199).** The binding was `grep -n '"compose"' internal/cli/doctor.go | wc -l` — must be 0. The target was stated only in prose: `wc -l` ends the pipeline, so the command exited 0 whatever it counted, and it prints `0` for a file that has been renamed or deleted exactly as it does for a file with no hardcoded literal. Since this criterion exists to catch a *sixth copy* appearing in a file that keeps changing, "the file is gone" is the reading it could least afford to score as a pass. The line count is printed so a later reader can see the file was actually read. Sabotaged against a fixture `internal/cli/doctor.go` containing `[]string{"docker", "compose"}` — `occurrences=1`, exit 1 — and against a tree without the file — exit 2
- [x] Full suite passes | verify: `make test`

## Resolution

`checkComposeConfigResolves` now builds its argv with `dvaexec.ComposeArgv` — the same builder the
four runners use — and `LookPath`s whatever command that returns. All three defects fall out of that
one change; the fourth item below is a scope extension that does not.

### Measured, real binary, one fixture

`tmp/task-119/` declares `vars: {STAGE: dev}`, `command: podman-compose`, and
`files: [compose.${STAGE}.yml]`. `compose.dev.yml` is on disk the whole time and every other
consumer resolves it. Both binaries are PATH shims that log their argv and exit 0, so nothing here
depends on a real container tool being installed or on a daemon being up.

| | before | after |
|---|---|---|
| binary executed | `docker` | `podman-compose` |
| `-f` argument | `…/compose.${STAGE}.yml` | `…/compose.dev.yml` |
| `Compose config resolves` | `[pass]` | `[pass]` |
| `Compose file exists` | `[FAIL]` | `[pass]` |
| summary line | `3 passed, 2 failed` | `4 passed, 1 failed` |

The remaining failure is the unrelated `.sb/dva/ is ignored in .gitignore` check, present in both
columns.

The before column is the interesting one. Doctor emitted a green `Compose config resolves` while
running a tool the config never named, against a path that does not exist — the shim exited 0, so
the check passed on the shim rather than on the configuration. One line above it, the file-existence
check reported `[FAIL]` on that same path. The two built-in checks contradicted each other about the
same file, and the one that was wrong was the one wearing the check mark.

With docker removed from PATH:

| | before | after |
|---|---|---|
| result emitted | *none — the check vanished* | `[pass] Compose config resolves (skipped: podman-compose is not on PATH)` |
| binary executed | *none* | *none* |
| `--json` `fix_hint` | *absent* | `install podman-compose, or point stack.<entry>.runners.compose.command at a binary on PATH` |

Before the fix, `LookPath("docker")` failing returned `nil` and the check disappeared from the
output entirely, with nothing saying so. That the docker-only column also skips now is the point:
the binary that decides whether the check can run is the configured one, not the hardcoded one.

### Scope extension: `checkComposeFiles`

The scope line names only `checkComposeConfigResolves`, and `checkComposeFiles` was fixed alongside
it. Stating that plainly rather than leaving it in the diff: same file, same root cause — neither
function had an `Environment` threaded in — and criterion 3 says a `files:` entry with `${VAR}` must
be checked expanded without naming a single check. Leaving the other half unexpanded would have
satisfied the criterion's letter while `dva doctor` still printed a false `[FAIL]` about the same
file on the same run.

Two deliberate choices inside it:

- The result still reports the name **as written** (`compose.${STAGE}.yml`), not as expanded. That is
  the line the user has to go and edit.
- The dedup key is the **resolved** path, so a config spelling one file two ways reports once. A
  test pins this, because keying on the written form would print the same missing file twice under
  two names.

### The skip is a pass, and why

`printDoctorResults` has exactly two states, `[pass]` and `[FAIL]`, and prints `FixHint` only on
`[FAIL]`. A skipped check had to pick one. It is a pass because a missing binary is not evidence
that the user's compose files are wrong, which is what this check claims to be about — so the whole
fact lives in the name, where a human will read it, and the hint reaches `--json` consumers. Calling
it a `[FAIL]` would have inflated the failed count for something that is not a config error, and —
see below — would not have changed the exit code anyway.

### Counts

- `go test ./internal/cli/ -run 'Doctor.*Compose' -v` → exit 0, 7 PASS lines, 0 FAIL
- `grep -n '"compose"' internal/cli/doctor.go | wc -l` → **0**
- `make test` → exit 0, 6 packages ok
- `make lint` → exit 0, 0 issues (`modernize` caught one `strings.Split` in the new test and it was
  changed to `strings.SplitSeq`)
- coverage: `checkComposeConfigResolves` 71.4% → **95.2%**, `checkComposeFiles` → **93.8%**. The
  uncovered remainder is one early-return guard in each.

### Mutation testing

Three mutants, each reverted afterwards and confirmed byte-identical with `diff -q`:

| mutant | killed by | what the failure said |
|---|---|---|
| whole function bodies restored from HEAD | 4 of 5 tests, 6 of 8 subtests | `doctor ran the wrong binary: "docker compose -f …/compose.${DVA_TEST_STAGE}.yml …"` and `expected 1 result, got 0` |
| `CommandContext(ctx, "docker", …)` while `LookPath` still honours the config | `TestDoctorComposeConfigRunsTheConfiguredCommand` | `doctor ran the wrong binary: "docker -f …"` |
| `return nil` restored in the missing-binary branch | `TestDoctorComposeMissingBinaryIsReportedNotSkipped` | `expected 1 result, got 0: []` |

The second mutant is the one worth keeping: it passes `LookPath` on the right binary and then spawns
the wrong one, which is precisely the shape a test asserting only on `DoctorResult` would have
missed. The tests assert on the argv the shim recorded, not on the verdict.

One subtest survives the first mutant — `expanded name is absent` — and that is correct rather than a
gap. It asserts that a `files:` entry which expands to nothing still fails, so it passes both before
and after; it exists to stop the fix from trading a false failure for a blanket pass.

### Left open, deliberately

- **`dva doctor` exits 0 with `[FAIL]` lines on screen.** `doctorExitError` counts only
  `r.UserDefined && !r.Passed`, so no built-in check can reach the exit code. Measured: the before
  binary printed `3 passed, 2 failed` and exited **0**. This is deliberate and pinned —
  `TestDoctorExitError_BuiltinFailedOnly_Advisory` says so in its name — so it is not a defect
  found here. What this task does raise is narrower: `Compose config resolves` failing is not a
  heuristic like the `.gitignore` advice it is grouped with, it means the files genuinely do not
  parse. Filed as a decision, not a fix.
- **Whether "the configured compose binary is absent" deserves its own failing check.** It is a real
  problem on that machine — `dva up` will fail — but it is not a statement about whether the compose
  files resolve, and this check should say only what it measured.

## Related

- [TASK-115](115-four-compose-argv-builders-share-two-bugs.md) — the four copies this one
  escaped, and the builder it should now call.
- [TASK-156](../todo/156-doctors-compose-hint-tells-a-podman-user-to-run-docker.md) — ⚠️ the same
  wrong-binary defect, one line below the fix. `doctor.go:614` still hardcodes `docker compose
  config` in the hint printed when the check fails, so a podman-compose user is told to reproduce
  it with a binary they do not have. Found while verifying this task for archival; the test
  asserts only the hint's leading line, which is why it went unseen.
