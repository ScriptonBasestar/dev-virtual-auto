# flow-taskchain-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (22,353 bytes, 733줄) — 8개 프로젝트 중 최대 규모
- 사용 섹션: `version`, `env_file`(files 객체 형식), `stack`(5 엔트리: compose 1 + native 4), `plans`(local-infra/local-dev/local-full), `default_plan`, `environments`, `sites`, `checks`, `suggestion_ignore`(약 120항목), `interaction`(다층 subcommands), `provision`, `endpoints`
- `dva validate`: **통과** (Makefile 타겟 매핑 suggestion warn 4건만)

## 문제점
- 구조 위반 없음. 새 stack/plans 모델을 가장 충실히 따르는 config다.
- `stack.infra.runners.compose.services.*.tags` (L29-55): compose 서비스 선택은 plan 책임(docs/40 §5)이지만, 여기서는 태그 메타데이터로만 쓰고 실제 선택은 `plans.*.entries[].services`가 수행하므로 허용 범위("선택적 메타데이터화", docs/42 §11-1) 안이다.
- `sites.local.entry_overrides` (L207-215): native→native 오버라이드로 default_runner와 동일해 사실상 no-op — 삭제 가능한 군더더기.
- `interaction.clean` (L653): 주석에 "DVA clean built-in removed in 0.1.44+"라고 정확히 기록하고 일반 interaction으로 재정의 — 올바른 마이그레이션 사례.

## dva 개선 힌트
- `suggestion_ignore`가 120여 줄로 config의 1/4을 차지한다. Makefile이 큰 devbox에서 suggestion 소음을 개별 나열로 억제해야 하는 구조 — `suggestion_ignore`에 파일 분리(`suggestion_ignore_file`)나 "Make-only 네임스페이스 일괄 선언" 같은 축약 수단이 필요하다는 신호.
- `stack.<entry>.health_checks`(L67-122)를 4개 native 엔트리 모두 사용 — 문서(docs/43 §16)는 최상위 `health_checks`가 advisory라고만 말하고, stack 엔트리 하위 health_checks의 지위(스키마상 허용되지만 canonical 예시에 없음)를 정식 문서화할 필요가 있다.
- `interaction.db.subcommands.restore`처럼 `${BACKUP_FILE}` 필수 변수를 받는 interaction에 "필수 변수 선언" 기능이 없어 미설정 시 빈 문자열로 실행된다.

## 마이그레이션 난이도
**하** — 이미 신규 구조로 완전히 이행됨. entry_overrides no-op 정리 정도만 남는다.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] `sites.local.entry_overrides` (engine-api/mcp-server/portal-ui/admin-ui native→native, 4개 엔트리 모두 `default_runner: native`와 동일) 삭제 — no-op 정리. 배정 범위는 이 항목만.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 4 — 모두 사전 존재하던 Makefile suggestion: gap-303-status, sigdock-idp-health, validate-dva-config, validate-task-graph)
```

### 보류/예외 항목
- Makefile suggestion warn 4건: 배정 범위 밖(coordinator 지시: entry_overrides 제거만)이라 미처리. 처리 시 `suggestion_ignore` 4줄 추가 또는 interaction 매핑.
- `${VAR:-default}` 사용처 확인하지 않음(범위 밖).

### 발견된 dva 개선점
1. no-op `entry_overrides` semantic warn 후보 (postkit과 동일 패턴 실증).

## CLI 잔재 정리 (2026-09-05)
- CLAUDE.md:79 (AGENTS.md 심링크) `dva clean` → `dva run clean` (clean interaction = `make infra-clean`), 금지 문구의 `dva clean -v` → `dva down --volumes`/`--purge`
- 보류 0.

## docs/57 §4 재점검 (2026-09-05, TASK-310 가이드 기준)

- §4-3 해당(미적용, 소유자 결정): `engine-api`/`mcp-server`(`make run`)·`admin-ui`(`make dev`)가 자식 디렉토리 Makefile 타겟을 루트가 기억한다.
  자식이 타겟을 바꾸면 루트는 validate를 통과한 채 실행에서 깨진다. 해법은 자식 저장소에 `dva.yml`을 두고 §2 `subprojects` import로 전환하는 것인데
  devbox 밖(자식 저장소) 변경이라 이 dogfood 범위에서는 기록만 한다. 루트 Makefile 타겟을 가리키는 경우(flow-knowchain, gorisa, postkit)는 루트가 소유자이므로 해당 없음.
