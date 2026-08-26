---
id: TASK-198
title: "`dva restart` reports success on a typo'd flag while doing nothing"
type: bug
priority: P2
effort: S
created-at: 2026-08-19T17:36:18+09:00
source: "found reviewing a843c74's help-text corrections — measured 1 of the 4 lifecycle verbs exits 0 on an unknown flag, against a 3-verb control that exits 1"
scope: "internal/cli/compose.go restartCmd RunE (412-492): add the guard up already has at :168. No change to which flags restart accepts."
status: done
completed-at: 2026-08-20T14:30:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-20T14:30:00+09:00
verified-at: 2026-08-20T14:30:00+09:00
---

# Task 198: `dva restart` reports success on a typo'd flag while doing nothing

## Summary

`dva restart --no-wat` exits 0, restarts nothing, and prints
`[warn] no lifecycle entries matched filters`. The unknown flag is not rejected
— it falls through `parseDvaFlags` into the service-name list, matches no stack
entry, and the empty selection is reported as success.

Measured against `./bin/dva` at `a843c74`, fixture `tmp/planroute/e_tagged`
(`s1` tagged alpha, `s2` tagged beta, mode `fast` declared):

```
up       --zzznonsense    rc=1   unknown flag "--zzznonsense"
down     --zzznonsense    rc=1   unknown flag "--zzznonsense"
stop     --zzznonsense    rc=1   unknown flag "--zzznonsense"
restart  --zzznonsense    rc=0   no lifecycle entries matched filters   <-- defect
```

Three of four verbs reject it; `restart` is the outlier. Plausible typos behave
the same as the nonsense control — `--no-wat`, `--dev`, `--docker` and `--force`
each exit 0 with nothing restarted.

The parser is not broken, which is what makes this narrow. Real flags do reach
`restart` and work:

```
restart --tag alpha       rc=0   entry=s1 only          tag filter honoured
restart --mode fast       rc=0   entry=s1,s2            mode applied
restart --mode nosuchmode rc=1   mode 'nosuchmode' not found
restart s1                rc=0   entry=s1 only          name selection works
```

Only *unrecognised* tokens fall through.

## Why restart specifically

The three verbs guard by two different means, and `restart` fits neither:

- `up` calls `rejectUnknownFlags` (`internal/cli/compose.go:168`) with an
  allowlist, because it accepts flags but no positional names.
- `down` and `stop` reject every leftover wholesale in `teardownCommon`
  (`internal/cli/compose.go:261`): *"takes no service names or flags of its
  own"*.
- `restart` is the only verb that legitimately takes **both** flags and
  positional service names, so it needs `up`'s allowlist form. Its RunE
  (`internal/cli/compose.go:412-492`) calls neither guard. `parseDvaFlags`
  returns leftovers as `names` and they are used unchecked.

## Inherited, not new

TASK-113 fixed exactly this defect for `dva up` and the `dva app` family
(`tasks/_archive/113-up-and-app-commands-swallow-unknown-flags.md`). `restart`
was not in its scope, and the `app` family has since been deleted with
`applications:`. `restart` is the last surviving instance of the class.

This card was opened while correcting `restart`'s help text in `a843c74`. That
commit documents `dva restart <service>` as the supported form — which is what
makes the silent fallthrough worth closing: a mistyped flag is read as a service
name by the very path the help now advertises.

## Completion Criteria

None of these bindings use `go test -run`. A `-run` naming a test that does not
exist yet prints "no tests to run" and exits **0**, so it would pass from the
moment this card was filed — `tools/doccheck` rejects exactly that pattern
(`verifyrun.go:66`), and it caught this card's first draft. The test's existence
is asserted on its source instead, which is false today and true after the fix.

