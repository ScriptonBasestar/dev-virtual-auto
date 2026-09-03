---
id: TASK-283
title: "Repair the plan-route flag suggestion, which still dead-ends and now hides --dry-run"
type: bug
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-03T19:30:00+09:00
source: "Independent review of TASK-273 (`206918a`) by a reviewer that did not write it; every claim below re-measured by the TASK-273 implementer against a fresh `make build` of master at 916b07e"
scope: "internal/cli/plan_lifecycle.go rejectSuppressedDefaultPlan and stripStackPathOnlyFlags, the logs route in internal/cli/compose.go, the manifest entries the message echoes"
status: todo
depends-on: []
---

# Task 283: repair the plan-route flag suggestion

## Summary

[TASK-273](../done/273-repair-misleading-cli-guidance.md) set out to stop the CLI proposing a
command the plan path rejects. It narrowed the defect and did not close it. Four inputs still
produce a suggestion that fails when followed, and a fifth — the one that matters most — now
produces a suggestion that **succeeds when it should not**: a user who asked for `--dry-run` is
handed a command that executes for real.

The last one is a regression. Before `206918a` that input produced a suggestion which failed
loudly and ran nothing. The narrowing converted a loud failure into a silent wrong action.

## Problem

All five measured against `bin/dva` from `make build` at `916b07e`, on two fixtures: `fx`
declares one `script` entry `web` tagged `app`, one plan `p1`, and `default_plan: p1`; `np` is
the same file with the `plans:` and `default_plan:` keys removed.

1. **`--dry-run` is dropped from the suggestion, so the suggestion runs for real.** (severity:
   highest — this is the regression)

   `--dry-run` is a root persistent flag. `consumeRootPersistentFlags` removes it from `args`
   before `rejectSuppressedDefaultPlan` ever sees them, so the guard cannot echo it back and
   does not know it was typed.

   ```
   $ dva up --tag app --dry-run                                    (fx)
   ERROR: flags suppress the default plan "p1"; --tag works only on the whole-stack path
   and the plan path rejects it, so name the plan without it: dva up p1
   rc=1
   $ dva up p1                                                     (following the suggestion)
   [plan: p1] environment= site= entries=1
   [lifecycle] web (script)
     [web] script
   rc=0
   ```

   The user asked to preview and was told, by DVA, to run a command that started the entry.
   Pre-`206918a` the same input suggested `dva up p1 --tag app`, which exits 1 and starts
   nothing. `--debug` and `--json` travel the same consumer and are lost the same way; they are
   not dangerous, but they are equally silently discarded.

2. **`logs` claims a whole-stack path it does not have, and its suggestion silently drops the
   filter.**

   ```
   $ dva logs --tag app                                            (fx)
   ERROR: ... --tag works only on the whole-stack path and the plan path rejects it,
   so name the plan without it: dva logs p1
   rc=1
   $ dva logs --tag app                                            (np — the whole-stack path)
   unknown flag: --tag
   rc=1
   ```

   `logs` consumes root flags at `internal/cli/compose.go:859` and never calls `parseDvaFlags`,
   so `--tag` works on **neither** path. The message's central claim is false for this command.
   The manifest agrees with reality and contradicts the message: the `logs` entry
   (`/usr/bin/grep -n '"logs":' internal/cli/manifest.go`) carries no Options at all. TASK-273's
   own criterion required the runtime message and the manifest to agree; on `logs` they do not.

   The suggestion is also worse than a dead end here. `dva logs p1` exits 0 and prints logs for
   the whole plan — the filter the user asked for is gone and nothing says so.

3. **`restart`'s positional entry name is stranded.** `restart` is the one plan-route verb with
   a positional entry slot, and the strip walk passes non-selector tokens through untouched.

   ```
   $ dva restart --tag app web                                     (fx)
   ERROR: ... so name the plan without it: dva restart p1 web
   rc=1
   $ dva restart p1 web
   ERROR: unexpected argument in plan mode: web
   rc=1
   ```

   The control proves the input is a real working invocation and not a typo:

   ```
   $ dva restart --tag app web                                     (np)
   [lifecycle] stopping web (script) / [lifecycle] web (script)    rc=0
   ```

   This is the original TASK-273 defect, unchanged, on the one route the fix did not consider.

4. **A flag-shaped selector value is left behind as a stray positional.**

   ```
   $ dva up --tag -5     → suggests: dva up p1 -5      → the plan route rejects -5
   ```

   `stripStackPathOnlyFlags` deliberately declines to consume a following flag-shaped token
   (see its doc comment), which is right for `dva up --tag --no-wait` and wrong here: `-5` is
   not a flag the plan path accepts, it is the malformed value of the flag just removed.

