---
id: TASK-266
title: "Deprecate then reject the inert interaction env_file field"
type: chore
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-02T20:30:00+09:00
source: "TASK-265 decision record, section 4"
scope: "interaction/subcommand env_file deprecation warning, config migrate guidance, tracked examples and docs, path-scoped schema rejection, rollback"
status: todo
depends-on: [TASK-265]
---

# Task 266: deprecate and reject interaction env_file

## Summary

Implement the two-release rejection contract frozen in
[TASK-265](265-decide-interaction-env-file-contract.md) §4. Stage A announces the deprecation in 0.1.48
without changing runtime behavior; Stage B removes the field from the schema in 0.1.49.

## Problem

`interaction.*.env_file` and its nested `subcommands.*.env_file` are schema-valid, decoded, merged and
carried through subproject import, but no runner or CLI reads them. A configuration can declare
`required: true` and run with nothing enforced. TASK-265 rejected both silent support and immediate
removal, so the field has to be announced in one release and rejected in the next.

## Constraint

Do not add runtime file I/O at either stage. Stage B must not start before 0.1.48 has shipped, and the
guidance for the removed key must be path-scoped — `removedSchemaKeys` is keyed by property name alone and
would attach removal guidance to the still-valid root `env_file`, breaking both the user-facing error and
the generator-corpus test.

`0.1.48` and `0.1.49` name the two consecutive minors after the current `0.1.47`. If the real tags differ,
change the warning text, the CHANGELOG entry and this card together — the release named to the user and
the release that ships must not disagree.

## Completion Criteria

### Stage A — 0.1.48 (announce)

- [x] Add one semantic warning per declaring node through the existing `eachInteractionNode` walker, carrying the exact text frozen in TASK-265 §4 and reported only through the existing `[warn] semantic:` channel and `semantic` JSON category of `dva config validate`; no new category, flag, route or file read | verify: `go test ./internal/config ./internal/cli -count=1`
- [x] Keep exit codes unchanged — default `dva config validate` stays 0 and `--strict` fails only through the pre-existing semantic-warning rule; `dva run`, lifecycle verbs, `doctor` and `show` stay silent | verify: `go test ./internal/cli -count=1`
- [x] Add a `dva config migrate` step that reports the declaration in `MigrationReport.Blocked` with the frozen text and never rewrites the file | verify: `go test ./internal/config -count=1`
- [x] Remove both interaction declarations from `examples/env-file-priority.yml` and the "Command-specific env_file" entry from `examples/README.md`; add the USAGE.md line stating `env_file` is not an interaction field and a CHANGELOG `### Deprecated` entry naming the rejecting release and both replacements | verify: `make doc-check`

### Stage B — 0.1.49 (reject), only after 0.1.48 ships

**릴리스 게이트로 보류 중 (2026-09-03).** 현재 태그는 `v0.1.47`이고
`internal/config/version.go`의 `Version`도 `"0.1.47"`입니다. 카드의 Constraint가
"Stage B must not start before 0.1.48 has shipped"라고 못박고 있으므로 착수하지
않았습니다. 0.1.48이 나가면 이 카드를 그대로 이어서 처리하면 됩니다.

게이트 확인 방법 (착수 전 두 가지가 모두 참이어야 한다):

```bash
git tag --list 'v0.1.48*'            # 비어 있으면 아직 아님
/usr/bin/grep -n 'Version =' internal/config/version.go   # 0.1.47이면 아직 아님
```

둘 중 하나라도 아니면 Stage B를 시작하지 않는다. Constraint가 말하듯, 경고 문구가 이름 댄
릴리스와 실제로 나가는 릴리스가 어긋나면 사용자에게 한 약속이 깨진다.

Stage A는 이미 완료됐고 (`c6aa64b`), 그 결과
[TASK-276](276-correct-example-corpus-and-close-md-yaml-gap.md)은 더 이상 이 카드를 기다리지
않는다 — 두 카드가 공유하던 `examples/` 파일을 Stage A가 정리했으므로 2026-09-03에 해당
`depends-on` 선언을 해제했다. Stage B는 스키마와 Go 타입만 건드리므로 `examples/`에서 다시
충돌하지 않는다.

Stage B가 물려받는 별건: TASK-245 §2-4의 `#/definitions/env_file_plain` 정리.
`env_file_plain`은 `interaction_command`가 `sops_source`를 거부하도록 두려고 만든
정의인데, Stage B가 `interaction_command.properties.env_file`을 통째로 지우면
참조가 사라지므로 그때 함께 삭제합니다.


- [ ] Delete `definitions.interaction_command.properties.env_file` from the schema plus `InteractionCommand.EnvFile`, its `UnmarshalYAML` twin and assignment, and the `EnvFile` branch in `mergeInteractionCommand` | verify: `go test ./internal/config ./internal/cli ./internal/runner -count=1`
- [ ] Add a path-scoped removed-key map beside `removedRootKeys`, consulted only when the schema error field names an interaction node, carrying the frozen guidance; the root `env_file` error and the generator corpus stay untouched | verify: `go test ./internal/config -count=1`
- [ ] Drop the Stage A semantic warning, which schema rejection makes unreachable, and keep the `config migrate` blocked line as the remaining guidance path for a rejected config | verify: `go test ./internal/config ./internal/cli -count=1`
- [ ] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No interaction-level `env_file` support, precedence layer, inheritance or file I/O at any stage.
- No change to top-level `env_file` behavior, `dva doctor` env checks or the TASK-247 precedence table.
- No rewriting of user configurations; `dva config migrate` reports and never edits this declaration.