- [x] A regression test named `TestRestartRejectsUnknownFlag` exists beside the sibling restart tests | verify: `/usr/bin/grep -c 'func TestRestartRejectsUnknownFlag(' internal/cli/restart_names_test.go` returns 1 — **binding corrected during disposition**: without the trailing `(` it now returns 2, because `TestRestartRejectsUnknownFlagBesideANameWithPlansConfigured` extends the same prefix. The looser form still passed, which is the point: it would also have passed on a file containing only the second test
- [x] That test asserts the message names the flag, as the siblings do | verify: `/usr/bin/grep -A20 'func TestRestartRejectsUnknownFlag' internal/cli/restart_names_test.go | /usr/bin/grep -c 'unknown flag'` returns 3
- [x] The whole cli package passes with it, so nothing over-rejects | verify: `go test ./internal/cli/ -count=1` → ok, 74.5% (also clean under `-race`)
- [x] The existing restart name/plan tests still pass unchanged | verify: **binding corrected during disposition** — the recorded `git diff --stat internal/cli/restart_names_test.go` reads the *working tree* and prints nothing once the work is committed, so it certifies itself. The falsifiable form is `git diff --numstat master...HEAD -- internal/cli/restart_names_test.go` → `259 0`, and `git diff master...HEAD -- internal/cli/restart_names_test.go | grep -c '^-[^-]'` → `0`, i.e. no line removed from any pre-existing `TestRestart_` function
- [x] Confirmed against the built binary, not only the harness | verify: human — done, binary sha `e44e4357e7be390c`; the 4-verb table and every control row are reproduced in Verification below, in both the `e_tagged` and `e_twoplans` fixtures
- [x] `make test` passes | verify: `make test` → rc=0, 9 packages
- [x] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check` → rc=0, 0 broken links, 0 unmatched run patterns, cilabels OK, flowcheck OK

## References

- `internal/cli/compose.go:412-492` — `restartCmd` RunE, the guard-free path
- `internal/cli/compose.go:168` — `up`'s `rejectUnknownFlags` call, the form to copy
- `internal/cli/compose.go:261` — `teardownCommon`'s wholesale rejection used by down/stop
- `internal/cli/selectors.go:57` — `rejectUnknownFlags` signature
- `tasks/_archive/113-up-and-app-commands-swallow-unknown-flags.md` — same defect, fixed for the siblings
- `tasks/_archive/087-unrecognized-stack-args-become-entry-names.md` — the name-fallthrough half of the class

## Verification (2026-08-20)

Seven commits, `e9aeeea..6bdd2a0`. The guard itself is four lines; the rest is
what it took to be sure the four lines do only what they claim.

### The card's table, on the built binary (sha `e44e4357e7be390c`)

```
                                e_tagged            e_twoplans (2 plans, no default)
