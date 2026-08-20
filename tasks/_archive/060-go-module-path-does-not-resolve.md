---
id: TASK-060
title: "The Go module path resolves to nothing — the documented go install cannot work"
type: chore
priority: P1
normalized-by: "TASK-194 — was type: decision"
status: done
effort: M
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/config/corpus_urls_test.go, validate_warnings.go, reference-examples.md, dva.yml, skills|workflows|examples READMEs; plus the GitHub repo rename (human)"
verified-at: 2026-08-03T12:15:00+09:00
archived-at: 2026-08-03T12:15:00+09:00
verification-summary: |
  go.mod declares github.com/ScriptonBasestar/dva; proxy.golang.org and GitHub's
  go-import meta both resolve it live. corpus_urls_test.go's canonicalRepo() derives
  the name from go.mod at runtime (no transcribed constant); TestGeneratorCorpusURLs,
  TestRepoFromModuleDirective and TestGeneratorCorpusURLsDetectsPlantedDefects all pass
  (audited 5 URLs / 137 files). A repo-wide grep for the old name (excluding .git/tmp/
  .omo/tasks) hits only the guard's own test fixture. README.md and examples/MAKEFILE.md
  install lines, and .goreleaser.yml, all name ScriptonBasestar/dva. Of ~mydevbox's live
  dva.yml configs, 50 carry the canonical schema URL; the 11 still naming the old repo
  are precisely the tmp/.omo evidence fixtures TASK-057 documented as intentionally
  excluded — confirmed by listing the 11 file paths. Git history (65895f4, b8a5fa7)
  corroborates the derive-from-go.mod fix and the decision record commit.
---

# Task 060: Decide what the canonical module path is

## Problem

> State **before** the rename. Everything below was true when measured; the Decision
> section records what changed. Kept as written so the reasoning that led to B survives.

`go.mod:1` declares:

```
module github.com/ScriptonBasestar/dva
```

There is no repository at `github.com/ScriptonBasestar/dva`, and no vanity redirect
pointing the path somewhere else. So the install command DVA documents in two places
cannot succeed for anyone:

```
go install github.com/ScriptonBasestar/dva/cmd/dva@latest
```

- `README.md:15`
- `examples/MAKEFILE.md:11` and `:163`

## Evidence (verified 2026-07-30)

| probe | result |
| --- | --- |
| `proxy.golang.org/github.com/!scripton!basestar/dva/@v/list` | `not found: module github.com/ScriptonBasestar/dva` |
| `proxy.golang.org/github.com/!scripton!basestar/dva/@latest` | 404 |
| `github.com/ScriptonBasestar/dva?go-get=1` | no `go-import` meta tag — no vanity redirect |
| `github.com/ScriptonBasestar/dva` | 404 |
| `github.com/ScriptonBasestar/dev-virtual-auto` | 200 |
| `git remote -v` | `git@github.com:ScriptonBasestar/dev-virtual-auto.git` |

The proxy's own error names the cause: `git ls-remote https://github.com/ScriptonBasestar/dva`
fails, and it then asks for credentials — which is why a casual check can look like "it
is just private" rather than "it does not exist". The repo that does exist is public
(200 unauthenticated), so private-ness is not the explanation.

## Why it went unnoticed

Nothing in the local workflow needs the module path to resolve. `make build` compiles
from the working tree, and intra-repo imports resolve through the module declaration
regardless of whether it maps to a URL. Only a **consumer** — someone running
`go install`, or `go get`ing DVA as a library — hits the failure, and consumers do not
file bugs against a tool they could not install.

This is the same shape as the dead `$schema` URL in TASK-057: a claim about the outside
world that the inside of the repo never exercises.

## The decision

Two coherent answers; they are mutually exclusive.

**A. Rename the module path to match the repository.**
`module github.com/ScriptonBasestar/dev-virtual-auto`, and rewrite every internal import
(`internal/...`, `cmd/...`), `.goreleaser.yml:20`'s ldflags path, the install commands,
and `docs/31-execution-plan-resolution.md:264`. Mechanical but wide. Anyone who somehow
depends on the old path breaks — in practice nobody can, because it never resolved.

