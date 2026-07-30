---
id: TASK-059
title: "Nothing detects a subproject declaring its parent's compose project name — dva down in the child reaps the parent's stack"
type: feat
priority: P2
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/config/validate.go (or validate_warnings.go); found in ~/mydevbox/scripton-nd-stack-devbox"
---

# Task 059: Warn when a subproject shares its parent's compose project name

## Problem

`scripton-nd-stack-devbox/dva.yml` and its declared subproject
`nd-stack-rs/dva.yml` both resolve to compose project **`nd-stack-dev`** while
defining **different service sets**. Because compose project identity is what
`docker compose down` scopes to, `dva down` inside the subproject removes the
parent's containers too.

| config | compose files | services |
| --- | --- | --- |
| `scripton-nd-stack-devbox/dva.yml` | `deploy/local/compose.yaml` | 14 — proxynd, depond, safend, admind, sigdock-idp, sigdock-gateway, nd-webui, redis, postgres, minio, redis-commander, adminer, prometheus, grafana |
| `nd-stack-rs/dva.yml` | 6 files under `compose/` | 7 — postgres, minio, depond, proxynd, safend, admind, webui |

Overlapping but not equal, and `webui` vs `nd-webui` is a rename, not a match.

## Why DVA can and should catch this

The parent declares the child as a subproject (`dva.yml:488-491`):

```yaml
subprojects:
  nd-stack-rs:
    path: nd-stack-rs
    exclude_tags: [infra]
```

### Correction: `validate` does *not* have both configs (measured 2026-07-30)

An earlier draft of this task claimed "both configs are loaded in one process — DVA has
full visibility of both `project_name` values". **That is false for this config**, and it
changes the fix.

`resolveSubprojectImports` (`internal/config/subproject.go:86-89`) skips any subproject
where `hasSubprojectImports` is false — i.e. one that declares no `import:` block with
plans, interactions, or provision profiles. The nd-stack subproject declares only `path`
and `exclude_tags`, so **its `dva.yml` is never opened during `dva validate`**.

```
$ cd ~/mydevbox/scripton-nd-stack-devbox && dva validate 2>&1 | grep -i 'nd-stack-rs'
(nothing)
$ dva manifest 2>&1 | grep -ci 'nd-stack-rs'
40
```

`config.LoadSubprojects` has exactly three callers — `subproject.go:95` (imports only),
`internal/cli/manifest.go:183`, and `internal/cli/run.go:79`. None runs under `validate`.

So this is not "compare two values DVA already holds"; it is "decide which command may
open the child's file". That makes `dva doctor` the right home rather than `validate`:
doctor is diagnostic, is expected to touch the filesystem, and already owns the sibling
check `checkComposeProjectNameAlignment` (`internal/cli/doctor.go:311`).

`exclude_tags: [infra]` is the tell that the author understood the child should not
own infra. But tag filtering removes **stack entries** from an invocation, while
compose project identity governs **docker resources**. Excluding a tag does not stop
`docker compose --project-name nd-stack-dev down`, run from the child directory, from
reaping every container in that project. The two mechanisms operate on different
things, and the config looks correct at every single-file level.

`ValidateComposeProjectNames()` (`internal/config/validate.go:196`) checks a
different axis entirely — that one `dva.yml`'s `project_name` matches its *own*
compose file's top-level `name:`. Here that check **passes on both configs**:
`deploy/local/compose.yaml:8` and `nd-stack-rs/compose/compose.infra.yml:1` each say
`name: nd-stack-dev`. Correct alignment on the axis that is checked, collision on the
axis that is not.

## Failure mechanism

`--project-name` reaches docker from `internal/lifecycle/compose.go:187`,
`internal/runner/compose.go:40`, `internal/cli/compose.go:802,836`.

- `dva down` in `nd-stack-rs/` → `docker compose -p nd-stack-dev down` → removes all
  14 of the parent's containers, not the child's 7.
- `dva up` in one after the other → the other's containers become orphans (compose
  warns; removed outright with `--remove-orphans`).

## Not part of this task: the three identical clones

`scripton-zai-batch`, `scripton-zai-review` and `scripton-nd-stack-devbox` each
declare `project_name: nd-stack-dev`, but their `deploy/local/compose.yaml` files are
**byte-identical** (md5 `ca95683ca05f9ad1102786258ae614c1`) — the same stack copied
into three repos, so sharing one compose project is self-consistent and plausibly
deliberate. There is also no config graph linking them, so DVA cannot see the
relationship. Mention it to the user as a "three repos, one stack" observation; do
not build detection for it.

## Fix shape

A `dva doctor` check that loads each declared subproject and reports when the child's
resolved compose project name equals the parent's **while the two point at different
compose files**. Warn, do not error: sharing a project name is legitimate when both
configs describe the same stack (an overlay-style split).

Load each subproject **individually**, as `manifest.go:183` does, rather than passing the
whole map: `LoadSubprojects` returns `nil, err` on any single failure, so one missing or
malformed child would otherwise hide every other subproject's result.

### Compare compose *files*, not service sets

The original phrasing said "whose service sets differ". Implementing that literally would
mean parsing every referenced compose file for its `services:` keys — and **DVA has no
compose service parser**. `internal/lifecycle/orchestrator.go:54`'s `composeServices` is
config-derived, not parsed, and `readComposeNameKey` only extracts the top-level `name:`.

Comparing the resolved compose **file sets** instead needs no new parser (`AllComposeFiles()`
plus `FileDir()` already exist) and is closer to the actual failure mechanism: what
`docker compose -p X down` scopes to is the project identity, and whether two configs are
"the same stack" is settled by whether they hand docker the same files. Identical files ⇒
same stack ⇒ silent. Disjoint files ⇒ two stacks under one identity ⇒ warn.

Known limitation, accepted: byte-identical compose content at *different paths* would warn.
That is the three-clones shape below, which is out of scope here because those repos have no
config-graph link — parent↔subproject is the only pair this check considers.

## Non-goals

- Do not touch `~/mydevbox` configs as part of this task; fixing the user's tree is
  their call and a separate step.
- Do not extend `ValidateComposeProjectNames()` — different axis, keep it focused.
- No detection for unrelated repos with no config-graph link.

## Acceptance criteria

- [ ] Warning fires for parent+subproject sharing a project name with differing compose files | verify: `go test ./internal/cli/ -run TestCheckSubprojectComposeProjectNames`
- [ ] Silent when both point at the same compose files | verify: `go test ./internal/cli/ -run TestCheckSubprojectComposeProjectNames`
- [ ] One unloadable subproject does not suppress the others | verify: `go test ./internal/cli/ -run TestCheckSubprojectComposeProjectNames`
- [ ] Existing per-config name alignment check unaffected | verify: `go test ./internal/config/ -run TestValidateComposeProjectNames`
- [ ] Reproduces on the real config | verify: `cd ~/mydevbox/scripton-nd-stack-devbox && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva doctor 2>&1 \| /usr/bin/grep -q 'nd-stack-dev'`
- [ ] Full suite green | verify: `make test`
- [ ] No new doctor failures across the real corpus beyond this one | verify: `human — re-run the doctor sweep and compare counts`

## Evidence

Verified 2026-07-30 by reading both `dva.yml` files, all 7 compose files, and the
`--project-name` call sites. Service sets extracted between `services:` and the next
top-level key (an earlier count wrongly included `networks:`/`volumes:` keys).
Context: `tmp/71-mydevbox-migration-result.md`, 남은 별건 item 4.
