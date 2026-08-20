---
id: TASK-206
title: "The archive skip runs after canonical detection, so a card that fails detection is audited forever"
type: bug
priority: P3
effort: S
created-at: 2026-08-20T09:37:34+09:00
source: "registered as the gap left behind by de3f7e9, which added `id: TASK-00N` to eight legacy archived cards so they would classify as history; that restored the eight and left untouched the ordering defect that produced them"
scope: "A dva-side guard over `tasks/_archive/` frontmatter, in `tools/doccheck`. No change to ce-agent-kit — `ce` is installed here as a binary only. No change to any archived card's content, no change to which linters or doc checks run."
status: todo
---

## Summary

`ce task validate` has a rule that archived documents are history and are not maintained.
The rule is real and it works — but it is evaluated **after** frontmatter parsing and
canonical detection have both succeeded. A card that fails detection never reaches it, and
is then judged against a card format that postdates it, in a directory literally named
`_archive`.

That is not a hypothesis about the past. It reproduces today, on the current `ce`:

| arm (same file, same `_archive` directory, outside this repo) | rc | result |
|---|---|---|
| `001-config-model.md` as committed | 0 | `Skipped: archived documents are kept as history` |
| the same file with the single line `id: TASK-001` deleted | 1 | 5 errors — missing title, Overview, Status, Priority, Description |
| the same file with `id:` deleted and `type: bug` added | 0 | `Skipped: archived documents are kept as history` |

One line apart. Detection is satisfied by **either** `id:` or `type:`; the archive's
location on disk never enters into it.

The mechanism was already written down — TASK-202 cites it after `b57d650`:
`canonicalFrozenZone` (ce-agent-kit `canonical_validator.go:277-284`, pinned to `c99d1921`
and drifting) tests for the `archive`/`_archive` directory name per file, but only once
parsing and detection have passed. What has never been registered is that **the fix applied
to it was a data-layer patch to a tool-layer defect**, and nothing marks it as one.

## Why it matters

The eight cards are fixed. The condition that produced them is not.

`de3f7e9` put a number where the tool looks, on eight specific files. It did not change what
happens to the ninth. Any card that lands in `tasks/_archive/` without `id:` or `type:` —
an import from another tracker, a hand-written record, a card predating whatever the format
becomes next — goes red silently, with error text that reads as if it were unfinished
current work. It was not noticed the first time either: nine cards sat red until an archive
sweep counted them.

Measured on `master` today:

    archived cards                 198
    without `id:`                    0
    without `type:`                  8
    without either                   0

The archive is clean, and the eight legacy cards are held there by exactly one field each.
Nothing in this repository asserts that, so the next card to break it breaks it quietly.

The honest framing of the priority: this is a latent regression guard, not a live failure.
It is P3 because nothing is red today, and it is worth registering because the cost of
rediscovery has already been paid once — the mechanism took a full sweep, a source read of
an external repository, and two commits to establish.

## Scope, and what this card cannot do

The root cause is ce-agent-kit's and is **out of dva's reach**: `ce` is on PATH as a binary,
its source is not in this repository, and a dva reader who opens `canonical_validator.go`
gets `No such file or directory`. Fixing the ordering — testing the frozen zone before
detection rather than after — is an upstream change and is not proposed here.

What dva can do is refuse to depend on a property it never checks. This card adds a local
guard so the ninth card fails a dva gate loudly, at the moment it is added, instead of
turning up in a future audit as an unexplained red.

## Proposed fix

Add an archive-frontmatter check to `tools/doccheck`, which is already the home for
repo-hygiene gates and already runs under `make doc-check`:

- every `tasks/_archive/*.md` carries `id:` or `type:` in its frontmatter
- the check prints the denominator it scanned (`checked N archived card(s)`), because a
  check that silently matched nothing prints the same thing as a check that passed
- it fails with the file name and the missing field, not with a bare count

The condition is deliberately "`id:` **or** `type:`" and not "`id:`" — measured above.
Requiring `id:` alone would fail the guard on a card that `ce` classifies correctly, which
is the same class of error as a check that cannot fire: a gate whose verdict disagrees with
the tool it exists to protect.

## Completion Criteria

- [ ] `tools/doccheck` checks archived-card frontmatter.
      verify: `grep -rc 'checkArchiveFrontmatter' tools/doccheck/` → at least 1
      (today: **0**)
- [ ] The check has a test that fails when a card carries neither field, so the guard
      itself can fail.
      verify: `grep -rc 'checkArchiveFrontmatter' tools/doccheck/*_test.go` → at least 1
      (today: **0**)
- [ ] The check accepts `type:` alone, matching what `ce` actually does rather than what
      `de3f7e9` happened to write.
      verify: human — a fixture carrying `type:` and no `id:` must pass the guard, and must
      also be measured to exit 0 under `ce task validate` with the archive skip message
- [ ] The check reports its denominator, so a run that scanned nothing is distinguishable
      from a run that passed.
      verify: human — `make doc-check` output names a non-zero count of archived cards
      scanned (**198** on master today)
- [ ] The archive stays clean while this lands.
      + regression guard, not an acceptance test: every archived card carries `id:` or
      `type:` — today 198 of 198, 0 carrying neither
- [ ] No change to which doc checks run.
      + regression guard, not an acceptance test:
      `grep -c 'go run ./tools/' Makefile` → **6**, unchanged

## Open Questions

1. Should the guard cover `tasks/done/` too? It is empty today (0 files), and cards move
   through it before `_archive`, so a card can only acquire the defect on the way in.
   Leaning no — guarding an empty directory is a rule with no denominator, and the axis to
   state would be "every directory `ce` treats as a frozen zone", which is upstream
   vocabulary this repository would then be duplicating.
2. Is `id:`-or-`type:` the *whole* of canonical detection, or only the part these three
   fixtures exercised? Measured on one file with two mutations; not swept. A sweep would
   vary each frontmatter field of an archived card independently and record which ones flip
   the verdict, with the field count as the denominator.
3. Upstream: does ce-agent-kit want the frozen-zone test moved ahead of detection? Not
   raised there. This card does not depend on the answer — the local guard is useful even
   if upstream fixes the ordering, because it fails at the moment of the commit rather than
   at the next audit.
