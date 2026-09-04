# lottomaster dva 도입 적합성 분석 (2026-09-05, 읽기 전용)

## 현황
- 구조: docs-only devbox (docs/ 00-product·api·competitors·legal·ops, e2e/ 페르소나·시나리오, scripts/verify_docs.py). Git 하위 저장소 없음 — `.gz-git.yaml`이 `lottokit-kmp` 1개를 선언하지만 미클론.
- compose 파일: 없음. Dockerfile: 없음.
- `port-mappings.yaml`(소문자 파일명): backend 18400, frontend 18401, postgres 18410, redis 18420 — 실체 없는 예약.
- mise.toml: "Docs-only devbox — no language tools required".
- Makefile: `.make/{base,help,setup,utils,validate}.mk` 공용 타겟(setup/doctor/status/deps/validate*/ws-*)뿐. `deps`는 "docs-only — 없음" echo.
- 개발 시작 문서: README "make setup / make doctor".

## dva 도입 적합성
**도입 불필요(현 시점).** 실행할 서비스·프로세스가 하나도 없다. 예약 포트는 미래의 KMP 앱과 backend를 위한 자리일 뿐이다. `lottokit-kmp`가 클론되고 서버가 생기는 시점에 재평가.

## 제안 dva.yml 골격 (미래용 — 지금 쓰지 말 것)
```yaml
version: "0.1.48"
stack:
  infra:               # compose.yaml이 생기면
    default_runner: compose
    runners: {compose: {files: [compose.yaml], services: {postgres: {}, redis: {}}}}
plans:
  local-infra: {entries: [{name: infra, runner: compose}]}
default_plan: local-infra
endpoints:
  postgres: {url: "postgres://localhost:18410", label: "PostgreSQL"}
  redis:    {url: "redis://localhost:18420",    label: "Redis"}
```

## init 생성기 결과 vs 골격
- `dva init --dry-run` (0.1.48, 루트 파일만 복사한 scratch 사본에서 실행): `ERROR: no Docker Compose file detected in .; dva.yml was not created` + "no recognized language manifest" — exit 1, 파일 미생성. `--recursive`도 동일 메시지로 즉시 종료.
- `port-mappings.yaml`은 읽지 않는다. 격차: init 0줄 vs 골격은 endpoints 2개(포트 예약 파일에서 기계 유도 가능) — 단, 이 프로젝트는 골격 자체가 시기상조.

## 도입 난이도
**해당 없음** — 도입 대상 아님.
