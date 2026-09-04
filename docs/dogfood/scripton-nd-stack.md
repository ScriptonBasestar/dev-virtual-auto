# scripton-nd-stack dva 적용 분석

## 현황
- 파일: `dva.yml` (15,370 bytes, version `0.1.44`)
- 사용 섹션: `environment`, `env_file`, `stack`(compose + native 3개: proxynd/depond/flownd), `plans`(7개), `default_plan`, `environments`, `sites`, `checks`, `suggestion_ignore`, `interaction`, `provision`, `subprojects`(2개), `endpoints`(19개)
- `dva validate` (0.1.48): **✅ valid** — 신모델(stack 선언 + plans + environments + sites) 완전 전환. 8개 중 모범 사례.

## 문제점
치명 오류 없음. 경고·개선 여지만 있다.

1. **중복 plan (validate warn)** — `infra`와 `local-infra`가 environment/site/entries까지 완전 동일. "legacy-compatible name"이라는 설명뿐이라 하나는 alias여야 할 자리다. `hybrid`도 `local-dev`의 슈퍼셋 변형으로 서비스 목록이 4중 복제돼 있다.
2. **plan 간 서비스 목록 복제** — `[postgres, redis, minio, ...]`가 7개 plan에 걸쳐 반복. plan 합성/상속이 없는 현 모델의 구조적 결과.
3. **Makefile 매핑 gap (validate warn 6건)** — `test-compose`, `test-docs-links` 등 6개 target 미매핑. `suggestion_ignore`가 이미 33줄인데도 남는다.
4. **`stack.*.health_checks` (proxynd/depond/flownd, line 50-85)와 destructive interaction** — `db reset`/`redis flush`가 DESTRUCTIVE 설명만으로 노출됨. agent-deny 계열 보호와의 연동 여부 확인 가치.
5. **`interaction.clean`이 `steps:` 형식 (line 389-397)** — 제거된 built-in 이름을 독립 커맨드로 재정의한 올바른 신형 패턴이나, `dva run clean`으로만 호출됨을 description이 알리지 않는다.

## dva 개선 힌트
- **plan alias/상속 부재가 가장 선명하게 드러난 코퍼스.** `infra`≡`local-infra` 완전 중복과 서비스 목록 4중 복제는 `plans.<name>.extends` 또는 alias 기능의 실수요 근거다. validate가 중복을 경고까지 하면서 해소 수단은 없는 상태.
- `suggestion_ignore` 33줄은 suggestion 기능의 signal/noise 비율 문제를 보여준다 — glob 하나로 안 되는 세분화된 무시 목록은 기본 휴리스틱 조정(예: CI 전용 target 자동 제외) 후보.
- native 엔트리 3개가 dir/build/run/health 구조를 그대로 반복 — 구 applications가 하던 일을 stack이 잘 흡수했다는 이행 성공 증거인 동시에, 동일 dir 워크스페이스 다중 바이너리 선언의 축약 문법 후보.

## 마이그레이션 난이도
**하(완료)** — 이미 신모델로 전환 완료. 남은 작업은 중복 plan 정리와 warn 해소 수준.

## 적용 결과 (2026-09-05)

### 변경 항목 체크리스트
- [x] 중복 plan 정리 — `infra`("legacy-compatible name") 삭제, `local-infra` 유지, `default_plan: local-infra`. 근거: README/Makefile/.make/docs/deploy 어디에도 `dva up infra`·`local-infra` 참조가 없고(유일한 plan 참조는 deploy/runbooks/local.md의 별도 schema-isolation 파일), 다른 devbox(postkit/gizzahub)와 canonical 이름을 맞춤. validate warn 해소.
- [ ] (보류) `hybrid` vs `local-dev` — compose 서비스 목록이 다름(hybrid는 redis-commander/adminer/prometheus/grafana 추가). 완전 중복이 아니라 삭제 시 "infra-full + native" 조합을 잃으므로 유지. TASK-307 extends 도입 시 `hybrid: {extends: local-dev, …}`로 축약 후보.
- [x] Makefile 미매핑 6건(`test-compose`, `test-docs-links`, `test-documented-helm`, `test-guard-wiring`, `test-openapi-drift`, `test-tool-guards`) — 전부 `.make/quality.mk`의 CI 가드 자기검증(`scripts/ci/tests/*`, "pin the gate in both directions")이라 개발 루프 커맨드가 아님. interaction 매핑 대신 `suggestion_ignore`에 주석과 함께 추가. warn 해소.
- [ ] (질문 보고, 미변경) destructive interaction(`db reset`, `redis flush`)과 agent-deny 연동 — 아래 개선점 1 참조. 동작 변경 없음.
- [ ] (보류) `build`/`logs` replace 훅 — `dva build/logs <plan>`은 현행 built-in이므로 "제거된 built-in의 replace 훅" 규칙에 해당하지 않음. 다만 `logs` 훅은 `docker compose -f … logs -f`를 직접 호출해 `dva logs <plan>`과 동일 동작 — 삭제 가능해 보이나 이 프로젝트는 PLAN의 build/logs 훅 정리 대상 목록에 없어 유지.
- [ ] (보류) `interaction.clean` description에 `dva run clean` 호출 안내 없음 — 문구 수정은 계획 항목 아님, 미변경.

### validate 최종 출력
```
✅ dva.yml is valid
exit=0   (warning 0)
```

### 보류/예외 항목
- `hybrid` plan 유지, build/logs 훅 유지 (위).
- `${VAR:-default}` 사용처: dva.yml 내 없음 (provision/interaction 모두 리터럴).

### 발견된 dva 개선점
1. **destructive interaction에 대한 agent-deny 연동이 없다.** `dva agent-deny`가 배포하는 deny 목록(`internal/agentdeny/rules.go`, docs/agent-deny-rules.md)은 `dva config env seal|show` 2건뿐이며, 스키마에도 interaction을 "destructive"로 표시하거나 deny 패턴에 편입시킬 필드가 없다. 이 프로젝트의 `db reset`(DROP DATABASE)·`redis flush`(FLUSHALL)는 description 문구 "DESTRUCTIVE"로만 구분된다. 제안: `interaction.<name>.destructive: true`(또는 `confirm: true`) 필드 + `dva agent-deny install`이 해당 interaction의 `Bash(dva <name> <sub> *)` 패턴을 프로젝트 scope에 함께 투영. repro: `dva agent-deny status`에 db/redis 항목 없음.
2. `logs` replace 훅이 `dva logs <plan>`과 기능이 겹치는데 validate가 "built-in과 동등한 replace 훅" warn을 내지 않음 — build/logs 훅 정리 대상 프로젝트들(PLAN 공통 행)을 기계적으로 찾아주는 semantic check 후보.
3. 7개 plan에 걸친 서비스 목록 복제 — TASK-307 extends 실수요 (기존 힌트 재확인).
