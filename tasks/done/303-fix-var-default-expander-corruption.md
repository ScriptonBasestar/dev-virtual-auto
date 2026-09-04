---
id: TASK-303
title: "Fix ${VAR:-default} expander corruption"
type: bug
priority: P0
effort: S
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "docs/dogfood/gorisa.md (dogfood sweep 2026-09-05)"
status: done
---

# Task 303: `${VAR:-default}` 확장기 오염 버그 수정

## Summary

gorisa-devbox에서 검증됨: `${VAR:-default}` 형태의 변수 확장이 값을 오염시킴.
matdosa, primeno1, funbricks-elemhant에도 같은 패턴이 잠복.

## Evidence

- docs/dogfood/gorisa.md 의 재현 근거 참조 (dva 0.1.48).
- 재현 픽스처를 먼저 작성해 실패를 확인한 뒤 수정할 것.

## Completion Criteria

- [x] `${VAR:-default}` (및 `${VAR-default}`, 중첩/인접 케이스) 재현 테스트 추가, 수정 전 실패 확인 | verify: `go test ./internal/config/ -run 'TestInterpolateDefaultSyntax|TestMergeVarsDefaultSyntax'`
- [x] 확장기 수정 후 전체 테스트 통과 | verify: `make test`
- [x] gorisa dva.yml에 대해 `dva validate`/`dva show` 산출 값이 셸 의미론과 일치함을 확인 | verify: human — 실행 출력 첨부

## Completion Evidence (2026-09-05)

**Root cause.** `internal/config/environment.go` expanded references with the single regex
`\$\{?([a-zA-Z_][a-zA-Z0-9_]*)\}?`. The closing brace was optional, so `${POSTGRES_USER:-gorisa}`
matched only `${POSTGRES_USER` and the replacement left `:-gorisa}` in place. A *set* variable
therefore produced `gorisa:-gorisa}`; an unset one left the whole reference literal, which the
shell then expanded correctly — exactly the "hides on a clean machine, bites once .env exists"
pattern docs/dogfood/gorisa.md describes.

**Fix.** Replaced the regex with a small scanner (`interpolateWith` / `parseBracedRef`) that
handles `$VAR`, `${VAR}`, `${VAR:-default}` (unset or empty → default) and `${VAR-default}`
(unset only → default), interpolates the default text itself (so `${A:-${B}:5432}` nests), and
leaves malformed or unsupported forms (`${A`, `${A:+x}`) literally. `warnUnresolvedEnvVars`
now uses `findVarRefs`, the read-only counterpart; `warnSuspiciousEnvPatterns` no longer flags
`:-`/`-` and still flags `:+`, `:=`, `:?`, `$#`. USAGE.md gained a "변수 참조 문법" table.

**Failure before fix** (`go test ./internal/config/ -run TestInterpolateDefaultSyntax`, pre-fix):

```
Interpolate("${SET:-fallback}") = "gorisa:-fallback}", want "gorisa"
Interpolate("${EMPTY:-fallback}") = ":-fallback}", want "fallback"
Interpolate("${DVA_TEST_UNSET_303:-fallback}") = "${DVA_TEST_UNSET_303:-fallback}", want "fallback"
```

**Human criterion — binary check.** Scratch `dva.yml` with `environment.PSQL_USER:
"${POSTGRES_USER:-fallback}"` (POSTGRES_USER=gorisa) and
`DB_URL: "postgres://${POSTGRES_USER:-u}@${UNSET:-${DB_HOST}}:5432/app"`, run through
`dva run echo-env`:

```
installed 0.1.48:  PSQL_USER=gorisa:-fallback} DB_URL=postgres://gorisa:-u}@localhost:5432/app
this branch:       PSQL_USER=gorisa           DB_URL=postgres://gorisa@localhost:5432/app
```

`dva validate` in `~/mydevbox/gorisa-devbox` with the new binary: `✅ dva.yml is valid`. The
gorisa config itself avoids the form (L290-297 comment), so its follow-up is removing that
workaround once this ships.

**Not in scope.** `$$` / `\$` escaping (also noted in the gorisa comment) is unchanged.
