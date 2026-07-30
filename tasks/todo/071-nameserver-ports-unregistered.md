---
id: TASK-071
title: "Four nameserver host ports are published but absent from the port registry"
type: fix
priority: P4
status: todo
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox — PORT_MAPPINGS.yaml only"
---

# Task 071: Register the nameserver ports

## Problem

`compose/infra-nameserver.yaml` publishes four host ports that `PORT_MAPPINGS.yaml` does
not record:

| env var | port | service |
| --- | --- | --- |
| `POWERDNS_DNS_PORT` | 11253 | `powerdns` (tcp + udp) |
| `COREDNS_DNS_PORT` | 11254 | `coredns` (tcp + udp) |
| `POWERDNS_API_PORT` | 11260 | `powerdns` HTTP API |
| `ETCD_PORT` | 11261 | `etcd` |

The registry has no `nameserver:` group at all. Every other published port in the project
is registered; these four are the whole remainder.

## Why it matters

This is the same root cause as [TASK-062](062-dns-bridge-host-port-collisions.md), which
resolved six double-booked ports: the registry is authoritative for what it records, and
nothing makes recording mandatory. Unregistered ports are how the redis-sentinel file came
to borrow 11230-11231 from kafka and 11240-11242 from monitoring — the numbers looked free
because the registry did not say otherwise.

Nothing is broken today. These four do not collide with anything, which is why this is P4
rather than a bug. The cost of leaving it is that the next person allocating a port in
1125x or 1126x gets the same false "unused" signal.

## Fix shape

Add a `nameserver:` group to `services:` in `PORT_MAPPINGS.yaml` with the four entries
above. Nothing else changes: no compose file, no `.env.example`, no `dva.yml`.

Note that 11253/11254 are each bound twice — once tcp, once udp, by a single service. That
is legal and is not a duplicate; the registry format has no protocol field, so one entry
per port is correct. TASK-062's first duplicate detector flagged these two as collisions
because it ignored protocol; do not "fix" them.

## Non-goals

- Do not renumber anything. The current numbers are uncontested.
- Do not add a protocol field to the registry format for these two entries alone.

## Acceptance criteria

- [ ] All four ports registered | verify: `/usr/bin/grep -cE 'POWERDNS_DNS_PORT|COREDNS_DNS_PORT|POWERDNS_API_PORT|ETCD_PORT' ~/mydevbox/scripton-dns-bridge-devbox/PORT_MAPPINGS.yaml` — expect 4
- [ ] Registry agrees with the compose defaults | verify: `human — for each of the four, registry port == the ${VAR:-NNNNN} default in compose/infra-nameserver.yaml`
- [ ] Registry still parses | verify: `python3 -c "import yaml; yaml.safe_load(open('$HOME/mydevbox/scripton-dns-bridge-devbox/PORT_MAPPINGS.yaml'))"`
- [ ] No published port remains unregistered | verify: `human — re-run the three-way reconciliation from TASK-062; the UNREGISTERED list must be empty`

## Evidence

Measured 2026-07-30 while closing TASK-062, by diffing every `${VAR:-NNNNN}` default in
`compose.yaml compose/*.yaml` against every `env:`/`port:` pair in `PORT_MAPPINGS.yaml`.
All shared variables agreed on their value; these four were present in compose and absent
from the registry. `.env.example` also declares all four, so the registry is the only
source missing them.
