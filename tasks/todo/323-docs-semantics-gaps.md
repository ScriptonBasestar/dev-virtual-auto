---
id: TASK-323
title: "docs: undocumented semantics surfaced by devbox migration"
type: docs
priority: P3
effort: S
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{gorisa,funbricks-postkit,scripton-nd-stack,matdosa,scripton-db-orchestrator}.md"
status: todo
---

# Task 323: 마이그레이션 중 드러난 미문서 의미론

- `subprojects.exclude_tags`는 부모 stack 태그가 아니라 하위 프로젝트 자신의 interaction/compose 태그를 거른다
  (run.go:149, list.go:82). stack entry tags는 `dva up/down/stop --tags/--exclude-tags`가 소비. 분석자 2명이 오독.
- `script_file:`은 exec 방식(shebang + 실행권한 필수, internal/exec/exec.go:230) — 문서/init 노출 없음.
- native runner `env:` 필드 존재 — 예시 부재.
- `suggestion_ignore` 정본 위치(checks 뒤, interaction 앞)가 docs 예시에 없음.
- plan 경로에서 `--env`가 거부됨 — "같은 plan, 다른 env"는 plan을 복제해야 함을 명시(또는 TASK-307에서 해결).
- `dva logs <plan>`이 엔트리 2개 이상이면 이름 지정 요구 — native 엔트리 로그 경로 문서화.
- interaction step에서 `dva down …` 재귀 호출 시 `--dry-run`이 내부 계획을 보여주지 않음 (funbricks-elemhant `dva --dry-run run clean`).

## Completion Criteria

- [ ] 각 항목 docs/USAGE 반영, `make generate` 갱신 | verify: `make generate && git diff --stat`
- (2026-09-05 추가) `endpoints.*.url`/`source`는 `${VAR}`/`${VAR:-default}`를 치환하지 않는다(`dva show`에 원문 출력, internal/cli/endpoints.go는 ep.URL을 그대로 사용). 문서화하거나 치환을 지원해야 한다 — devbox 3곳(sigdock-pass, matdosa, notifire)이 이 때문에 리터럴 포트를 중복 선언 중.
- (2026-09-05 추가) composition plan(`composes:`)에 `environment:`/`site:`를 두면 validate ERROR — 자식 plan이 각자 갖는다는 규칙이 §2 문서에 없음 (familybook `hybrid` 전환 중 발견).
