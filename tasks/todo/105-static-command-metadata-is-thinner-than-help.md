---
id: TASK-105
title: "`static_commands` carries options for 1 of 27 commands and 12 descriptions that paraphrase their own `Short`, so the LLM surface stays thinner than `--help` after TASK-096"
type: fix
priority: P3
effort: M
status: todo
created-at: 2026-07-31T14:20:00+09:00
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

Same argument as 096 and [TASK-088](../done/088-validate-json-covers-only-the-failure-it-does-not-produce.md):
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

## Acceptance criteria

- [ ] Every command that takes flags advertises them | verify: for each of the 8, `static_commands[<cmd>].options | length` must be > 0; print the table above with the new column
- [ ] `up` and `down` describe plans | verify: `dva manifest --format json | jq -r '.static_commands.up.description'` must mention a plan; print both descriptions
- [ ] Descriptions cannot drift from `Short` again | verify: a test must compare all 27, not the 14 that TASK-096 pinned; print the number compared
- [ ] No command loses information | verify: diff `static_commands` before and after; every description must be at least as specific — print the count of entries changed
- [ ] Not vacuous | verify: human — re-word one description away from its `Short` and confirm the widened test names it
- [ ] Full suite passes | verify: `make test`

## Related

- [TASK-096](../done/096-manifest-static-commands-undercounts.md) — the parent. It fixed the key
  set and pinned the 14 it added; `TestStaticCommandDescriptionsMatchTheirShort` is deliberately
  scoped to those 14 and is the test option B would widen to 27.
- [TASK-088](../done/088-validate-json-covers-only-the-failure-it-does-not-produce.md) — the same
  audience getting a worse answer than the human one.
