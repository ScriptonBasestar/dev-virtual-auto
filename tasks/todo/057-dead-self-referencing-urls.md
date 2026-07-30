---
id: TASK-057
title: "Self-referencing URLs are dead — $schema points at a path that never existed, migration guide names the wrong repo and branch"
type: fix
priority: P2
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — agent-mesh-flows/shared/library/, internal/cli/library_reference.txt (generated), internal/config/validate_warnings.go, dva.yml"
---

# Task 057: Make DVA's own URLs resolve

## Problem

Every URL DVA prints or writes about itself is broken, and one of them is stamped
into user configs by the AI generator.

### 1. `$schema` — right branch, path that never existed

```
# yaml-language-server: $schema=https://raw.githubusercontent.com/ScriptonBasestar/dev-virtual-auto/master/schema.json
```

`schema.json` has **never existed at the repo root** (`git log --all -- schema.json`
is empty); the file lives at `internal/config/schema.json`. So editor validation
silently does nothing for everyone who has this header — the failure mode of a bad
`$schema` is no diagnostics, not an error, which is why it survived.

### 2. Migration guide — wrong repo *and* wrong branch

`internal/config/validate_warnings.go:13`

```go
const migrationGuideURL = "https://github.com/ScriptonBasestar/dva/blob/main/docs/40-declarative-stack-and-plans.md#11-migration"
```

- repo is `ScriptonBasestar/dev-virtual-auto`; `ScriptonBasestar/dva` is the **Go
  module path** (`github.com/ScriptonBasestar/dva`), reused here as if it were the
  repository name;
- branch `main` does not exist — `origin/HEAD -> origin/master`, and `git branch -r`
  lists only `origin/master`;
- the document itself is fine: `docs/40-declarative-stack-and-plans.md` exists.

This URL is printed to users in validate warnings, so it is the one they are most
likely to click.

## Why it spread — same mechanism as the removed-keys contamination

```
agent-mesh-flows/shared/library/reference-examples.md:12   ← authored source
  → make generate
internal/cli/library_reference.txt:1151                    ← embedded copy
  → compiled into bin/dva
  → AI flows emit the header into generated dva.yml
~/mydevbox: 56 of 83 real configs carry the dead URL
```

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

An earlier count of "27 places in the repo" was wrong: 24 of those were copies of
user configs inside `tmp/mydevbox-backup-20260730-101632/`, not repo source.
`/blob/main/`'s "12 files" was likewise inflated — 10 were `.opencode/node_modules`
and 1 was the compiled `bin/dva`. Real source occurrences are the 4 rows above.

15 other files use the working relative form `$schema=../internal/config/schema.json`
(the `examples/` directory), which is correct for in-repo editing and should stay.

## Open question — settled 2026-07-30: the repo is public

Probed directly rather than assumed (`gh` was unusable — the org forbids fine-grained
tokens with a lifetime over 366 days):

| URL | status |
| --- | --- |
| `github.com/ScriptonBasestar/dev-virtual-auto` | **200** |
| `raw.githubusercontent.com/.../dev-virtual-auto/master/internal/config/schema.json` | **200** |
| `raw.githubusercontent.com/.../dev-virtual-auto/master/schema.json` | 404 |
| `github.com/ScriptonBasestar/dev-virtual-auto/blob/master/docs/40-declarative-stack-and-plans.md` | **200** |
| `github.com/ScriptonBasestar/dva` | 404 |
| `github.com/ScriptonBasestar/dva/blob/main/docs/40-...` | 404 |

So the canonical form is confirmed reachable, not merely plausible:
`https://raw.githubusercontent.com/ScriptonBasestar/dev-virtual-auto/master/internal/config/schema.json`

## Fix shape

1. Settle the open question above.
2. Fix the authored source (`reference-examples.md`), then `make generate` to
   propagate into `library_reference.txt` — never hand-edit the generated file.
3. Fix `migrationGuideURL`: correct repo name and branch.
4. Fix the repo's own `dva.yml:1`.
5. Add a guard so this cannot rot again: extend the generator-corpus test to assert
   that any `raw.githubusercontent.com` / `github.com/ScriptonBasestar` URL the
   corpus teaches names the real repo and a branch that exists, and that a
   `$schema=` path resolves to a file present in the tree. Offline string checks
   only — do not add a network call to the test suite.
6. Decide separately whether to rewrite the 56 user configs. Mechanical
   (`sed` on line 1) but it is the user's tree; ask before touching it.

## Non-goals

