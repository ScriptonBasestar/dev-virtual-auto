---
id: TASK-283
title: "Repair the plan-route flag suggestion, which still dead-ends and now hides --dry-run"
type: bug
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-03T19:30:00+09:00
source: "Independent review of TASK-273 (`206918a`) by a reviewer that did not write it; every claim below re-measured by the TASK-273 implementer against a fresh `make build` of master at 916b07e"
scope: "internal/cli/plan_route_guard.go (split out of plan_lifecycle.go), the flagValue contract in internal/cli/flagtoken.go, the manifest entries the message echoes"
status: done
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

§1–§5 measured against `bin/dva` from `make build` at `916b07e`; §6 was found during
implementation and is measured the same way. Two fixtures: `fx`
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

   **Two corrections, measured during implementation.**

   - The consumer named above is wrong. `consumeRootPersistentFlags` serves the *passthrough*
     commands; the up/down/stop/restart path loses `--dry-run` to `consumeDryRunFlag`
     (`internal/cli/hooks.go:29`), which `wrapWithHooks` runs before a hookable command's
     `RunE`, and on the paths cobra does not parse, `parseDvaFlags` writes the same package
     global (`internal/cli/compose.go:1082`). Both routes hand the guard args with no
     `--dry-run` in them, so the defect is exactly as described; only the attribution moves.
   - `--debug` and `--json` are **not** lost the same way on this path. Because
     `consumeRootPersistentFlags` does not run here, they are still present in the args the
     guard reads, and are already echoed back by any suggestion built from those args.
     Re-adding them alongside `--dry-run` would print them twice. The rule that covers all
     three is therefore "restore what this path consumed", not "restore the root flags" —
     and on this path that set is `--dry-run` alone.

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

3. **`restart`'s positional entry name is stranded.** The strip walk passes non-selector tokens
   through untouched, so the entry name survives into a suggestion that cannot accept it.

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

   **Correction, measured during implementation.** The card's phrase "the one plan-route verb
   with a positional entry slot" was wrong, and the draft repair that followed from it — an
   exception letting `restart` keep the name — would have printed a command that fails.
   `parsePlanFlags` is the plan path's whole authority on arguments, and it answers
   `unexpected argument in plan mode` for **every** bare word on all four verbs; `dva restart
   p1 web` is rejected exactly like `dva up p1 web`. The entry-name slot shown in the control
   above belongs to the *whole-stack* path only. So `restart` is not an exception to be carved
   out but the clearest case for the general rule: when the guard has rewritten an invocation,
   it must ask the real parser whether the result is accepted, and stay silent when it is not.

4. **A flag-shaped selector value is left behind as a stray positional.**

   ```
   $ dva up --tag -5     → suggests: dva up p1 -5      → the plan route rejects -5
   ```

   `stripStackPathOnlyFlags` deliberately declines to consume a following flag-shaped token
   (see its doc comment), which is right for `dva up --tag --no-wait` and wrong here.

   **Correction, measured during implementation.** The card first wrote that `-5` "is the
   malformed value of the flag just removed". It is not malformed at all. `parseDvaFlags`'
   `takeValue` consumes the next token as the flag's value *whatever it spells*, and errors
   only when that token is a **recognized DVA flag** (`--tag requires a value, got the flag
   -T`), when there is nothing to take, or when the value is empty or blank. `-5` is none of
   those, so `dva up --tag -5` declares a tag literally named `-5`, runs on the whole-stack
   path, and matches nothing. The defect is therefore not a bad value the guard should
   diagnose — it is the strip walk failing to consume a value the real parser does consume,
   which strands it as a positional the plan path answers with `unsupported plan flag: -5`.
   The repair is to make the walk take the value exactly as `takeValue` would, after which
   `dva up p1` is a correct and runnable suggestion.

   This distinction is load-bearing rather than pedantic: an implementation that treats every
   flag-shaped value as malformed withholds advice on eight well-formed invocations
   (`--tag`/`--exclude-tag` × `-5`/inline `=-5` × four verbs). §6 is the case where withholding
   is right, and it is separated from this one by whether the plan path would *honour* the
   swallowed token, not by whether it looks like a flag.

5. **Two tokens vanish and one is stranded.**

   ```
   $ dva up --tag -T web → suggests: dva up p1 web
   ```

   `-T` is a recognized selector so the walk consumes it as `--tag`'s value and removes it
   without reporting it; `web` — which was `-T`'s value — is left as a positional the plan route
   rejects. `parseDvaFlags` would have reported `--tag requires a value, got the flag -T`. The
   suggestion reports neither.

6. **A selector that swallowed a real plan flag cannot be dropped without changing the
   invocation.** (added during implementation; not in the original review)

   ```
   $ dva up --tag --no-wait     → --tag's value is the string "--no-wait"; the stack path runs
   $ dva up p1 --no-wait        → --no-wait is an active plan flag; a different command
   ```

   Both §4's repair and the original strip behaviour are wrong here, in opposite directions.
   Consuming `--no-wait` as a value and then suggesting `dva up p1` silently drops a token the
   user typed; leaving it behind produces `dva up p1 --no-wait`, which runs, and means
   something the user did not write — their `--no-wait` was a *value*, this one is a *flag*.
   Neither rewrite is the guard's to make, so this is the one input where the right output is
   a diagnosis with no command attached.

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

## Resolution

Direction (a) for §1 and direction (b) for the rest, with one addition the card did not
anticipate: **the guard does not decide whether a rewritten invocation is acceptable, it asks
`parsePlanFlags`.** Both places where the implementation hand-derived that rule instead were
wrong on the first measurement — a hand-written stray-token scan flagged `--var`'s own value
`K=V` as a leftover positional, and the drafted `restart` exception printed a command the plan
path rejects. Delegating to the real parser is not a convenience here; it is the only version
that stays correct when TASK-279 changes what that parser accepts.

