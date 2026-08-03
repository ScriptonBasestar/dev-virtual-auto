---
id: TASK-105
title: "`static_commands` carries options for 1 of 27 commands and 12 descriptions that paraphrase their own `Short`, so the LLM surface stays thinner than `--help` after TASK-096"
type: fix
priority: P3
effort: M
status: done
created-at: 2026-07-31T14:20:00+09:00
resolved-at: 2026-07-31T00:00:00+09:00
resolution: "B then A — Description derived from cobra Short for all 27; Options derived for the 8 commands with registered flags and hand-written for the 5 with hand-parsed ones"
scope: "internal/cli/manifest.go — StaticCommands: the Description field of the original 13 entries, and the Options field on all but `run`"
---

# Task 105: the count is right now; the contents are still second-hand

[TASK-096](../done/096-manifest-static-commands-undercounts.md) closed the coverage gap — all 27
commands appear. It deliberately did not touch what each entry *says*, because its own acceptance
criteria required the original 13 descriptions to stay byte-identical. This is what was left.

## Measured (bin/dva at TASK-096)

### Options is empty for 26 of 27

```
commands with a non-empty options map: 1 of 27   (only `run`)
```

| command | flag lines in `dva <cmd> --help` | `static_commands[<cmd>].options` |
| --- | --- | --- |
| `up` | ~15 | **0** |
| `down` | ~11 | **0** |
| `stop` | ~10 | **0** |
| `build` | ~3 | **0** |
| `clean` | ~3 | **0** |
| `compose` | ~3 | **0** |
| `provision` | ~3 | **0** |
| `ktl` | ~3 | **0** |
| `run` | ~4 | 2 |

`dva up --help` documents `--force`, `--no-wait`, `--dev`, `--docker`, `--mode`, `--env`, `--tag`,
`--exclude-tag`, `--var` in its `Long` text. An agent reading the manifest sees none of them, so it
cannot construct any invocation more specific than bare `dva up`.

### 12 of 13 descriptions paraphrase the command's own `Short`

`version` is the only original entry that matches. The other twelve say the same thing in different
words, which is harmless in itself but means two strings must be edited together forever. Two are
not merely reworded — they predate the plan concept and no longer describe the primary behaviour:

| command | `static_commands` description | cobra `Short` |
| --- | --- | --- |
| `up` | Start compose + local services (--no-wait for immediate return) | **Start a named plan** (or all declared entries) |
| `down` | Stop and remove containers | **Tear down a named plan** (or all declared entries) |

`dva up --help` leads with "Start a named plan when plans are configured." The manifest does not
contain the word "plan" anywhere in `static_commands`.

(`--no-wait` is *not* stale — it is hand-parsed in `compose.go:120-131` on `upCmd` rather than
registered as a cobra flag, which is why `dva up --help` omits it from the `Flags:` block while
documenting it in `Long`. Checked before filing; it is real.)

## Why it matters

Same argument as 096 and [TASK-088](../_archive/088-validate-json-covers-only-the-failure-it-does-not-produce.md):
`manifest.go` states its audience is an LLM, and on the two fields that would let an agent actually
*use* a command — what it does now, and what flags it takes — the human surface is still richer.
096 fixed how many commands are listed; it did not fix what a listing is worth.

## Options

- **A — populate `Options` by hand for the 8 commands that take flags, and re-word the 2 stale
  descriptions.** Smallest change; leaves the two-strings-in-sync problem in place, so
  `TestStaticCommandDescriptionsMatchTheirShort` should be widened to cover whatever is re-worded.
- **B — derive `Description` from `Short` for all 27** and delete the field from the literal.
  Removes the drift class outright and shrinks the table to Type plus Options. Changes 12
  descriptions in one commit, which is why 096 could not do it.
- **C — derive `Options` from the cobra flag set too.** Only reaches flags that are actually
  registered; `up`'s eight hand-parsed ones would still be invisible, so it needs the flags to be
  registered first — a larger change to `compose.go`'s manual `for _, a := range args` parsers.

B is the one that stops the recurrence for descriptions. Options needs A or C-plus-registration.
**Decision needed.**

## Resolution

**Decision: B then A.** Both shipped.

### B — descriptions derived

`fillStaticCommandDescriptions` copies each command's cobra `Short` into its entry; the literal
keeps `Type` and the hand-parsed `Options` only. Deriving is at least as specific in 11 of the 12
cases that changed:

| command | was | now |
| --- | --- | --- |
| `up` | Start compose + local services (--no-wait for immediate return) | Start a named plan (or all declared entries) |
| `down` | Stop and remove containers | Tear down a named plan (or all declared entries) |
| `build` | Build service images | Build or rebuild services (mode-aware: docker or native) |
| `stop` | Stop services | Stop applications and stack without removing resources |
| `ktl` | Run kubectl commands | Execute kubectl commands within the configured namespace |

The one place information was lost is `up`: its old text was the manifest's only mention of
`--no-wait`. That belongs in `Options`, which is what A restored — the flag is now listed on `up`,
`restart` and nowhere it does not apply.

