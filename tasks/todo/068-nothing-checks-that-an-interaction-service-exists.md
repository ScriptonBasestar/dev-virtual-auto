---
id: TASK-068
title: "Nothing checks that an interaction's service: exists, and one live interaction is unreachable — but the check needs an include:-following resolver first"
type: decision
priority: P3
status: todo
effort: M
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — a new check would need a compose service resolver (none exists); motivating instance in ~/mydevbox/sigdock-pass-devbox"
---

# Task 068: Decide whether to build the resolver an interaction-service check requires

## Problem

`dva run <name>` executes an interaction against `service:`. Nothing verifies that service
exists. `internal/config/validate.go` contains no reference to `Service` at all.

One live interaction is genuinely unreachable as a result —
`~/mydevbox/sigdock-pass-devbox/dva.yml:35-38`:

```yaml
  redis-cli:
    description: Connect to Redis master (requires compose-redis.yaml overlay)
    service: redis-master
    command: redis-cli
```

The stack declares exactly one compose file, `compose.yml`, whose services are
`db, minio, minio-init, server, sigdock-idp`. `redis-master` is defined only in
`compose-redis.yaml`, which **no `dva.yml` key references** — it appears solely inside that
`description:` string.

**And it cannot be made reachable from the interaction.** `ComposeOptions`
(`internal/config/config.go:471-475`) has only `Method`, `Profiles` and `RunOptions` — no
`files`. A compose *profile* can gate a service that is already defined, but cannot pull in an
undeclared file. So the description documents a manual step outside DVA, and `dva run
redis-cli` fails on a config that `dva validate` calls clean.

Compare the neighbouring `db` interaction, whose description also says "requires postgres
profile" — but `db` *is* in `compose.yml`, so that one is fine. Prose-vs-reality is not
uniformly broken here; exactly one instance is.

## Why this is a decision and not a straightforward fix

The obvious check — "is `service:` among the declared compose files' services?" — is
**unusable without resolving compose `include:`**. Measured across the corpus:

| outcome | count |
| --- | --- |
| genuinely missing service | **1** |
| absent from declared files, but a declared file uses `include:` | **40** |
| service references checked | 164 |
| `dva.yml` files declaring `interaction:` | 64 |

A checker reading only the declared files reports **41 failures, 40 of them wrong.**
`gizzahub-devbox/compose.yaml` is the clearest case: it declares **zero** top-level services
and 14 `include:` entries, so a naive check sees an empty service set and condemns every
interaction in the repo. Verified that the includes do supply them —
`compose/app-backend.yaml` → `backend`, `compose/infra-postgres.yaml` → `postgres`,
`compose/infra-redis.yaml` → `redis`, matching the three interactions flagged there.

So the real cost is not the comparison, it is the resolver. And per
[TASK-059](../done/059-subproject-compose-project-name-collision.md), **DVA has no compose
service parser at all**: `internal/lifecycle/orchestrator.go:54`'s `composeServices` is
config-derived rather than parsed, and `readComposeNameKey` extracts only the top-level
`name:`. Building this means following `include:` (and deciding about `extends:`,
`!reset`/`!override` tags, and `${VAR}` interpolation in service keys) before a single
comparison happens.

## Options

**A. Build the resolver and add the check.** Correct and general; would have caught this
instance. Cost: a real compose-subset parser in a tool that has deliberately avoided one. Must
follow `include:` recursively, and must degrade to silence — not to a false alarm — on
`extends:`, interpolated service names, and unreadable files. Effort M, not S.

**B. Check only the unambiguous subset.** Warn only when no declared compose file uses
`include:` *and* every declared file parsed cleanly *and* the service name has no `${`. On
today's corpus that is exactly the 1 true positive and 0 false positives — the 40 uncertain
cases all trip the `include:` guard and stay silent. Much cheaper than A, and honest about its
own blind spot, at the price of checking nothing for the 40.

**C. Accept and close.** One bad interaction in 164 references, failing loudly at run time
with a compose error. Record it and move on.

**Recommendation: B.** It captures the entire measured benefit of A at a fraction of the cost,
and its blind spot is explicit rather than pretended-away. A is worth revisiting only if DVA
grows a compose parser for another reason — the resolver should not be built *for* this check.
B must be implemented with the guard as a hard precondition, not a heuristic: the failure mode
of getting it wrong is 40 false alarms, which would make `validate` untrustworthy.

## The user's tree — separate from the DVA decision

`sigdock-pass-devbox`'s `redis-cli` interaction is the user's config to fix, and there are
three plausible shapes, none obviously right:

- add `compose-redis.yaml` to `files:` — but that starts 6 containers (master, 2 replicas,
  3 sentinels) on every `dva up`, which is likely exactly why it was left as an opt-in overlay;
- move redis into `compose.yml` behind a compose profile, so `Profiles` can gate it — the
  shape DVA actually supports, but a larger config change;
- delete the interaction and keep the overlay a documented manual step.

Do not choose on the user's behalf.

## Non-goals

- Do not add a `files:` field to `ComposeOptions` to make the current config work. That is a
  config-model change dressed up as a bug fix, and it belongs to its own decision.
- Do not edit `~/mydevbox/sigdock-pass-devbox/dva.yml` as part of this task.
- Do not implement the check without the `include:` guard. 40 false positives is worse than no
  check.
- Do not treat compose profiles as affecting service *existence* — they gate activation only.

## Acceptance criteria

- [ ] Option chosen and recorded here | verify: `human — decision recorded`
- [ ] If B: warns on a service absent from declared files with no `include:` present | verify: `go test ./internal/config/ -run TestInteractionServiceExists`
- [ ] If B: silent when any declared compose file uses `include:` | verify: `go test ./internal/config/ -run TestInteractionServiceExists`
- [ ] If B: silent on interpolated service names and unreadable compose files | verify: `go test ./internal/config/ -run TestInteractionServiceExists`
- [ ] If B: corpus sweep reports exactly 1 | verify: `human — re-run the Evidence sweep, expect 1 flagged / 0 new`
- [ ] If C: closed with the count recorded | verify: `human — this file`

## Evidence

Measured 2026-07-30 by `tmp/`-style probe (`probe_interaction_services.py`, scratchpad),
parsing each `dva.yml` and the compose files its `stack.*.runners.*.files` declare, recursing
into `subcommands`, and skipping interactions whose `runner:` is not `compose`.

Scope: every `dva.yml` up to 3 directories below `~/mydevbox` — 64 declare `interaction:`,
164 `service:` references. (The 31-config figure used elsewhere in `tasks/` is the
`-maxdepth 2` top-level set; this sweep goes deeper to include nested configs such as
`gorisa-devbox/gorisa-rails`.)

The probe classifies rather than asserts: anything it cannot establish — `include:` present,
`${}` interpolation, missing or non-mapping `services:` — is reported as *uncertain* and not
counted as a defect. All 40 uncertain results have the single cause `include:`. That is what
makes option B's guard sufficient today, and also what makes it fragile if a config ever
combines `include:` with a genuinely wrong service name — B would stay silent. Accepted.
