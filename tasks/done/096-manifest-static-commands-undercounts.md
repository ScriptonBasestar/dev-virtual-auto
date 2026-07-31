---
id: TASK-096
title: "`dva manifest` documents 13 of 27 commands, and its doc comment says the audience is LLMs"
type: fix
priority: P3
effort: S
status: done
created-at: 2026-07-31T00:00:00+09:00
scope: "internal/cli/manifest.go:103-124 — StaticCommands, a hand-maintained literal; internal/config/reserved.go:12-20 — the 27-entry list it should agree with"
---

# Task 096: the machine-readable command list is a hand-copied subset

## Measured

```
$ dva manifest --format json | jq '.static_commands | length'
13
```

against 27 real top-level commands (`reservedCommands`, `internal/config/reserved.go:12-20`,
which `USAGE.md:617-623` also documents as 27).

Present: `run, ls, compose, up, down, stop, build, clean, provision, validate, manifest, ktl,
version`.

**Missing (14):** `help, ssh, infra, console, completion, init, status, config, logs, restart,
show, doctor, app, stack`.

The mismatch is one-directional — `StaticCommands ⊂ reservedCommands`, no phantom entries — which
is the signature of a list that was written once and not updated as commands were added.

## Why it matters

`manifest.go`'s own doc comment states the output is for LLMs. An agent that reads
`static_commands` to decide what dva can do concludes there is no `dva doctor`, no `dva status`,
no `dva stack`, and no `dva logs`. `dva help` documents all 27 for humans, so the machine-readable
surface is strictly worse than the human one — the inverse of the flag's purpose, and the same
theme as [TASK-088](088-validate-json-covers-only-the-failure-it-does-not-produce.md).

## Options

- **A — derive it.** Walk `rootCmd.Commands()` at build time so the list cannot drift again. The
  per-command `Description`/`Type`/`Options` metadata in the literal has no cobra equivalent, so
  it would need to move onto the commands (e.g. annotations) or into a lookup keyed by name that
  a test asserts is total.
- **B — complete the literal and pin it with a test** asserting
  `len(StaticCommands) == len(reservedCommands)` with the diff printed on failure. Keeps the
  curated descriptions, costs one test to stop the drift.

B is the smaller change and directly prevents recurrence; A is the one that makes the class of bug
impossible. **Decision needed.**

## Acceptance criteria

- [x] Every real command appears | verify: `dva manifest --format json | jq '.static_commands | length'` must equal 27; print both numbers
- [x] No phantom commands | verify: every `static_commands` key must be in `reservedCommands`; print the count checked
- [x] Drift cannot recur silently | verify: `go test ./internal/cli/ -run Manifest` — a test must fail if a command is added to root without a manifest entry, and print the diff
- [x] The 13 existing descriptions are unchanged | verify: human — diff the descriptions for the original 13 keys before and after
- [x] Not vacuous | verify: human — delete one entry and confirm the new test names it
- [x] Full suite passes | verify: `make test`

```
1  static_commands = 27   reservedCommands = 27   rootCmd = 27
2  27 keys checked, 0 not in reservedCommands (TestStaticCommandsAgreeWithReservedCommands)
3  probe B: `teleport` added to root -> "1 command(s) registered on rootCmd with no
   static_commands entry: teleport"
4  the 13 old-vs-new: 13 compared, byte-identical (0 differing)
5  probe A: `doctor` entry deleted -> 3 tests name it (table below)
6  make test -> 0 FAIL; cli 63.4%, config 66.5%, exec 63.3%, lifecycle 56.2%, runner 52.3%
```

Criterion 3's verify string says `-run Manifest`, which does not select the new tests — they are
named `TestStaticCommands…`, so the matching selector is `-run 'StaticCommand'`. The criterion's
*intent* (a test fails when a command is added to root without a manifest entry) is met and probed;
the pattern in the text was written before the tests had names.

## Decision: B

**B — complete the literal and pin it with a test**, with the test anchored on `rootCmd.Commands()`
rather than on `reservedCommands`, because criterion 3 asks specifically that adding a command *to
root* fail.

A was rejected on a measurement, not on size. `Type` and `Options` have no cobra equivalent —
`GroupID` is 5 coarse groups against the 8 types this table uses — so a walk over `rootCmd` still
needs this literal for them, and a command added without an annotation would get an empty `Type`
while passing any count check. **A needs the same totality test that B needs.** Since the test is
what actually stops the drift in both designs, B buys the test and skips relocating metadata into
the 12 files the command definitions live in.