- Do not rewrite the 15 correct relative `$schema` paths in `examples/`.
- Do not add network access to tests.
- Root `README.md` is **ai=deny**; it contains neither dead URL (verified, 0 hits),
  so no human escalation is needed for this task.

## Acceptance criteria

- [x] Authored corpus teaches a resolving `$schema` URL | verify: `grep -q 'internal/config/schema.json' agent-mesh-flows/shared/library/reference-examples.md`
- [x] Generated embed matches the source | verify: `make generate && git diff --exit-code internal/cli/library_reference.txt`
- [x] No source file references the nonexistent root schema path | verify: `/usr/bin/find . -path ./.git -prune -o -path ./tmp -prune -o -path ./bin -prune -o -path ./.opencode -prune -o -path ./tasks -prune -o -type f -print0 | xargs -0 /usr/bin/grep -l 'master/schema.json' ; test $? -ne 0`
- [x] Migration guide URL names the real repo and an existing branch | verify: `grep -q 'dev-virtual-auto/blob/master/docs/40-declarative-stack-and-plans.md' internal/config/validate_warnings.go`
- [x] Corpus URL guard exists and fails on a planted bad URL | verify: `go test ./internal/config/ -run TestGeneratorCorpusURLs`
- [x] Full suite green | verify: `make test`
- [ ] 56 user configs rewritten, or the sweep explicitly declined | verify: `human — the user's tree; needs their go-ahead`
- [ ] README.md release-download URL resolved | verify: `human — README.md is ai=deny, see escalation below`

(The `tasks` prune was added to the third criterion: this file quotes the dead URL as
evidence, and a guard that forbids its own evidence is a guard that cannot be documented.)

## Result — repo side done

- `agent-mesh-flows/shared/library/reference-examples.md:12` → canonical raw URL,
  propagated to `internal/cli/library_reference.txt:1151` via `make generate` (the
  generated file is listed in `.claudeignore`, so it is never hand-edited).
- `dva.yml:1` → canonical raw URL.
- `migrationGuideURL` → `dev-virtual-auto` / `master`. Confirmed against a rebuilt
  binary: the link `dva validate` prints in primeno1-devbox is now the 200 one.
- New guard `TestGeneratorCorpusURLs` (`internal/config/corpus_urls_test.go`) audits
  5 URLs across 125 files, offline. `TestGeneratorCorpusURLsDetectsPlantedDefects`
  pins the detector against four planted defects so a regex edit cannot defang it.
  Both counters (files and URLs) are asserted non-zero, because a walk that matches
  nothing passes forever while guarding nothing.
- `make test` green.

The guard skips `_test.go` files. Its first run failed on its own fixtures — the
planted bad URLs in the test file are the corpus it was auditing.

## Escalation — root README.md is ai=deny and holds a dead URL

`README.md:25` documents

```
curl -sL https://github.com/ScriptonBasestar/dva/releases/latest/download/dva_linux_amd64.tar.gz | tar xz
```

on the repo name that 404s. The same URL on `dev-virtual-auto/releases/latest` returns
302, so the release path exists — only the repo name is wrong. Root `README.md` is
`core` level, **ai=deny**, so a human must make this edit. The URL guard deliberately
excludes README.md rather than failing the build on a file no agent may fix.

## Follow-up found here — the Go module path does not resolve

Bigger than this task and left untouched. `go.mod` declares
`github.com/ScriptonBasestar/dva`, and README.md:15 plus
`examples/MAKEFILE.md:11,163` document `go install
github.com/ScriptonBasestar/dva/cmd/dva@latest`. Verified: the module proxy returns
404 for that path, and `github.com/ScriptonBasestar/dva?go-get=1` serves no `go-import`
meta tag, so there is no vanity redirect. **The documented install command cannot
work.**

The two possible fixes are a module-path rename (touches every import in the repo) or
renaming/aliasing the GitHub repo to `dva` — a decision, not a mechanical edit. The URL
guard intentionally does not flag these, because inside a Go import path
`ScriptonBasestar/dva` is the module's real name; only `https://` URLs are audited.
Needs its own decision record.

## Evidence

- `git log --all --oneline -- schema.json` → empty (root schema never existed).
- `git branch -r` → `origin/HEAD -> origin/master`, `origin/master` only; no `main`.
- `git remote -v` → `git@github.com:ScriptonBasestar/dev-virtual-auto.git`.
- Counts measured 2026-07-30 against `bin/dva` @ `b20fee8`.
