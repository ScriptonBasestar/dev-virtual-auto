---
id: TASK-021
title: "Document CLI flags and fix the --site heading that names a nonexistent flag"
type: docs
priority: P2
status: todo
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

- [ ] `USAGE.md:194`'s heading no longer names `--site` as a flag | verify: `! grep -nE '^#+ .*--site' USAGE.md`
- [ ] `--site` is still absent from the binary (heading fixed, not flag added) | verify: `! ./bin/dva run --site foo test 2>&1 | grep -q 'unknown flag' && exit 1 || exit 0`
- [ ] `--no-wait`, `--exclude-tag`, `--force` are documented | verify: `for f in -- --no-wait --exclude-tag --force; do grep -rqF -- "$f" USAGE.md README.md || { echo "MISSING $f"; exit 1; }; done; echo OK`
- [ ] `ssh up` flags and `init --template` documented | verify: `grep -qF -- "--template" USAGE.md && grep -qF -- "--key" USAGE.md`
- [ ] Docs match the binary (no newly-invented flag) | verify: `./bin/dva up --help | grep -q 'no-wait'`

## References

- [evidence-flags.md](../../tmp/gap-analysis/evidence-flags.md) — full set comparison
- [016-document-missing-surface.md](../_archive/016-document-missing-surface.md) — command paths (this task covers flags)