`rejectSuppressedDefaultPlan` now has four outcomes instead of one:

1. The command does not read these selectors (`logs`, `status`) — strip nothing and claim
   nothing about where the flag applies. Closes §2. DVA cannot know which tokens docker
   compose accepts, so the message says only what the guard knows: the default plan is
   suppressed, so name it. Whether docker then accepts `--tag` is docker's to report and is
   unchanged by naming the plan.
2. The selector is one `parseDvaFlags` will reject outright (`--tag` with nothing to take,
   `--tag -T`, `--tag=`) — step aside so the parser names the broken rule a few lines later.
   Closes §5: `--tag requires a value, got the flag -T` is the answer, and no wording available
   in the guard improves on it.
3. The selector swallowed a token the plan path would honour (`--tag --no-wait`) — diagnose and
   offer no command. Closes §6. `planPathHonoursFlag` is what separates this from §4, so the
   eight well-formed `--tag -5` invocations keep their working suggestion.
4. Otherwise — hand what survives the strip to `parsePlanFlags`. Accepted, so print the
   suggestion; rejected, so quote the parser's own reason and print no command. Closes §3.

`planSuggestion` re-appends `--dry-run` when the package global is set, which closes §1; per
the correction in §1, `--debug` and `--json` are deliberately *not* re-appended on this path.

### The TASK-279 interaction, observed on the rebase

[TASK-279](../done/279-repair-plan-flag-behaviour-defects.md) landed first (`ab5c479`) and made
`parsePlanFlags` verb-aware: `--no-wait` is honoured on `up`/`restart` and rejected on
`stop`/`down`, because "wait for readiness" has no meaning once the direction is teardown. The
guard reads that parser, so its answers moved with it — and moved *correctly*, without an edit
to the rule:

| input | before 279 | after 279 |
| --- | --- | --- |
| `dva up --tag --no-wait` | diagnosis, no command | unchanged — dropping `--tag` would arm a flag the user wrote as a value |
| `dva stop --tag --no-wait` | diagnosis, no command | `dva stop p1` — restoring `--no-wait` restores nothing that verb honours, so the token is only ever a tag value and dropping the pair is faithful |

This is the payoff of asking the parser rather than copying it, and it is pinned by the two
subtests of `TestSuppressedDefaultPlanRefusesToSuggestWhenASelectorAteAFlag` so that a later
change to either card's surface cannot silently collapse the two answers into one. The only
mechanical cost of the rebase was threading the verb into `planPathHonoursFlag` and the
remainder check.

### The file split

`internal/cli/plan_route_guard.go` is new. The guard grew `plan_lifecycle.go` from 472 to 533
code lines and crossed that file's 500-line limit; the split follows the seam the defect
exposed rather than an arbitrary cut. `plan_lifecycle.go` decides what a plan invocation
*does*; the new file decides what DVA *says* when it will not do it. Dependency runs one way,
from the guard into the runner's parser, which is the property this card was opened to
establish.

The `removed == 0` branch is left semantically unchanged on purpose. There the guard is echoing
what the user typed and asserting only where the plan name goes, so an unknown flag keeps its
suggestion — the plan path's own `unsupported plan flag` is the answer, and moving the user to
that path is what produces it. Validation applies only where the guard *rewrote* the
invocation and therefore owns the result.

## Completion Criteria

- [x] `dva up --tag app --dry-run` no longer yields a suggestion that executes: either the suggestion carries `--dry-run`, or no runnable command is offered | verify: `go test ./internal/cli -count=1`
- [x] `--debug` and `--json` are handled by the same rule as `--dry-run` — measured to be already present in the args the guard reads on this path, so the rule "restore what this path consumed" leaves them alone rather than double-printing them | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [x] `dva logs` no longer tells the user `--tag` works on the whole-stack path, and its runtime message agrees with the `logs` manifest entry | verify: `go test ./internal/cli -count=1`
- [x] `dva restart --tag app web` does not suggest a form the plan route answers with `unexpected argument in plan mode` | verify: `go test ./internal/cli -count=1`
- [x] A flag-shaped selector value (`--tag -5`) and a swallowed recognized selector (`--tag -T web`) each produce either a working suggestion or none | verify: `go test ./internal/cli -count=1`
- [x] A selector that swallowed a token the plan path would honour (`--tag --no-wait`) produces a diagnosis and no command, since neither keeping nor dropping it preserves what was asked | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [x] Every verb the guard can fire on is covered, so a route cannot silently miss it: `logs`/`status` through `selectorRejectingCommands`, and `build` through a test of the routing fact that no selector reaches its guard call at all | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [x] `withDryRun` no longer hides the difference between a suggestion that previews and one that acts: the dry-run case sets the flag through the argv the guard reads, not by assigning the package variable | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [x] `stackPathOnlyCases` gains a trailing-positional case and a flag-shaped-value case, so §3, §4 and §5 are reachable by the table that was supposed to cover them | verify: `go test ./internal/cli -run SuppressedDefaultPlan -count=1`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No change to which flags `parsePlanFlags` accepts. That surface is
  [TASK-279](279-repair-plan-flag-behaviour-defects.md)'s.
- No change to the `validate.go` clean-hook advice, which TASK-273 fixed and the same review
  confirmed correct end to end.
- No change to `build`'s pre-route `parseDvaFlags` call, which is why `build` behaves unlike the
  other verbs; that ordering is named in TASK-279 §3.
