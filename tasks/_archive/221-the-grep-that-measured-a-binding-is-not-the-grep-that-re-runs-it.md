---
id: TASK-221
title: "The grep that measured a binding is not the grep that re-runs it"
type: docs
priority: P3
effort: M
created-at: 2026-08-20T20:45:00+09:00
source: "found while fixing TASK-199's bindings: a sweep of agent-mesh-flows/ returned nothing for a string that is demonstrably served from that directory. The cause was the grep, not the string"
scope: "verify: bindings in tasks/**/*.md and the convention that governs them. No Go source outside tools/doccheck, no behaviour change to dva. TASK-220's three axes are separate and neither supersedes this one"
status: done
completed-at: 2026-08-26T13:13:28+09:00
completion-summary: "Gate mechanical verify bindings against agent-wrapped grep/find names with shell-aware command detection."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "dva test && dva lint && make doc-check"
    result: "passed; doccheck reports 171 wrapped-tool bindings and zero bare-tool bindings"
  - kind: fail-closed
    command-or-step: "unit and scratch probes for bare, quoted, escaped, substitution, redirection, and human binding forms"
    result: "all required forms classified correctly; planted bare grep/find made doccheck fail with two exact findings"
  - kind: runtime
    command-or-step: "plant the 067 prohibited sentence through the canonical symlink target and a regular library file"
    result: "explicit symlink and ordinary recursive paths each failed in their intended state; planted lines were removed line-scoped"
  - kind: review-rework
    command-or-step: "correct two failed-review rounds and rerun focused/full gates"
    result: "human/lexer/vacuity findings closed; precursor census corrected to 127/168 and transition reconciled to final 0/167"
quality-review: pass
quality-reviewed-at: 2026-08-26T13:14:54+09:00
quality-review-evidence:
  - "independent contract judge confirmed the corrected 127/168 to 0/167 close census and 126-rewrite plus one-reclassification accounting"
  - "normal versus malformed human forms, shell quote removal, escaped commands, redirections, substitutions, controls, and real-corpus floor are covered"
  - "TASK-025 and TASK-067 special bindings remain valid; focused/full tests, lint, doccheck, and diff validation passed"
quality-review-receipt: tmp/task-management/direct/queue-run/task-221-review-receipt.json
archived-at: 2026-08-26T13:16:15+09:00
verified-at: 2026-08-26T13:16:15+09:00
verification-summary: "Mechanical task bindings now use stable absolute grep/find binaries, and doccheck fails closed on shell-obfuscated bare tool invocations while preserving explicit human-review forms."
---

# Task 221: The grep that measured a binding is not the grep that re-runs it

## Summary

`grep` inside an agent session is a shell function, not `/usr/bin/grep`. It
execs the Claude Code binary as `ugrep` with, among others,
`-G --ignore-files --hidden -I --exclude-dir=.git`. `--ignore-files` makes it
honour `.gitignore`. A human re-running the same binding in a terminal gets BSD
grep 2.6.0-FreeBSD, which honours nothing of the sort.

Same tree, same second, same command text:

```
$ printf 'SENTINEL\n' > tmp/greprobe/probe.txt      # tmp/ is gitignored (.gitignore:34)
$ grep -rl SENTINEL . | wc -l                        # agent's grep
0
$ /usr/bin/grep -rl SENTINEL . | wc -l               # a reader's grep
1
```

Every binding in `tasks/` was measured in an agent shell unless it names an
absolute path. At `330f96e` — the commit this card is filed on top of —
**132 of the 180 wrapped-tool bindings call a bare `grep` or `find`**; 48 name an
absolute path. So for 132 bindings the number recorded beside the criterion is
the agent's number, and nothing establishes that it is the reader's.

That census is taken over criterion lines only, with fenced regions removed
exactly where `tools/doccheck` removes them, `human —` bindings dropped, and
**the first code span after `verify: `** taken as the binding — the population
rule [TASK-220](220-a-verify-binding-that-only-runs-on-this-machine-is-not-a-binding.md)
states in full. Each clause is load-bearing. `tasks/todo/221-*.md` is excluded
from every figure in this card: it quotes bare greps in its prose and its own
criteria are bindings, so a sweep that included it would move the number it
reports by the act of writing it down. Filing this card raises the first row's
denominator from 180 to 184.