**B. Make the repository match the module path.**
Rename the GitHub repo to `dva` (GitHub keeps a redirect from the old name), or add a
vanity-import host. One change instead of many, and it matches the binary name, the
config filename, and the CLI name — everything user-facing is already "dva". Costs: the
repo URL everyone has bookmarked changes, and TASK-057 just standardised on
`dev-virtual-auto` in the `$schema` and migration-guide URLs, so those would need
revisiting along with the guard's `canonicalRepo` constant.

**Recommendation: B.** Every user-facing name is already `dva`; `dev-virtual-auto` only
appears in the clone URL. B is one action versus a repo-wide import rewrite, and GitHub's
own redirect keeps existing clones and links working — which is exactly the failure mode
A cannot offer for the module path.

A fact found while writing the checklist strengthens B past the argument above:
`.goreleaser.yml:45` already declares `name: dva` as the release target. The release
pipeline was written for the renamed repo before the repo had that name, so under A it
would have needed changing too — and today it points at a repo that only exists because
of B.

## Decision: B — chosen, and already executed

The user chose B, and the rename is **live** (measured 2026-07-30, after the choice):

| probe | result |
| --- | --- |
| `github.com/ScriptonBasestar/dva` | 200, title `ScriptonBasestar/dva: dev virtual auto` |
| `github.com/ScriptonBasestar/dev-virtual-auto/releases/latest` | **301 → `/dva/releases`** (rename redirect) |
| `proxy.golang.org/.../dva/@latest` | `v0.0.0-20260729101905-eebf11135a70` — the module resolves |
| `?go-get=1` | `go-import: github.com/ScriptonBasestar/dva git https://github.com/ScriptonBasestar/dva.git` |
| `raw.githubusercontent.com/.../dev-virtual-auto/...` | **200 directly, no redirect** |

That last row mattered: the two GitHub hosts treat a rename differently. `github.com`
301s old→new, while the raw host serves the old name with a plain 200. So the 45 user
configs TASK-057 had just rewritten were never broken — but that had to be measured, not
inferred from "GitHub keeps a redirect".

### What the rename fixed for free

`go.mod` is now correct as written, `README.md:15`'s `go install` line works, and
`.goreleaser.yml:45` names a repo that exists. None of it required touching the ai=deny
README.

### What it broke — the guard turned against itself

`internal/config/corpus_urls_test.go` held `canonicalRepo = "dev-virtual-auto"` as a
hand-written constant. After the rename it enforced the dead name: **rejecting** correct
URLs and accepting stale ones. A test cannot notice that its own constant went out of
date, which is the defect this guard exists to catch, one level up.

Fixed by deriving the name from `go.mod`'s module path. That is sound under either answer
here, because A (module → repo) and B (repo → module) both converge on
`repo name == module path base`; the derivation is not a coincidence of B.

### Sweep performed

11 URLs in tracked source (`reference-examples.md`, `dva.yml`, `validate_warnings.go`,
`skills/README.md`, `workflows/README.md`, `examples/README.md`, and
`library_reference.txt` via `make generate`) plus the same 45 live user configs, backed up
to `tmp/schema-sweep-dva-20260730-124938.tar`. The 11 fixture/evidence configs were left
alone for the reason given in TASK-057. The old name still resolves, so this was hygiene,
not a repair — the redirect dies if `dev-virtual-auto` is ever reused as a repo name.

## Non-goals

- Do not edit `README.md` — `core` level, **ai=deny**. Both its URLs are now correct in
  the name they use; the download line has a *different* problem, see TASK-063.
- Do not change `git remote` here. It still points at the old name and works via the
  redirect; `git remote set-url` needs an explicit instruction.

## Acceptance criteria

