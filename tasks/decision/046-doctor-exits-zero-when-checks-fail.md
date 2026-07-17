---
id: TASK-046
title: "DECISION: should 'dva doctor' exit non-zero when its checks fail?"
type: bug
priority: P3
status: decision
needs-human: true
effort: S
created-at: 2026-07-17T08:15:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (schema.json examples lens, doctor probe)
source-severity: LOW
moved-at: 2026-07-17T10:55:00+09:00
---

# Task 046: Doctor Says "3 failed" To Humans And "Success" To Scripts

## Summary

`dva doctor` reports failing checks to the screen and **always exits 0**. Its `RunE`
(`internal/cli/doctor.go:38-53`) returns `nil` unconditionally on the printing path — the check
results are rendered and then discarded, never consulted for the exit code.

The command's own help text (`doctor.go:33-37`) recommends it as a **pre-flight gate**:

> `Useful for diagnosing setup problems before running 'dva up' or 'dva provision'.`

That is precisely the workflow its exit code makes impossible. `dva doctor && dva up` proceeds to
`dva up` no matter how many prerequisites failed.

## Evidence — measured at `c6c8447`

Probe (`dva validate` EXIT=0 on it, so the liveness gate holds):

```yaml
version: "0.1.0"
checks:
  - name: "Impossible check"
    type: command
    command: "exit 7"
    fix_hint: "this check can never pass"
```

```
$ dva doctor
Environment Checks:

  [FAIL] Docker socket accessible
         -> Docker not running or socket path incorrect
  [pass] Docker daemon accessible
  [pass] Compose project name alignment
  [FAIL] Impossible check
         -> this check can never pass
  [FAIL] .sb/dva/ is ignored in .gitignore
         -> Create .gitignore and add '.sb/dva/' to avoid committing transient state

  2 passed, 3 failed
DOCTOR_EXIT=0            # <-- three failures, including one that can never pass
```

### The control — doctor's exit code IS plumbed and DOES work

This is the part that makes the finding decisive rather than a guess about cobra. Run in a directory
with **no `dva.yml` anywhere in its ancestry** (verified by walking every parent to `/`):

```
$ dva doctor
ERROR: could not find dva.yml (searched from .../iso-nocfg to /).
  Hint: run 'dva init' or set DVA_FILE=/path/to/dva.yml
DOCTOR_EXIT=1            # <-- non-zero. The wiring is fine.
```

So "doctor can never fail" is **false**. `dva doctor` exits 1 when it cannot *find* its config, and
exits 0 when the checks it exists to run all fail. The exit code reports infrastructure problems and
stays silent about the one thing the command is for.

**A note on how this control was reached, because the first attempt was vacuous.** The first control
ran `dva doctor` in an empty subdirectory *of the probe dir* and also got EXIT=0 — which looked like
"doctor never exits non-zero" but proved nothing: config discovery walks **up**, so it found the
parent's `dva.yml` and ran a normal check pass. Only an isolated directory with no `dva.yml` ancestor
distinguishes "no config" from "config with failures".

## Why it matters

Same family as TASK-041 and the same organizing theme as this whole sweep — **a green surface that
certifies nothing**. A human reading the terminal sees `2 passed, 3 failed` in red. A script, a
Makefile, or a CI job sees success.

The concrete failure: `dva doctor && dva up` — the exact composition the help text proposes — runs
`dva up` against an environment doctor just declared broken. The `dva up` failure then surfaces
somewhere downstream, with an error that does not mention the prerequisite that was already known to
be missing. A pre-flight check that cannot fail the flight is decoration.

## Severity: LOW / P3

Not higher: the information is not lost, only unusable by machines — it is printed clearly, in full,
with fix hints. A human running `dva doctor` interactively (the dominant use) is correctly informed.
`--json` output (`doctor.go:47-49`) also carries per-check `passed` values, so a determined scripter
can already gate on `dva doctor --json | jq`. So there is a workaround, and nothing is silently
skipped or destroyed.

