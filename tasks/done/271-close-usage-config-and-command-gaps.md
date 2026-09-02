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
status: done
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

- [x] The 설정 섹션 레퍼런스 table lists every root property declared by `internal/config/schema.json`, in the canonical order `validate` checks, with `environment` and `suggestion_ignore` each carrying a one-line description that distinguishes `environment` from `environments` | verify: `python3 -c "import json,sys,re;b=chr(96);p=chr(124);s=set(json.load(open('internal/config/schema.json'))['properties']);t=set(re.findall('^\\\\'+p+'\\\\s*'+b+'([a-z_]+)'+b, open('USAGE.md').read(), re.M));m=sorted(s-t);print('missing:',m);sys.exit(1 if m else 0)"`
- [x] `completion` is documented as a command with its synopsis and at least one shell-setup example, reachable from the same place the other root commands are documented | verify: human — the section must name the command, its supported shells, and how to install the output
- [x] Repository gates pass | verify: `make lint && make test && make doc-check && make commit-check`

## Non-goals

- No schema change, no new root key, and no change to env precedence — `environment` already works
  as documented in the env-precedence prose; only the table is wrong.
- No change to `completion`'s behavior or shell coverage; this card documents what ships.
- No renaming of `environment`/`environments` — [TASK-261](261-decide-vnext-vocabulary-and-migration.md)
  owns vocabulary decisions, and this card must not pre-empt it.
- No cobra long-help or example-field work — [TASK-268](268-add-long-help-to-concept-commands.md)
  and [TASK-269](269-promote-help-examples-to-example-fields.md) own the in-binary help surface.

## Evidence

- 설정 섹션 레퍼런스 표는 이제 `internal/config/schema.json`의 root property 22개를 모두
  담고, 순서는 `internal/config/validate_warnings.go:20-31`의 `canonicalSectionOrder`와
  정확히 일치한다. 표 헤더가 "정규 섹션 순서 (validate에서 검증)"이라 주장하므로 완결성만으로는
  부족했다 — `environments`/`sites`가 `modes` 뒤에 놓여 캐노니컬 위치(`default_plan` 직후)를
  벗어나 있어 함께 옮겼다. 카드 verify 바인딩은 완결성만 검사하므로 순서는 별도 확인:
  `canonicalSectionOrder`와 표 행 순서를 직접 비교해 일치 확인(22/22).
- `environment` 설명의 우선순위 주장은 `internal/cli/root.go:452-455`
  (`newConfigEnvironmentAt` — `MergeVars(c.Environment)`가 `ApplyEnvFiles`보다 먼저 실행)에서
  확인했다. 복수형 `environments`와의 구분을 같은 행에 명시해 TASK-261의 어휘 결정을
  선점하지 않으면서 오독만 차단한다.
- `suggestion_ignore` 설명은 `internal/config/schema.json`의 해당 property description 및
  `internal/cli/validate.go:519-531`(`matchesSuggestionIgnore`)과 일치한다.
- `completion`은 커맨드 표 행과 `#### completion` 절로 문서화했다. 지원 셸 4종
  (bash/zsh/fish/powershell)은 빌드된 바이너리의 `dva completion --help` 실행으로 확인했으며,
  소스에 `CompletionOptions` 오버라이드가 없어 cobra 기본 집합이다.

## Gate results

- `make lint` — 통과 (go vet 0 issues, gofmt 337 files 0 unformatted)
- `make test` — 통과 (FAIL 0건)
- `make doc-check` — 통과 (doc-check/cilabels/flowcheck OK, flowcheck "24 built-in command(s)")
- `make commit-check` — **baseline 실패 3건**, 이 카드와 무관하다. 위반은 전부 기존 커밋의
  subject 형식(`type(scope): summary`에서 scope 누락)이며 해당 커밋은 이미 master에 푸시되어
  재작성 불가: `a6666c1a`, `6ab9c643`, `095f525b`. 이 카드의 커밋은 scope를 포함한다.

