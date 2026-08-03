---
id: TASK-065
title: "The .gitignore check only matches its own path literally, so a parent-directory rule reads as unignored"
type: fix
priority: P2
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/cli/gitignore.go (isDvaIgnored), plus the test file it never had"
verified-at: 2026-08-03T11:55:33+09:00
archived-at: 2026-08-03T11:55:33+09:00
verification-summary: |
  All 4 criteria MET. isDvaIgnored (internal/cli/gitignore.go:92-153) resolves the
  ancestors of config.DotDirName outermost-first, matching git's subtree-exclusion plus
  negation-carve-out semantics. All 22 TestIsDvaIgnored subtests pass, including the
  three ancestor forms (.sb/, .sb, /.sb/) and — importantly — the negative cases
  ("empty" and "unrelated rules only" both false), so the fix does not trade a false
  positive for a false negative.
  The false positive is gone where git already ignores the path: in
  ~/mydevbox/gorisa-devbox (real ancestor rule ".sb/" at .gitignore:178), `dva validate`
  emits 0 lines matching "not in your .gitignore".
  Two commits match the task's narrative: f2c21e3 (honor ancestor rules) then d43eb42
  (read negations instead of stopping at the first match).
  The third criterion's literal shell repro no longer reproduces, for a reason that is
  not a defect: scripton-dns-bridge-devbox no longer has a .sb/dva directory on disk, and
  the later TASK-080 added an existence gate (gitignore.go:191-193) that suppresses the
  warning when there is nothing committable. That interaction is itself covered by
  TestGitignoreWarningNeedsSomethingCommittable, so it is tracked, not hidden.
---

# Task 065: Recognize ancestor rules in the `.gitignore` check

## Problem

Every DVA command prints this when it thinks `.sb/dva/` is uncommitted-unsafe:

```
⚠️  [warn] .sb/dva/ is not in your .gitignore. Transient markers might be committed.
         Run 'dva doctor --fix' to auto-fix or add '.sb/dva/' to .gitignore manually.
```

In `gorisa-devbox` that warning is **false**. Git ignores the directory, via a rule one
level up:

```
$ git check-ignore -v .sb/dva
.gitignore:178:.sb/	.sb/dva
```

`isDvaIgnored` (`internal/cli/gitignore.go:67`) compares each line against `.sb/dva` and
`.sb/dva/` and nothing else. `.sb/` equals neither, so the check concludes "not ignored"
about a path git is already excluding — and then tells the user to add a rule they do not
need.

## Root cause

`config.DotDirName` is `.sb/dva` — a **two-segment** path. Git excludes an entire subtree
when any ancestor directory is listed, so `.sb/` covers `.sb/dva` completely. An exact-line
comparison cannot express that relationship, so the check holds a second, weaker copy of a
rule git already owns.

Same shape as [TASK-057](057-dead-self-referencing-urls.md) and
[TASK-060](060-go-module-path-does-not-resolve.md): a fact restated where nothing
verifies the restatement. Here the authority is git's ignore semantics.

Notably `.sb/` is the *more natural* way to write the rule — it ignores DVA's whole dot
directory rather than one child — so the check misfires on the better-written config.

## Why it survived

`internal/cli/` has **no `gitignore_test.go`**. `isDvaIgnored` has never had a test, so no
case ever encoded what "ignored" is supposed to mean.

## Fix shape

1. Match any **ancestor prefix** of `config.DotDirName`, not just the full path — for
   `.sb/dva` that is `.sb` and `.sb/dva`, each accepted bare, with a trailing slash, and
   root-anchored with a leading `/`.
2. Add the missing test file with a table covering ancestor rules, the anchored and
   trailing-slash spellings, and the negative cases that must still warn.

Deliberately **not** implementing gitignore semantics: globs (`.sb/*`), negations, and
non-root anchoring stay uninterpreted. The goal is to stop warning about rules that plainly
cover the path, not to reimplement git.

A negation cannot make this wrong, incidentally — git documents that a file cannot be
re-included once a parent directory is excluded, so treating `.sb/` as decisive is correct
rather than approximate.

## Non-goals

- Do not shell out to `git check-ignore`. It would make a warning depend on git being on
  PATH and on the cwd being a work tree, and it costs a subprocess on every command.
- Do not touch `ensureGitignore`'s append behaviour beyond what the shared helper gives it.
  Fixing the predicate already stops `doctor --fix` writing a redundant line.
- Do not widen `DotDirName`'s meaning or move it.

## Acceptance criteria

