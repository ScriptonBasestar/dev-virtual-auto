---
id: TASK-023
title: "Stale inverted OS precedence chain remains in docs/31 and docs/40"
type: docs
priority: P2
status: done
archived-at: 2026-07-16T22:20:00+09:00
verified-at: 2026-07-16T22:20:00+09:00
verification-summary: >-
  Verified by orchestrator REPO-WIDE, not against a named-file list: no 'OS < env_file'
  chain remains anywhere in docs/, USAGE.md, README.md, CLAUDE.md or schema.json; no
  numbered list places OS at the bottom; docs/31 and docs/40 now state OS highest; runtime
  re-confirmed OS beats a config value. All five sources finally agree.
  Root cause recorded: the original audit enumerated three INSTANCES of the inverted chain
  and TASK-012's verify binding grepped exactly those same three files, so it passed green
  while two more files kept the identical false claim. A criterion scoped to the instances
  an audit happened to find cannot detect the ones it missed - hence the repo-wide greps here.
effort: XS
created-at: 2026-07-16T22:15:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: convergence-check
source-severity: MEDIUM
---

# Task 023: Stale OS Chain In docs/31 And docs/40

## Summary

TASK-012 corrected the inverted OS precedence chain in three sources. Two more carry the
identical false claim and were never fixed: `docs/31:90` and `docs/40:442`. `docs/31` was
not touched by **any** of the 16 commits in this run.

## Evidence

Repo-wide grep — the stale chain still present:

```
docs/31-execution-plan-resolution.md:90:OS < env_file < global vars < environment vars < site vars < plan vars < CLI vars
docs/40-declarative-stack-and-plans.md:442:OS < env_file < global vars < environment vars < site vars < plan vars < CLI vars
```

Both also carry a numbered list placing `OS 환경 변수` at position **1** (lowest) and
`CLI 일회성 override` at **7** (highest) — `docs/31:79-85`, `docs/40:432-438`.

Runtime proves them false: `FOO=from_OS dva build` → `FOO=from_OS` — OS wins. This
contradicts the now-corrected `USAGE.md:488`, `docs/30:330` and `schema.json:519`
("An OS environment variable always wins").

Aggravating: `docs/30:325` states *"vars runtime 우선순위는 31-execution-plan-resolution.md를
따릅니다"* — it **delegates authority** for this chain to `docs/31`, then states the opposite
order itself. A reader following `docs/30`'s own pointer lands on the wrong answer.

## Why this was missed (root cause — worth recording)

The original audit's contradiction table enumerated **three instances** of the inverted
chain and treated that as the finding. It never grepped repo-wide for the *class*. TASK-012's
verify binding then inherited exactly that scope:

```
! grep -rnE 'OS *<' USAGE.md docs/30-config-merge-semantics.md internal/config/schema.json
```

Three files — the same three. So the criterion passed green while two other files kept the
identical false claim. A verification that only checks the instances the audit happened to
find cannot detect the ones it missed. The criteria below grep the whole docs tree instead.

## Out Of Scope

- Any Go behavior change. The code is authoritative and correct.

## Completion Criteria

- [x] No stale chain anywhere in the repo, not just in named files | verify: `! grep -rn 'OS < env_file' docs/ USAGE.md README.md CLAUDE.md internal/config/schema.json`
- [x] No numbered list places OS at the bottom | verify: `! grep -rn -A1 '^1\. OS 환경 변수' docs/`
- [x] Both files state OS highest, consistent with the other sources | verify: `grep -c 'CLI vars < OS' docs/31-execution-plan-resolution.md docs/40-declarative-stack-and-plans.md`
- [x] Runtime still agrees | verify: `cd "$(mktemp -d)" && printf 'version: "0.1.44"\nvars:\n  P: from-config\ninteraction:\n  s:\n    description: p\n    script: '"'"'echo "R=[$P]"'"'"'\n' > dva.yml && P=from-os "$OLDPWD/bin/dva" run s 2>&1 | grep -q 'R=\[from-os\]'`

## References

- [012-fix-env-precedence-docs.md](../_archive/012-fix-env-precedence-docs.md) — fixed only three of five sources
- `convergence.md` (gap-analysis working set, 2026-07-16, untracked) — found by the convergence check
