---
id: TASK-037
title: "DECISION: should devcontainer.config_path be honored, or deleted from the schema?"
type: bug
priority: P3
status: decision
effort: M
created-at: 2026-07-17T04:10:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: TASK-034 (split out so the decision is not buried in an archived file)
source-severity: LOW
needs-human: true
moved-at: 2026-07-17T10:55:00+09:00
---

# Task 037: `devcontainer.config_path` — Honor It Or Delete It

## Summary

`devcontainer.config_path` is in `schema.json`, validates green, and does nothing. TASK-034 fixed
the half that needed no decision (it was leaking into the generated `devcontainer.json`, where it
is not a spec key). What remains is a genuine product question that an implementer must not answer
unilaterally.

**The field currently has no effect at all.** It is dead schema surface either way; the question is
which direction to resolve it.

## The question

Should DVA **honor** `config_path` — writing the generated devcontainer to the configured location —
or **remove it from `schema.json`** as surface that has never worked?

## Evidence (established by TASK-034, still true at HEAD)

The write path hardcodes the location — `internal/cli/validate.go:55`:

```go
dcPath := filepath.Join(c.FileDir(), ".devcontainer", "devcontainer.json")
```

`grep -rn "\.ConfigPath\b" --include="*.go" internal/ | grep -v _test.go` → **zero** non-test reads.
(Control: the same grep shape finds `.Tags` consumed at `orchestrator.go:366`.)

Probe at HEAD, `dva validate` exiting 0 first so it is not vacuous: setting
`config_path: custom/somewhere/devcontainer.json` still writes `.devcontainer/devcontainer.json` and
never creates `custom/`.

## What each option costs

**Honor it** — the path is hardcoded in at least three places that must all agree, so this is not a
one-liner:

- `validate.go:55` (the `--fix` write path)
- `doctor.go:100-105` (the "devcontainer.json exists" check, `Path: ".devcontainer/devcontainer.json"`)
- `init.go:57` (the init-time creation message and path)

It also raises questions this task must answer before implementation: is `config_path` relative to
the project root or to `dva.yml`'s directory? `generateDevcontainerJSON` already rewrites relative
compose paths with `../` on the assumption that output lives in `.devcontainer/`
(`toDevcontainerRelative`, and the comment at `devcontainer.go:58`: "devcontainer.json paths are
relative to `.devcontainer/`"). A configurable output directory **breaks that assumption** — the
`../` prefix is only correct for one specific location. Honoring `config_path` therefore means
fixing `toDevcontainerRelative` to compute the prefix from the actual output directory, not
assuming one level down.

**Remove it** — cheap and honest. Deletes a key nobody can be relying on, since it has never had an
effect. Risk is limited to configs that currently set it, which today are silently no-ops; after
removal they would fail validation, which is the point.

## Recommendation (not a decision — the maintainer's call)

Lean **remove**. Nothing has ever consumed it; the devcontainer ecosystem strongly expects
`.devcontainer/devcontainer.json` at a conventional location (VS Code and the devcontainer CLI
discover it by convention); and honoring it drags in a real correctness fix to
`toDevcontainerRelative` in exchange for a capability no evidence suggests anyone wants. If someone
does want a custom location, that is better filed as a feature request with a concrete use case
than preserved as schema surface that has never worked.

Recorded as a recommendation because this run's rule is that decisions are surfaced, not assumed.

## Completion Criteria

- [ ] DECISION recorded: honor or remove | verify: `human — a maintainer picks one and records why`
- [ ] If REMOVE: `config_path` is gone from `schema.json`, and a config using it now fails validation naming the key rather than silently ignoring it | verify: `human — probe a config with config_path; assert dva validate exits non-zero and the message names config_path`
- [ ] If REMOVE: `dvaOnlyDevcontainerKeys` in `internal/cli/devcontainer.go` drops the now-impossible `config_path` entry, or keeps it deliberately for back-compat with a comment saying so | verify: `human — confirm the key set matches the schema`
- [ ] If HONOR: the generated file lands at the configured path, proven with a control showing the default still lands at `.devcontainer/` | verify: `human — probe both; assert the configured path is created and the default is unchanged`
- [ ] If HONOR: `validate.go:55`, `doctor.go:100-105`, and `init.go:57` all agree on the location — no path left hardcoded | verify: `human — assert dva doctor reports green against a custom config_path, which it cannot do today`
- [ ] If HONOR: `toDevcontainerRelative`'s `../` prefix is computed from the real output directory, not assumed | verify: `human — set config_path two directories deep with a relative compose file; assert the emitted dockerComposeFile path resolves correctly`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [034-devcontainer-config-path-ignored-and-leaks.md](../_archive/034-devcontainer-config-path-ignored-and-leaks.md) — fixed the leak; explicitly deferred this question
- [035-env-file-interpolate-and-priority-ignored.md](./035-env-file-interpolate-and-priority-ignored.md) — same honor-vs-remove shape for `env_file.interpolate`/`priority`
- [036-service-related-and-hint-ignored.md](./036-service-related-and-hint-ignored.md) — same shape for `services.<svc>.related`/`hint`
