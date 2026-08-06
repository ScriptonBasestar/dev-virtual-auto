---
id: TASK-179
title: "`config validate` passes and `manifest` publishes four `start` commands that no mode can reach"
type: bug
priority: P2
status: todo
effort: M
created-at: 2026-08-05T17:46:00+09:00
source: "dogfood run 20260805-143543-f82daf stage 10 — the run's owner finding, DVA-007"
scope: "dva repo — internal/config/validate_warnings.go, internal/cli/manifest.go"
---

# Task 179: Say that a top-level health check with no mode to reach it will never run

## Problem

Top-level `health_checks.<name>.start` executes through exactly one path, and that path is
gated three times on a section the config need not declare:

```go
// internal/lifecycle/orchestrator.go:428
func (o *Orchestrator) startModeProcesses(...) error {
	if opts.Mode == "" { return nil }                        // gate 1
	mode, ok := o.cfg.Modes[opts.Mode]
	if !ok || len(mode.HealthChecks) == 0 { return nil }     // gates 2, 3
	...
	hc, ok := o.cfg.HealthChecks[hcName]                     // :445 — never reached
```

`signalModeProcesses` (`:548`) gates identically for the down/stop half. Every other reader of
`c.HealthChecks` is display (`cli/show.go:299,477`, `cli/manifest.go:396`) or merge
(`config/merge.go:50,452`). The one validation-side reader,
`warnHealthCheckRedundancy` (`validate_warnings.go:323`), checks `start` against `start_hint`
for redundancy and never asks whether anything can reach the entry.

The nested `stack.<entry>.health_checks` is a **different field** (`config/lifecycle.go:40`),
read un-gated at `orchestrator.go:148,328`. A config can therefore have a live check and a dead
twin under the same service name, and nothing distinguishes them.

Measured against `flow-pipechain-devbox` on the installed `0.1.44` / `488fc19`
(`sha256 ce8c3be9…`), a config with four `health_checks.*.start` entries and no `modes:`:

| | result |
|---|---|
| `dva config validate` | exit 0, `✅ dva.yml is valid` + 4 unrelated Makefile suggestions. **0** lines naming health / start / any of the four services |
| non-vacuity control | the same pattern matches **5×** in `dva.yml` itself, so that 0 is a real zero |
| `dva config show -f yaml` | line 1151: `modes: {}`, `default_mode: ""` |
| `dva manifest -f json` | publishes all four `health_checks` **including their `start` keys**, and has no `modes` key at all |

The machine-readable surface is the worse half: it advertises four runnable commands while
omitting the only section that can run them. A consumer reading the manifest has no way to tell
these four from four that work.

## Acceptance criteria

- [ ] `dva config validate` names every `health_checks.<name>` that declares `start` (or
      `start_hint`) when no `modes.*.health_checks` entry references it, and says why it will not
      run. A warning, not an error — the config is not malformed.
      Verify: `go test ./internal/config/ -run HealthCheck -count=1`
- [ ] Negative control: a config that **does** declare `modes:` referencing those checks
      validates clean, with no new warning. Without this the fix trades a false negative for a
      false positive.
      Verify: `go test ./internal/config/ -run HealthCheck -count=1`
- [ ] A config with `modes:` declared but a mode referencing a `health_checks` name that does not
      exist is covered by the same pass, or the decision to leave it alone is recorded with its
      reason.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] `dva manifest -f json` either marks unreachable health checks or stops publishing their
      `start`. Decide which and say why: a consumer that scripts against the manifest currently
      cannot tell a live `start` from a dead one.
- [ ] The nested `stack.*.health_checks` form draws **no** warning from this pass. It is a
      different field on a different code path and is reached un-gated.
- [ ] Corpus: report how many configs under `examples/` declare a top-level `health_checks` with
      `start` and no `modes:`, including zero, measured from `dva config validate` output rather
      than grep.
- [ ] `make test` exits 0.

## References

- `internal/lifecycle/orchestrator.go:428-445`, `:548-561` — the gating, both halves
- `internal/config/validate_warnings.go:319-346` — the only validation-side reader
- `internal/config/lifecycle.go:40` — the nested field this must not touch
- `internal/cli/manifest.go:396` — the publishing site
- Baseline evidence:
  `flow-pipechain-devbox/tmp/dogfood-dva/20260805-143543-f82daf/stages/10-baseline/20260805-164343-c571/report.md`

## Notes

Found as the owner finding of dogfood run `20260805-143543-f82daf`, where it is being carried
through the improve → forward-test → evaluate stages. Filing it here so the defect outlives the
run directory, which is `tmp/` and git-ignored.

The competing reading — "the target project should just delete the block" — was considered and
rejected: deleting it from one `dva.yml` leaves every other project with the same silent config.
The target-side half (those four dead commands have drifted from their live twins and omit `.env`
sourcing) survives this fix and is the target repository's to fix, not this one's.

Same shape as [TASK-137](../done/137-manifest-advertises-the-unroutable-namespaced-form-with-no-mark.md)
— the manifest advertising a form nothing can route — one section over.
