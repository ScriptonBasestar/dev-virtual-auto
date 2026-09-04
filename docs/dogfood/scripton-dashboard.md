# scripton-dashboard dva 도입 적합성 분석 (2026-09-05, 읽기 전용)

## 현황
- 구조: pnpm 3-패키지 devbox — `dashboard-webui/`(SvelteKit host, 이 저장소에 포함) + Git 하위 저장소 `scripton-ui-components/`, `scripton-mfe-protocol/`(`.gz-git.yaml` 선언, 현재 미클론). `ci/dependency-lock.env`로 하위 revision 고정.
- compose 파일: 없음. Dockerfile: 없음.
- `PORT_MAPPINGS.yaml`: dashboard 11600 (`DASHBOARD_PORT`). `.env.example`/`.env.sops` 존재.
- mise.toml: node 24, pnpm 10, typescript/svelte LSP.
- Makefile: install(protocol→components→dashboard 순 pnpm install/build), dev(안내 echo만), dev-dashboard(`pnpm dev`), dev-components(watch), build, check, verify + verify-* 9 레인(revisions/ci-contract/make-validation/protocol/components/dashboard/unit/e2e-offline/e2e-live), clean, env-*(SOPS).
- 개발 시작 문서: README "make install → make dev-dashboard (port 11600) → make verify".

## dva 도입 적합성
**native-only — 적합.** 6개 중 dva 가치가 가장 뚜렷하다. `make dev`가 "터미널 두 개 띄우라"는 echo로 끝나는 지점이 정확히 dva plan(components watch + dashboard dev, order/depends_on)이 해결하는 문제이고, 11600 health check와 endpoints도 바로 선언 가능. compose 없이 native 엔트리 2개.

## 제안 dva.yml 골격
```yaml
version: "0.1.48"
env_file: {files: [{path: .env.example, required: true}, {path: .env, required: false}]}
stack:
  components:
    description: "scripton-ui-components watch build"
    default_runner: native
    runners: {native: {dir: scripton-ui-components, build: "pnpm install && pnpm build", run: "pnpm dev"}}
  dashboard:
    description: "SvelteKit dashboard host"
    default_runner: native
    runners: {native: {dir: dashboard-webui, build: "pnpm install", run: "pnpm dev", env: {DASHBOARD_PORT: "11600"}}}
    health_checks:
      dashboard: {type: http, url: "http://localhost:11600/", start_hint: "dva up dev", ready_timeout: 60}
plans:
  dashboard: {description: "dashboard only (components prebuilt)", entries: [{name: dashboard, runner: native}]}
  dev:       {description: "components watch + dashboard", entries: [{name: components, runner: native, order: 10}, {name: dashboard, runner: native, order: 20, depends_on: [components]}]}
default_plan: dashboard
checks:
  - {name: "scripton-mfe-protocol cloned", type: file_exists, path: scripton-mfe-protocol/package.json, fix_hint: "make prepare"}
  - {name: "scripton-ui-components cloned", type: file_exists, path: scripton-ui-components/package.json, fix_hint: "make prepare"}
interaction:
  install: {description: "install+build protocol/components/dashboard", runner: local, command: "make install"}
  check:   {description: "type check all packages", runner: local, command: "make check"}
  verify:  {description: "full verification lanes", runner: local, command: "make verify",
            subcommands: {unit: {command: "make verify-unit"}, e2e-offline: {command: "make verify-e2e-offline"}, e2e-live: {command: "make verify-e2e-live"}}}
  build:   {replace: [{step: "Build all packages", run: "make build"}]}
suggestion_ignore: ["env-*", "k8s-secret-*", "prepare*", "validate*", "verify-*", "ws-*", "dev", "dev-*", "clean", "help", "version"]
subprojects:
  dashboard-webui: {path: dashboard-webui}
  scripton-ui-components: {path: scripton-ui-components}
  scripton-mfe-protocol: {path: scripton-mfe-protocol}
endpoints:
  dashboard: {url: "http://localhost:11600", label: "Dashboard host"}
```
미확정: `pnpm dev`가 `DASHBOARD_PORT`를 읽는지(vite.config.ts 확인 필요), e2e-live 레인이 별도 서버를 요구하는지.

## init 생성기 결과 vs 골격
- `dva init --dry-run` (0.1.48, 루트 파일만 복사한 scratch 사본에서 실행): `ERROR: no Docker Compose file detected in .; dva.yml was not created` + "no recognized language manifest" — exit 1, 파일 미생성. `--recursive`도 동일 메시지로 즉시 종료.
- 루트에 package.json이 없고(`dashboard-webui/package.json`), `--recursive`도 루트 compose 부재에서 멈춘다. 격차: init 0줄 vs 골격 stack 2 + plans 2 + interaction 4 + endpoints 1 + subprojects 3. PORT_MAPPINGS(11600)와 Makefile `dev-dashboard`/`dev-components`에서 native 엔트리 2개를 기계 유도할 수 있는 사례 — TASK-249 핵심 증거.

## 도입 난이도
**하** — compose 없음, 프로세스 2개, 포트 1개. 하위 저장소 2개 클론(`make prepare`) 후 vite 포트 처리만 확인하면 됨. 6개 중 1순위.

## 도입 결과 (2026-09-05, 사용자 승인)

`dva.yml` 신규 생성(미커밋). validate **exit 0 / warn 0**.
- stack: `components`(scripton-ui-components, pnpm dev), `dashboard`(dashboard-webui, pnpm dev, health http :11600, ready_timeout 60). vite.config.ts가 `DASHBOARD_PORT`(기본 11600)를 읽는 것 확인 → 골격의 `env:` 주입은 불필요해 제거.
- plans: `dashboard`(기본), `dev`(components → dashboard, depends_on). `make dev`의 "터미널 두 개" 안내를 대체.
- checks: pnpm, 하위 저장소 2개 클론 여부(fix_hint `make prepare` — 골격의 `ws-clone`은 존재하지 않는 타깃이라 정정), .env.
- interaction: prepare/install/check/test/e2e(+live)/verify(+revisions/unit/e2e-offline/e2e-live). `workdir:`는 local 러너가 무시(TASK-313)해서 `cd dashboard-webui &&`로 명시.
- 골격의 `build` replace 훅은 제거된 built-in 우회라 넣지 않음 — `dva build dev`가 native build를 실행.
- subprojects: dashboard-webui만(하위 저장소는 미클론 + dva.yml 없음). endpoints는 리터럴 11600(dva가 endpoints url을 치환하지 않음).
- dry-run: `up dev`가 components(dir scripton-ui-components) → dashboard(dir dashboard-webui) 순서로 해석, `build dev` 정상.
- 미확인: 하위 저장소 미클론 상태라 실기동 불가. `make prepare` 후 `dva up dev` 실기동은 사용자 몫.