5. **Two tokens vanish and one is stranded.**

   ```
   $ dva up --tag -T web → suggests: dva up p1 web
   ```

   `-T` is a recognized selector so the walk consumes it as `--tag`'s value and removes it
   without reporting it; `web` — which was `-T`'s value — is left as a positional the plan route
   rejects. `parseDvaFlags` would have reported `--tag requires a value, got the flag -T`. The
   suggestion reports neither.

## Why the existing test did not catch any of this

`TestSuppressedDefaultPlanSuggestionRuns`
(`internal/cli/plan_path_flag_guidance_test.go:132`) was written for exactly this property — it
executes the printed suggestion and fails if it errors. It passes on all five inputs above, and
each miss is a separate hole in the same table:

- **`withDryRun` assigns the package-level `dryRun` variable directly** (line 96-99) rather than
  putting `--dry-run` into the args the guard reads. So the guard is never given the flag, the
  suggestion is never asked to carry it, and every suggestion the test runs is a dry run by
  construction. §1 — a suggestion that acts when the user asked to preview — is not merely
  uncovered; it is structurally unobservable through this helper.
- **`planAwareLifecycleCommands`** (line 88-93) holds `up`, `down`, `stop`, `restart` only.
  `logs`, `build` and `status` are absent, so §2 is out of reach.
- **`stackPathOnlyCases`** (line 74-86) is eight well-formed selector pairs with nothing after
  them. No case has a trailing positional (§3) and none has a flag-shaped value (§4, §5).

The test is not weak in principle — running the suggestion is the right assertion. It is scoped
to the inputs the implementer already had in mind. Widening the two tables and fixing the
dry-run helper is most of this card's work.

## Direction

The card does not prejudge the mechanism, but it does fix the property, because §1 shows a
suggestion that runs is more dangerous than one that fails:

**A suggestion must never be printed unless it is known to work.** Where that cannot be
established, the guard should explain the conflict and stop, rather than emit a command that
might execute something the user did not ask for.

Two mechanisms are available and can be mixed:

- **(a) Re-attach what was consumed.** Give `rejectSuppressedDefaultPlan` access to the root
  flags that `consumeRootPersistentFlags` took, and echo them back into the suggestion. Closes
  §1 exactly and leaves the rest.
- **(b) Refuse to suggest.** When anything was stripped that cannot be faithfully reproduced —
  a consumed root flag, a stray positional, a malformed selector value — print the diagnosis
  without a runnable command. Closes §1, §3, §4 and §5 together and is the smaller change.

§2 is separate from both and must be fixed regardless: `logs` cannot keep asserting a
whole-stack path it does not have. Either it stops claiming one, or `--tag` starts working
there. Deciding that is in scope; it is the same disagreement between message and manifest that
TASK-273 was opened to remove.

The verb-conditional part of §3 (which routes have a positional slot) overlaps
[TASK-279](279-repair-plan-flag-behaviour-defects.md) in file but not in defect: 279 is about
flags parsed and discarded, this is about advice. Do not merge them.

## Completion Criteria

- [ ] `dva up --tag app --dry-run` no longer yields a suggestion that executes: either the suggestion carries `--dry-run`, or no runnable command is offered | verify: `go test ./internal/cli -count=1`
- [ ] `--debug` and `--json` are handled by the same rule as `--dry-run`, not left as a second instance of the same hole | verify: `go test ./internal/cli -count=1`
- [ ] `dva logs` no longer tells the user `--tag` works on the whole-stack path, and its runtime message agrees with the `logs` manifest entry | verify: `go test ./internal/cli -count=1`
- [ ] `dva restart --tag app web` does not suggest a form the plan route answers with `unexpected argument in plan mode` | verify: `go test ./internal/cli -count=1`
- [ ] A flag-shaped selector value (`--tag -5`) and a swallowed recognized selector (`--tag -T web`) each produce either a working suggestion or none | verify: `go test ./internal/cli -count=1`
- [ ] `planAwareLifecycleCommands` covers every verb the guard can fire on — `logs`, `build` and `status` are absent today — so a route cannot silently miss it | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [ ] `withDryRun` no longer hides the difference between a suggestion that previews and one that acts: the dry-run case sets the flag through the argv the guard reads, not by assigning the package variable | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [ ] `stackPathOnlyCases` gains a trailing-positional case and a flag-shaped-value case, so §3, §4 and §5 are reachable by the table that was supposed to cover them | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No change to which flags `parsePlanFlags` accepts. That surface is
  [TASK-279](279-repair-plan-flag-behaviour-defects.md)'s.
- No change to the `validate.go` clean-hook advice, which TASK-273 fixed and the same review
  confirmed correct end to end.
- No change to `build`'s pre-route `parseDvaFlags` call, which is why `build` behaves unlike the
  other verbs; that ordering is named in TASK-279 §3.
