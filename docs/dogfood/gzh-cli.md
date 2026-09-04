# gzh-cli dva 도입 적합성 분석 (2026-09-05, 읽기 전용)

## 현황
- 구조: Go 멀티모듈 devbox. `go.work`(추적 안 함, `make go-work`로 재생성) + Git 하위 저장소 12개(gzh-cli, -core, -dev-env, -gitforge, -mcp-plugin, -net-env, -os-env, -package-manager, -project, -quality, -shellforge, -template).
- compose 파일: 없음. `docker/ci-toolchain/`은 README뿐(CI 이미지 설명).
- `PORT_MAPPINGS.yaml`: dev_server 17000, debug 17001, docs 17002 — `.env.example`에 동일 값. 실제 상주 서버는 없음(CLI 제품).
- mise.toml: go 1.26, golangci-lint, gh, just.
- Makefile: `.make/{base,env,quality,prepare,dev,build,test,utils,validate}.mk` — build/build-all/install/install-all/test/test-all/lint/fmt/check/coverage, local-dev(-enable/-disable/-status: go.mod replace 토글), go-work/go-work-status, env-*(SOPS), prepare*.
- 개발 시작 문서: README "make prepare → make status", GUIDELINES.md.

## dva 도입 적합성
**native-only(interaction 중심) — 조건부.** 인프라도 데몬도 없고 모든 개발 동작이 Makefile에 잘 정리돼 있다. dva의 가치는 (1) 12개 서브프로젝트를 `subprojects`로 노출해 `dva run <sub> <cmd>`로 통일, (2) `dva doctor` checks(go.work 존재, mise 도구). stack/plans는 선언할 것이 없다. Makefile 타겟이 60개 이상이라 suggestion 경고를 `suggestion_ignore`로 대량 억제해야 하는 부담이 있다.

## 제안 dva.yml 골격
```yaml
version: "0.1.48"
env_file: {files: [{path: .env.example, required: true}, {path: .env, required: false}]}
checks:
  - {name: "go.work present", type: file_exists, path: go.work, fix_hint: "make go-work"}
  - {name: "go toolchain", type: command, command: "go version", fix_hint: "mise install"}
interaction:
  build:     {description: "Build gzh-cli", runner: local, command: "make build"}
  build-all: {description: "Build all modules", runner: local, command: "make build-all"}
  test:      {description: "Test gzh-cli", runner: local, command: "make test", subcommands: {all: {command: "make test-all"}}}
  lint:      {description: "golangci-lint", runner: local, command: "make lint", subcommands: {all: {command: "make lint-all"}}}
  check:     {description: "lint+fmt+test", runner: local, command: "make check"}
  local-dev: {description: "toggle go.mod replace", runner: local, command: "make local-dev-status", subcommands: {enable: {command: "make local-dev-enable"}, disable: {command: "make local-dev-disable"}}}
  go-work:   {description: "regenerate go.work", runner: local, command: "make go-work"}
suggestion_ignore: ["env-*", "k8s-secret-*", "prepare*", "validate*", "versions*", "deps-graph*", "install*", "coverage*", "fmt*", "health*", "clear-projects", "dev-gh-prepare", "status", "clean", "help", "version"]
subprojects:
  gzh-cli: {path: gzh-cli}
  gzh-cli-core: {path: gzh-cli-core}
  # … 나머지 10개 동일 패턴
```

## init 생성기 결과 vs 골격
- `dva init --dry-run` (0.1.48, 루트 파일만 복사한 scratch 사본에서 실행): `ERROR: no Docker Compose file detected in .; dva.yml was not created` + "no recognized language manifest" — exit 1, 파일 미생성. `--recursive`도 동일 메시지로 즉시 종료.
- `go.work`(루트 Go 워크스페이스 매니페스트)를 language manifest로 인식하지 않고, `--recursive`도 하위 12개 `go.mod`를 보기 전에 루트 compose 부재로 종료한다. 격차: init 0줄 vs 골격 checks 2 + interaction 7 + subprojects 12 (subprojects는 `.gz-git.yaml`/go.work에서 기계 유도 가능). TASK-249 증거 중 가장 큰 격차.

## 도입 난이도
**하~중** — 파일 작성 자체는 쉬우나 Makefile 타겟 60+개의 suggestion 정리와 12개 subproject 선언이 노동. 효용은 "루트에서 하위 실행 통일" 정도라 우선순위 중.
