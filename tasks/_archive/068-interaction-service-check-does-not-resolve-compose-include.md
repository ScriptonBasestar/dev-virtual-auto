---
id: TASK-068
title: "The interaction-service check exists and is precise, but does not resolve compose `include:` — 3 configs are at latent false-positive risk"
type: decision
priority: P4
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "dva repo — internal/cli/validate.go detectConfigDriftWarnings/configuredComposeServices; motivating instance in ~/mydevbox/sigdock-pass-devbox"
verified-at: 2026-08-03T12:15:00+09:00
archived-at: 2026-08-03T12:15:00+09:00
verification-summary: |
  Doc-comment-only change verified byte-for-byte against commit 794a05e: comments added at
  validate.go:256-259 (early return) and :410-420 (configuredComposeServices), no logic diff.
  Corpus re-swept live with ./bin/dva v0.1.44: 31 configs (-maxdepth 2) and 69 configs
  (-maxdepth 4, corpus grew from the task's 64) both yield exactly 1 drift warning,
  sigdock-pass-devbox's redis-cli → redis-master, matching the task's claimed single true
  positive with zero false positives. ComposeOptions struct (config.go:475-479) confirmed
  to have only Method/Profiles/RunOptions, no files: field, backing the "profiles gate
  existence, not create it" claim. sigdock-pass-devbox/dva.yml still contains the
  unresolved redis-cli interaction untouched, matching the non-goal "do not edit the
  user's config."
---

# Task 068: Decide whether the interaction-service check should resolve compose `include:`

## Correction: this task's original premise was false

It was filed as "nothing checks that an interaction's `service:` exists", on the strength of
`internal/config/validate.go` containing no reference to `Service`. **The check exists.** It
lives in `internal/cli/validate.go:111-141` (`detectConfigDriftWarnings`), builds its service
set in `configuredComposeServices` (`:276-288`), and is wired into `dva validate`.

I grepped one `validate.go` and concluded absence from the other. Two independent probes
reported the check by running `dva validate` instead of reading a file, which is the method
this repo's measurement notes already prescribe — the program is the oracle, not a grep.
Everything below is re-measured on that basis; the original option A ("build the resolver")
is largely already built.

## What actually ships

`dva validate` emits, for the motivating config:

```
[warn] config drift: interaction "redis-cli" references compose service "redis-master",
but configured compose files expose db, minio, minio-init, server, sigdock-idp
```

Measured across the corpus — **1 warning from 64 configs**, and it is the one true positive.
Zero false positives. So the concern that motivated this task is already handled, and handled
well.

The unreachability finding itself stands and was independently confirmed: `redis-master` is
defined only in `compose-redis.yaml`, which no `dva.yml` key references, and `ComposeOptions`
(`internal/config/config.go:471-475`) has only `Method`, `Profiles` and `RunOptions` — no
`files`, so an interaction cannot pull in an undeclared file. A compose *profile* gates a
service that already exists; it cannot make one exist. `dva run redis-cli` fails at
`docker compose exec` with "no such service".

## The real, much narrower gap

`configuredComposeServices` reads only the declared files' top-level `services:`. It does not
follow compose `include:`. What keeps that from producing false positives today is the early
return in `detectConfigDriftWarnings`:

```go
availableServices := configuredComposeServices(c)
if len(availableServices) == 0 {
    return warnings
}
```

A compose file that declares no services of its own and pulls everything in via `include:`
yields an empty set, so the check bails rather than condemning every interaction. That is the
right instinct — but it is **incidental**, not stated as an `include:` guard, and it only
covers the *fully* empty case. Measured populations:

| population | configs | check behavior |
| --- | --- | --- |
| no `include:` in any declared file | 47 | fully applies — correct |
| `include:` present, `services:` empty | 14 | bails on the empty set — silent |
| `include:` present, `services:` also present | **3** | compares against a **partial** set |

The 3 are `funbricks-elemhant-devbox`, `scripton-dns-bridge-devbox`, and
`scripton-dns-bridge-devbox/dns-bridge-rs`. All three are clean today only because their
interactions happen to name services their own declared file defines. An interaction added
against an `include:`-supplied service in any of them produces a **false** drift warning.

`gizzahub-devbox` shows why the empty-set case matters: `compose.yaml` declares zero
top-level services and 14 `include:` entries. Without the early return, every interaction in
that repo would be condemned. Verified the includes do supply them —
`compose/app-backend.yaml` → `backend`, `compose/infra-postgres.yaml` → `postgres`,
`compose/infra-redis.yaml` → `redis`.

## Options

**A. Leave it.** 0 false positives today, 1 true positive caught. The 3 at-risk configs are
latent, not broken. Cheapest, and nothing is currently wrong.

**B. Make the guard explicit.** Skip the interaction-service comparison when any declared
compose file uses `include:`, instead of relying on the set coming out empty. ~5 lines plus a
test. Closes the 3-config gap and turns an incidental behavior into a stated one — this repo
has been bitten repeatedly by knowledge that nothing states. **Cost: the 3 configs lose a
check that currently works**, trading a possible false positive for a certain false negative.

**C. Resolve `include:` one level.** Union in the `services:` of each `include:`d file. Fixes
both the false-positive risk *and* the 14 configs' missing coverage — strictly better than B
on capability. Cost: `include:` has several forms (a bare path string, or a mapping with
`path`/`project_directory`/`env_file`), `path` may itself be a list, and getting it wrong
reintroduces false positives at 17-config scale. Needs a depth cap and must degrade to
silence, never to a warning, on anything it cannot read. S, not XS.

**Recommendation: A now, C when someone touches this code for another reason.** B spends a
working check to buy protection against a hypothetical, and for a *validator* the asymmetry
usually favors avoiding false positives — but at 3 latent configs against 3 configs of real
coverage, that trade is no longer clearly positive. C is the only option that improves both
numbers, and it is cheap enough to be worth doing opportunistically rather than as its own
project.

Whichever is chosen, state the `include:` limitation in a comment on
`configuredComposeServices`. Its correctness currently depends on an early return in a
different function, which is the kind of coupling that survives only as long as nobody
refactors either half.

## The user's tree — separate from the DVA decision

`sigdock-pass-devbox`'s `redis-cli` interaction is the user's config to fix, and there are
three plausible shapes, none obviously right:

- add `compose-redis.yaml` to `files:` — makes `dva run redis-cli` work, but puts a 6-container
  Redis Sentinel HA stack (master, 2 replicas, 3 sentinels) into every `dva up`/`dva down`,
  since interactions and lifecycle share the same file list. Probably why it was left an
  opt-in overlay;
- move redis into `compose.yml` behind a compose profile, so `Profiles` can gate it — the shape
  DVA actually supports, and the pattern `db`/`sigdock-idp` already use in that same file, but
  a larger config change;
- delete the interaction and keep the overlay a documented manual step.

Do not choose on the user's behalf.

## Non-goals

- Do not re-file "nothing checks interaction services". It does.
- Do not add a `files:` field to `ComposeOptions` to make the current config work. That is a
  config-model change dressed up as a bug fix, and belongs to its own decision.
- Do not edit `~/mydevbox/sigdock-pass-devbox/dva.yml` as part of this task.
- Do not treat compose profiles as affecting service *existence* — they gate activation only.
- If C: do not grow this into a general compose parser. One level of `include:`, a depth cap,
  and silence on anything unparseable.

## Acceptance criteria

- [x] Option chosen and recorded here | verify: `human — decision recorded, see Resolution`
- [x] `configuredComposeServices` states the `include:` limitation | verify: `/usr/bin/grep -n 'include' internal/cli/validate.go`
- [x] If B or C: an `include:`-supplied service does not warn | verify: `n/a — option A, no behavior change`
- [x] If C: silent on unreadable or non-path `include:` forms | verify: `n/a — option A, no behavior change`
- [x] Corpus still reports exactly 1 | verify: `human — re-ran the sweep 2026-07-30: 31 configs, 1 warning (sigdock-pass-devbox)`
- [x] Full suite green | verify: `make test`

## Resolution

**Option A, chosen by the user.** No behavior change: the check keeps its 0 false positives
and its 1 true positive, and the 3 at-risk configs stay latent rather than being traded for a
certain false negative.

What did change is that the limitation is now stated instead of merely being true.
`configuredComposeServices` carries a doc comment saying it does not resolve `include:`, that
14 corpus configs declare services it therefore cannot see, and — the part worth writing down —
that what prevents the gap from becoming a false positive is the empty-set early return in
`detectConfigDriftWarnings`, not anything in the function itself. The early return got a
matching comment saying it is load-bearing rather than an optimization.

That is the whole reason to spend a commit on a decision that changes no behavior: a refactor
that "simplified away" the empty-set check would have turned every `include:`-only project's
interactions into warnings, with nothing in the diff to suggest it.

Re-verified after the comments landed: 1 drift warning (`sigdock-pass-devbox`),
`go test ./internal/cli/` green. The sweep scanned 31 configs at `-maxdepth 2` where Evidence
below used `-maxdepth 4` and found 64 — different depth, same single warning.

Left for the user: the `redis-cli` interaction's three fixes, none obviously right (see above).

## Evidence

Measured 2026-07-30. The warning count comes from DVA itself, matched on the literal string
emitted at `internal/cli/validate.go:135`:

```
for f in $(find ~/mydevbox -maxdepth 4 -name dva.yml); do
    (cd $(dirname $f) && dva validate 2>&1 | grep -F 'references compose service')
done
→ configs-scanned=64  drift-warnings-emitted=1  (sigdock-pass-devbox)
```

The three populations come from `probe_include_risk.py` (scratchpad), parsing each `dva.yml`
and the compose files its `stack.*.runners.*.files` declare, and classifying by whether those
files carry `include:`, `services:`, or both → 47 / 14 / 3 of 64.

An earlier probe reported "40 uncertain, all `include:`". That is the same finding in a
different unit — 40 *interaction references* spread across the 17 configs that use `include:`
— not 40 configs, and not 40 false positives. DVA emits none of them, because 14 of those 17
hit the empty-set early return.
