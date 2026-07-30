---
id: TASK-057
title: "Self-referencing URLs are dead — $schema points at a path that never existed, migration guide names the wrong repo and branch"
type: fix
priority: P2
status: done
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — agent-mesh-flows/shared/library/, internal/cli/library_reference.txt, internal/config/validate_warnings.go, dva.yml"
---

# Task 057: Make DVA's own URLs resolve

## Problem

Every URL DVA prints or writes about itself is broken, and one of them is stamped
into user configs by the AI generator.

### 1. `$schema` — right branch, path that never existed

The header points at `.../dev-virtual-auto/master/schema.json`. `schema.json` has **never
existed at the repo root** (`git log --all -- schema.json` is empty); it lives at
`internal/config/schema.json`. So editor validation silently does nothing for everyone
carrying this header — the failure mode of a bad `$schema` is no diagnostics, not an error,
which is why it survived into 56 configs.

### 2. Migration guide — wrong repo *and* wrong branch

`internal/config/validate_warnings.go:13`

```go
const migrationGuideURL = "https://github.com/ScriptonBasestar/dva/blob/main/docs/40-declarative-stack-and-plans.md#11-migration"
```

Two independent errors: the repo is `dev-virtual-auto` (`ScriptonBasestar/dva` is the **Go
module path**, reused here as if it named the repository), and branch `main` has never
existed — `git branch -r` lists only `origin/master`. The document itself is fine.

This URL is printed in validate warnings, so it is the one users are most likely to click.

## Why it spread — same mechanism as the removed-keys contamination

```
agent-mesh-flows/shared/library/reference-examples.md:12   ← authored source
  → make generate → internal/cli/library_reference.txt:1151
  → am flows read it from disk and emit the header into generated dva.yml
~/mydevbox: 56 of 83 real configs carry the dead URL
```

(An earlier diagram had a `→ compiled into bin/dva` step. Wrong: `init.go`'s `//go:embed`
of that file bound a variable nothing referenced, so the binary was a dead end. Corrected
in [TASK-061](../done/061-go-facts-hand-copied-into-flow-library.md).)

`TestRemovedKeysAbsentFromGeneratorCorpus` already guards this corpus against
teaching *removed keys*. Nothing checks that URLs the corpus teaches resolve. That
is the actual gap: a second copy of knowledge that nothing compiles.

## Measured scope (2026-07-30, verified with `/usr/bin/find | xargs /usr/bin/grep`)

| Location | Count | Note |
| --- | --- | --- |
| `agent-mesh-flows/shared/library/reference-examples.md:12` | 1 | authored source of the `$schema` header |
| `internal/cli/library_reference.txt:1151` | 1 | generated — fixed by `make generate` |
| `dva.yml:1` | 1 | the repo's own config |
| `internal/config/validate_warnings.go:13` | 1 | migration guide URL |
| `~/mydevbox/**/dva.yml` | 56 of 83 | user configs already stamped |

Two earlier counts were inflated by unpruned paths: "27 places" counted 24 backup copies of
user configs; "12 files" counted 10 `.opencode/node_modules` plus `bin/dva`. The 15
`examples/` files using the relative form are correct and stay.

## Open question — settled 2026-07-30: the repo is public

Probed with `curl` rather than assumed (`gh` was unusable — the org forbids fine-grained
tokens over 366 days). `dev-virtual-auto` and its `internal/config/schema.json` and
migration-guide paths returned **200**; root `schema.json` 404'd; `ScriptonBasestar/dva`
404'd everywhere. So the canonical form was confirmed reachable, not merely plausible.
(Hours later the repo was renamed to `dva` and these URLs were swept again — TASK-060.)

## Fix shape

1. Fix the authored source (`reference-examples.md`), then `make generate` to propagate
   into `library_reference.txt` — never hand-edit the generated file.
2. Fix `migrationGuideURL` (repo name and branch) and the repo's own `dva.yml:1`.
3. Add a guard so this cannot rot again: extend the generator-corpus test to assert that
   any self-referencing GitHub URL names the real repo and an existing branch, and that a
   `$schema=` path resolves to a file in the tree. Offline string checks only.
4. Decide separately whether to rewrite the 56 user configs — it is the user's tree.

## Non-goals

- Do not rewrite the 15 correct relative `$schema` paths in `examples/`.
- Do not add network access to tests.

## Acceptance criteria

