---
id: TASK-285
title: "Close the migrate/validate coverage gap and correct the Stage A exit-code claim"
type: bug
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-03T19:55:00+09:00
source: "Independent review of TASK-266 Stage A (`c6aa64b`) by a reviewer that did not write it; both findings re-measured by the TASK-266 implementer against a fresh `make build` of master at 916b07e"
scope: "internal/config/migrate_report.go coverage and its doc comment, the CHANGELOG and commit-message claim that Stage A changes no runtime behaviour"
status: todo
depends-on: []
---

# Task 285: close the migrate/validate coverage gap

## Summary

`dva config migrate` reads one raw document; `dva config validate` runs on the merged `Config`.
A deprecation declared in a module is therefore reported by `validate` and invisible to
`migrate` — including to `migrate`'s "Left for you" list, which is the surface a user goes to
for the exact edit to make. The gap predates Stage A; Stage A is the first check to expose it,
because it is the first one whose subject can arrive through a merge.

Separately, Stage A states that it changes no runtime behaviour and that exit codes are
unchanged. `dva config validate --strict` now exits 1 on a config whose only issue is an
`interaction.*.env_file` declaration. That is a real exit-code change for anyone who runs
`--strict` in CI, and it is not written down anywhere a user would look.

Neither finding is a defect the Stage A diff introduced. Both are things it made reachable.

## Problem

Measured on a fixture whose `dva.yml` declares `modules: [extra]` and whose
`.sb/dva/extra.yml` holds the only `interaction:` block:

```
$ dva config validate
[warn] semantic: interaction.greet: 'env_file' is inert and will be rejected in 0.1.49 — ...
✅ dva.yml is valid                                                              rc=0

$ dva config migrate
dva.yml: nothing to convert.
Run 'dva validate' for the deprecations this command does not convert.           rc=0
```

1. **`migrate` reports nothing and names no file.** There is no "Left for you" section at all,
   although one appears for the identical declaration written inline. The user is pointed back
   at `validate`, which names the dotted path `interaction.greet` — a path that exists in the
   merged view and in no file the user can open. `.sb/dva/extra.yml`, the file that actually
   has to be edited, is never named by either command.

   `ReportInteractionEnvFile`'s doc comment (`internal/config/migrate_report.go`) describes the
   check without stating that its input is the raw document, so the limitation reads as
   coverage.

2. **`--strict` exits 1 where it previously exited 0.**

   ```
   $ dva config validate --strict          (config with plans; env_file the only issue)
   [warn] semantic: interaction.greet: 'env_file' is inert and will be rejected in 0.1.49 — ...
   ERROR: config warnings detected; review warnings above or run 'am run dva-improve'
   rc=1
   ```

   The commit message says "Exit codes are unchanged — ValidateWarnings has exactly one call
   site, so `dva run`, lifecycle verbs, `doctor` and `show` stay silent structurally." That is
   accurate for the five surfaces it names and it does not name `--strict`, which converts
   warnings to a failure by design. The claim is true of the surfaces enumerated and false as
   the general statement the CHANGELOG entry reads as.

## Direction

For §1, the honest minimum is to stop implying coverage: say in the doc comment and in
`migrate`'s output that the report covers the document it was given, and point at the merged
view for the rest. Making `migrate` walk the merged config is the larger option and drags in
the question of which file an edit lands in, which is a decision, not a repair — do not take it
without saying so.

For §2, add a CHANGELOG line naming `--strict` as the surface whose exit code changes in
0.1.48. This is a documentation repair; the `--strict` behaviour itself is correct and should
not change.

Two adjacent warning-duplication behaviours were examined in the same review and are **not** in
scope, because both are inherited from `eachInteractionNode` and reproduce with the pre-existing
inert-step warning: a subproject imported under an `as:` alias yields two warnings at two paths,
and a YAML merge key yields two warnings where only one has an editable line. If they are worth
fixing they belong on their own card against the walker, not here.

## Completion Criteria

- [ ] `migrate` no longer implies it covered declarations it never read — its output or its doc comment states that the report is scoped to the document it parsed | verify: `go test ./internal/config -count=1`
- [ ] A module-declared `interaction.*.env_file` is either reported by `migrate` or explicitly described as out of its reach; it is not silently absent from "Left for you" | verify: `go test ./internal/config -count=1`
- [ ] The CHANGELOG names `dva config validate --strict` as a surface whose exit code changes in 0.1.48 | verify: `human — read the 0.1.48 CHANGELOG entry and confirm --strict is named`
- [ ] A test pins the module-declared case so the gap cannot close and silently reopen | verify: `go test ./internal/config -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No change to Stage B, which is release-gated behind 0.1.48 and owned by
  [TASK-266](266-deprecate-and-reject-interaction-env-file.md).
- No change to `--strict`'s warnings-are-failures behaviour, which is correct.
- No change to `eachInteractionNode`'s duplicate-path walking, for the reason given above.
