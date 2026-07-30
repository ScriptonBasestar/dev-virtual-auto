---
id: TASK-060
title: "The Go module path resolves to nothing — the documented go install cannot work"
type: decision
priority: P1
status: todo
effort: M
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — go.mod, every import, README.md (ai=deny), examples/MAKEFILE.md, .goreleaser.yml; or the GitHub repo name"
---

# Task 060: Decide what the canonical module path is

## Problem

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

## Non-goals

- Do not start either rename before the decision is recorded here.
- Do not edit `README.md` — `core` level, **ai=deny**. It holds both the install command
  and a release-download URL on the unresolvable name (TASK-057 escalation).

## Acceptance criteria

- [ ] Option chosen and written into this file | verify: `human — decision recorded`
- [ ] `go install <canonical-path>/cmd/dva@latest` succeeds from a clean module cache | verify: `human — run outside this repo`
- [ ] The URL guard's canonical repo constant matches the decision | verify: `go test ./internal/config/ -run TestGeneratorCorpusURLs`
- [ ] Install commands in docs agree with the decision | verify: `/usr/bin/grep -rn 'go install github.com/ScriptonBasestar' README.md examples/MAKEFILE.md`

## Related

- [TASK-057](../todo/057-dead-self-referencing-urls.md) — fixed the `$schema` and
  migration-guide URLs and standardised on `dev-virtual-auto`; option B would revisit
  that choice. The URL guard deliberately does not flag module paths, because inside a
  Go import `ScriptonBasestar/dva` is the module's real name.
