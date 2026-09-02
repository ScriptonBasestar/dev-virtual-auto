---
id: TASK-271
title: "Close USAGE.md config-section and command-surface gaps"
type: bug
priority: P2
effort: S
exec-tier: standard
created-at: 2026-09-03T09:40:00+09:00
source: "USAGE.md audit during TASK-259 review repair"
scope: "USAGE.md 설정 섹션 레퍼런스 table, completion command documentation, and the doc-check binding that keeps them honest"
status: todo
depends-on: []
---

# Task 271: close USAGE.md config-section and command-surface gaps

## Summary

`USAGE.md`'s 설정 섹션 레퍼런스 table claims to list the canonical section order "validate에서
검증", but it is missing two of the schema's root keys. Separately, `completion` ships as a root
command and appears in `USAGE.md` only inside a reserved-word list, never as a documented command.

## Problem

Measured against `internal/config/schema.json` at `bd4267b`:

1. The schema declares 22 root properties. The 설정 섹션 레퍼런스 table lists 20. The two absent
   from the table are **`environment`** and **`suggestion_ignore`**.

   `environment` is the more serious omission: it is the lowest-precedence layer of the documented
   env chain (`environment:` < `env_file` < OS env), applied first by `loadEnv`
   (`internal/cli/root.go`) and then overwritten by `env_file`. A reader consulting the table for
   the canonical section list is told the key does not exist. It is also one character from
   `environments` (plural), which *is* in the table with a different meaning — so the omission
   reads as a deliberate spelling correction rather than a gap.

2. `completion` is a root command. `USAGE.md` mentions the token once, in the reserved-word block,
   with no description, no synopsis, and no shell-setup instructions. A reader cannot learn from
   `USAGE.md` that the command exists or what it emits.

Both are documentation-only defects: no schema, loader, or command behavior is wrong.

## Completion Criteria

- [ ] The 설정 섹션 레퍼런스 table lists every root property declared by `internal/config/schema.json`, in the canonical order `validate` checks, with `environment` and `suggestion_ignore` each carrying a one-line description that distinguishes `environment` from `environments` | verify: `python3 -c "import json,sys,re;b=chr(96);p=chr(124);s=set(json.load(open('internal/config/schema.json'))['properties']);t=set(re.findall('^\\\\'+p+'\\\\s*'+b+'([a-z_]+)'+b, open('USAGE.md').read(), re.M));m=sorted(s-t);print('missing:',m);sys.exit(1 if m else 0)"`
- [ ] `completion` is documented as a command with its synopsis and at least one shell-setup example, reachable from the same place the other root commands are documented | verify: human — the section must name the command, its supported shells, and how to install the output
- [ ] Repository gates pass | verify: `make lint && make test && make doc-check && make commit-check`

## Non-goals

- No schema change, no new root key, and no change to env precedence — `environment` already works
  as documented in the env-precedence prose; only the table is wrong.
- No change to `completion`'s behavior or shell coverage; this card documents what ships.
- No renaming of `environment`/`environments` — [TASK-261](261-decide-vnext-vocabulary-and-migration.md)
  owns vocabulary decisions, and this card must not pre-empt it.
- No cobra long-help or example-field work — [TASK-268](268-add-long-help-to-concept-commands.md)
  and [TASK-269](269-promote-help-examples-to-example-fields.md) own the in-binary help surface.
