---
id: TASK-071
title: "Four nameserver host ports are published but absent from the port registry"
type: fix
priority: P4
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "Cross-repo: ~/mydevbox/scripton-dns-bridge-devbox — PORT_MAPPINGS.yaml only"
verified-at: 2026-08-03T12:15:00+09:00
archived-at: 2026-08-03T12:15:00+09:00
verification-summary: |
  Commit aec4a7d ("docs(ports): register the four nameserver ports...") is HEAD in
  ~/mydevbox/scripton-dns-bridge-devbox, adding exactly the claimed 24-line
  `nameserver:` group between `monitoring:` and `dev:` — no compose, .env.example,
  or dva.yml touched, matching the stated non-goals.
  All four values match live: POWERDNS_DNS_PORT=11253, COREDNS_DNS_PORT=11254,
  POWERDNS_API_PORT=11260, ETCD_PORT=11261 across compose/infra-nameserver.yaml,
  PORT_MAPPINGS.yaml, and .env.example. tcp+udp double-bind comment for
  11253/11254 is present in the group header (lines 117-118), per the non-goal
  against adding a protocol field.
  `dva validate` in that repo: rc=0, "dva.yml is valid" (only pre-existing
  migration/Makefile-suggestion warnings unrelated to ports).
  Full three-way reconciliation re-derived independently: UNREGISTERED=[],
  VALUE MISMATCH compose-vs-registry=[], VALUE MISMATCH .env.example-vs-registry=[].
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

- [x] All four ports registered | verify: human — inspect the named personal port registry for the four nameserver variables; the archived review records all four
- [x] Registry agrees with the compose defaults | verify: `human — three-way reconciliation below; VALUE MISMATCH empty on both axes`
- [x] Registry still parses | verify: `python3 -c "import yaml; yaml.safe_load(open('$HOME/mydevbox/scripton-dns-bridge-devbox/PORT_MAPPINGS.yaml'))"`
- [x] No published port remains unregistered | verify: `human — reconciliation below; UNREGISTERED is empty over 23 compose vars`

## Evidence

Measured 2026-07-30 while closing TASK-062, by diffing every `${VAR:-NNNNN}` default in
`compose.yaml compose/*.yaml` against every `env:`/`port:` pair in `PORT_MAPPINGS.yaml`.
All shared variables agreed on their value; these four were present in compose and absent
from the registry. `.env.example` also declares all four, so the registry is the only
source missing them.

## Resolution

Fixed in dns-bridge `aec4a7d`. A `nameserver:` group was added between `monitoring:` and
`dev:` with the four entries as filed — no renumbering, no compose, `.env.example` or
`dva.yml` change.

The group spans **two** decades (1125x for DNS, 1126x for the control plane) where every
other group occupies one. Recorded in the group comment rather than normalized: renumbering
was an explicit non-goal, and the numbers are uncontested.

Re-running the three-way reconciliation after the edit:

```
compose vars=23  registry entries=24  .env.example vars=27
UNREGISTERED (compose default but not in registry): none
VALUE MISMATCH compose vs registry: none
VALUE MISMATCH .env.example vs registry: none
```

The non-zero counts matter more than the three `none` lines — an empty parse would produce
the same three verdicts vacuously, which is exactly how TASK-062's first five port checks
passed while measuring nothing. Registry entries (24) exceed compose vars (23) because
`API_PORT` and friends are application-level and not published through a compose default.

The tcp+udp double bind on 11253/11254 is now stated in the group comment, so the next
duplicate detector that ignores protocol has a written answer waiting for it.