- [x] A parent-directory rule counts as ignored | verify: `go test ./internal/cli/ -run TestIsDvaIgnored`
- [x] The false positive is gone where git already ignores the path | verify: `cd ~/mydevbox/gorisa-devbox && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva validate 2>&1 \| /usr/bin/grep -c 'not in your .gitignore' ; test $? -ne 0`
- [x] A repo with no rule at all still warns | verify: `cd ~/mydevbox/scripton-dns-bridge-devbox && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva validate 2>&1 \| /usr/bin/grep -q 'not in your .gitignore'`
- [x] Full suite green | verify: `make test`

## Evidence

Measured 2026-07-30 across the six `~/mydevbox` repos that have a `.sb/dva` directory:

| repo | literal `.sb/dva` line | `git check-ignore` | DVA warned |
| --- | --- | --- | --- |
| cwrapper-devbox | yes | ignored | no |
| flow-knowchain-devbox | yes | ignored | no |
| resume-devbox | yes | ignored | no |
| **gorisa-devbox** | **no** | **ignored** (via `.sb/`) | **yes — false positive** |

`scripton-dns-bridge-devbox` warns correctly: it has no `.sb` rule in `.gitignore` and
`git check-ignore` agrees the path is unignored. That case is the reason the warning exists
and must keep firing.

An earlier pass recorded "6 dirs, 6 gitignored, 0 unignored" and read that as refuting the
warning entirely. It did not: that measurement used `git check-ignore` and so described
git's view, while the warning describes `isDvaIgnored`'s view. The two disagreeing on
exactly one repo *is* the defect, which aggregate counts hid.

## Result

`isDvaIgnored` now tests each line against every ancestor prefix of `DotDirName`, generated
by `ignoreRulesCovering` rather than written out, so the set follows the constant if it ever
changes. `gorisa-devbox` went 1 warning → 0; `scripton-dns-bridge-devbox`, which genuinely
has no rule, still warns. `make test` green, `internal/cli` coverage 54.5%.

`internal/cli/gitignore_test.go` is new — 15 predicate cases plus a derivation guard. Two
are pinned *negative* on purpose: a descendant-only rule (`.sb/dva/cache/`) must still warn,
because markers elsewhere under `.sb/dva` remain committable, and `.sb/*` stays
uninterpreted per the non-goals above.

### `git check-ignore` is not a usable oracle for absent paths

A verification sweep across all 31 configs first reported **14 repos where git said
unignored and DVA said ignored**, which read as a regression the fix had introduced. It was
not. Each of those repos has a literal `.sb/dva/` line — so the *old* code agreed with the
new one and the fix changed nothing for them — and no `.sb/dva` directory on disk:

```
$ cd ~/mydevbox/matdosa-devbox            # .gitignore:66 is `.sb/dva/`
$ git check-ignore -v .sb/dva             # -> no match
$ git check-ignore -v .sb/dva/            # -> .gitignore:66:.sb/dva/
```

A pattern ending in `/` matches directories only, and for a path that does not exist git
cannot establish directory-ness, so it declines to match. The sweep queried without the
trailing slash. DVA was correct in all 14; the baseline was wrong.

Worth keeping because the failure is silent and inverted — the flawed probe accused the
correct code. Any future comparison against git must pass the trailing slash, or the
directory must exist.

## Follow-up: negations, found in review of this fix (d43eb42)

This task's fix read the file for the first line naming `.sb/dva` or an ancestor and stopped
there. Two independent reviewers flagged that as a false positive, and they were right — the
doc comment written here claimed "a later negation cannot make this conclusion wrong", which
is true for an ancestor's *descendant* but not for the ancestor itself. Verified against git
with the paths on disk:

| `.gitignore` | git | this fix said |
| --- | --- | --- |
| `.sb/` then `!.sb/dva/` | ignored | ignored ✓ |
| `.sb/` then `!.sb/` | **not ignored** | ignored ✗ |

gitignore has two rules that look like one. Ordinary *last-matching-pattern-wins* applies to
the path being tested; the carve-out — a path cannot be re-included once a **parent**
directory is excluded — applies only to descendants. Collapsing both into "any covering line
means ignored" is right for the carve-out and wrong for the ordinary rule, and it fails in the
dangerous direction: the warning is suppressed while git leaves the markers committable.

Fixed in `d43eb42` by resolving each ancestor independently, outermost first, with the first
excluded one decisive. Seven cases added, each expectation checked against real git rather
than against the new implementation.

The lesson is narrower than "read negations": my review of this fix reasoned from the
gitignore spec and concluded no false positive was reachable, having deliberately avoided the
`git check-ignore` probe that misled the sweep above. The reasoning was sound for the eight
*positive* forms the code generates and never considered a ninth line that unmakes them. The
reviewers ran real git against real files on disk — the trailing-slash trap avoided by
creating the directory, not by dropping the probe.
