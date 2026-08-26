---
id: TASK-059
title: "Nothing detects a subproject declaring its parent's compose project name — dva down in the child reaps the parent's stack"
type: feat
priority: P2
status: done
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/cli/doctor.go; found in ~/mydevbox/scripton-nd-stack-devbox"
verified-at: 2026-08-03T11:52:20+09:00
archived-at: 2026-08-03T11:52:20+09:00
verification-summary: |
  All criteria MET, including the corpus sweep the task had marked human-verify.
  checkSubprojectComposeProjectNames (internal/cli/doctor.go:371-415) is registered in
  runDoctor (doctor.go:97) after checkComposeProjectNameAlignment. All 14 subtests pass
  (5 TestCheckSubprojectComposeProjectNames + 9 TestSameStringSet); sibling
  TestValidateComposeProjectNames still passes its 6 subtests, so no regression.
  Real-config repro in ~/mydevbox/scripton-nd-stack-devbox prints the exact FAIL line.
  Supervisor re-ran the corpus sweep rather than trusting the recorded figure:
  `dva doctor` executed in all 31 live ~/mydevbox configs (configs-swept=31), matching
  the literal string "shares compose project name" → collision-hits=1, the nd-stack-rs
  subproject only. Zero false positives, exactly as claimed.
  TASK-132/133 were checked and are unrelated: they fix --project-name duplication and
  detection scope in internal/runner, and touch neither doctor.go nor its tests.
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

- [x] Warning fires for parent+subproject sharing a project name with differing compose files | verify: `go test ./internal/cli/ -run TestCheckSubprojectComposeProjectNames`
- [x] Silent when both point at the same compose files | verify: `go test ./internal/cli/ -run TestCheckSubprojectComposeProjectNames`
- [x] One unloadable subproject does not suppress the others | verify: `go test ./internal/cli/ -run TestCheckSubprojectComposeProjectNames`
- [x] Existing per-config name alignment check unaffected | verify: `go test ./internal/config/ -run TestValidateComposeProjectNames`
- [x] Reproduces on the real config | verify: human — run doctor in the named personal config; the archived corpus evidence records 31 configs swept and one collision hit
- [x] Full suite green | verify: `make test`
- [x] No new doctor failures across the real corpus beyond this one | verify: `human — re-run the doctor sweep and compare counts`

## Result

`checkSubprojectComposeProjectNames` in `internal/cli/doctor.go`, registered in `runDoctor`
next to the existing `checkComposeProjectNameAlignment`. Tests in
`internal/cli/doctor_subproject_test.go` (new file), 14 subtests.

Measured 2026-07-30:

| check | outcome |
| --- | --- |
| `go test ./internal/cli/ -run 'TestCheckSubprojectComposeProjectNames\|TestSameStringSet' -v` | 14 subtests PASS |
| `go test ./internal/config/ -run TestValidateComposeProjectNames -v` | 6 subtests PASS — sibling check untouched |
| corpus sweep, 31 live configs | new check fired **1×** — the intended one, zero false positives |
| `make test` | green |
| `internal/cli` coverage | 54.5% → 55.1% |

Real config output:

```
[FAIL] Subproject "nd-stack-rs" shares compose project name "nd-stack-dev" with the parent
       but references different compose files
    -> Give nd-stack-rs its own project_name; otherwise 'dva down' in nd-stack-rs removes
       the parent's containers too
```

Two design points settled during implementation and recorded in the code comments:

1. **doctor, not validate** — forced by the measurement in the Correction section above, not
   chosen for convenience. validate never opens the child's file.
2. **file sets, not service sets** — no compose service parser exists, and file identity is
   what `docker compose -p X down` actually scopes to.

The `~/mydevbox/scripton-nd-stack-devbox` config itself is **not** fixed here; per Non-goals
that is the user's call. DVA now reports it.

## Evidence

Verified 2026-07-30 by reading both `dva.yml` files, all 7 compose files, and the
`--project-name` call sites. Service sets extracted between `services:` and the next
top-level key (an earlier count wrongly included `networks:`/`volumes:` keys).
Context: `tmp/71-mydevbox-migration-result.md`, 남은 별건 item 4.
