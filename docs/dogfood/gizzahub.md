# gizzahub-devbox dva 적용 분석

## 현황
- 파일: `dva.yml` (23,961 bytes, 799줄) — 최대 규모급
- 사용 섹션: `version`, `env_file`(files 객체 형식), `stack`(compose 3엔트리: 메인 + 격리 E2E 인프라 2), `plans`(9개: infra/local-infra/workflow/monitoring/apps/tools/full + 격리 E2E 2), `default_plan`, `sites`(local), `suggestion_ignore`(약 190항목), `interaction`(대규모 subcommands), `provision`, `endpoints`
- 미사용: `environments`
- `dva validate`: **통과** (semantic warn: `infra`와 `local-infra` plan이 완전 동일)

## 문제점
- 구조 위반 없음. 신규 모델 준수 + 격리 테스트 인프라를 별도 stack 엔트리/plan으로 분리한 좋은 사례.
- plan 중복: `infra` ≡ `local-infra` (L111-127, validate warn) — alias 의도라면 한쪽 제거 또는 주석 필요.
- **stack 선언과 plan services의 드리프트**: `workflow`/`monitoring`/`apps`/`tools`/`full` plan이 `temporal-postgres`, `temporal`, `temporal-ui`, `zookeeper`, `kafka`, `prometheus`, `grafana`, `alertmanager`, `*-exporter` 등을 참조(L129-244)하지만 `stack.compose.runners.compose.services`(L62-89)에는 이들이 선언돼 있지 않다. 검증이 이를 잡지 않으므로 stack의 서비스 목록이 부분 목록으로 부패한 상태.
- `environments` 미사용: plans에 `site: local`만 있고 environment 축이 없다 — dev 전용이라 실해는 없으나 vars 레이어를 쓰지 않는 만큼 `.env` 의존이 커진다.
- `suggestion_ignore` 190항목: config의 1/4. flow-taskchain과 동일한 스케일 문제.

## dva 개선 힌트
- **plan entry `services`가 stack 선언의 services 맵에 없는 이름을 참조해도 validate가 침묵한다** — services 맵을 선언 정본으로 볼지(참조 검증 추가), 아니면 순수 메타데이터로 볼지(문서 명시) 결정이 필요하다. 현 상태는 절반 선언·절반 드리프트.
- 완전 동일 plan 감지 warn은 잘 작동했다(이 프로젝트에서 실증). alias 의도를 표현할 `plans.<name>.alias_of` 같은 경량 수단이 있으면 warn 억제와 의도 기록이 동시에 된다.
- suggestion_ignore 축약 수단 필요(flow-taskchain 리포트와 동일 결론).

## 마이그레이션 난이도
**하** — 구조는 이행 완료. stack services 목록 동기화와 plan 중복 정리는 정합성 청소 수준.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] 중복 plan `infra` 삭제, `local-infra` 유지 (default_plan·README·USAGE가 `local-infra`를 참조, `dva up infra` 참조는 리포지토리 내 0건). validate warn 해소. TASK-307 alias 도입 시 `infra: {alias_of: local-infra}`로 복원 가능.
- [x] stack `compose.runners.compose.services` 드리프트 복구 — plan이 참조하지만 미선언이던 11개 서비스를 추가: `temporal-postgres`, `temporal`, `temporal-ui`, `zookeeper`, `kafka`, `prometheus`, `grafana`, `alertmanager`, `postgres-exporter`, `redis-exporter`, `node-exporter`. 이름은 `compose.yaml`의 `include:` 대상 14개 파일을 파싱해 실제 서비스명·profile과 대조 (plan 참조 서비스 25개 전부 실존 확인). 태그는 compose profile에 맞춰 `workflow`/`obs, monitoring`/`obs, alerting` 부여.
- [x] `suggestion_ignore` 184항목 재검토 — Makefile(+include mk) 타겟 255개와 대조한 결과 stale 0건이라 삭제 항목 없음. 축약은 TASK-309 대기.
- [ ] (보류) `environments` 미사용 — 계획에 명시 없음, 변경하지 않음.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 0)
```

### 보류/예외 항목
- `${VAR:-default}` 사용처: dva.yml 내 없음.

### 발견된 dva 개선점
1. **plan entry `services`가 stack `services` 맵에 없는 이름을 참조해도 validate가 침묵** — 이 프로젝트에서 11개 드리프트 실증 (TASK-308 검증 항목). repro: stack services에 `postgres`만 두고 plan `services: [postgres, nope]` → warn 없음.
2. compose `include:`로 분산된 서비스는 `dva validate`의 compose drift 검출이 include 대상 파일까지 따라가는지 미확인 — 이 프로젝트는 drift warn이 0건인데 루트 `compose.yaml`에는 services 블록이 없다. include 미해석이면 false negative.
