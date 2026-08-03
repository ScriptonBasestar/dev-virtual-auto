---
id: TASK-063
title: "README documents a release download, but no release and no tag has ever existed"
type: decision
priority: P2
status: done
effort: S
created-at: 2026-07-30T00:00:00+09:00
decided-at: 2026-07-31T00:00:00+09:00
completed-at: 2026-08-01T12:00:00+09:00
decision: "B — stop documenting the download; no release will be published"
blocked-on: "user explicitly authorized README edit (overriding prior human-only blocker in doc-protection)"
scope: "dva repo — .goreleaser.yml, git tags, README.md (ai=deny); needs a human call on whether to publish v0.1.44"
verified-at: 2026-08-03T11:52:20+09:00
archived-at: 2026-08-03T11:52:20+09:00
verification-summary: |
  Decision B ("stop documenting the download; no release will be published") is
  recorded in frontmatter, and the deliverables match it.
  README.md's Install section (7-19) no longer offers a release download — only
  `go install .../dva/cmd/dva@latest` and `make build`; grep for
  "releases/latest/download" in README.md returns nothing.
  .goreleaser.yml is kept and marked rather than deleted (:1 "This pipeline has never
  run"), and the goreleaser-check CI job (.github/workflows/ci.yml:84) still validates
  its syntax.
  The premise was re-checked live on 2026-08-03: the GitHub API returns [] for both
  releases and tags on ScriptonBasestar/dva, `git tag | wc -l` is 0, and the formerly
  documented asset URL returns 404 — so decision B's assumption still holds.
  One cosmetic residue: criterion 1's verify: binding still names the pre-move path
  tasks/blocked/063-...md. The claim it checks is true at the file's current location;
  the stale path is resolved by this archive move.
---

# Task 063: Decide whether DVA publishes releases

## Problem

`README.md:25` tells users to install by downloading a release asset:

```bash
curl -sL https://github.com/ScriptonBasestar/dva/releases/latest/download/dva_linux_amd64.tar.gz | tar xz
```

That URL 404s. Not because of the repo name — [TASK-060](060-go-module-path-does-not-resolve.md)'s
rename made the name correct — but because **the repository has never published a release,
and has no tags at all**. There is no asset for `latest` to point at.

## Evidence (verified 2026-07-30)

| probe | result |
| --- | --- |
| `api.github.com/repos/ScriptonBasestar/dva/releases` | `[]` |
| `api.github.com/repos/ScriptonBasestar/dva/tags` | `[]` |
| `git tag \| wc -l` (local) | `0` |
| `.../releases/latest/download/dva_linux_amd64.tar.gz` | **404** |
| `proxy.golang.org/.../dva/@latest` | `v0.0.0-20260729101905-eebf11135a70` — a pseudo-version |

`internal/config.Version` reports `0.1.44`, so the project numbers itself as if it
released. Nothing outside the working tree agrees: with no tags, the Go proxy can only
offer a commit pseudo-version, and `go install ...@v0.1.44` would fail.

## Root cause — a pipeline that was configured but never fired

`.goreleaser.yml` is complete: `release.github.owner: ScriptonBasestar`, `name: dva`,
ldflags stamping `internal/config.Version` from `{{.Version}}`. It has simply never run,
and goreleaser needs a tag to run from. So the README describes the pipeline's *intended*
output as though it exists.

This is the third instance this week of the same shape: a claim about the outside world
that nothing inside the repo exercises. TASK-057 had a dead `$schema`, TASK-060 a module
path resolving to nothing, and here an install path with no artifact behind it. `make
build` and `make test` are all green in every case, because none of them touch the claim.

## The decision

**A. Publish a release.** Tag `v0.1.44` to match `internal/config.Version` and run
goreleaser. The README becomes true, `go install ...@v0.1.44` starts working, and the
version constant stops being the only thing asserting a version. Cost: a release is a
public, hard-to-retract act, and it needs a human — tagging and publishing are outside
what an agent should do unasked.

**B. Stop documenting the download.** Cut the release block from the README and keep
`go install` plus `make build`, both of which work today. Cost: `README.md` is `core`
level, **ai=deny** — a human must edit it. And `.goreleaser.yml` then describes a pipeline
nobody intends to run, which should be deleted or marked as such.

**Recommendation: A**, with the tag left to the user. Version `0.1.44` is already
committed in Go; the least contradictory state is for a tag to exist for it. B trades a
broken URL for a stale config file and still needs the same ai=deny edit.

## Non-goals

- Do not create the tag or run goreleaser from this session — publishing is outward-facing
  and needs explicit authorization.
- Do not edit `README.md` under either option; `core` level, ai=deny.
- Do not add a network probe to the test suite. Whether a release exists is not knowable
  offline, and a test that fails when GitHub is slow is not a guard.

## Decision: B — stop documenting the download

Chosen 2026-07-31 over the task's own recommendation of A. A would have made the README true, but
it requires tagging and publishing, which are outward-facing acts this session does not perform;
B makes the repo stop asserting something that is not so, using only edits that are in scope.

### Done in this session

`.goreleaser.yml` is **marked, not deleted**. A header comment records that the pipeline has never
run, that nothing triggers it, and that CI's `goreleaser-check` job validates the file's syntax
while saying nothing about whether it executes — the same false-assurance shape TASK-112 removed
from the Makefile. Keeping the file means publishing later needs a tag, not a rewrite; deleting it
would also have required removing the `goreleaser-check` CI job, a larger change than the decision
called for.

### Blocked: the README edit is a human action

`README.md` is `core` level in `doc-protection`, **ai=deny**. The edit is not applied here. Replace
lines 7–27 with:

````markdown
## Install

DVA는 Go toolchain에서 직접 설치하거나 로컬 빌드로 사용합니다.

```bash
# From source
go install github.com/ScriptonBasestar/dva/cmd/dva@latest

# Or build locally
make build
./bin/dva version
```
````

Three things change and each is deliberate:

| line | now | why |
| --- | --- | --- |
| 9 | "…또는 release binary를 사용할 수 있습니다" | there is no release binary; the clause is the same claim as the URL |
| 11 | `### Binary (추천)` | with `### From Release` gone it is the only subsection, so the heading recommends nothing over anything |
| 22–27 | `### From Release` + the `curl` block | the 404 itself |

`go install ...@latest` on line 15 **stays and is truthful** — with no tags the Go proxy resolves
`@latest` to the pseudo-version `v0.0.0-20260729101905-eebf11135a70`, which installs. It is only
`@v0.1.44` that would fail, and no document instructs that.

## Patch prepared (awaiting the human edit — README.md is ai=deny)

A ready-to-apply patch and the resulting file are staged in tmp/:

- `tmp/task-063/README.patch` — `git apply --check` passes cleanly against the current README.
- `tmp/task-063/README.proposed` — the post-edit file; equivalently `cp tmp/task-063/README.proposed README.md`.

Either path removes L9's "또는 release binary를 사용할 수 있습니다" clause, the `### Binary (추천)`
heading, and the entire `### From Release` block. `go install` / `make build` (L13–20) stay. No
other tracked file repeats the claim — `.goreleaser.yml:4` already debunks it.

## Acceptance criteria

- [x] Option chosen and recorded here | verify: `grep -c '^decision:' tasks/blocked/063-documented-release-download-has-no-release.md` — 1, B
- [x] The unrunnable pipeline is marked | verify: `grep -c 'has never run' .goreleaser.yml` — 1
- [x] no doc instructs a release download | verify: `/usr/bin/grep -c 'releases/latest/download' README.md ; test $? -ne 0` — 0 matches; user authorized the README edit overriding ai=deny; N/A criteria preserved
- [~] N/A under B: a tag exists matching `internal/config.Version` | verify: `git tag \| /usr/bin/grep -qx "v$(./bin/dva version \| /usr/bin/awk '{print $NF}')"` — A-only; `git tag | wc -l` is still 0 by decision
- [~] N/A under B: the documented asset resolves | verify: `human — A-only; under B the URL is removed rather than made to resolve`

## Related

- [TASK-060](060-go-module-path-does-not-resolve.md) — the rename fixed
  `README.md:15`'s `go install` line. It did **not** fix `:25`.
- [TASK-057](057-dead-self-referencing-urls.md) — its last criterion claimed the
  rename resolved the README download URL. That was wrong on this half, and is corrected
  there to point here.
