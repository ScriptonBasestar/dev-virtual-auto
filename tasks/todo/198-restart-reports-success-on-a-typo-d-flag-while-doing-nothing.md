---
id: TASK-198
title: "`dva restart` reports success on a typo'd flag while doing nothing"
type: fix
priority: P2
effort: S
created-at: 2026-08-19T17:36:18+09:00
source: "found reviewing a843c74's help-text corrections — measured 1 of the 4 lifecycle verbs exits 0 on an unknown flag, against a 3-verb control that exits 1"
scope: "internal/cli/compose.go restartCmd RunE (412-492): add the guard up already has at :168. No change to which flags restart accepts."
status: todo
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

- [ ] A regression test named `TestRestartRejectsUnknownFlag` exists beside the sibling restart tests | verify: `grep -c 'func TestRestartRejectsUnknownFlag' internal/cli/restart_names_test.go` returns 1 (today: the file exists, the function does not)
- [ ] That test asserts the message names the flag, as the siblings do | verify: `grep -A20 'func TestRestartRejectsUnknownFlag' internal/cli/restart_names_test.go | grep -c 'unknown flag'` returns ≥ 1
- [ ] The whole cli package passes with it, so nothing over-rejects | verify: `go test ./internal/cli/ -count=1`
- [ ] The existing restart name/plan tests still pass unchanged | verify: `git diff --stat internal/cli/restart_names_test.go` shows additions only — no line removed from a `TestRestart_` function that predates this card
- [ ] Confirmed against the built binary, not only the harness | verify: human — rebuild, recreate the fixture in Technical Notes, and re-run the 4-verb `--zzznonsense` table from Summary; `restart` must join the other three at rc=1 while `--tag alpha`, `--mode fast` and `s1` keep their rc=0 rows
- [ ] `make test` passes | verify: `make test`
- [ ] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## References

- `internal/cli/compose.go:412-492` — `restartCmd` RunE, the guard-free path
- `internal/cli/compose.go:168` — `up`'s `rejectUnknownFlags` call, the form to copy
- `internal/cli/compose.go:261` — `teardownCommon`'s wholesale rejection used by down/stop
- `internal/cli/selectors.go:57` — `rejectUnknownFlags` signature
- `tasks/_archive/113-up-and-app-commands-swallow-unknown-flags.md` — same defect, fixed for the siblings
- `tasks/_archive/087-unrecognized-stack-args-become-entry-names.md` — the name-fallthrough half of the class

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