### A — options populated

`static_commands` went from **1 of 27** commands carrying options to **13 of 27**, 44 option
entries total. Split by how the flag is parsed, because the two halves have different guarantees:

| how parsed | commands | source |
| --- | --- | --- |
| registered on cobra | clean, doctor, init, ls, manifest, provision, run, validate | derived by `fillStaticCommandOptions` from the flag's own `Usage` |
| hand-parsed from args | up, down, stop, restart, build | written literally, shared strings in a const block |

### Two premises in this task were wrong, and measurement is what showed it

1. **"8 commands take flags", listing `build`, `compose`, `ktl` with "~3 flag lines".** Those three
   lines are `-h, --help` plus the two global flags. `compose`, `ktl` and `logs` take no flags of
   their own at all. Meanwhile five commands that *do* register flags — `doctor`, `init`, `ls`,
   `manifest`, `validate` — were not in this task's table and would have been missed by a purely
   hand-written A. Deriving found them.
2. **`run` was described as already correct.** Its two hand-written options had already drifted
   from cobra's wording (`Publish container ports to host` against `Publish container port(s) to
   host`; `Show execution plan without running` against `Alias for --dry-run`) and it was missing
   `--project` entirely. Deriving corrected all three.

### The bug this introduced, and how it was caught

The first implementation used cobra's `LocalFlags()`, which reads as the obvious way to say "this
command's own flags". `LocalFlags` calls `mergePersistentFlags`, which copies the root's persistent
set into the command's own `FlagSet` and leaves it there — and these commands are package-level
globals, so the edit outlives the call.

It passed every manifest test. `TestRootValidateMatchesConfigValidate` failed in the full suite
while passing in isolation: after `buildManifest` ran, root `validate`.Flags() held `--debug`,
`--dry-run` and `--json` while `config validate`.Flags() did not. `Flags()` performs no merge, so
the fix reads that and filters against `rootCmd.PersistentFlags()` — side-effect free and
order-independent.

Worth recording: `go test ./internal/cli/ -run <the new tests>` was green on the broken version.
Only the whole suite, in its own order, disagreed.

### Residual, deliberately not closed

The root persistent flags (`--debug`, `--dry-run`, `--json`) are still undocumented in the
manifest. They are skipped per command on purpose — repeating them 27 times says nothing — but that
leaves an agent unable to discover `--json` from the manifest, which is the flag its own audience
most needs. Closing it wants a top-level `global_flags` field, which is a schema change and not
what B-then-A decided.

## Acceptance criteria

- [x] Every command that takes flags advertises them | verify: `dva manifest --format json` — 13 of 27 commands carry options, 44 entries; every cobra-registered flag is covered by `TestStaticCommandOptionsCoverEveryRegisteredFlag` (8 commands, 17 flags) and every hand-parsed one by `TestHandParsedOptionsAreDocumented` (5 commands, 27 flags)
- [x] `up` and `down` describe plans | verify: `dva manifest --format json | jq -r '.static_commands.up.description'` → `Start a named plan (or all declared entries)`; `.down` → `Tear down a named plan (or all declared entries)`
- [x] Descriptions cannot drift from `Short` again | verify: `go test ./internal/cli/ -run StaticCommandDescriptionsMatchTheirShort -v` → `checked=27 of 27 root commands`, and it fails if fewer than all 27 are compared
- [x] No command loses information | verify: 12 descriptions changed, 11 strictly more specific; the single loss (`up`'s `--no-wait`) is restored in `options`, table above
- [x] Not vacuous | verify: mutation table below — every new assertion fails when its fix is disabled
- [x] Full suite passes | verify: `make test` — all ok, `internal/cli` coverage 63.6% → 63.8%; `make lint` reports `0 issues.`

### Mutation results

| mutation | result |
| --- | --- |
| `fillStaticCommandDescriptions` skips `up` | `TestStaticCommandDescriptionsMatchTheirShort` and `TestEveryStaticCommandCarriesAType` both FAIL naming `up` |
| `fillStaticCommandOptions` skips `clean`'s `--images` | `TestStaticCommandOptionsCoverEveryRegisteredFlag` FAILs: `clean registers --images but static_commands["clean"].options does not list it` |
| drop `--var` from `up`'s literal options | `TestHandParsedOptionsAreDocumented` FAILs: `up accepts --var but static_commands["up"].options does not list it` |
| reintroduce a literal `Description`, or a literal option for a registered flag | **PASSES** — measured. The fill functions run after the table and overwrite it, so a stray literal is dead code, not drift. Recorded because the opposite is the natural assumption; both test comments now say so. |

## Related

- [TASK-096](../done/096-manifest-static-commands-undercounts.md) — the parent. It fixed the key
  set and pinned the 14 it added; `TestStaticCommandDescriptionsMatchTheirShort` is deliberately
  scoped to those 14 and is the test option B would widen to 27.
- [TASK-088](../_archive/088-validate-json-covers-only-the-failure-it-does-not-produce.md) — the same
  audience getting a worse answer than the human one.
