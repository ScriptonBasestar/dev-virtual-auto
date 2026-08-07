---
id: TASK-172
title: "A malformed bool value still reaches docker, on the one parseDvaFlags path with no rejection behind it"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T17:50:00+09:00
completed-at: 2026-08-03T19:05:00+09:00
source: "TASK-145 session review"
depends-on: [TASK-145]
scope: "internal/cli/flagtoken.go, internal/cli/compose.go, internal/cli/app.go, internal/cli/stack.go"
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
    result: build_flag_leak_test malformed bool blocked
verification-summary: |
  quality-review pass; re-checked deliverables. build_flag_leak_test malformed bool blocked. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 172: `parseDvaFlags` promises a rejection that 5 of its 12 callers do not have

## Problem

TASK-145 made `parseDvaFlags` hand a malformed boolean back to its caller rather than
guess, and `internal/cli/flagtoken.go` records the reason:

> `parseDvaFlags` leaves the token in place for its caller's own unknown-flag rejection
> to name.

That is true for 7 of its 12 call sites. It is not true for the rest. Measured
2026-08-03 against `bin/dva` at 53cdba2 with an argv-recording `docker` shim:

```
$ dva build --debug=notabool
ARGV: [compose] [-f] [<compose.yml>] [build] [--debug=notabool]   # DVA's flag, handed to docker

$ dva build --debug=true                                          # control
ARGV: [compose] [-f] [<compose.yml>] [build]                      # correctly consumed
```

`compose.go:517` ends in `execComposePassthrough(e, c, append([]string{"build"},
remaining...))` — `remaining` *is* the external argv. There is no `rejectUnknownFlags`
between the two, so the leftover is not named by DVA, it is forwarded. This is the same
leak TASK-145 closed, surviving in the one spelling TASK-145 deliberately did not claim.

The second caller mishandles it differently — not a leak, but advice that cannot work:

```
$ dva down --debug=notabool
ERROR: 'dva down' downs all services. Use 'dva stack down --debug=notabool' or
       'dva app down --debug=notabool' for selective down
```

`teardownCommon` (`compose.go:269`) treats any leftover as a service name and quotes it
into the suggestion. Running the suggested command does not help, because the token was
never a service name.

`compose.go:470` (`restart`) passes leftovers on as service names; `compose.go:340` and
`:404` discard `filtered` entirely, so those two are unaffected.

## Why this is small and why it is still worth a task

Only the *malformed* spelling is affected — `--debug=true` and `--debug=false` are
consumed correctly on every path, which is what TASK-145 measured and closed. What is
wrong here is narrower: a comment states a guarantee the code does not uniformly provide,
and on `dva build` the consequence is a DVA-owned token in docker's argv.

## Acceptance criteria

- [x] Decide and record which it is: `parseDvaFlags` rejects a malformed bool itself, or
      the comment is corrected to say which callers reject and which forward. Do not
      leave a claim that holds for 7 of 12 call sites.
- [x] `dva build --debug=notabool` is re-measured with the shim and the actual argv
      printed. No DVA-owned token appears in it.
- [x] `dva down --debug=notabool` no longer suggests a command containing DVA's own flag.
- [x] `dva build --debug=true` and `--debug=false` still behave as TASK-145 measured —
      print both argvs to show the fix did not widen into the well-formed spellings.
- [x] A test drives the `build` path specifically. The existing TASK-145 table drives
      `stack log`/`compose`/`logs`, none of which reach `compose.go:517`.
- [x] `make test` exits 0.

## Notes

Check `app.go`'s three callers too — they do call `rejectUnknownFlags`, so they are in
the safe 7, but confirm rather than assume: the noun they pass is "an application name",
and a `--debug=notabool` reported as an unknown application name is only half an
improvement over forwarding it.

## Result

### The choice was forced, not preferred

Criterion 1 offered two ways out: reject in `parseDvaFlags`, or correct the comment to
name which callers reject and which forward. Only the first is available.

Correcting the comment would mean leaving the rejection to the call site, and `dva build`
cannot perform it. Its leftovers are docker's argv by design — `dva build --no-cache`
has to reach docker — so at that point nothing can distinguish a malformed DVA flag from
a valid docker one. `--debug=notabool` and `--no-cache` are the same shape to any code
that does not already know `--debug` belongs to DVA. `parseDvaFlags` is the last code
that knows. A second scan at the call site was already considered and rejected by
TASK-145, whose comment survives in `compose.go:699`: it would be "a walk with its own
idea of where the `--` terminator is."

So the rejection moved into `parseDvaFlags`, which gained an `err` return. The compiler
then enumerated the call sites for me — changing the arity made it list exactly 12
non-test callers, independently confirming the task's "7 of its 12" count:

```
$ grep -n "parseDvaFlags(" internal/cli/*.go | grep -v _test.go   # 12 + the definition
app.go:126  app.go:228  app.go:287
compose.go:120  :272  :354  :421  :490  :543
stack.go:78  :167  :224
```

### Measured after (argv-recording `docker` shim, `tmp/scripts/172-measure.sh`)

Criterion 2 — the malformed spelling. The absence of an `ARGV:` line is the evidence:
the shim appends one every time it runs, so no line means docker was never invoked.