Not lower: the help text explicitly proposes the broken composition, so this is not a missing feature
someone might want — it is the documented workflow not working.

## Scope note — needs a decision, and it is the same decision as TASK-041

This is a **contract question about diagnostic commands**, not a bug with an obvious fix, and it
should not be settled unilaterally per-command:

- **Exit non-zero on failed checks** — matches `brew doctor`, `flutter doctor --machine` style
  gating, and makes the advertised `dva doctor && dva up` work. **Breaking change**: any existing
  script or CI job running `dva doctor` today succeeds unconditionally and would begin failing. Note
  the probe above: a stock run FAILs `Docker socket accessible` and the `.gitignore` check on a
  perfectly ordinary machine, so a naive "non-zero if any FAIL" would fail constantly and get
  `|| true`'d into meaninglessness within a week.
- **Keep exit 0 (advisory)** — then the help text must stop proposing it as a gate, and the
  `--json`/`jq` path should be documented as the supported way to gate.
- **Middle ground** — non-zero only when a **user-defined** check from `checks:` fails, leaving
  built-in environment observations advisory. This preserves the `checks:` contract (the user
  explicitly declared these as prerequisites) without failing on an unrelated `.gitignore` nit. Or:
  gate behind a `--strict`/`--exit-code` flag, which is additive and breaks nothing.

**Decide this together with TASK-041** (`dva stack status` exit code), which is the identical
question for a different command. Two commands answering "should a diagnostic exit non-zero when it
reports a problem?" differently would be worse than either answer applied consistently. TASK-041 is
open, P3, `needs-human`, and unstarted — this task is deliberately filed separately rather than
folded in, because 041's scope is `stack status` and was committed as such, but the two should be
resolved in one sitting.

The lean, weakly: **middle ground** — user-defined `checks:` failures are the strongest candidate for
a real exit code, because `checks:` exists precisely so the user can declare "these must hold". But
the flag-vs-default and the back-compat break are a maintainer's call.

## Completion Criteria

- [ ] DECISION recorded, jointly with TASK-041: what is DVA's exit-code contract for diagnostic commands | verify: `human — maintainer picks one of exit-non-zero / advisory / middle-ground and records why; 041 and 046 must not diverge`
- [ ] If EXIT-NON-ZERO or MIDDLE-GROUND: the built-in noise problem is addressed | verify: `human — a stock 'dva doctor' FAILs 'Docker socket accessible' and the .gitignore check on an ordinary machine; assert the chosen rule does not make failure the default state`
- [ ] If EXIT-NON-ZERO or MIDDLE-GROUND: the back-compat break is acknowledged | verify: `human — scripts running 'dva doctor' today always succeed; confirm the change is intended and note it in the changelog`
- [ ] If ADVISORY: the help text stops proposing 'dva doctor' as a pre-'dva up' gate | verify: `human — doctor.go:33-37 currently says "Useful for diagnosing setup problems before running 'dva up'"; assert it no longer implies a gate, and documents the --json path instead`
- [ ] A regression test pins the chosen contract, proven to fail without the change | verify: `human — reproduce the probe: a check with 'command: exit 7' must produce the decided exit code; revert the change, confirm the test FAILS, restore, confirm it passes`
- [ ] The control still holds: doctor still exits non-zero when dva.yml cannot be found | verify: `human — run 'dva doctor' in a directory with no dva.yml in any ancestor; must remain EXIT=1 and must not be confused with a checks failure`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [041-status-exit-code-on-unrunnable-entry.md](./041-status-exit-code-on-unrunnable-entry.md) — **the same question for `dva stack status`; decide both together**
- [045-doctor-check-fix-implemented-but-schema-forbids-it.md](./045-doctor-check-fix-implemented-but-schema-forbids-it.md) — found in the same probe; the other `dva doctor` contract gap
- [044-legacy-structured-provision-shell-sleep-docker-inert.md](./044-legacy-structured-provision-shell-sleep-docker-inert.md) — same theme: a green surface certifying work that did not happen
