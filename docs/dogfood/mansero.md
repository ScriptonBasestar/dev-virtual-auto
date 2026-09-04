# mansero dva 도입 적합성 분석 (2026-09-05, 읽기 전용)

## 현황
- 구조: docs-only devbox (docs/ 00-product·ai·contracts·i18n·legal·ops, e2e/ Playwright spec 6개 + 페르소나·시나리오). Git 하위 저장소 없음 — `.gz-git.yaml`이 `mansero-client`/`mansero-specs`/`mansero-server`/`mansero-kmp` 4개를 선언하지만 미클론.
- compose 파일: 없음. Dockerfile: 없음.
- `PORT_MAPPINGS.yaml`: backend 18100, frontend 18101, postgres 18110, redis 18120.
- mise.toml: settings만(도구 없음).
- Makefile: 공용 `.make/*.mk` + `docs.mk`(docs-frozen, docs-frozen-coverage, validate-docs).
- 개발 시작 문서: README Quick Start "make setup / make help / make prepare".

## dva 도입 적합성
**도입 불필요(현 시점), 클론 후 hybrid 후보.** lottomaster와 달리 server/client 저장소가 4개 선언돼 있어 `make prepare` 이후에는 mansero-server(native 또는 compose)+postgres/redis 조합이 자연스럽다. 현재 e2e spec은 실행 대상 URL을 갖지 않는 문서 수준.

## 제안 dva.yml 골격 (클론 이후)
```yaml
version: "0.1.48"
stack:
  infra:
    default_runner: compose
    runners: {compose: {files: [compose.yaml], services: {postgres: {tags: [infra]}, redis: {tags: [infra]}}}}
  server:
    default_runner: native
    runners: {native: {dir: mansero-server, run: "<server dev cmd>"}}   # 저장소 클론 후 확정
    health_checks: {server: {type: http, url: "http://localhost:18100/health", ready_timeout: 60}}
plans:
  local-infra: {entries: [{name: infra, runner: compose}]}
  local-dev:   {entries: [{name: infra, runner: compose, order: 10}, {name: server, runner: native, order: 20, depends_on: [infra]}]}
default_plan: local-infra
interaction:
  e2e: {description: "Playwright persona/scenario", runner: local, command: "pnpm exec playwright test e2e"}
subprojects:
  mansero-server: {path: mansero-server}
  mansero-client: {path: mansero-client}
endpoints:
  backend: {url: "http://localhost:18100", label: "Mansero Backend"}
  frontend: {url: "http://localhost:18101", label: "Mansero Frontend"}
```

## init 생성기 결과 vs 골격
- `dva init --dry-run` (0.1.48, 루트 파일만 복사한 scratch 사본에서 실행): `ERROR: no Docker Compose file detected in .; dva.yml was not created` + "no recognized language manifest" — exit 1, 파일 미생성. `--recursive`도 동일 메시지로 즉시 종료.
- `.gz-git.yaml`의 workspaces(4개)와 PORT_MAPPINGS를 전혀 읽지 않는다. 격차: init 0줄 vs 골격 stack 2 + plans 2 + endpoints 2 + subprojects 2. subprojects/endpoints 부분은 두 파일에서 기계 유도 가능(TASK-249 증거).

## 도입 난이도
**중** — 저장소 클론과 server 실행 명령 확정이 선행돼야 함. 지금은 보류.
