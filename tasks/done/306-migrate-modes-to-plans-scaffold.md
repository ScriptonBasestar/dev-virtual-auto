---
id: TASK-306
title: "migrate: emit actual plans scaffold YAML for modes"
type: feature
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-05T09:00:00+09:00
source: "docs/dogfood/{primeno1,sigdock-idp,sadawiki}.md"
status: done
---

# Task 306: migrate가 modes → plans 스캐폴드 YAML을 실제 출력

## Summary

현재 migrate는 modes에 대해 "Left for you"만 남긴다(primeno1: modes 6개 전부 수동).
그러나 "stack 선택만 하는 mode"(sigdock-idp)와 "compose_services 목록만 있는
mode"(sadawiki)는 plan으로 기계 변환 가능한 하위 클래스임이 확인됐다.

## Direction

- 기계 변환 가능한 mode 하위 클래스를 식별해 plans 스캐폴드 YAML을 preview로 출력.
- 변환 불가능한 mode는 사유와 함께 명시적으로 남긴다. 파일 자동 수정은 opt-in.
- interaction.clean → down.after 자동 이관도 이 변환기에 포함 검토.

## Completion Criteria

- [x] mode 하위 클래스별(stack 선택형/compose_services형/변환 불가형) 픽스처 테스트 | verify: `make test`
- [x] primeno1 설정에서 migrate preview가 6개 mode 중 기계 변환분의 plans YAML을 출력 | verify: human — 출력 첨부

## Completion Evidence (2026-09-05)

**Implementation.** New pipeline step `MigrateModes` (`internal/config/migrate_modes.go`),
run after `MigrateStackOrder` and before `ScaffoldModes`. A mode whose keys are a subset of
`{description, stack, compose_services, endpoint_tags}` becomes `plans.<same name>`:
`stack` → `entries[].name`; `compose_services` → `entries[<entry>].services` on the single
compose entry among the selected stack (no `stack:` means every stack entry). Converted mode
spans are removed line-exact (the `modes:` key goes when empty), the plans are appended to an
existing `plans:` block or a new one opened where `modes:` was, and `default_mode: X` is
rewritten to `default_plan: X` when X converted and no `default_plan` exists. Each blocked
mode is reported as `modes.<name>: not converted — <reason>` and still gets its
`ScaffoldModes` field targets. Blocked reasons: foreign field (compose_profiles, environment,
build, run, health_checks, provision, applications), zero or several compose entries for
`compose_services`, `plans.<name>` exists, undeclared stack entry, empty selection,
`default_mode` mode while `default_plan` is already set (the tool never picks a default).

**Deliberately not converted.** `interaction.clean` before/after → `interaction.down`:
`clean` was a volume-purging teardown and `down` is not, so a hook moved there fires at a
different moment. Left for the author, as validate's error message already explains.

**Tests.** `internal/config/migrate_modes_test.go` — stack-select (default_mode follows,
result loads with 2 plans / 0 modes), compose_services (no `stack:`, services attach to the
sole compose entry), five unconvertible shapes each with reason while a sibling converts,
default conflict leaves the file untouched, pipeline scaffolds only the remaining modes.
`internal/cli/config_migrate_test.go` no-op test now uses a `compose_profiles` mode.
`make test`, `make lint`, `make doc-check` pass.

**Human criterion (primeno1).** The devbox working tree had already been migrated by its
owner, so the committed `HEAD:dva.yml` (6 stack-select modes, `default_mode: full`, no
plans) was previewed with `bin/dva config migrate <dir>`: rc 0, all 6 modes converted
(`minimal`, `full`, `observability`, `tracing`, `sigdock-local` with 2 entries,
`external-db` with 2 entries) into a new `plans:` block where `modes:` was, and
`default_mode: full → default_plan: full`; `Left for you` empty. The previewed YAML was
saved to a scratch dir and `dva validate` loaded it as plans; the only remaining hard error
is the pre-existing `interaction.clean` hook, which this converter leaves by design.

## Dogfood evidence (2026-09-05 실행)

- 4/4 legacy 프로젝트(sadawiki, signalhub, sigdock-idp, familybook)에서 migrate가 modes를 전혀 변환하지 않았고 전부 수동 전환됨.
- `modes.*.stack` → plan entries는 1:1 매핑(sigdock-idp), `compose_services` 목록형(sadawiki)도 기계 변환 가능했음.
- 최상위 health_checks의 `start`/`start_hint`는 stack 엔트리로 옮길 때 `start`를 버려야 하는데 안내 없음 (flow-agent-mesh).
- 힌트 자체의 오류는 TASK-317로 분리.
