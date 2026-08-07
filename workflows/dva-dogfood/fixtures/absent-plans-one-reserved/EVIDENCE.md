# Fixture case-derivation evidence

**When:** 2026-08-08T07:55:14.520675+09:00
**Fixture:** `workflows/dva-dogfood/fixtures/absent-plans-one-reserved`
**DVA:** `/Users/archmagece/go/bin/dva` (`dva version 0.1.44 commit: dfb78d0 build date: 2026-08-07T16:29:55`)
**Rules:** `workflows/dva-dogfood/ref-evaluation.md` (post-edit installed-binary derivation)

## Fixture shape
| property | value |
|----------|-------|
| stack | present (`fixture-compose`) |
| plans | **absent** |
| applications | absent (not a live command family on this binary) |
| reserved interactions | ['up'] |
| other interactions | ['fixture-ping'] |

## Live command-family sections (installed DVA)
['stack', 'plans'] — `dva app` rc=1 (unknown → applications not live)

## Derived case_ids (stage-10 style)
```text
config_schema
lifecycle_boundary:up
absent_section_route:plans
no_change
```

## AC checks
| AC | expected | measured | ok |
|----|----------|----------|-----|
| TASK-082 absent live section scored | `absent_section_route:plans` in case_ids | ['absent_section_route:plans'] | True |
| TASK-123 exactly one reserved-name case | one `lifecycle_boundary:*` | ['lifecycle_boundary:up'] | True |
| applications not invented | no `absent_section_route:applications` | [] | True |

## validate
```
rc=0
[warn] semantic: ℹ No 'plans' defined — consider adding execution plans for 'dva up <name>' support
  Migration guide: https://github.com/ScriptonBasestar/dva/blob/master/docs/42-migration-and-compatibility.md#11-migration
  Hint: plans combine stack entries, environments, and sites into named execution targets
  Example:
    plans:
      local-dev:
        environment: dev
        site: local
        entries:
          - name: <stack-entry>
            runner: <runner-name>
            order: 10
✅ dva.yml is valid
```

## Safety
No `up`/`down`/`stop`/`restart`/`provision` run against this fixture.
