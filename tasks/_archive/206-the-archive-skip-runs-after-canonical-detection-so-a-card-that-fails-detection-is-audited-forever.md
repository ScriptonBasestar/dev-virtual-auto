---
id: TASK-206
title: "The archive skip runs after canonical detection, so a card that fails detection is audited forever"
type: bug
priority: P3
effort: S
created-at: 2026-08-20T09:37:34+09:00
source: "registered as the gap left behind by de3f7e9, which added `id: TASK-00N` to eight legacy archived cards so they would classify as history; that restored the eight and left untouched the ordering defect that produced them"
scope: "A dva-side guard over `tasks/_archive/` frontmatter, in `tools/doccheck`. No change to ce-agent-kit — `ce` is installed here as a binary only. No change to any archived card's content, no change to which linters or doc checks run."
status: done
completed-at: 2026-08-20T11:36:42+09:00
quality-review: pass
quality-reviewed-at: 2026-08-20T11:36:42+09:00
verified-at: 2026-08-20T11:36:42+09:00
archived-at: 2026-08-20T11:36:42+09:00
quality-review-evidence: |
  Implemented in da5efde, corrected by the review that followed it. Reviewed by two
  independent sessions that did not write the change; in-process review agents were not
  used, because this session is instructed not to call the Agent tool.

  The review found a false pass in the shipped guard, and it was a real one. A closing
  frontmatter fence carrying a single trailing space did not match the reader's
  `TrimRight(line, "\r")` comparison, so the scan walked past it and took the next
  horizontal rule in the body as the close. Everything between was then read as
  frontmatter — including a fenced yaml example — and a card whose real frontmatter is
  `title:` + `status:` passed the guard on an `id:` that was documentation. Reproduced
  here before being acted on: ARM A (trailing space) `archive_missing: 0`, `doc-check: OK`;
  ARM B (same file, one byte removed) `archive_missing: 1`, `doc-check: FAIL`; and `ce`
  rejects the file in both arms. Fixed by comparing both fences with `TrimSpace`, which
  also closed a false alarm in the other direction — an indented opening fence, which `ce`
  accepts and this guard had been rejecting.

  Not every reported item survived measurement. The review listed a `...` document
  terminator as a shape `ce` accepts and the guard would wrongly reject; measured, `ce`
  rejects it too, with `Invalid YAML frontmatter: unterminated frontmatter` — that list had
  been reasoned against PyYAML semantics rather than against `ce`. The guard and `ce`
  agree there, and the guard now quotes `ce`'s message verbatim in its diagnosis.

  Two message defects were fixed rather than the verdicts they accompanied: an unterminated
  block was being reported as "no frontmatter block" (false — it opened one), and a YAML
  flow mapping as "carries neither `id:` nor `type:`" (false — it carries both; the truth
  is that a line-based reader cannot evaluate it). Both now say what is true. The flow
  mapping still fails the gate, and that choice is argued in the code rather than implied.

  Both new regression cases were checked for the ability to fail, and the check changed the
  outcome. The trailing-space fixture stayed green when only the fence fix was reverted,
  because `stripFencedRegions` was independently blanking the fenced-yaml bait — so it
  pinned the two defences only together. A second fixture was added with the bait outside a
  code fence; reverting the fence trim alone now turns it red, and reverting both turns both
  red. Measured, not assumed.

  Non-vacuity on the real corpus was re-proven after the reader changed, by indenting `id:`
  in `tasks/_archive/001-config-model.md` (`doc-check: FAIL`, naming the file) and restoring
  it. That mutation also demonstrates the substring trap the guard is written against: the
  file still literally contains `id:` once.
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

Measured on `master` when this card was written:

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

- [x] `tools/doccheck` checks archived-card frontmatter.
      verify: `grep -rc 'checkArchiveFrontmatter' tools/doccheck/` → at least 1
      (today: **0**; resolved: 3 files — `archive.go` 2, `check.go` 1, `archive_test.go` 1)
- [x] The check has a test that fails when a card carries neither field, so the guard
      itself can fail.
      verify: `grep -rc 'checkArchiveFrontmatter' tools/doccheck/*_test.go` → at least 1
      (today: **0**; resolved: 1)
- [x] The check accepts `type:` alone, matching what `ce` actually does rather than what
      `de3f7e9` happened to write.
      verify: human — a fixture carrying `type:` and no `id:` must pass the guard, and must
      also be measured to exit 0 under `ce task validate` with the archive skip message
      (resolved: guard `archive_missing: 0`; `ce` rc=0,
      `Skipped: archived documents are kept as history, not maintained`)
- [x] The check reports its denominator, so a run that scanned nothing is distinguishable
      from a run that passed.
      verify: human — `make doc-check` output names a non-zero count of archived cards
      scanned (**198** on master today; resolved: prints
      `archive_cards: 200 (from 200 file(s) under tasks/_archive/)`, and `Check` raises
      `vacuous: N file(s) under tasks/_archive/, zero read as cards` when the sweep stops
      reaching the corpus)
- [x] The archive stays clean while this lands.
      + regression guard, not an acceptance test: every archived card carries `id:` or
      `type:` — today 198 of 198, 0 carrying neither
      (resolved: 200 of 200, 0 carrying neither — this card lands as the 201st)
- [x] No change to which doc checks run.
      + regression guard, not an acceptance test:
      `grep -c 'go run ./tools/' Makefile` → **6**, unchanged
      (resolved: 6)

## Open Questions

1. **Should the guard cover `tasks/done/` too? Answered: no — and the original reason was
   the weaker one.** The card first argued from the denominator: `tasks/done/` is empty, so
   a rule over it has nothing to measure. That argument is self-undermining here, because it
   would equally argue against this card, which found 0 defects across 200 archived cards.
   The reason that holds is that **there is no skip to miss in `tasks/done/`**. A card there
   is current work, `ce` has no frozen-zone rule for it, and `ce` auditing it as unfinished
   is the correct verdict rather than the defect this guard exists to catch. The guard is
   bound to the frozen zone because that is where the ordering defect lives.
2. **Is `id:`-or-`type:` the whole of canonical detection? Swept, and the answer is yes for
   the field test — but detection is not the only gate.** Ten frontmatter shapes were run
   through both `ce` and the guard and compared in both directions — silent hole versus
   false alarm — rather than the three fixtures this card was written from. `ce` accepts:
   `id:` with an empty value, a quoted key (`"id":`), an indented opening fence, a flow
   mapping. `ce` rejects: uppercase `ID:`/`Type:` (the lookup is case-sensitive), a `...`
   terminator and an unterminated block (both `unterminated frontmatter`), and `id:`/`type:`
   appearing only inside a block scalar. One divergence is documented rather than closed: a
   card carrying `id:` **and** malformed YAML is a step-2 hard error in `ce` and passes this
   guard, because `doccheck` is stdlib-only and cannot parse YAML to see it. The flow
   mapping is a deliberate false alarm with an honest message; both are argued in the code.
3. **Upstream: does ce-agent-kit want the frozen-zone test moved ahead of detection?** Still
   not raised there by this card, and this card does not depend on the answer. The shape of
   the change is now known, from a source read at `01e4dc52`: hoist
   `canonicalFrozenZone(path)` into `Validator.Validate` ahead of **both**
   `validateCanonicalTask` and `isDecisionDoc`. The second is the part a dva-side guard
   cannot reach at all — `validator.go:55` routes archived *decision* documents
   (`decision`/`decisions`/`adr`/`adrs` as a path component) to a validator with no
   frozen-zone check whatsoever, so no frontmatter rescues them. dva has no such path today;
   if it ever adds one, the guard added here will not help.
