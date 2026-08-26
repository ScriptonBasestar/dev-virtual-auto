---
id: TASK-058
title: "dns-bridge endpoints publish four wrong ports — one points at a different service"
type: fix
priority: P2
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox/dva.yml — user's real project, needs their go-ahead to edit"
verified-at: 2026-08-03T11:52:20+09:00
archived-at: 2026-08-03T11:52:20+09:00
verification-summary: |
  All 7 criteria verified against the live config, not the task's prose.
  The four corrected sources are present in ~/mydevbox/scripton-dns-bridge-devbox/dva.yml:
  jaeger:11245 (:584), powerdns:11260 (:594), coredns:11254 (:602), mock-auth:11290 (:606).
  Each cross-checks against its compose host-port binding (obs-monitoring.yaml:64,
  infra-nameserver.yaml:11 and :58-59, dev-tools.yaml:11), the .env.example default,
  and PORT_MAPPINGS.yaml. The full 13-endpoint reconciliation table was re-derived
  independently and matches the task's table entry for entry.
  `dva validate` → rc=0, "dva.yml is valid" (warnings only, no errors).
  The loss-check script against the named backup reports "0 fail / 29 files" with
  endpoints=13, matching the recorded Result verbatim. Follow-up TASK-062 exists.
---

# Task 058: Correct four endpoint source ports in dns-bridge

## Problem

Four `endpoints.<name>.source` ports in `~/mydevbox/scripton-dns-bridge-devbox/dva.yml`
do not match the compose files, so `dva show` prints URLs that cannot open. One of
them points at a **different service's** port.

Pre-existing — not introduced by the 2026-07-30 removed-keys migration. Surfaced
during it and reported rather than silently fixed, which was the right call.

| endpoint | `dva.yml` declares | actual host port | compose |
| --- | --- | --- | --- |
| jaeger (line 564) | `jaeger:11243` | `${JAEGER_PORT:-11245}` | `compose/obs-monitoring.yaml:64` |
| powerdns-api (line 574) | `powerdns:11250` | `${POWERDNS_API_PORT:-11260}` | `compose/infra-nameserver.yaml:11` |
| coredns (line 582) | `coredns:11270` | `${COREDNS_DNS_PORT:-11254}` | `compose/infra-nameserver.yaml:58-59` |
| mock-auth (line 586) | `mock-auth:11280` | `${MOCK_AUTH_PORT:-11290}` | `compose/dev-tools.yaml:11` |

**`11243` is `OTEL_HTTP_PORT`** (`compose/obs-monitoring.yaml:82`) — the
OpenTelemetry collector's HTTP port, a service that already has its own endpoint at
line 570 (`otel-collector:11242`). So the jaeger endpoint currently advertises
another service's port, not merely a stale number.

## Which side is authoritative

The project keeps a port registry, `PORT_MAPPINGS.yaml`, and it **agrees with the
compose files, not with `dva.yml`**:

- `otel_http: 11243` (line 65), `jaeger: 11245` (line 69), `mock_auth: 11290` (line 75)

So `dva.yml` is the drifted copy. Fix `dva.yml` to match compose; do not touch
compose or the registry.

## Judgment call to settle — coredns is not an HTTP endpoint

coredns exposes only `53/udp` and `53/tcp` (mapped from host `11254`). There is no
HTTP port. Correcting the number to `11254` makes the mapping accurate but still
leaves `dva show` advertising a browser URL for a DNS listener.

Options, needs the user's call:

1. correct the port to `11254` and accept that the printed URL is informational;
2. drop coredns from `endpoints:` — port visibility is already available from
   `dva status` / compose;
3. keep it but add `url:` explicitly so what is displayed is deliberate.

The other three are unambiguous number fixes.

## Fix shape

Four one-line edits to `endpoints.*.source` in `dva.yml`, plus whichever coredns
option is chosen. Use the compose default values (not literal ports) only if the
`endpoints.source` syntax supports interpolation — verify before assuming; if it
does not, the literal defaults above are correct.

