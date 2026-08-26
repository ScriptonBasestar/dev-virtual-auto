---
id: TASK-064
title: "dns-bridge advertises six endpoints no mode can start, and a compose profile shares a name with an unrelated mode"
type: fix
priority: P3
status: done
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox/dva.yml — endpoints:/modes:; needs the user's go-ahead to edit"
verified-at: 2026-08-03T11:55:33+09:00
archived-at: 2026-08-03T11:55:33+09:00
verification-summary: |
  All 4 criteria MET. The deliverable lives in
  ~/mydevbox/scripton-dns-bridge-devbox at commit d310013 ("feat(modes): add kafka and
  nameserver modes, wire mode dev to its profile"): dva.yml:201-207 adds the kafka and
  nameserver modes with compose_profiles, and :188-190 changes mode dev to select
  profile dev instead of naming services. No endpoint was deleted — all 13 remain.
  Verified by running the real command, not by reading: `dva up --mode dev --dry-run`
  now emits `--profile dev` with zero positional service names; kafka and nameserver
  modes emit their own profiles.
  All 13 endpoints were re-mapped to compose `profiles:` keys by scanning the five
  compose files: postgres/redis unprofiled (always on), zookeeper+kafka via kafka,
  dns-bridge-api-rs via rust, the four observability services via monitoring,
  powerdns/etcd/coredns via nameserver, mock-auth via dev. Zero unreachable.
  `dva validate` rc=0.
  Citation drift, harmless: the Resolution cites internal/lifecycle/compose.go:197-202,
  now 182-187 after commit 3171945 collapsed the four argv builders. The mechanism it
  describes — ComposeServices appended as positional args only for "up" — still holds.
---

# Task 064: Reconcile `endpoints:` with what a mode can actually start

## Problem

`dva show` lists all 13 `endpoints:`. Six of them name services that **no declared mode
can bring up**, so the URL is advertised and nothing will ever answer on it.

| endpoint | service | compose profile | reachable? |
| --- | --- | --- | --- |
| `postgres`, `redis` | same | *(none — always on)* | yes |
| `api` | `dns-bridge-api-rs` | `rust` | yes — full-stack |
| `prometheus`, `grafana`, `jaeger`, `otel-collector` | same | `monitoring` | yes — full-stack-monitoring |
| `kafka`, `zookeeper` | same | `kafka` | **no** |
| `powerdns-api`, `etcd`, `coredns` | `powerdns`, `etcd`, `coredns` | `nameserver` | **no** |
| `mock-auth` | `mock-auth` | `dev` | **no** — see below |

The five modes select only `compose_services: [postgres, redis]` (infra, hybrid, dev) or
`compose_profiles: [rust]` / `[rust, monitoring]` (full-stack, full-stack-monitoring).
Profiles `kafka` and `nameserver` are named by no mode at all.

## The `dev` name collision is the interesting part

`mock-auth` sits in compose profile **`dev`**, and `dva.yml` also declares a **mode** named
`dev`. They are unrelated: mode `dev` activates `compose_services: [postgres, redis]` and
never mentions `compose_profiles`. So `dva up --mode dev` does *not* start the `dev`
profile, which is the single most reasonable thing a reader would assume it does.

This is worse than a missing endpoint, because nothing looks wrong. The other five are
plainly unreachable once you check; this one reads as deliberately wired and is not.

## Root cause

`endpoints:` is a flat, hand-maintained list with no relationship to `modes:`. Nothing
cross-checks the two, so an endpoint outlives the mode that justified it — or, as with
`kafka` and `nameserver`, is added for a profile a mode was never written for.

Same shape as [TASK-062](062-dns-bridge-host-port-collisions.md): a fact that is correct in
isolation (the port really is what compose publishes — TASK-058 reconciled all 13) and
wrong in relation to something else.

## Fix shape

Needs the user's direction; all three options are small.

- **Add modes** for `kafka` and `nameserver` if those stacks are meant to be usable through
  DVA. `compose.yaml:3` documents both profiles as intended usage, which argues for this.
- **Drop the six endpoints** if the profiles are compose-only experiments. Cheapest, and
  makes `dva show` honest.
- **Fix the `dev` collision either way** — either give mode `dev` a
  `compose_profiles: [dev]`, or rename one of the two so the coincidence stops reading as a
  connection.

## Non-goals

- Do not change any port. [TASK-062](062-dns-bridge-host-port-collisions.md) owns the
  collisions, and this task must not renumber anything while that is undecided.
- Do not "fix" this by deleting `endpoints:` wholesale — seven entries are correct and
  reconciled.

## DVA angle

DVA could warn here without the compose-parsing feature TASK-062 would need: an endpoint's
`source:` service name could be checked against the union of every mode's
`compose_services:`. That covers `mock-auth` and misses the profile-only cases, so it is a
partial guard — worth noting, not worth claiming as a fix.

## Acceptance criteria

- [x] Direction chosen for the six unreachable endpoints | verify: `human — decision recorded in Resolution`
- [x] The `dev` name collision is resolved or explicitly accepted | verify: human — dry-run the named personal config's dev mode and confirm the dev profile appears; the archived result records that outcome
- [x] Every remaining endpoint is startable by some mode | verify: `human — re-run the mapping in Evidence; expect 0 "NO" rows`
- [x] Config still validates | verify: human — validate the named personal config with an installed DVA; the archived review records success

## Evidence

Measured 2026-07-30 by parsing `compose.yaml` and `compose/*.yaml` with PyYAML and joining
each endpoint's `source:` service to its `profiles:` list — not by grep. An earlier grep
pass put the count at 10 by treating the monitoring group as unreachable; it is reachable
through `full-stack-monitoring`, and the indentation-sensitive grep that produced that
answer had already missed two files in TASK-062.

## Resolution

Added the modes. Fixed in dns-bridge `d310013`; no endpoint was deleted, so the non-goal
about not gutting `endpoints:` held.

```
kafka       -> compose_profiles: [kafka]       (kafka, zookeeper)
nameserver  -> compose_profiles: [nameserver]  (powerdns, etcd, coredns)
dev         -> compose_profiles: [dev]         (mock-auth)  — compose_services dropped
```

All three also start `postgres` and `redis`: those declare no profile, so every `up`
includes them whichever profile is selected.

### Dropping `compose_services` from `dev` was the load-bearing detail

Giving mode `dev` a `compose_profiles: [dev]` while leaving
`compose_services: [postgres, redis]` in place would **not** have started `mock-auth`.
`internal/lifecycle/compose.go:197-202` appends that list as positional service names to
`docker compose up`, and naming services suppresses profile activation — so the mode would
have gained a `--profile dev` flag that changed nothing, reproducing exactly the
"looks wired, isn't" defect this task was filed about. Omitting the key instead means
`nil`, documented in `config.go:141` as "start all services", which under `--profile dev`
is the two unprofiled services plus the profile.

Confirmed by argv rather than by reading the code twice:

```
--mode dev    -> compose ... --profile dev up -d --wait          (no service names)
--mode infra  -> compose ... up -d --wait postgres redis         (unchanged)
```

`infra` still exists as the minimal DB+Redis mode, so making `dev` heavier cost nothing.

### DVA does not check profile names

`compose_profiles` values are never validated: `validate_warnings.go` has no
profile-aware check, and DVA does not read external compose files at all (established in
[TASK-062](062-dns-bridge-host-port-collisions.md)). A typo in `nameserver` would produce
a mode that silently starts only postgres and redis. The compensating measurement was
`docker compose --profile <p> config --services`, i.e. asking docker rather than trusting
the spelling — it returned exactly `{mock-auth, postgres, redis}`,
`{kafka, postgres, redis, zookeeper}` and `{coredns, etcd, postgres, powerdns, redis}`.

That check belongs to the same unfiled compose-parsing proposal as TASK-062's warning
idea and TASK-068's `include:` gap; recorded here rather than filed a third time.

### Result

13 endpoints, 0 unreachable. `dva validate` rc=0; the remaining warnings
(`applications.*` compose ownership, `stack.*.order` → plans migration) predate this
change and are untouched by it.
