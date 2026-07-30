---
id: TASK-063
title: "README documents a release download, but no release and no tag has ever existed"
type: decision
priority: P2
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — .goreleaser.yml, git tags, README.md (ai=deny); needs a human call on whether to publish v0.1.44"
---

# Task 063: Decide whether DVA publishes releases

## Problem

`README.md:25` tells users to install by downloading a release asset:

```bash
curl -sL https://github.com/ScriptonBasestar/dva/releases/latest/download/dva_linux_amd64.tar.gz | tar xz
```

That URL 404s. Not because of the repo name — [TASK-060](../decision/060-go-module-path-does-not-resolve.md)'s
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

## Acceptance criteria

- [ ] Option chosen and recorded here | verify: `human — decision recorded`
- [ ] If A: a tag exists matching `internal/config.Version` | verify: `git tag \| /usr/bin/grep -qx "v$(./bin/dva version \| /usr/bin/awk '{print $NF}')"`
- [ ] If A: the documented asset resolves | verify: `human — curl -sI the README URL, expect 200`
- [ ] If B: no doc instructs a release download | verify: `/usr/bin/grep -c 'releases/latest/download' README.md ; test $? -ne 0`

## Related

- [TASK-060](../decision/060-go-module-path-does-not-resolve.md) — the rename fixed
  `README.md:15`'s `go install` line. It did **not** fix `:25`.
- [TASK-057](../done/057-dead-self-referencing-urls.md) — its last criterion claimed the
  rename resolved the README download URL. That was wrong on this half, and is corrected
  there to point here.