```
$ dva build --debug=notabool
ERROR: invalid boolean value "notabool" for --debug
rc=1
$ dva build --json=maybe
ERROR: invalid boolean value "maybe" for --json
rc=1
$ dva build web --debug=notabool
ERROR: invalid boolean value "notabool" for --debug
rc=1
```

Criterion 4 — the well-formed spellings TASK-145 measured, unchanged. `--debug=true`
emits `level=DEBUG` lines and `--debug=false` emits none, so the value is still read
rather than the name merely matched; neither token reaches docker.

```
$ dva build --debug=true
time=... level=DEBUG msg="debug mode enabled" json=false
ARGV: [compose] [-f] [<compose.yml>] [build]
$ dva build --debug=false
ARGV: [compose] [-f] [<compose.yml>] [build]
$ dva build --no-cache web            # docker's own flags still pass through
ARGV: [compose] [-f] [<compose.yml>] [build] [--no-cache] [web]
```

Criterion 3 — teardown. A dash-leading leftover is a flag, not a service name:

```
$ dva down --debug=notabool
ERROR: invalid boolean value "notabool" for --debug
$ dva down --bogus
ERROR: unknown flag "--bogus" for "dva down"
       → 'dva down' takes no service names or flags of its own; it downs everything declared
```

### The Notes concern, measured rather than assumed

`app.go`'s three callers are in the safe 7, but the note asked whether a malformed bool
would be reported as an unknown *application name* — half an improvement. It is not;
`parseDvaFlags` now errors before `rejectUnknownFlags` is reached, and the control shows
a real unknown name still reads as a name:

```
$ dva app up --debug=notabool       # app.go:126   ERROR: invalid boolean value "notabool" for --debug
$ dva app restart --json=maybe      # app.go:228   ERROR: invalid boolean value "maybe" for --json
$ dva app build --dry-run=perhaps   # app.go:287   ERROR: invalid boolean value "perhaps" for --dry-run
$ dva app up nosuchapp              # control      ERROR: no such application: nosuchapp
                                                          → declared in dva.yml: api
```

### Falsification (`tmp/scripts/172-falsify.sh`)

Two ways of undoing the fix, each run against the new tests and against TASK-145's table.
A test that cannot be made to fail is not evidence.

| Falsification | New tests | TASK-145 table |
|---|---|---|
| F1 — `takeBool` stops assigning `err` (malformed value silently dropped) | FAIL: 4 build + 1 teardown + 2 parse subtests | **ok** (5.2s) |
| F2 — `takeBool` restores the original leak (`filtered = append(filtered, name+"="+value)`) | FAIL: same 7 subtests | **ok** (5.2s) |
| neither (as committed) | ok, 34/34 subtests | ok |

The two columns are disjoint, which is the point: TASK-145's table drives `stack log`,
`compose` and `logs`, none of which reach `compose.go:517`, so it stays green through
both falsifications and could never have caught this.

F2 reproduces the original bug exactly — when `flagBoolValue` reports not-ok the token
always had a value, so `name+"="+value` rebuilds it byte for byte. Its failure message is
the criterion stated as an observation:

```
build_flag_leak_test.go:52: dva build [--debug=notabool] returned nil — the malformed value was accepted
build_flag_leak_test.go:64: docker was invoked 1 time(s) despite the bad flag:
      [compose -f <tmp>/docker-compose.yml build --debug=notabool]
```

**Reading that message changed the test.** On the first F2 run the second line was not
there: the nil-error check was a `t.Fatalf`, so it aborted the subtest before the argv
assertion ran. The test still failed, so a pass/fail-only falsification would have called
this fine — the weakness was only visible in the message. The argv check was therefore
unreachable in precisely the failure it exists for, and would have stayed unreachable for
any future regression that stopped raising the error. It is now an `Errorf` followed by
the argv `Fatalf`, so both report. Two different regressions are being watched here: "did
DVA complain" and "did the token reach docker" — a change that restored the error while
still forwarding would satisfy the first alone.

`TestBuildStillForwardsWhatIsDockersAndConsumesWhatIsDvas` passes under both
falsifications, correctly: it only drives well-formed spellings and docker's own flags,
which neither falsification touches. It is the guard against the fix widening, not
against it being absent — the two halves are deliberately separate tests.

The falsification script snapshots `compose.go` to the scratchpad and restores by file
copy rather than `git checkout --`, which would have discarded the whole uncommitted fix;
the run ends by diffing against the snapshot and printing `identical`.

### An artifact worth recording

The first pass at the `app.go` measurement looped `for a in "app up --debug=notabool" ...`
and ran `$DVA $a`. Under zsh an unquoted expansion is not word-split, so DVA received one
30-character argument and answered `unknown command "app up --debug=notabool"`. That
reads exactly like a finding and is entirely an artifact of the shell. Re-run with literal
arguments, all three sites answered correctly. Same trap as the corpus-counting note: in
zsh a "list" in a plain string is one item, so a probe can report a clean pass having
tested nothing.

### Test-run times are environmental here

An 11-minute `go test` timeout during this task was not a hang in the code. Under a load
average of ~17 from concurrent sessions, macOS first-execution validation stalls each
freshly written binary and shim for minutes; the same selection later completed in 128s
with all 34 subtests passing. Sequential falsification runs took 4–5s each once the
binaries had been validated once.