up       --zzznonsense          rc=1                --
down     --zzznonsense          rc=1                --
stop     --zzznonsense          rc=1                --
restart  --zzznonsense          rc=1  <- closed     rc=1
restart  s1 --zzznonsense       --                  rc=1
restart  --tag alpha            rc=0  s1 only
restart  --mode fast            rc=0  s1,s2
restart  --mode nosuchmode      rc=1
restart  s1                     rc=0  s1 only       rc=0  s1 only
restart  p1                     --                  rc=0  plan p1
restart  -- s1                  rc=0  s1 only
restart  --                     rc=0  nothing ran
restart  --no-wat -- s1         rc=1
```

### Three things the work found that the card did not say

**The card's premise about the plan path was wrong, and mine replacing it was
wrong too.** `e9aeeea` justified rejecting `--var`/`--no-wait` by claiming the
stack path means "no plans configured". `requirePlanSelection` returns nil as
soon as `planRoutingArgs` leaves any token behind, and it strips only `--debug`
and `--json`, so a leading flag counts; with two plans and no `default_plan`,
`DefaultPlan()` is `""` and `rejectSuppressedDefaultPlan` returns nil as well.
The stack path runs with plans present. Corrected in `c0f108e`, and the
conclusion survived the premise: `Wait` is hardcoded true there and there are no
plan variables, so the flags were being accepted and ignored either way.

**The defect was broader than the table above it.** With a name in front of the
typo — `restart s1 --zzznonsense` — master restarted s1 and exited 0 having
silently discarded the argument. That is worse than the documented form, which
at least prints `no lifecycle entries matched filters`, and nothing in the card
covered it.

**The fix changed two behaviours the commits did not mention, and both were
found by probing rather than reading.** `e9aeeea` broke `dva restart -- s1`
(rc=1 `unknown flag "--"`), fixed in `f95872b`. `f95872b`'s exemption then
deleted `--` from the name list, and an empty `Names` means every entry, so
`dva restart --` went from a no-op to a full stop+start of the whole stack —
the idiom `dva restart -- "$@"` with an empty `"$@"` escalating silently at exit
0. Reverted in `606a8c6`.

### Deliberate behaviour changes, stated

`dva restart s1 --no-wait` and `dva restart s1 --var K=V` were rc=0 (s1
restarted, flag ignored) and are now rc=1. The flags were never honoured on this
path; the change is from a silent ignore to an explicit refusal. Whether they
should instead warn and continue, as `dva up` does for `--var`, is not ruled
here.

`restart --help` now marks both `(plan only)` and states the rule. The error
message still explains the rejection as a name collision, which is wrong for a
flag the same help advertises — **TASK-209**.

### What review and sweep added

An independent sweep classified all 6 non-test `parseDvaFlags` call sites: 0
remain unguarded on this branch, 1 did on master and it was this card's. It also
found that five comments size that class at **12** call sites — correct at
`f4c83d7`, halved by `6710766` — which is **TASK-208**, and that
`restart zzznosuchservice` is still rc=0 with nothing run.

An independent adversarial review refuted two of the five claims it was asked to
attack, found the `--` escalation above as a P1, and showed that
`TestRestartTerminatorStillNamesEntries` passes on master too and so was
structurally unable to see it. It also found that the `--dry-run` rows of
`TestRestartAcceptsKnownFlagsAfterGuard` never reach the guard, because
`wrapWithHooks`' `consumeDryRunFlag` strips the flag before `RunE`'s body runs —
the comment blamed `parseDvaFlags`. Comment corrected in `606a8c6`; the
save/restore is still required.

### Left open, deliberately

`restart zzznosuchservice`, `restart -`, `restart --`, and a flag after `--` are
four tokens into one unmatchable-name path, all rc=0 with nothing or the wrong
thing done. **TASK-207** rules on all four together. This card did not decide any
of them, and `606a8c6` exists specifically so that it did not decide them by
accident: an earlier draft answered the `--` case as a side effect of slice
arithmetic, and got it wrong in the direction of doing more.

### Gates

`make test` rc=0 (9 packages, `-race`, cli uncached at `-count=1`, 74.5%),
`make lint` rc=0 (`go vet`, gofmt 285 files 0 unformatted, golangci-lint 0
issues), `make doc-check` rc=0 (0 broken links, 0 unmatched run patterns),
cilabels OK, flowcheck OK. All run with `PATH="$HOME/.local/share/mise/shims:$PATH"`.

One gate failure during the work was self-inflicted and is worth recording:
`go vet ./...` compiles `.go` files under the gitignored `tmp/`, so the A/B
scratch copies of `compose.go` broke `make test` and `make lint` with
`composeCmd redeclared`. Go scratch must live outside the module, not merely
outside git.

## Open Questions

- Should the empty selection itself be an error? `[warn] no lifecycle entries
  matched filters` with exit 0 is also how a legitimately empty tag filter
  reports. Fixing the flag guard leaves that behaviour untouched, which is the
  smaller change; whether an empty match should ever exit 0 is a separate
  ruling and is not assumed here.

## Technical Notes

The measurement that first contradicted this finding was wrong, and the reason
is worth recording. Under zsh an unquoted `$a` holding `--tag alpha` is passed
as **one** argument, not two, so `dva restart "--tag alpha"` took the
unknown-flag path and looked like proof that `--tag` was unparsed. The controls
above were re-run passing arguments individually. Any future sweep here must do
the same — see the same trap recorded against the `examples/*.yml` sweep in
TASK-197.

The fixture used for every measurement in this card:

```yaml
# tmp/planroute/e_tagged/dva.yml
version: "0.1.44"
modes:
  fast:
    description: a real declared mode
stack:
  s1:
    tags: [alpha]
    default_runner: native
    runners: {native: {run: "sleep 1"}}
  s2:
    tags: [beta]
    default_runner: native
    runners: {native: {run: "sleep 1"}}
```

Note `plans:` entries are `PlanEntry` structs, not strings — `entries: [s1]`
fails to parse with `cannot unmarshal !!str s1 into config.PlanEntry`. The
plan-path fixtures use `entries:\n      - name: s1`.
