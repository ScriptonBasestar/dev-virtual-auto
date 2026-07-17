---
id: TASK-026
title: "Shipped examples pass validate then hard-fail at stack up"
type: bug
priority: P1
status: blocked
effort: S
created-at: 2026-07-16T23:15:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: convergence-check-2
source-severity: HIGH
depends-on: [TASK-017]
blocked-reason: dependency
blocked-detail: "Whether the fix is code (make runners.native/docker run) or docs (rewrite 12 locations) is exactly TASK-017's undecided question"
unblock-condition: "TASK-017 decided Option A for docker only; native stack default_runner still unservable — need native plugin OR rewrite 12 native example/doc locations (split follow-up)"
depends-on-status: "TASK-017 done (docker only); native residual remains"
blocked-at: 2026-07-17T10:55:00+09:00
---

# Task 026: Shipped Examples Validate Green, Then Hard-Fail

## Summary

`stack.<entry>.default_runner: native` + `runners.native:` is the documented backend-app
pattern in **12 locations, including three shipped `examples/` files**. The shape passes
`dva validate` and then fails at `dva stack up`. A user copying `examples/full-stack.yml`
gets a green validate and a hard runtime failure.

## Evidence

Reproduced by the orchestrator on the repo's own example at HEAD `f4b2063`:

```
$ cp examples/full-stack.yml $T/dva.yml && cd $T
$ dva validate
[warn] semantic: stack: entries app-k8s, core-infra, frontend, web have order 0 (default) ...
✅ dva.yml is valid
EXIT=0

$ dva stack up web
ERROR: entry "web": unknown lifecycle plugin "" (implemented: [helm tilt skaffold sam
serverless compose docker kubectl kustomize vagrant multipass process script podman-compose],
planned: [])
EXIT=1
```

`examples/full-stack.yml:43-47` is the offending shape:

```yaml
    default_runner: native
    runners:
      native:
        dir: myapp
        build: bundle exec rails assets:precompile
```

Cause (established in TASK-009/TASK-017): `native` is absent from the plugin registry
(`internal/lifecycle/plugin_type.go`), and `runners.docker` decodes to `*DockerRunnerConfig`,
which no plugin consumes. The resolved plugin name is the empty string. Control: nested and
flat `script` shapes both execute (EXIT=0, marker emitted), so the stack path itself works.

Locations: `examples/full-stack.yml:43,60`, `examples/applications.yml:37,49,61`,
`examples/service-orchestration.yml:37,49`, `USAGE.md:396`, `docs/30:91,351,387`,
`docs/31:305`, `docs/40:169,332,348`, plus two
`claude-plugin/skills/dva/assets/templates/*.yml`.

## Why this is the harmful direction

Earlier findings in this run were dismissed as non-gaps because they ran the safe way:
`dva validate` rejects, runtime tolerates. Runtime leniency behind a red gate harms nobody.

**This is the inverse.** The gate is green and the runtime is red, on files shipped as
copy-paste starting points. `dva validate`'s entire value is telling a user their config will
run; here it certifies a config that cannot.

TASK-010 hardened `schema.json` with `runners.additionalProperties: false` over a 16-name
allowlist that **includes `native` and `docker`** — so the schema now actively blesses the
broken shape. Tightening it to reject would silently pre-commit Option B.

## Why this is blocked, not just fixed

The remediation is decision-dependent and the decision is TASK-017:

- **Option A** (`runners.<name>` means the plugin): the 12 locations are already right; the
  fix is to make the backend real. For `docker` that means routing to the registered plugin;
  for `native` it means **writing a lifecycle plugin that does not exist**. That is new
  feature work, not gap remediation.
- **Option B** (reject `docker`/`native` on the stack path): the 12 locations must be
  rewritten toward `applications:` or the nested `process:`/`script:` shapes, and
  `schema.json` must reject the shape so validate fails loudly instead of lying.

Guessing costs either 12 doc rewrites or a discarded plugin. Recorded as blocked.

## Bearing on TASK-017

This is why TASK-017 was raised **P2 -> P1**: it is not a tidiness question, it is what makes
three shipped examples unrunnable. It also strengthens **Option A** — the product's entire
documented surface (12 locations plus `patterns.md`'s migration map) says this shape is
intended. Weigh against: the runner structs are application-shaped (`dir`/`build`/`run`), and
Option A for `native` requires building a whole plugin.

## Full runnability sweep of the example corpus (orchestrator, 2026-07-17)

The whole corpus was swept statically (every `stack.<entry>`'s resolved runner vs. the real
registry) and spot-checked against runtime. Scope: 18 files — all of `examples/*.yml` plus
`claude-plugin/skills/dva/assets/templates/*.yml`.

| Result | Count |
| --- | --- |
| CONFIG-BROKEN entries | **10** |
| CLEAN entries | 19 |
| UNKNOWN / unresolvable by inspection | 0 |

All 10 broken entries are the **same single shape** — `default_runner: native`. No other
broken runner shape exists anywhere in the corpus; notably no example uses `runners.docker` as
a `default_runner`, so `native` is the entire blast radius:

```
examples/applications.yml            api, worker, web
examples/full-stack.yml              web, frontend
examples/service-orchestration.yml   api, worker
claude-plugin/.../migrate-modes-to-plans.yml   app
claude-plugin/.../root-devbox-plan.yml         api, web
```

Static prediction confirmed against runtime on three independently chosen entries — every one
`validate=0`, `stack up=1`, signature `unknown lifecycle plugin ""`:

```
examples/applications.yml [api]                : validate=0  stack-up=1  ✅ predicted
examples/service-orchestration.yml [worker]    : validate=0  stack-up=1  ✅ predicted
claude-plugin/.../root-devbox-plan.yml [web]   : validate=0  stack-up=1  ✅ predicted
```

The 19 CLEAN entries are the positive control: the sweep does resolve real plugins, so "10
broken" is a measurement, not a scan that matched everything or nothing.

This narrows the fix: whichever option TASK-017 picks, the change is mechanical and touches
**5 files / 10 entries**, and `native` is the only name that needs a decision. `docker` is
implicated in the docs but not in any shipped example.

## Note on why the example suite missed this

The suite's check is "all 18 examples validate". Schema validity is not runnability, so a
green suite is exactly what masked a broken example. Whichever option wins, the suite should
gain a check that exercises examples past validate.

## Completion Criteria

- [ ] TASK-017 is decided | verify: `human — blocked on the runners.docker/native semantics decision`
- [ ] `examples/full-stack.yml` runs past plugin resolution, or no longer uses the shape | verify: `human — after TASK-017, cp examples/full-stack.yml to a temp dir and confirm dva stack up web does not fail with unknown lifecycle plugin ""`
- [ ] validate and runtime agree on the shape in both directions | verify: `human — the shape must either validate-and-run, or fail validate; never validate-then-fail`

## References

- [017-runners-docker-native-semantics.md](./017-runners-docker-native-semantics.md) — the blocking decision
- [024-patterns-md-recommends-broken-migration.md](./024-patterns-md-recommends-broken-migration.md) — same root cause, docs surface
- [009-fix-runners-plugin-resolution.md](../_archive/009-fix-runners-plugin-resolution.md) — why the shape resolves to ""
- [010-schema-runner-allowlist.md](../_archive/010-schema-runner-allowlist.md) — the schema that blesses it