- [x] Option chosen and written into this file | verify: `human — B, recorded above`
- [x] The module path resolves | verify: `curl -s https://proxy.golang.org/github.com/\!scripton!basestar/dva/@latest`
- [x] The URL guard's canonical repo matches the decision, and is derived not transcribed | verify: `go test ./internal/config/ -run TestGeneratorCorpusURLs`
- [x] No tracked source names the old repo outside the guard's own fixtures | verify: `files=$(git ls-files -s | /usr/bin/grep -v '^120000 ' | cut -f2- | /usr/bin/grep -v '^tasks/' | /usr/bin/grep -v '^\.omo/'); all=$(printf '%s\n' "$files" | /usr/bin/grep -c .); [ "$all" -gt 0 ] || { echo "no tracked files swept — nothing was measured"; exit 2; }; n=$(printf '%s\n' "$files" | tr '\n' '\0' | xargs -0 /usr/bin/grep -l 'ScriptonBasestar/dev-virtual-auto' | /usr/bin/grep -vc corpus_urls_test.go); echo "offenders=$n of $all tracked files swept"; [ "$n" -eq 0 ]` — **`offenders=0 of 420 tracked files swept`, exit 0 (TASK-199).** Both pipes were written `\|`. GFM processes a backslash escape inside a code span **only in a table row**, and this is a list item, so the backslash was literal: as published the command handed `|` to `find` as an argument and died with `find: |: unknown primary or operator` before reaching `xargs`. `tools/doccheck/verifyrun.go:85-91` already states this rule and applies it only to `go test … -run` bindings. The first rewrite fixed only the pipes and kept `… ; test $? -ne 0`, which restored a command that ran but still printed **nothing** on success — indistinguishable from a sweep over a path that does not exist, which is the defect the sibling criterion below was rewritten for. Corrected in review to print its own denominator; from an empty directory it exits 2, and against a planted `ScriptonBasestar/dev-virtual-auto` it prints `offenders=1` and exits 1. The corpus is `git ls-files` rather than `find .` because the criterion says *tracked*, and because `find` counts whatever the checkout happens to hold: the same `find` prints **4395** in the primary checkout and **423** in a fresh worktree at this commit, a denominator that moves by a factor of ten without a single tracked file changing. `git ls-files` prints 420 in both. Mode `120000` entries are dropped because this repo single-sources six paths by symlink and `grep` on a symlinked *directory* is an error, not a read
- [x] Install commands in docs agree with the decision | verify: `/usr/bin/grep -rn 'go install github.com/ScriptonBasestar' README.md examples/MAKEFILE.md`
- [x] Live user configs carry the canonical URL | verify: `r='ScriptonBasestar/dva/master/internal/config/schema.json'; all=$(/usr/bin/find ~/mydevbox -name dva.yml 2>/dev/null | wc -l | tr -d ' '); [ "$all" -gt 0 ] || { echo "no dva.yml under ~/mydevbox — nothing was measured"; exit 2; }; n=$(/usr/bin/find ~/mydevbox -name dva.yml -print0 | xargs -0 /usr/bin/grep -l "$r" | wc -l | tr -d ' '); echo "canonical=$n of $all"; [ "$n" -gt 0 ]` — **`canonical=40 of 81`, exit 0 (TASK-199).** The binding was the same `find … -print0 \| xargs -0 grep -l … \| wc -l`, and it failed twice over. The `\|` was literal for the reason given two criteria above, so it never ran; and had it run, a pipeline's status is its last command's and `wc -l` exits 0 on empty input, so it would have passed at a count of 40 and at a count of 0 alike — recording a number while gating on nothing. Three directions now: `canonical=0 of 2` and **exit 1** against a fixture holding configs that carry no canonical ref, and **exit 2** for a path with no `dva.yml` at all, which is what any reader without this machine's `~/mydevbox` gets instead of a vacuous pass. The denominator is printed because `40` is not the whole population and never was: 81 configs exist, 54 carry some `schema.json` reference and 14 carry a non-canonical one, so what this criterion can honestly assert is that the canonical URL is *in use*, not that the migration is complete. The summary above records 50 for the same measurement on 2026-08-03

## Follow-ups, not folded in

- **[TASK-063](063-documented-release-download-has-no-release.md)** — `go install`
  works, but `README.md:25`'s download URL 404s because **no release and no tag has ever
  existed**. The rename made that line name the right repo; it did not make it work. An
  earlier note in TASK-057 called this resolved by the rename, which was wrong.
- `git remote -v` still says `dev-virtual-auto`. Works via redirect; left for the user:
  `git remote set-url origin git@github.com:ScriptonBasestar/dva.git`.

## Related

- [TASK-057](057-dead-self-referencing-urls.md) — fixed the `$schema` and
  migration-guide URLs, standardising on `dev-virtual-auto`; this decision inverted that
  choice one commit later. The URL guard deliberately does not flag module paths, because
  inside a Go import `ScriptonBasestar/dva` is the module's real name — and now the
  repository's name as well.
