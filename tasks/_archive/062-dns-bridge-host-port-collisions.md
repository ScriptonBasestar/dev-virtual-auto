---
id: TASK-062
title: "dns-bridge publishes six host ports twice — two compose files were added without registering their ports"
type: fix
priority: P2
status: done
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox — compose/infra-redis-{cluster,sentinel}.yaml and PORT_MAPPINGS.yaml; needs the user's go-ahead to edit"
verified-at: 2026-08-03T11:52:20+09:00
archived-at: 2026-08-03T11:52:20+09:00
verification-summary: |
  All 4 criteria re-verified against live repo state.
  Commit b61ab84 ("fix(compose): move the alternative redis topologies off six
  double-booked ports") is on branch in ~/mydevbox/scripton-dns-bridge-devbox.
  PORT_MAPPINGS.yaml:30-80 registers all 8 alternative-topology ports at 11221-11228,
  matching infra-redis-cluster.yaml (11221-11223) and infra-redis-sentinel.yaml
  (11224-11228) exactly.
  The acceptance criterion's own command was re-run with all 6 profiles active
  (kafka, monitoring, redis-cluster, redis-sentinel, nameserver, dev): 24 published
  port/protocol pairs, `uniq -d` empty — no host port published twice.
  .env.example:73-76 replaced the old "by design" overlap comment with an honest one.
  `dva validate` rc=0. No .env present to mask stale defaults. TASK-071, referenced
  under "Left open", is itself closed.
---

# Task 062: Resolve six duplicate host-port bindings in dns-bridge

## Problem

Six host ports are published by two different services each. Docker fails the second
bind, so the affected profile combination cannot start.

| host port | service A | service B | collides when |
| --- | --- | --- | --- |
| 11220 | `redis` — **no profile, always-on** | `redis-node-1` [redis-cluster] | **whenever redis-cluster is used at all** |
| 11230 | `kafka` [kafka] | `redis-master` [redis-sentinel] | kafka + redis-sentinel |
| 11231 | `zookeeper` [kafka] | `redis-replica` [redis-sentinel] | kafka + redis-sentinel |
| 11240 | `prometheus` [monitoring] | `redis-sentinel-1` [redis-sentinel] | monitoring + redis-sentinel |
| 11241 | `grafana` [monitoring] | `redis-sentinel-2` [redis-sentinel] | monitoring + redis-sentinel |
| 11242 | `otel-collector` gRPC [monitoring] | `redis-sentinel-3` [redis-sentinel] | monitoring + redis-sentinel |

### 11220 is the sharp one — a comment stands in for an invariant

`compose.yaml:15-16` says the two alternative redis topologies are "mutually exclusive
with default redis". But `redis` and `postgres` sit under the "Core Infrastructure
(always-on)" heading with **no `profiles:` key**, so they start on every
`docker compose up`. Compose has no way to express "this profile turns an unprofiled
service off". So `--profile redis-cluster up` starts `redis` *and* `redis-node-1`, both
binding 11220, and the exclusivity the comment asserts is unenforceable as written.

The redis-sentinel file avoids this only by accident: `redis-master` took 11230 instead
of 11220, which trades the certain collision for a conditional one against kafka.

## Root cause — the registry was bypassed twice

`PORT_MAPPINGS.yaml` is the project's allocation authority, and it is **silent about both
alternative topologies**: no `redis-master`, no `redis-node`, no `redis-sentinel` entry
anywhere in it. It allocates 11230/11231 to `kafka`/`zookeeper` and 11240-11243 to the
monitoring group.

So the two files were written by picking plausible-looking numbers from the project's
range without registering them — and the numbers they picked were already assigned. The
collisions are not a drift between two copies of the same fact; they are the absence of
a fact in the one place that was supposed to hold it.

This is why the registry agreed with compose in [TASK-058](058-dns-bridge-endpoint-source-ports.md)
and still failed to prevent this: it is authoritative for what it records, and nothing
makes recording mandatory.

## Reachability from DVA today

No declared mode reaches any of these combinations. `modes:` uses only
`compose_services: [postgres, redis]` (infra, hybrid, dev) and `compose_profiles: [rust]`
/ `[rust, monitoring]` (full-stack, full-stack-monitoring). The `kafka`, `nameserver`,
`redis-cluster`, and `redis-sentinel` profiles are activated by **no mode at all**.

Two consequences worth separating:

1. The collisions are only reachable through raw `docker compose --profile ...`, not
   through `dva up --mode ...`. That lowers urgency but does not make it a non-issue —
   `compose.yaml:3` documents those profiles as the intended usage.
2. `endpoints:` publishes **6** entries for services no mode can start (kafka, zookeeper,
   powerdns-api, etcd, coredns, mock-auth), so `dva show` advertises URLs no declared mode
   brings up. **Separate finding — [TASK-064](064-dns-bridge-endpoints-no-mode-can-start.md).**
   An earlier note said 10 and wrongly included the monitoring group, which
   `full-stack-monitoring` does reach; corrected by mapping each endpoint's service to its
   `profiles:` list.

## Fix shape

Two directions, both mechanically small: renumber the offending services out of the kafka
and monitoring ranges (the 11270-11289 band appears unused), or give `redis`/`postgres` an
explicit profile so the alternatives really are exclusive — the second changes what a bare
`docker compose up` does, so it is a behaviour decision, not a cleanup. Registering the
topologies in `PORT_MAPPINGS.yaml` is required either way; leaving them unregistered is
what allowed this. See Resolution for the direction taken.

## Non-goals

- Do not touch `endpoints:` — [TASK-058](058-dns-bridge-endpoint-source-ports.md) closed
  that, and all 13 entries reconcile against the ports as they are today.
- Do not add profiles to `redis`/`postgres` as a drive-by; it changes default behaviour.
- Do not build a DVA feature for this here — see below.

## DVA angle — measured, not assumed

DVA **cannot** detect this today. `internal/config/lifecycle.go:117,777` parse a `ports:`
list only for DVA's own `docker:` application strategy; external compose files are passed
to `docker compose` unread, so published host ports are outside DVA's model. A warning
would require parsing referenced compose files and reasoning about which profiles can
co-activate.

That reasoning is the same question `modesIsolateEntries` answers for stack entries
(TASK-056): *can these two things be live at once?* Here the partitioning key is
`profiles:` inside compose rather than `modes.<name>.stack` inside `dva.yml`. Worth its
own proposal if the user wants it; recorded here so the connection is not lost.

## Acceptance criteria

- [x] Direction chosen for 11220 and for the sentinel range | verify: `human — decision recorded in Resolution`
- [x] No host port is published by two services | verify: human — render the named personal Compose profiles and confirm the duplicate published-port set is empty, as recorded in the archived result
- [x] Alternative topologies registered | verify: human — inspect the named personal port registry for redis master, node, and sentinel entries; the archived result records all eight ports
- [x] Config still validates | verify: human — validate the named personal config with an installed DVA; the archived review records success

## Evidence

Measured 2026-07-30 by walking `compose.yaml compose/*.yaml` with awk, tracking the
current service key and printing each `ports:` entry against it. Superseded by the
`docker compose config` criterion above, which resolves profiles instead of guessing.

Profile ownership per service was extracted the same way rather than by a recursive grep:
an earlier `grep -A2 '^    profiles:'` missed `obs-monitoring.yaml` and
`infra-redis-sentinel.yaml` entirely because their `profiles:` keys sit at a different
indentation, which would have made both files look unprofiled — i.e. always-on — and
inflated the collision set. Per-file counts (7 files, all non-zero) caught the error.

## Resolution

Renumber, not re-profile. Fixed in dns-bridge `b61ab84`; the registry entries and
`.env.example` moved in the same commit.

| service | old | new |
| --- | --- | --- |
| `redis-node-1..3` | 11220, 11221, 11222 | 11221, 11222, 11223 |
| `redis-master`, `redis-replica` | 11230, 11231 | 11224, 11225 |
| `redis-sentinel-1..3` | 11240, 11241, 11242 | 11226, 11227, 11228 |

Default `redis` keeps 11220, so `dva.yml` needed no edit: its `REDIS_URL` and `redis`
endpoint stay correct, and the `endpoints:` non-goal held.

**Chose the 1122x cache decade over the 11270-11289 band this task suggested.** The
registry allocates by decade — 1120x application, 1121x database, 1122x cache, 1123x
queue, 1124x monitoring, 1129x dev — and cache had nine free slots for eight ports. A
fresh band would also have worked, but staying in the decade makes a repeat collision with
queue or monitoring structurally impossible, not merely avoided once.

The exclusivity claim was **not** made true: the non-goal forbade giving `redis` a
profile, and compose cannot express "this profile disables an unprofiled service" anyway,
so a stray standalone redis on 11220 now runs beside either topology. Both file headers
state that outcome instead of repeating the unenforceable "MUTUALLY EXCLUSIVE" line.

A third copy of the wrong fact turned up: `.env.example` called the overlap intentional
(`# Ports overlap with Kafka ... by design`) — same defect class as TASK-057/060/065/067,
a claim living where nothing checks it.

### Measurement corrections

Two of my own measurements were wrong before they were right, both worth recording
because the failure mode is the same one this task's Evidence already warns about:

1. Profile flags built in a shell variable and passed unquoted read as one argument (zsh
   does not word-split unquoted expansions), so `docker compose` failed, the port list came
   back empty, and five combinations reported "no duplicates" vacuously. Caught by printing
   the port *count* beside each verdict — 5/7/9/12/24, never zero.
2. The first detector ignored protocol and flagged 11253/11254, which are powerdns and
   coredns publishing one host port each on tcp *and* udp. Legal. The criterion above pairs
   `published` with `protocol`.
3. The first negative control copied `.env.example` into the throwaway tree as `.env`,
   overriding the old compose defaults with the new values and making the pre-fix state
   look clean. Re-run with no env file, it reproduces exactly the six documented
   collisions: `redis-cluster` alone duplicates 11220/tcp;
   `kafka + redis-sentinel` duplicates 11230/tcp and 11231/tcp;
   `monitoring + redis-sentinel` duplicates 11240/tcp, 11241/tcp, 11242/tcp.

Post-fix, all six profiles active at once publish 24 host ports with no duplicate
port/protocol pair. `dva validate` rc=0.

### Left open

Four nameserver ports (11253, 11254, 11260, 11261) are published but unregistered — same
bypassed-registry root cause, different service group. Filed as
[TASK-071](071-nameserver-ports-unregistered.md).
