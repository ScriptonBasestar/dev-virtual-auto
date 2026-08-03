---
id: TASK-172
title: "A malformed bool value still reaches docker, on the one parseDvaFlags path with no rejection behind it"
type: bug
priority: P3
status: todo
effort: S
created-at: 2026-08-03T17:50:00+09:00
source: "TASK-145 session review"
depends-on: [TASK-145]
scope: "dva repo — internal/cli parseDvaFlags callers (compose.go build/down/restart)"
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

- [ ] Decide and record which it is: `parseDvaFlags` rejects a malformed bool itself, or
      the comment is corrected to say which callers reject and which forward. Do not
      leave a claim that holds for 7 of 12 call sites.
- [ ] `dva build --debug=notabool` is re-measured with the shim and the actual argv
      printed. No DVA-owned token appears in it.
- [ ] `dva down --debug=notabool` no longer suggests a command containing DVA's own flag.
- [ ] `dva build --debug=true` and `--debug=false` still behave as TASK-145 measured —
      print both argvs to show the fix did not widen into the well-formed spellings.
- [ ] A test drives the `build` path specifically. The existing TASK-145 table drives
      `stack log`/`compose`/`logs`, none of which reach `compose.go:517`.
- [ ] `make test` exits 0.

## Notes

Check `app.go`'s three callers too — they do call `rejectUnknownFlags`, so they are in
the safe 7, but confirm rather than assume: the noun they pass is "an application name",
and a `--debug=notabool` reported as an unknown application name is only half an
improvement over forwarding it.