| population rule | `3ad895a` | `330f96e` |
|---|---|---|
| criterion line, fences stripped, `human —` dropped, first span after `verify: ` | **136 of 167** | **132 of 180** |
| the same, but **every** code span on the line | 136 of 167 — *identical* | 135 of 183 — **8 lines disagree** |
| the same, but fenced regions **kept** | 138 of 169 | 135 of 186 |

An earlier draft reported `145 of 176` for the third row at `3ad895a`. The rule
as stated above reproduces `138 of 169` at that same commit, so the earlier
figure is **dropped rather than carried forward**: it is a number no written rule
reproduces, which is the defect this card is about, one layer up.

The third row is wrong in two
directions at once. It counts two **fenced quotations** — `199:75` and `199:86`,
both inside ``` blocks in
[TASK-199](../_archive/199-three-archived-bindings-cannot-produce-the-result-recorded-beside-them.md),
the card this one cites as precedent — as live bare greps. And because it
requires the backtick to be adjacent to `verify: `, it cannot see a binding
introduced by prose: `213:85` (`grep -c 'nothing should have run' …`) and
`213:90` (`go list … | grep -c 'dva/internal/cli'`) are both bare and both
invisible to it. So the criterion could print `bare=0` with two bare greps
standing. A card whose census counts its own precedent's quotations and misses
two live instances is the defect it exists to name.

The second row is the important one, and it has now sprung. At `3ad895a` "first
span" and "every span" agreed on all 167 lines — **zero disagreement** — so
nothing in the corpus distinguished them, and an earlier draft of this card could
only *predict* where they would part. TASK-199 integrated at `14e5472`, and at
`330f96e` the two rules disagree on **8 lines**. Every disagreement is an
absolute-path binding whose *later* spans contain a bare tool word, so under
"every span" all eight read as unfixed. They come in three shapes, and only the
first was predicted:

| shape | lines | the later span |
|---|---|---|
| the annotation quotes the bare original the fix replaced | `060:162`, `060:164`, `066:89`, `115:125`, `115:126`, `119:89` | e.g. `` `grep -rn 'cfgDir + "/"' internal/ \| wc -l` `` |
| the tool name is used as a noun in prose | `220:135` | `` `grep` `` — "bound to the checker's own output rather than to a `grep` over `tasks/`" |
| the annotation quotes a **program's error message** | `199:106` | `` `find: \|: unknown primary or operator` `` |

The prediction named seven lines and all seven are present; the eighth,
`199:106`, was not predicted and is the one that settles the design question.
The first shape is produced by a convention this repo deliberately adopted — keep
the original beside the fix so the record is not rewritten — so it is permanent
and grows with every rewrite this card causes. The second is unavoidable in any
card *about* tooling. But the third is neither convention nor prose: it is a
string of captured program output that happens to begin with a tool name. No
line-local heuristic separates it from a binding, because nothing about the
characters distinguishes `find: |: unknown primary or operator` from a command.
That is the whole argument for criterion 4 being bound to `doccheck`'s declared
population rather than to any sweep of the text.

There is no line-local rule that separates a binding from a quotation of one, and
`task-validator` does not have one either: it is an agent that reads the line and
decides (`agents/task-validator.md:39`, *"extract each acceptance criterion AND
its `| verify:` binding"*). Nothing deterministic defines this population. That is
why criterion 4 below is bound to `doccheck`'s own output rather than to a `grep`
over `tasks/` — the check does not *discover* the population, it **declares** it,
and every number in this card is then the number the gate enforces.

**The census moves with whoever is writing that afternoon, in both directions.**
Under the rule stated above:

| commit | census | delta | what moved it |
|---|---|---|---|
| `dc762ca` | 128 of 159 | | |
| `3ad895a` (+11 commits) | 136 of 167 | +8 spans, **all bare** | cards `216` (3), `217` (1), `218` (1), `219` (3), filed by one session in one sitting |
| `330f96e` (+8 commits) | 132 of 180 | +13 spans, **all absolute-path**, and 4 bare spans converted | cards `199` (+4), `220` (+4), `222` (+5) new; `115` (2), `119` (1), `199` (1) rewritten by TASK-199 |

An earlier draft read the first delta alone and concluded *"the convention is
losing ground while the card describing it is being written."* The second delta
falsifies that sentence, and it is deleted rather than softened. But the
replacement is not the mirror claim — seventeen consecutive absolute-path spans
(13 new, 4 rewritten) do not show the convention winning
either. **Both deltas are one session's habits.**
Twenty-one spans across 19 commits, and the direction is set by which afternoon
you sample; nothing in the corpus resists either move. That is the argument for a
check, and it is stronger than either count, because it survives the next
sampling.

## What actually diverges, measured rather than feared

117 of the bare-grep spans are pure lookups (no `make`, `go`, `dva`, `git`,
redirection or in-place edit). Each was executed twice — as written, and with
every bare `grep` replaced by `/usr/bin/grep` — and the pair compared on both
stdout and exit status:

| | count |
|---|---|
| pure lookups actually executed both ways | 117 |
| differed in **stdout** | 8–10, one run each (see below) |
| differed in **exit status** | **0** |

The exit-status row is the one that matters for scoring, and it is zero: **of
the 117 executed**, no criterion changes verdict today. This is a latent trap,
not a live failure, and the card is filed at P3 for that reason.

The stdout row is a sample, not a measurement, and the reason is this card's own
subject one layer up: **the wrapper's output ordering is not stable.** Five runs
of `grep -rn 'checkArchiveFrontmatter' tools/doccheck/` over an unchanged tree
produced four identical md5s and one different; five runs of `/usr/bin/grep`
produced one md5 five times. Intermittent is worse than always-different here,
because a single re-run appears to confirm the first. Re-running the 117-pair
harness gives 8, 9 or 10 stdout differences depending on the run. Most of them
are file ordering or the `./` prefix that BSD grep prepends and the wrapper does
not — but "only ordering" is itself too weak: at `_archive/022:64` the ordering
feeds `| head`, so the two greps display entirely different evidence at the same
exit status. **Ordering is load-bearing wherever a binding pipes into `head`,
`tail`, `sed -n` or `awk NR`.**

## One difference is a live defect, for two independent reasons

`tasks/_archive/025-advanced-md-precedence-omits-os.md:65`:

```
verify: grep -rn --include='*.md' -A9 'precedence order' . | grep -v '^\./tasks/' | grep -iE '[0-9]+\. *(Explicit )?CLI flags *$'
```

recorded as **"PASS (no match; 12 live doc lines swept, >0 confirms the sweep is
live)"**. It matches today under both greps, and for a different reason under
each:

- **Under the agent's grep** it matches `tasks/_archive/025-…md-42-…`, its own
  card. The `grep -v '^\./tasks/'` exclusion is anchored on a `./` prefix the
  wrapper never emits, so the filter is inert — it excludes nothing and always
  has.
- **Under `/usr/bin/grep`** it matches `./tmp/gap-analysis/convergence2.md-49-…`,
  a gitignored scratch file the wrapper cannot see at all.

One binding, two greps, two different unintended matches, and a recorded result
that is now wrong under either. The corpus drifted too: the recorded `12 live
doc lines` is **1** today — `skills/dva/references/advanced.md:90` is the only
non-`tasks/`, non-gitignored file left that says `precedence order`. So the
sweep is nearly empty *and* reports a match, and neither fact is visible from
the criterion. `12 live doc lines swept` is the denominator idea stated in prose
instead of printed, which is the shape
[TASK-199](../_archive/199-three-archived-bindings-cannot-produce-the-result-recorded-beside-them.md)
closed for five other bindings.

The inert-exclusion shape reproduces on demand: an independent reviewer of this
card filtered wrapper output with `/usr/bin/grep -v '/tasks/'` while checking it,
and the filter excluded nothing, for exactly the reason above.

## The second axis: `-r` cannot see this repo's single sources

This repo single-sources by symlink. Six tracked symlinks:

| link | target |
|---|---|
| `agent-mesh-flows/shared/library/dva-schema.md` | `skills/dva-config/references/schema-reference.md` |
| `.agents/skills`, `claude-plugin/skills` | `skills/` |
| `internal/{cli,config,lifecycle}/AGENTS.md` | `AGENTS.md` |

`grep -r` does not descend into a symlink; `-R` does — and only the agent's grep
honours `-R` for a symlinked *file*, which BSD grep skips under both flags.
Measured on the string `both forms work at runtime`, which lives in
`schema-reference.md` and is served from the library directory:

| command | result |
|---|---|
| `grep -rl … agent-mesh-flows/` (agent) | no match |
| `grep -Rl … agent-mesh-flows/` (agent) | `agent-mesh-flows/shared/library/dva-schema.md` |
| `/usr/bin/grep -rl … agent-mesh-flows/` | no match |
| `/usr/bin/grep -Rl … agent-mesh-flows/` | no match |

`tasks/_archive/067-version-field-rule-stated-three-incompatible-ways.md:148`
is the live instance: `! /usr/bin/grep -rniE '…' agent-mesh-flows/` claims *no
library file* makes a certain version claim, while structurally unable to read
the one library file most likely to make it.

It is not a gate that cannot fail — that is an overclaim, and it was checked
rather than assumed. `git ls-files -s -- agent-mesh-flows` reports **35 mode
`100644` entries and exactly one mode `120000`**. Planting the offending sentence
in any of the 35 turns the gate red; planting it behind the symlink leaves it
green. So the gate reads 35 of 36 tracked paths and is blind to precisely the
36th, `dva-schema.md` — the one whose target is the canonical statement of the
rule. The denominator is taken from the index and not from `find` for the reason
`060:162` records: `/usr/bin/find agent-mesh-flows -type f` prints **39** in the
primary checkout and **35** in a fresh worktree at this commit, without a tracked
file differing between them. Its sibling at `:147` is accidentally safe: it
also greps `internal/cli/library_reference.txt`, the generated concatenation,
which contains the symlinked file's bytes.

## What to do

The cheap correct answer is a convention plus a check: **a binding names the
absolute path of any wrapped tool it invokes**, so its recorded number is the
reader's number. 48 spans already do.

The rule cannot name only `grep`. `find` is wrapped too — the shell function
execs the same binary as `ARGV0=bfs … -S dfs -regextype findutils-default`. At
`330f96e`, three live binding spans call a bare `find` (`126:104`, `127:117`,
`197:67`) and six already name `/usr/bin/find` (`057:115`, `057:119`, `060:164`,
`066:89`, `072:94`, `115:126`), so the convention is already being applied to
`find` by hand, by people no rule tells to. A rule naming only `grep` will be read
as licensing bare `find`.

Two footnotes on those nine, both of which are the census rule doing work. A raw
line-wide sweep reports a fourth bare span, `199:86` — a fenced quotation.
And `197:67`'s span *opens* with `human —` **inside** the backticks rather than
after them, so it is not dropped by the `human —` clause and `task-validator`
would try to execute `human` as a command; that is a different defect and belongs
to whoever disposition `197`.

The expensive part is not the 132 rewrites, it is deciding which recorded results
must be re-measured afterwards — a rewrite that silently changes a recorded count
is worse than the bare grep was.

Rejected alternative: teach `task-validator` to run bindings under
`/usr/bin/grep`. That fixes scoring and leaves every recorded number
unverifiable by hand, which is the actual promise a binding makes.

## Completion Criteria

- [x] `doccheck` flags a verify binding that invokes a bare wrapped tool | verify: `n=$(/usr/bin/grep -rl 'func checkBindingTool' tools/doccheck/ | wc -l | tr -d ' '); echo "declarations=$n"; [ "$n" -eq 1 ]` — prints `declarations=0` and exits 1 today
- [x] The check is tested against a planted bare `grep`, a planted bare `find`, and their absolute forms | verify: `n=$(/usr/bin/grep -rho 'func TestBindingTool[A-Za-z]*' tools/doccheck/ | sort -u | wc -l | tr -d ' '); echo "test funcs=$n"; [ "$n" -ge 3 ]` — prints `test funcs=0` and exits 1 today. Bound on the test source: a `go test -run` naming a test that does not exist yet exits 0, and `doccheck`'s own TASK-136 guard rejects such a binding
- [x] The check reads the binding span the way `doccheck` reads it, not the way a line-wide grep does | verify: human — inspect the four population-shape tests. Four test cases are judged separately because they pull in opposite directions: a bare `grep` inside a ``` fenced quotation → 0 findings; a bare `grep` in a code span *not adjacent* to `verify: ` but before any annotation → 1 finding; an absolute-path binding whose **annotation** quotes the bare original it replaced → 0 findings; and an absolute-path binding whose annotation quotes a program's **error message** beginning with a tool name → 0 findings. All four shapes are live at `330f96e`: `199:75`/`199:86` fenced, `213:85`/`213:90` non-adjacent, the six annotation quotations the Summary tabulates, and `199:106` (`` `find: \|: unknown primary or operator` ``). A check that gets the second right by reading every span on the line gets the third and fourth wrong, which is why one criterion covering "reads the span correctly" would certify itself. The fourth case is separate from the third and not a duplicate of it: the third can be excluded by a rule about *where* the span sits relative to the em-dash annotation, and the fourth cannot be excluded by any rule about the characters at all
- [x] Every bare wrapped-tool call in a binding span is gone | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && n=$(make doc-check 2>/dev/null | /usr/bin/grep -c '^bare_tool_bindings: *0$'); echo "bare_tool_bindings=0 lines in doc-check output: $n"; [ "$n" -eq 1 ]` — prints `0` and exits 1 today, because `doc-check` reports no such counter. Bound to the checker's own output rather than to a `grep` over `tasks/` for the reason the Summary gives: every shell-expressible sweep of this population has been wrong, this card's first attempt included, and the number a reader sees must be the number the gate enforces
- [x] The recorded census is re-measured at the commit this card is closed at, not carried forward | verify: human — the Summary stamps every figure with a commit: `128 of 159` at `dc762ca`, `136 of 167` at `3ad895a`, `132 of 180` at `330f96e`. It moved twice in 19 commits and reversed direction, so a number with no commit beside it is the defect one layer up. Re-measure at close and record **which of the two mechanisms moved it** — new spans arriving, or existing spans rewritten — because they argue in opposite directions and a bare delta hides which happened. Exclude `tasks/todo/221-*.md` from the sweep, or the card's own four bindings enter the denominator it reports
- [x] No recorded count changed silently in the rewrite | verify: human — for every binding whose recorded number changes when it is rewritten, the new number is measured, recorded, and the old one kept beside it with the reason
- [x] `025:65` is rewritten so its exclusion fires and its denominator is printed | verify: `f=$(ls tasks/_archive/025-*.md); [ -f "$f" ] || { echo "025 card not found — nothing was measured"; exit 2; }; anchor=$(/usr/bin/grep -c "grep -v '\^" "$f" || true); den=$(/usr/bin/grep -c 'swept lines' "$f" || true); echo "inert ./ anchors=$anchor printed denominators=$den"; [ "$anchor" -eq 0 ] && [ "$den" -ge 1 ]` — prints `inert ./ anchors=1 printed denominators=0` and exits 1 today. Bound on the card text, not on the sweep's outcome: with the exclusion corrected to `/tasks/` the sweep already returns `offenders=0 over 1 swept lines`, so an outcome binding would pass before the rewrite and certify itself
- [x] `067:148` can fail | verify: human — plant the offending sentence in `skills/dva-config/references/schema-reference.md`, confirm the rewritten binding goes red through the `dva-schema.md` symlink, and remove the planted line by line, not by `git checkout`. Then plant it in any other file under `agent-mesh-flows/` and confirm the *unrewritten* binding already goes red there — the gate is blind to one path of 40, not to all of them, and a criterion that does not separate those two states certifies the wrong claim
- [x] `make doc-check` passes with the new check active | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## Resolution

`doccheck` now owns the binding population: task criteria only, fenced regions removed, the
first inline span after `verify:`, and human/table forms excluded. It recognizes `grep` and
`find` only in shell command position, reports the full population, and rejects bare names.

Close census at precursor `4088f9c`, re-evaluated by the final declared rule and excluding this
card: **127 bare of 168 wrapped-tool bindings**. Close census after the rewrite: **0 of 167**.
The transition comprises 126 existing tool rewrites and one malformed-human reclassification:
TASK-197's backticked human span was mechanical under the final rule, then left the population
when normalized to `verify: human — …`. No new span entered. The full gate prints 171 because this card contributes four absolute-path self-checks,
which the criterion explicitly excludes from its close census. The historical `132 of 180` at
`330f96e` remains above as the old text extractor's measurement rather than being retrofitted to
the new command-position population.

Only TASK-025's recorded number changed: its binding now prints `offenders=0 over 1 swept lines`
and keeps the former 12 beside the reason it was unreproducible. TASK-067 now checks the canonical
symlink explicitly in addition to the recursive corpus. The convention and both gate counters are
recorded in `AGENTS.md`.

## Review Rework

The first done-review failed. The shared extractor inverted the two human forms: it included
normal `verify: human — …` criteria and excluded the malformed backticked form that a validator
would try to execute. The shell lexer also missed quoted or escaped command words and commands
after leading redirections. The rework corrected the shared extractor, normalized the affected
human criteria in TASK-070/194/197/220 and this card, added quote-removal/redirection coverage,
and added a real-corpus floor. A second review found the precursor census had already assumed
TASK-197's final human form; the corrected before/after census is recorded above.

## Open Questions

- Whether the convention should be an absolute path per tool or a documented
  "run bindings with `env -i`"-style preamble. The absolute path is the smaller
  change, and since [TASK-199](../_archive/199-three-archived-bindings-cannot-produce-the-result-recorded-beside-them.md)
  landed at `14e5472` it is the whole convention in cards `060` (3 of 3 spans),
  `066` (1 of 1), `115` (2 of 2), `119` (1 of 1) and `199` (5 of 5 live spans,
  plus two fenced quotations of the originals at `199:75` and `199:86`). Measured
  at `3ad895a` — before that branch — the same five cards read 3, 1, **0**,
  **0**, **0**, so this sentence is true only from `14e5472` onward. It was
  written as "already the majority convention" at a point where it was 2 of 5.
- `--ignore-files` is the wrapper behaviour with the widest blast radius and it
  is invisible: a binding that sweeps a gitignored directory reports a clean tree
  to an agent and a dirty one to a human. Whether any binding intends to sweep
  gitignored paths at all is worth answering before rewriting them.
- Which tools besides `grep` and `find` are wrapped. The rule has to enumerate
  them, and the enumeration is the part that will rot. Whether `doccheck` should
  derive the list at runtime — by asking the shell what each tool resolves to —
  or hold a literal list is the real design question in this card.
- This overlaps [TASK-220](220-a-verify-binding-that-only-runs-on-this-machine-is-not-a-binding.md)
  axis 2 (`cd` into this checkout): both are "the binding measured something the
  reader cannot reproduce". If 220 is done first, its `doccheck` check is the
  natural home for this one, and this card shrinks to a rule plus a rewrite.

## References

- [TASK-220](220-a-verify-binding-that-only-runs-on-this-machine-is-not-a-binding.md)
  — the same promise broken by path, corpus and escaping rather than by tool, and
  the source of the population rule this card's census uses
- [TASK-199](../_archive/199-three-archived-bindings-cannot-produce-the-result-recorded-beside-them.md)
  — the `exit 2` "nothing was measured" pattern the criteria above copy, and the
  card whose two fenced quotations this card's first census miscounted
- [TASK-202](202-a-disposition-kept-two-bindings-that-no-longer-measure-what-they-claim.md)
  — the same failure mode reached from a different direction
