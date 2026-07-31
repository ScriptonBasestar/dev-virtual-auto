---
id: TASK-021
title: "Document CLI flags and fix the --site heading that names a nonexistent flag"
type: docs
priority: P2
status: done
archived-at: 2026-07-16T21:55:00+09:00
verified-at: 2026-07-16T21:55:00+09:00
verification-summary: >-
  Verified by orchestrator: --site no longer named as a flag in any heading and is still
  absent from the binary (heading fixed, flag not added); --no-wait/--exclude-tag/--force/
  --template/--key/--user/--volume/--var all documented; make build, make test, go vet green.
  This task's own premise was corrected during implementation: the siblings do NOT parse the
  same flags as `up`. --force/--no-wait come from an inline switch in upCmd, and down/stop
  route through teardownCommon, which errors on leftover args - `dva down --force` fails.
  Copying up's block verbatim (as this task suggested) would have documented broken flags.
  Each sibling's new Long block lists only the four flags parseDvaFlags actually provides;
  spot-checked that `dva down --tag x` really works. Flag sets are split by routing
  (nameless vs named-plan), since detectPlanRoute makes them genuinely differ.
effort: S
created-at: 2026-07-16T21:45:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: rediscovery-phase-1
source-severity: MEDIUM
---

# Task 021: Document CLI Flags

## Summary

CLI **flags** were a declared blind spot of the first audit ("coverage denominator
unknown"). A fresh audit closed it: of 29 DVA-specific flags, 22 are documented, 8 are
binary-only, and 1 is documented but does not exist.

## Evidence

**Denominator note:** the honest denominator is *DVA-specific* flags, not "flags the
binary accepts" — passthrough commands forward unrecognized flags to docker/kubectl, so
the accepted set is unbounded and any percentage against it would be fiction. The 29 were
enumerated from the three real parsers (`parseDvaFlags` compose.go:456, `parsePlanFlags`
plan_lifecycle.go:40, inline switches) plus cobra registration.

| Metric | Count |
| ------ | ----- |
| Binary total (DVA-specific) | 29 |
| Documented | 22 |
| Binary-only | 8 |
| Doc-only | 1 |

### Doc-only — `--site` does not exist (MEDIUM)

`USAGE.md:194` heading: ``#### `--env` / `--site` / `--var``` — but:

```
$ dva run --site foo test
ERROR: unknown flag: --site        (exit 1)
```

No `--site` handling exists in code (only YAML field references). The section **body** is
correct — it describes `site` as a `plans.<name>` field and shows only `--var` as a real
flag — so the defect is confined to the heading conflating YAML fields with CLI flags.

### Binary-only — undocumented flags (MEDIUM/LOW)

Verified counts (`grep` over USAGE.md + README.md vs `dva up --help`):

| Flag | In `up --help` | In docs | Severity |
| ---- | -------------- | ------- | -------- |
| `--no-wait` | yes | **no** | MEDIUM |
| `--exclude-tag(s)` | yes | **no** | MEDIUM |
| `--force` | yes | **no** | MEDIUM |
| `ssh up --key/--user/--volume` | yes | **no** | MEDIUM |
| `init --template` | yes | **no** | MEDIUM — USAGE.md:41-43 lists init's *other three* flags, reading as exhaustive |
| `--format` (ls/manifest/config show), `ls --detailed` | yes | no | LOW — the documented global `--json` covers the same ground |

Severity is MEDIUM, not HIGH: these flags **are** discoverable via `dva up --help`, which
carries a complete hand-written "DVA-specific flags:" block. They are undocumented, not
undiscoverable.

### Structural observation

`dva up --help` carries that full flag block; its siblings that parse the **same** flags —
`down`, `stop`, `restart`, `stack up/down`, `app up`, `compose` — carry none. Copying the
block to those siblings would close the discoverability half at its root rather than
per-flag.

## Out Of Scope

- Adding a real `--site` flag — that is a feature, not a doc fix. Fix the heading.
- Passthrough flags forwarded to docker/kubectl (unbounded, not DVA's contract).
- cobra built-ins (`--help`, `-h`, `completion`).

## Completion Criteria

- [x] `USAGE.md:194`'s heading no longer names `--site` as a flag | verify: `! grep -nE '^#+ .*--site' USAGE.md`
- [x] `--site` is still absent from the binary (heading fixed, not flag added) | verify: `! ./bin/dva run --site foo test 2>&1 | grep -q 'unknown flag' && exit 1 || exit 0`
- [x] `--no-wait`, `--exclude-tag`, `--force` are documented | verify: `for f in -- --no-wait --exclude-tag --force; do grep -rqF -- "$f" USAGE.md README.md || { echo "MISSING $f"; exit 1; }; done; echo OK`
- [x] `ssh up` flags and `init --template` documented | verify: `grep -qF -- "--template" USAGE.md && grep -qF -- "--key" USAGE.md`
- [x] Docs match the binary (no newly-invented flag) | verify: `./bin/dva up --help | grep -q 'no-wait'`

## References

- `evidence-flags.md` (gap-analysis working set, 2026-07-16, untracked) — full set comparison
- [016-document-missing-surface.md](../_archive/016-document-missing-surface.md) — command paths (this task covers flags)
