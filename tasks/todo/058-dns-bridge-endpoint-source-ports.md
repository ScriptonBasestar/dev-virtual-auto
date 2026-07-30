---
id: TASK-058
title: "dns-bridge endpoints publish four wrong ports — one points at a different service"
type: fix
priority: P2
status: todo
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox/dva.yml — user's real project, needs their go-ahead to edit"
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

- [x] jaeger source matches compose | verify: `grep -q 'source: "jaeger:11245"' ~/mydevbox/scripton-dns-bridge-devbox/dva.yml`
- [x] powerdns source matches compose | verify: `grep -q 'source: "powerdns:11260"' ~/mydevbox/scripton-dns-bridge-devbox/dva.yml`
- [x] mock-auth source matches compose | verify: `grep -q 'source: "mock-auth:11290"' ~/mydevbox/scripton-dns-bridge-devbox/dva.yml`
- [ ] coredns resolved per the chosen option | verify: `human — confirm which option was chosen and that dva.yml reflects it`
- [x] Config still validates | verify: `cd ~/mydevbox/scripton-dns-bridge-devbox && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva validate`
- [x] No comment or key loss vs. backup | verify: `uv run --with pyyaml python /Users/archmagece/mywork/scripton/dev-virtual-auto/tmp/scripts/verify-migration.py /Users/archmagece/mywork/scripton/dev-virtual-auto/tmp/mydevbox-backup-20260730-101632 ~/mydevbox`

## Result — three of four applied, coredns awaiting a decision

Applied 2026-07-30 to `~/mydevbox/scripton-dns-bridge-devbox/dva.yml`: jaeger
`11243`→`11245`, powerdns `11250`→`11260`, mock-auth `11280`→`11290`. `dva validate`
rc=0; loss harness unchanged at 0 fail / 29 files.

**coredns is deliberately left at `11270`** — wrong, but the right value depends on the
option chosen above, and renumbering it to a DNS port would make `dva show` advertise a
browser URL for a `53/udp` listener. Fixing the number alone would trade a visibly broken
link for a plausible-looking one, which is worse.

A full sweep of every published port while verifying these three (`compose/*.yaml`,
`${VAR:-default}` form) confirmed the four endpoints that were already right — prometheus
`11240`, grafana `11241`, otel-collector `11242` (OTEL_GRPC_PORT; the collector's second
port `11243` is OTEL_HTTP_PORT, which is what jaeger was wrongly pointing at), etcd
`11261`. Three endpoints did not appear in that sweep at all — `postgres:11210`,
`redis:11220`, `dns-bridge-api-rs:11200` — so they are published in some other form or
have no backing service; being audited separately.

## Follow-up found here, not part of this task

`compose/infra-redis-sentinel.yaml` publishes host `11230`/`11231`
(REDIS_MASTER_PORT/REDIS_REPLICA_PORT) and `compose/infra-kafka.yaml` publishes the same
host `11230`/`11231` (KAFKA_PORT/ZOOKEEPER_PORT). Whether that is a live collision depends
on whether the two files can load in one invocation, which is under audit. Record it as
its own task once that is settled — do not fold it in here.

## Evidence

Verified 2026-07-30 by reading `dva.yml`, `compose/*.yaml` and `PORT_MAPPINGS.yaml`
directly. Context: `tmp/71-mydevbox-migration-result.md`, section C.
