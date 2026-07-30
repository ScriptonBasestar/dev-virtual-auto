---
id: TASK-064
title: "dns-bridge advertises six endpoints no mode can start, and a compose profile shares a name with an unrelated mode"
type: fix
priority: P3
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox/dva.yml — endpoints:/modes:; needs the user's go-ahead to edit"
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

- [ ] Direction chosen for the six unreachable endpoints | verify: `human — decision recorded here`
- [ ] The `dev` name collision is resolved or explicitly accepted | verify: `human — recorded here`
- [ ] Every remaining endpoint is startable by some mode | verify: `human — re-run the mapping in Evidence; expect 0 "NO" rows`
- [ ] Config still validates | verify: `cd ~/mydevbox/scripton-dns-bridge-devbox && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva validate`

## Evidence

Measured 2026-07-30 by parsing `compose.yaml` and `compose/*.yaml` with PyYAML and joining
each endpoint's `source:` service to its `profiles:` list — not by grep. An earlier grep
pass put the count at 10 by treating the monitoring group as unreachable; it is reachable
through `full-stack-monitoring`, and the indentation-sensitive grep that produced that
answer had already missed two files in TASK-062.