## Non-goals

- Do not edit compose files or `PORT_MAPPINGS.yaml`.
- Do not add the ports dripter exposes but never declares — separate item.

## Acceptance criteria

- [x] jaeger source matches compose | verify: human — inspect the named personal config for jaeger source port 11245; the archived result records the match
- [x] powerdns source matches compose | verify: human — inspect the named personal config for powerdns source port 11260; the archived result records the match
- [x] mock-auth source matches compose | verify: human — inspect the named personal config for mock-auth source port 11290; the archived result records the match
- [x] coredns resolved per the chosen option | verify: human — inspect the named personal config for coredns source port 11254; the archived result records the match
- [x] Every endpoint resolves to a real service and its published port | verify: `human — 13/13 reconciled, table in the Result section`
- [x] Config still validates | verify: human — validate the named personal config with an installed DVA; the archived review records rc=0
- [x] No comment or key loss vs. backup | verify: human — compare the named personal config with its archived backup; the archived review records no loss

## Result — all four applied, all 13 endpoints reconciled

Applied 2026-07-30 to `~/mydevbox/scripton-dns-bridge-devbox/dva.yml`: jaeger
`11243`→`11245`, powerdns `11250`→`11260`, mock-auth `11280`→`11290`, coredns
`11270`→`11254`. `dva validate` rc=0; loss harness unchanged at 0 fail / 29 files.

The coredns option was settled by the user: **correct the number only**. `11270` was
published by nothing at all, so the endpoint was worse than imprecise — it named a port
that does not exist anywhere in the project. `11254` at least describes the real
`53/udp`+`53/tcp` mapping. That `dva show` still renders it as a browser URL is a display
concern about DNS services in `endpoints:` generally, not a fact about this port.

Every endpoint was then reconciled against every published host port, not just the four
being changed:

| endpoint | source | published by | verdict |
| --- | --- | --- | --- |
| postgres | `postgres:11210` | `compose.yaml:10` `DATABASE_PORT` | ok |
| redis | `redis:11220` | `compose.yaml:30` `REDIS_PORT` | ok |
| api | `dns-bridge-api-rs:11200` | `compose.yaml:79` `API_PORT` | ok |
| kafka | `kafka:11230` | `infra-kafka.yaml` `KAFKA_PORT` | ok |
| zookeeper | `zookeeper:11231` | `infra-kafka.yaml` `ZOOKEEPER_PORT` | ok |
| prometheus | `prometheus:11240` | `obs-monitoring.yaml` | ok |
| grafana | `grafana:11241` | `obs-monitoring.yaml` | ok |
| otel-collector | `otel-collector:11242` | `OTEL_GRPC_PORT` | ok (HTTP `11243` undeclared) |
| jaeger | `jaeger:11245` | `JAEGER_PORT` | fixed |
| powerdns-api | `powerdns:11260` | `POWERDNS_API_PORT` | fixed |
| etcd | `etcd:11261` | `ETCD_PORT` | ok |
| coredns | `coredns:11254` | `COREDNS_DNS_PORT` | fixed |
| mock-auth | `mock-auth:11290` | `MOCK_AUTH_PORT` | fixed |

The three that an earlier sweep could not find — `postgres`, `redis`,
`dns-bridge-api-rs` — live in the **root `compose.yaml`**, which that sweep never read; it
only walked `compose/*.yaml`. The endpoints were right and the measurement was incomplete.

## Follow-up found here, promoted to its own task

Six host-port collisions between compose files, including the redis-sentinel/kafka overlap
this task originally flagged. See [TASK-062](062-dns-bridge-host-port-collisions.md) — the
reconciliation above is what surfaced the full set, since it was the first pass to compare
all published ports against each other rather than each endpoint against its own service.

## Evidence

Verified 2026-07-30 by reading `dva.yml`, `compose/*.yaml` and `PORT_MAPPINGS.yaml`
directly. Context: `tmp/71-mydevbox-migration-result.md`, section C.