Deriving `Description` from `Short` was also rejected here for a narrower reason: it would rewrite
all 13 existing descriptions, which criterion 4 forbids. That is filed separately as
[TASK-105](../done/105-static-command-metadata-is-thinner-than-help.md).

## Resolution

14 entries added to `StaticCommands`, each taking its description from the command's own `Short`
so nothing in this change is invented prose:

| type | commands added |
| --- | --- |
| `lifecycle` | `stack`, `app`, `ssh`, `infra` |
| `compose_shortcut` | `logs`, `restart` |
| `passthrough` | `console` |
| `query` | `status`, `show`, `doctor` |
| `config` | `config`, `init` |
| `meta` | `help`, `completion` |

`infra`'s `Short` carries its own deprecation notice ("deprecated — folded into stack, use
'dva up'"), so the manifest now says so too rather than presenting it as a live command.

### The 27 that cobra actually registers

`grep -c AddCommand` finds **25**. `help` and `completion` are registered by cobra inside
`Execute()`, not by an `AddCommand` call, so a test that never calls `Execute()` sees 25 and would
have reported two phantom entries. `rootCommandNames` calls `InitDefaultHelpCmd()` and
`InitDefaultCompletionCmd()` — which is what `Execute()` does first, and both are idempotent — to
get the same 27 a user gets without running a command. Measured: 25 before, 27 after, 0 hidden.

### Non-vacuity

Three probes on a copy of the working tree:

| probe | result |
| --- | --- |
| **control** — unmodified | `ok` |
| **A** — `doctor` entry deleted | 3 failures, each naming it: `1 command(s) registered on rootCmd with no static_commands entry: doctor` / `1 reserved command(s) undocumented: doctor` / `"doctor" is in the added set but has no static_commands entry` |
| **B** — `teleport` added to root, manifest untouched | `1 command(s) registered on rootCmd with no static_commands entry: teleport` |
| **C** — `teleport` in root *and* manifest, `reserved.go` forgotten | `1 documented command(s) not reserved: teleport` |

B and C are the same probe at two stages, and together they show the two tests **compose**: the
three sources cannot be brought into partial agreement and pass. Fixing the failure B reports
produces the failure C reports, and only adding the command to all three is quiet. That is the
property worth having, and it is measured rather than argued.

### Tests

`internal/cli/manifest_static_commands_test.go`, four tests:

- `TestStaticCommandsCoverEveryRootCommand` — the registry is the source of truth; diffs both
  directions and names the commands, since two differing counts do not tell the reader which.
- `TestStaticCommandsAgreeWithReservedCommands` — the third source. A command in root and in the
  manifest but missing from `reserved.go` would let a user declare an interaction that silently
  never runs, which is [TASK-076](076-manifest-advertises-the-one-invocation-that-cannot-reach-the-interaction.md)
  reached from the other side.
- `TestEveryStaticCommandCarriesAType` — guards the half-filled entry that satisfies a count check
  but leaves `Type` empty, and rejects a type outside the eight in use.
- `TestStaticCommandDescriptionsMatchTheirShort` — scoped to the 14 added, by construction.

The package was run with `-shuffle=on` three times, because `rootCommandNames` mutates the global
`rootCmd`; no ordering failure.

## Left open

- `Options` is populated for **1 of 27** commands (`run`), while `dva up --help` documents ~15 flag
  lines — and 12 of the 13 original descriptions paraphrase their own `Short`, two of them
  (`up`, `down`) predating the plan concept and never using the word "plan". Filed as
  [TASK-105](../done/105-static-command-metadata-is-thinner-than-help.md) with the per-command table. The
  count is right now; what each entry *says* is the remaining half.
- `--no-wait` was checked and is **not** stale — it is hand-parsed at `compose.go:120-131` on
  `upCmd` instead of registered as a cobra flag, which is why `dva up --help` lists it in `Long`
  but not under `Flags:`. Recorded because the description reads like it advertises a removed flag
  and it does not.

## Related

- [TASK-088](088-validate-json-covers-only-the-failure-it-does-not-produce.md) — the same
  audience getting a worse answer than the human one.
- [TASK-097](097-interaction-usage-mishandles-keys-with-spaces.md) — the other manifest-correctness
  defect; both surface through `dva manifest`, which is documented as the agent-facing entry point.
- [TASK-105](../done/105-static-command-metadata-is-thinner-than-help.md) — the contents half of this
  defect, split out because criterion 4 pinned the 13 descriptions.
