---
id: TASK-013
title: "Fix docs/40 clause mandating vars for environments (fails validation)"
type: docs
priority: P1
status: todo
effort: XS
created-at: 2026-07-16T09:19:12Z
source-run-id: 20260716T091912Z-73dc094
source-unified: tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md
source-unified-sha256: e62018b67e3ac63021f034d888b8b6f64a2c008c299f3bfa82e5d2b2e94ef1b2
source-gap: G5
source-severity: HIGH
repo-snapshot: "dev-virtual-auto@73dc094 (master, clean)"
---

# Task 013: Fix docs/40 vars Field-Name Clause

## Summary

`docs/40-declarative-stack-and-plans.md:108` normatively instructs users to name env
var blocks `vars` rather than `environment`. Following it produces a config that
**fails schema validation**. The clause is stale — it is contradicted by its own
example 11 lines below, by every other doc, by all examples, and by the code.

## Evidence

`docs/40:108`:

> 환경 변수 블록의 공통 필드명은 `environment`가 아니라 `vars`를 사용합니다.
> (the common field name is `vars`, **not** `environment`)

Schema reality — the two sections use **different** field names, both with
`additionalProperties: false` (`internal/config/schema.json:1122-1134`):

| Section | Mandated field | `vars` accepted? |
| ------- | -------------- | ---------------- |
| `environments.<name>` | `environment` | **No** |
| `sites.<name>` | `vars` | Yes |

Following the clause literally:

```
$ ./bin/dva validate
ERROR: schema validation failed in dva.yml:
  - environments.dev: Additional property vars is not allowed
```

The schema-mandated form (`environments.dev.environment`) validates clean.

The clause is **self-contradicted by its own example** at `docs/40:119-121`, which uses
`environments.dev.environment:` (alongside `sites.local.vars:`). It also contradicts the
deliberate alignment in commits `67b0d15` ("docs(examples): align environments.environment
field name") and `2eab2fe` ("docs: use environments.environment in plan documentation") —
this line was missed by that work.

Everything else already agrees on `environment`: `docs/31:310-313`, `docs/40:119-122,357-360`,
`USAGE.md:358-361`, all 6 examples using the section, `internal/config/config.go:30`,
`internal/config/schema.json:1122-1134`.

## Suggested Approach

Rewrite `docs/40:106-109` (§3-5) to describe the shipped reality: `environments.<name>`
uses `environment:`, `sites.<name>` uses `vars:`. The section's stated rationale (avoiding
`environments`/`environment` collision) is a design argument that was **not** adopted;
either drop it or mark it explicitly as a rejected alternative rather than an instruction.

## Out Of Scope

- Renaming any schema field or changing `additionalProperties: false`. The naming is
  settled; only the doc is wrong.

## Completion Criteria

- [ ] `docs/40` §3-5 no longer instructs using `vars` in place of `environment` for `environments` | verify: `! grep -n '환경 변수 블록의 공통 필드명은' docs/40-declarative-stack-and-plans.md`
- [ ] §3-5 prose matches its own example and the schema | verify: `grep -n -A18 '### 3-5' docs/40-declarative-stack-and-plans.md`
- [ ] A config written from the corrected §3-5 validates | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\nenvironments:\n  dev:\n    environment:\n      APP_ENV: dev\n' > dva.yml && "$OLDPWD/bin/dva" validate`

## Dependencies

None. Doc-only.

## References

- [unified.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/unified.md) — G5
- [doc-to-code.md](../../tmp/gap-analysis-runs/20260716T091912Z-73dc094/doc-to-code.md) — L2