- [x] Authored corpus teaches a resolving `$schema` URL | verify: `grep -q 'internal/config/schema.json' agent-mesh-flows/shared/library/reference-examples.md`
- [x] Generated embed matches the source | verify: `make generate && git diff --exit-code internal/cli/library_reference.txt`
- [x] No source file references the nonexistent root schema path | verify: `/usr/bin/find . -path ./.git -prune -o -path ./tmp -prune -o -path ./bin -prune -o -path ./.opencode -prune -o -path ./tasks -prune -o -type f -print0 | xargs -0 /usr/bin/grep -l 'master/schema.json' ; test $? -ne 0`
- [x] Migration guide URL names the real repo and an existing branch | verify: `grep -q 'dva/blob/master/docs/40-declarative-stack-and-plans.md' internal/config/validate_warnings.go`
- [x] Corpus URL guard exists and fails on a planted bad URL | verify: `go test ./internal/config/ -run TestGeneratorCorpusURLs`
- [x] Full suite green | verify: `make test`
- [x] 56 user configs rewritten, or the sweep explicitly declined | verify: `/usr/bin/find ~/mydevbox -name dva.yml -not -path '*/node_modules/*' -print0 \| xargs -0 /usr/bin/grep -l 'master/schema.json' \| /usr/bin/grep -vE '/tmp/\|/\.omo/evidence/' ; test $? -ne 0`
- [x] README.md URLs name a repo that exists | verify: `human — via TASK-060's rename; the download URL still 404s, see TASK-063`

(Criterion 3 prunes `tasks`: this file quotes the dead URL as evidence, and a guard that
forbids its own evidence cannot be documented.)

## Result — repo side done

`reference-examples.md:12` and `dva.yml:1` → canonical raw URL; the first propagated to
`internal/cli/library_reference.txt:1151` via `make generate`. `migrationGuideURL` →
`dev-virtual-auto`/`master`, confirmed against a rebuilt binary in primeno1-devbox: the
link `dva validate` prints is now the 200 one. `make test` green.

New guard `TestGeneratorCorpusURLs` (`internal/config/corpus_urls_test.go`) audits 5 URLs
across 125 files, offline, and asserts **both** counters non-zero — a walk that matches
nothing passes forever while guarding nothing.
`TestGeneratorCorpusURLsDetectsPlantedDefects` pins the detector against four planted
defects so a regex edit cannot defang it. The guard skips `_test.go`; its first run failed
on its own fixtures, since the planted URLs live in the file it was auditing.

## User-config sweep — 45 rewritten, 11 fixtures deliberately excluded

The user approved the sweep. Applied 2026-07-30, backed up first to
`tmp/schema-sweep-backup-20260730-122123.tar` (all 56 files).

The 56 split into two dead forms, not one: 30 used `refs/heads/master/schema.json` and 26
used `master/schema.json`. Both were normalised to the single canonical form, so the corpus
no longer teaches two spellings of the same header.

**11 were left untouched on purpose.** Every one sits under `tmp/`, `.omo/evidence/`, or a
path naming itself a fixture/baseline, and several are *negative* fixtures — configs that
are deliberately malformed so a test can assert the failure. Rewriting them would edit the
record of what a past run produced, and nothing proves no test asserts on their bytes.

| bucket | count |
| --- | --- |
| live configs rewritten | 45 |
| fixtures / recorded evidence preserved | 11 |

Seven of the fixtures were rewritten before this distinction was drawn and were restored
from the backup. That mistake was caught by `sed` aborting on a **read-only** fixture —
the file's permissions were the safety net, not the plan. Without that stop, all 11 would
have been rewritten silently.

`dva validate` rc=0 on gizzahub-devbox, kc-ambench, funbricks-elemhant-devbox and
scripton-dns-bridge-devbox.

### The loss harness needed a narrow exemption

`tmp/scripts/verify-migration.py` went 0 fail → **24 fail**, every failure `주석 소실 1건`
naming the old `$schema` line: a changed comment is indistinguishable from a deleted one to
a harness that only knows what text disappeared.

Re-baselining would have silenced it in one step and cost the ability to detect regressions
from the removed-keys migration. Instead `comment_lines()` normalises the dead forms to the
canonical one on **both** sides, so a line-1 deletion is still caught. Back to 0 fail / 29
files. TASK-060's rename later added a third dead form to that list.

It does mean the harness would not notice a dead URL reintroduced into a user config. Out
of scope by design: `TestGeneratorCorpusURLs` covers the repo, user trees have no guard.

## README.md — no longer an escalation, the rename resolves it

`README.md:25`'s release-download URL and `:15`'s `go install` line both name
`ScriptonBasestar/dva`, which 404'd when this was written. The user chose **option B of
[TASK-060](060-go-module-path-does-not-resolve.md)** — rename the GitHub repo
to `dva` — which fixes the *name* in both lines without touching this ai=deny file.

Half of it is genuinely retired: `go install` now works. The download URL still 404s,
because the repo has never published a release or a tag — a different defect, found while
verifying this claim rather than assuming it, and tracked in
[TASK-063](../todo/063-documented-release-download-has-no-release.md).

That decision inverts what this task standardised on: every URL now says
`dev-virtual-auto`, and after the rename the canonical name is `dva`. The post-rename
sweep — the guard's `canonicalRepo` constant and the 45 configs just rewritten — is
tracked in TASK-060, not here.

## Evidence

- `git log --all --oneline -- schema.json` → empty (root schema never existed).
- `git branch -r` → `origin/HEAD -> origin/master`, `origin/master` only; no `main`.
- `git remote -v` → `git@github.com:ScriptonBasestar/dev-virtual-auto.git`.
- Counts measured 2026-07-30 @ `b20fee8`.
