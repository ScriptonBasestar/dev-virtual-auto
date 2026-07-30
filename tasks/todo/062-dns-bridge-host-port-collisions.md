---
id: TASK-062
title: "dns-bridge publishes six host ports twice — two compose files were added without registering their ports"
type: fix
priority: P2
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox — compose/infra-redis-{cluster,sentinel}.yaml and PORT_MAPPINGS.yaml; needs the user's go-ahead to edit"
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
   An earlier note here said 10 and included the monitoring group; that was wrong —
   `full-stack-monitoring` activates `compose_profiles: [rust, monitoring]`, so prometheus,
   grafana, jaeger and otel-collector are all reachable. Corrected by parsing each compose
   file and mapping every endpoint's service to its `profiles:` list.

## Fix shape

Needs the user's call on direction; the mechanical part is small either way.

- **Renumber the redis-sentinel services** out of the kafka and monitoring ranges, and
  register them in `PORT_MAPPINGS.yaml`. The 11270-11289 band appears unused.
- **Renumber `redis-node-1`** off 11220, or give `redis` and `postgres` an explicit
  profile (e.g. `default`) so the alternatives really are exclusive. The second option
  changes what a bare `docker compose up` does, so it is a behaviour decision, not a
  cleanup.
- Add the alternative topologies to `PORT_MAPPINGS.yaml` either way — leaving them
  unregistered is what allowed this.

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

- [ ] Direction chosen for 11220 and for the sentinel range | verify: `human — decision recorded here`
- [ ] No host port is published by two services | verify: `human — re-run the reconciliation in Evidence; expect 0 duplicates`
- [ ] Alternative topologies registered | verify: `/usr/bin/grep -qE 'redis-(master|node|sentinel)' ~/mydevbox/scripton-dns-bridge-devbox/PORT_MAPPINGS.yaml`
- [ ] Config still validates | verify: `cd ~/mydevbox/scripton-dns-bridge-devbox && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva validate`

## Evidence

Measured 2026-07-30. Every published host port, extracted per owning service:

```
/usr/bin/awk '/^  [a-z0-9_-]+:$/{svc=substr($1,1,length($1)-1)} \
  /^ *- "?\$\{?[A-Z_]*:?-?[0-9]+/{print svc": "$0}' compose.yaml compose/*.yaml
```

Profile ownership per service was extracted the same way rather than by a recursive grep:
an earlier `grep -A2 '^    profiles:'` missed `obs-monitoring.yaml` and
`infra-redis-sentinel.yaml` entirely because their `profiles:` keys sit at a different
indentation, which would have made both files look unprofiled — i.e. always-on — and
inflated the collision set. Per-file counts (7 files, all non-zero) caught the error.
