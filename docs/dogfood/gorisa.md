# gorisa-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (23,111 bytes, 673줄)
- 사용 섹션: `version`, `env_file`(files 객체 형식), `stack`(core-compose + sigdock-native + rails-native), `plans`(5개), `default_plan`, `checks`, `suggestion_ignore`(한국어 주석 포함, 잘 분류됨), `interaction`(대규모, up before 훅 포함), `provision`(4 프로파일), `subprojects`(3개, exclude_tags), `endpoints`
- 미사용: `environments`, `sites` — plans에 environment/site 미지정
- `dva validate`: **통과** (warn 0 — 8개 중 유일하게 완전 무경고)

## 문제점
- 스키마 위반 없음. 다만 잔재·드리프트:
- **제거된 명령 참조 잔존**: `suggestion_ignore` 주석 "dva app up 으로 대응"(L158 인근, `run-rails`/`stop-sigdock` 항목)과 `provision.default` 마지막 note "Rails: run 'dva app up --dev'"(L596) — `dva app`은 docs/43에서 완전 제거된 명령이라 사용자를 존재하지 않는 명령으로 안내한다.
- `stack.*.tags` (core-compose L22, sigdock-native L50, rails-native L65): docs/42 §11-1이 "축소 또는 선택적 메타데이터화" 대상으로 지정한 legacy 필드. `subprojects.*.exclude_tags`가 이 태그를 소비하므로 즉시 제거는 불가 — 의존 관계 확인 후 정리 대상.
- `provision.dev-full-setup`이 `dva up local-infra`를 내부 호출(L537)하고 `tmp/.setup-state` 파일로 멱등성 수제 구현(L560-577): provision 스텝이 사실상 상태 머신 — dva가 지원하지 않는 "1회성 스텝" 요구를 우회한 것.

## dva 개선 힌트
- **`${VAR:-default}` 확장기 버그의 1차 증거지** (L290-297 주석): 변수가 설정돼 있으면 `${POSTGRES_USER:-gorisa}`가 `gorisa:-gorisa}`로 오염됨(dva f3c7f47에서 검증, `shell: true`·`$$`·`\$`로도 회피 불가). 다른 devbox들(matdosa L170, primeno1 L242 등)은 여전히 이 형식을 쓰고 있어 잠복 중 — dva 측 수정이 최우선.
- "subcommands-only 네임스페이스가 bare 호출 시 조용히 exit 0" 문제(L282-287 주석)와 그 해법(주 동작을 부모로 승격)이 여기서 컨벤션화됨 — dva가 bare 네임스페이스 호출을 경고하거나 자동 help 출력하면 근본 해결.
- provision 스텝의 멱등성/1회성(`once:` 마커)과 wait-for-healthy 프리미티브 수요가 dev-full-setup에서 뚜렷하다(수제 sleep 루프 L538-550).

## 마이그레이션 난이도
**중** — 스키마는 통과하지만 `dva app` 잔재 문구 정리, stack tags → subprojects exclude_tags 의존 해소, environments/sites 축 도입 여부 결정이 남아 있고, `${VAR:-default}` 버그가 고쳐지면 방어적 주석/우회를 걷어내는 후속 작업이 필요하다.

## 적용 결과 (2026-09-05)

### 변경 항목
- [x] 제거된 `dva app up/stop` 문구 4곳 교체 — `suggestion_ignore` 주석 L158/L164/L165(`dva up local-dev`, `dva stop local-dev`), `provision.default` note L596(`dva up local-dev`). 파일 내 `dva app` 잔존 0건.
- [ ] `stack.*.tags` 제거 — **유지**. 소비처 확인 결과:
  - `subprojects.*.exclude_tags`는 부모 stack 태그를 읽지 않는다. `internal/cli/run.go:149`·`list.go:82`가 `subCfg.FilterInteractions(sub.ExcludeTags)`로 **하위 프로젝트 자신의 interaction.tags**를 거르고, `manifest.go:551`이 값을 그대로 노출할 뿐이다.
  - 그러나 `stack.<entry>.tags`는 죽은 필드가 아니다. `dva up/down/stop --tags/--exclude-tags`(`internal/cli/compose.go` parseDvaFlags → `orchestrator.filterByTags`, `validateDeclaredTags`)가 entry.Tags를 소비한다. "아무것도 소비하지 않으면 제거" 조건 미충족이라 그대로 둠.
  - 따라서 보고서의 "exclude_tags가 이 태그를 소비" 문장은 부정확. 실제 의존은 CLI 태그 셀렉터 쪽.
- [ ] `provision.dev-full-setup` 수제 멱등성(`tmp/.setup-state` 마커, L560-577) — 지시대로 미수정. 구조: step_4_build / step_5_db 마커를 grep으로 확인 후 건너뛰고, `make dev-full-setup-reset`이 마커를 지운다. `dva up local-infra`를 provision 스텝 안에서 재귀 호출(L537)하며 sleep 루프로 healthy 대기(L538-550).

### `${VAR:-default}` 위치
- 없음. L289-296 주석이 그 형식을 의도적으로 피하고 `${POSTGRES_USER}` 평문만 사용(L297). TASK-303 수정 후에도 이 파일은 변경 불필요.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 0)
```

### 발견된 dva 개선점
- **`exclude_tags` 의미 문서화 부족**: 이름이 부모 stack 태그를 거르는 것처럼 읽히지만 실제로는 하위 프로젝트의 interaction/compose service 태그 필터다. 보고서 작성자(이전 분석)도 오독했으므로 docs/USAGE에 한 줄 명시 필요.
- provision `once:`/wait-for-healthy 프리미티브 수요(기존 힌트 재확인). provision 스텝 내부의 `dva up` 재귀 호출은 lifecycle 동사를 provision에서 부르는 패턴이라 canonical shape 기준으로도 경계선 — 별도 결정 필요.

## CLI 잔재 정리 (2026-09-05)
- .make/compose.mk:68, .make/dev-full.mk:35 bare `dva logs` → `dva logs local-infra`. `dva logs $(SERVICE)`(서비스 passthrough)는 문서화된 유효 형태라 유지.
- CLAUDE.md:236 (AGENTS.md 심링크) "`dva clean` is DVA's reserved built-in" → "`dva clean` no longer exists — teardown is `dva down <plan> --volumes`/`--purge`, clean interaction 미선언"
- 보류 0.

## TASK-303 반영 후 재검증 (2026-09-05, dva d7636a3)

- `interaction.db.command`를 `${POSTGRES_USER:-gorisa} / ${POSTGRES_DB:-gorisa_rails_development}` (.env.example 기본값)로 되돌리고 L289-296 방어 주석을 2줄로 축약. validate exit 0.
- (결정 반영) bare `dva logs` → `dva logs local-infra`(compose.mk) / `dva logs docker-core`(dev-full.mk).
