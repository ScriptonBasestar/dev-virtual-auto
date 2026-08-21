# AGENTS.md — DVA (Dev Virtual Auto)

이 문서는 에이전트가 DVA 저장소를 탐색하고 변경할 때 필요한 작업 규칙과 코드 지도를 제공한다.

## Overview

DVA는 개발 환경 오케스트레이터입니다. 핵심 방향은 `stack:`을 선언 저장소로 두고, 실제 실행은 이름 있는 실행 계획을 통해 수행하는 것입니다.

- `stack` = 재사용 가능한 실행 대상 선언
- `plans` = 실제 실행 가능한 이름
- `environments` = `dev/stg/prd` 같은 환경 차이
- `sites` = `local/office/remote/cloud` 같은 실행 host 차이
- `env_file` = 공통 로컬/비밀값 입력
- `interactions` = 단발성 편의 명령
- `provision` = 준비/초기화 절차

## Documentation Ownership

문서 작업에서는 목적에 따라 다음 순서로 읽습니다.

| Need | Loading Order |
|------|---------------|
| 제품 판단 | `SOUL.md` → `PRODUCT.md` |
| 아키텍처·구현 | `SOUL.md` → `PRODUCT.md` → `ARCHITECTURE.md` |
| 사용법 | `README.md` → `USAGE.md` |
| 코드 작업 | `AGENTS.md` → `ARCHITECTURE.md` → 해당 소스 |

각 사실은 하나의 canonical document만 소유합니다.

- `SOUL.md`: Why, 핵심 신념, 변하지 않는 tradeoff
- `PRODUCT.md`: What/Who, 사용자 가치, 제품 경계와 현재 상태
- `ARCHITECTURE.md`: How, 컴포넌트 책임, 의존 방향, 데이터 흐름
- `README.md`/`USAGE.md`: 설치, 시작, 사용자 레퍼런스
- `AGENTS.md`/`CLAUDE.md`: 에이전트 명령, 금지사항, repository navigation

다른 문서에는 한두 문장 요약과 상대 링크만 둡니다. 같은 목록, 표, 다이어그램을
복사하지 않습니다. 미래 후보는 roadmap이 생기기 전까지 현재 지원 기능처럼 서술하지
않습니다.

## Repository Map

구현 경계와 데이터 흐름의 원본은 `ARCHITECTURE.md`입니다. 아래 목록은 코드 탐색용입니다.

```text
cmd/dva/main.go                → Entry point
internal/cli/                  → Cobra commands
  root.go                      → Dynamic routing (interaction → run)
  compose.go                   → up/down/stop/restart verbs (plan and whole-stack paths)
  run.go, list.go, show.go     → Core commands
  validate.go                  → dva config validate (schema + semantic warnings)
  config_migrate.go            → dva config migrate (legacy declaration conversion)
internal/config/               → dva.yml loading, env interpolation, schema validation
  config.go                    → Config struct (Stack, Plans, Environments, Sites, Interaction, etc.)
  lifecycle.go                 → LifecycleEntry, plugin config types (Compose, Kubectl, Helm, ...)
  lifecycle_helpers.go         → SortedStack(), PrimaryComposeEntry(), ComposeEntries(), etc.
  merge.go                     → Field-level deep merge (modules/override 적용)
  validate_warnings.go         → 19 semantic warning checks + canonical order (non-fatal)
  reserved.go                  → Reserved/restricted field definitions
internal/lifecycle/            → Execution plan resolution + runtime orchestration
  orchestrator.go              → Resolved entry execution and teardown
  process.go                   → Process execution and signal handling
internal/runner/               → Interaction execution (DockerCompose, Kubectl, Local)
internal/exec/                 → Process execution (syscall.Exec, subprocess)
```

## Key Flows

아래 흐름은 코드 탐색을 위한 요약이며, 시스템 경계의 원본은 `ARCHITECTURE.md`다.

### Plan Lifecycle (`dva up/down/stop/status <name>`)

1. CLI는 실행 이름을 받음. 예: `local-dev`, `backend/local-dev`
2. Resolver가 `plans`, `environments`, `sites`, `env_file`, 전역 `vars`를 결합해 immutable `ExecutionPlan` 생성
3. plan의 `entries`가 `stack` 선언을 참조
4. 각 resolved entry는 `default_runner`, site override, plan override를 거쳐 최종 runner를 결정
5. `depends_on` + `order`로 wave 계산
6. runtime은 계산된 순서대로 Up/Down/Stop 실행
7. Down은 역순 teardown (LIFO)

### Interaction Execution (`dva run ...`)

1. `cli/root.go`: Built-in command가 아니면서 `interaction`에 있으면 `run`으로 라우팅
2. `cli/run.go`: `InteractionTree`로 명령 resolve
3. subproject import인 경우 canonical name (`backend/shell`) resolve
4. subproject interaction은 해당 subproject root 기준으로 실행
5. `runner/runner.go`가 `DockerComposeRunner`, `KubectlRunner`, `LocalRunner` 중 선택

### Provision Execution (`dva provision ...`)

1. provision은 준비/초기화 절차를 담당
2. subproject import인 경우 canonical name (`backend/setup`) resolve
3. subproject provision 역시 해당 subproject root 기준으로 실행

### Config Loading

1. Walk up from CWD to find `dva.yml` (or use `$DVA_FILE`)
2. Merge `.sb/dva/*.yml` modules
3. Merge `dva.override.yml`
4. Validate against embedded `schema.json`
5. Resolve stack declarations, plans, environments, sites, subprojects

### Stack Declaration Principle

```yaml
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml, docker-compose.dev.yml]

plans:
  local-dev:
    environment: dev
    site: local
    entries:
      - name: core-compose
        runner: compose
        services: [postgres, redis]
```

원칙:

- `stack`은 선언 저장소
- `stack` 엔트리는 multi-runner logical unit이 될 수 있음
- compose의 부분 서비스 선택은 `plans.entries[].services` 에서 수행
- 최종 runner는 선언된 runner 중에서만 선택 가능
- 실행 순서는 `plans.entries[].order` 와 `depends_on` 에서 결정

### Config Merge (modules/override 적용)

1. `config/merge.go`: `mergeFrom()` — base config에 overlay 적용
2. Map 섹션 (stack, plans, environments, sites, interaction 등): key별 deep merge
3. List/Scalar: replace (나중 레이어 우선)
4. 구조적 필드 (`plugin`, `runner`): override 시 hard error
5. 상세 규칙은 `docs/30-config-merge-semantics.md` 와 새 구조 문서들에 정리

## File Map

| Task | Files |
|------|-------|
| Add new CLI command | `internal/cli/` + register in `root.go` init() |
| Add new stack plugin | `internal/config/lifecycle.go` (config type) + `internal/lifecycle/` (executor) |
| Add new runner | `internal/runner/` + update `NewRunner()` factory |
| Modify config schema | `internal/config/schema.json` + `config.go` structs |
| Add/modify plan resolution | `docs/31-execution-plan-resolution.md` + `internal/lifecycle/` resolver/orchestrator |
| Fix env interpolation | `internal/config/environment.go` |
| Modify compose behavior | `internal/runner/docker_compose.go` |
| Plan orchestration | `internal/lifecycle/orchestrator.go` |
| Legacy declaration migration (`dva config migrate`) | `internal/config/migrate*.go` + `internal/cli/config_migrate.go` |
| Config merge logic | `internal/config/merge.go` |
| Validation warnings | `internal/config/validate_warnings.go` |
| Stack declaration inspection | `internal/cli/show.go` (`dva show` lists every entry) |

## Build & Test

```bash
make build      # → ./bin/dva
make test       # go test -race -cover ./...
make doc-check  # 3 gates: markdown links/size, CI labels, flow decision paths
```

## Documentation gate (TASK-090 option B)

Limits (LLM-friendly linear reads):

- **Size:** ≤500 lines and ≤10240 bytes — enforced only under `docs/` and `workflows/`
- **Links:** relative file targets and heading anchors — checked on **all** inventory Markdown (not only docs/workflows)
- **Command:** `make doc-check` → `go run ./tools/doccheck`

Size exemptions (lookup / contract documents — splitting harms the use case; links are still checked):

| Path | Reason |
|------|--------|
| `USAGE.md` | User-facing manual kept as one document by design |
| `skills/*/references/` | Lookup tables; skillgen rewrites reference links |
| `agent-mesh-flows/shared/library/` | Lookup tables / schema corpus |

The checker inventories **tracked files that still exist in the worktree + non-ignored untracked** files (tracked deletions are excluded so mid-move index blobs cannot mask broken links; ignored `tmp/` cannot make a miss look valid), skips git symlink aliases (mode `120000`) and checks the canonical target once, and fails on zero candidates/links, any broken relative link/anchor in repository Markdown, or oversized docs under the size-enforced paths.

**Task links survive state transitions (TASK-143).** A task's identity is its number (`NNN-slug.md`); its directory is its state (`todo`/`done`/`_archive`/…), which changes when it is worked or archived. The checker resolves a `tasks/<state>/NNN-…` markdown link — and the same path written inside inline code (where `verify:` bindings live, invisible to the link scan) — to whichever state directory actually holds `NNN-…`. One match resolves the reference; zero is a genuine broken link; more than one is an ambiguity the gate refuses to guess. So archiving a task no longer breaks inbound links: `make doc-check` stays green across a move without a repoint pass.

## Flow decision-path gate (flowcheck)

`make doc-check` also runs `go run ./tools/flowcheck`, which reads every flow under
`agent-mesh-flows/` and fails the build on **16 rules**. Each one exists because am fails
*silently* in that case — an inert `when:` gate, a comment that blocks the command below it,
a probe that quietly stops matching — so `am validate` reports the flow valid while the run
produces a confident wrong answer.

Rule ids, the am behaviour behind each, and a wrong/right example:
[docs/51-flowcheck-rules.md](docs/51-flowcheck-rules.md). The id list there is derived from
the source string literals, not hand-kept — that document carries the extraction command.

## Source file length is not gated here (TASK-219 ruling)

**dva declares no limit on Go source file length, and nothing in this repository
enforces one.** `make lint` (`Makefile:44`) is four gates, not one: `go vet ./...`,
`gofmt -l` through `fmt-check`, `golangci-lint run ./...`, and `gopls check
-severity=hint` over `cmd internal tools`. CI runs the same four as separate steps —
Vet (`.github/workflows/ci.yml:34`), Format (`:48`), `golangci-lint-action@v8` (`:53`),
gopls check (`:72`). `.golangci.yml` enables the default set plus `modernize` and
`unparam`, and sets `govet: enable-all: true` with exactly two analyzers disabled by
name (`fieldalignment`, `shadow`). Not one of the four measures file length, and the two
golangci-lint linters that would — `funlen`, `lll` — are not enabled:

```bash
git grep -cE 'error_lines|max-lines|funlen|lll' -- Makefile .golangci.yml
# no output, exit 1
```

`make doc-check` does gate size, but only Markdown, and only under `docs/` and
`workflows/` (see the documentation gate above) — this file is under neither. That
contrast is the whole point: docs have a declared, versioned, CI-enforced limit; source
has none.

**A workstation may still gate it, and its verdict is advisory here.** ce-agent-kit
ships a `PostToolUse` hook (`ce-validate-filesize.sh` → `ce validate filesize`) whose
`file-size.yaml` sets `go` to `warning_lines: 300` / `error_lines: 500` and `test` to
`400` / `600`. A contributor cloning this repository does not have it. Do not cite it in
a commit message or a review as "the limit" without saying whose it is (TASK-211), and
do not cite the cache path without a date — it is versioned and moves.

"Advisory" describes its status here, not its track record: `tasks/_archive/187` and
`tasks/_archive/193` each split a Go file specifically to stay under 500, neither citing
a repository rule because there is none to cite. Splitting a file for that reason is a
legitimate choice; making it as though the repository required it is not.

**An error-shaped file length verdict does not mean your edit was rejected.** The hook
runs *after* the write has already landed — exit 2 is feedback, not a failed tool call.
What a retry costs depends on the shape of the edit, which is why "it is only a retry"
is not a safe default: a `Write` re-lands the same bytes harmlessly, an `Edit` whose
`old_string` the first pass consumed fails loudly, and an `Edit` that wrapped or
extended a still-present `old_string` matches again and lands the addition twice. Read
the verdict, then continue.

**A fourth option exists and is deliberately not taken.** `file-size.yaml` carries an
`exemption:` block matched on the bare token `size-limit: exempt`, so `// size-limit:
exempt` in the first 1KB of a Go file suppresses the verdict for that file entirely
(`<!-- size-limit: 2000 -->` sets a custom budget instead). That would end the
misreading by deleting the message — and with it the only signal anyone gets that a file
is growing, on precisely the thirteen files where the signal is worth most. The ruling
keeps the message and corrects what it means.

**The verdict names whichever file you touched, never the census.** Measured at
`3ad895a` over `git ls-files '*.go'` (289 files, not a sample): **13 of 117 non-test
files exceed 500 lines** — largest `internal/config/config.go` at 1162 and
`internal/cli/compose.go` at 1142 — and **4 of 172 `_test.go` files exceed 600**. So a
file being over the limit is the normal case in this repository, not an anomaly of the
file you happened to edit.

```bash
for f in $(git ls-files '*.go' | grep -v '_test.go'); do
  n=$(wc -l < "$f"); [ "$n" -gt 500 ] && printf '%5d  %s\n' "$n" "$f"
done | sort -rn
```

**No source file is split on the strength of that verdict alone.** Splitting one of the
thirteen is its own task, argued on its own merits and filed as its own card. If the
ruling is ever revisited to declare a limit here, it needs a starting threshold above
the current worst file or a grandfathered list, because CI would otherwise go red on
thirteen files on day one.

<!-- skills:auto:start -->
## AI Skills

Generated from `skills/` by `tools/skillgen` — do not edit this block; edit the canonical skill and run `make generate`. Open the linked `SKILL.md` on demand for full guidance.

- **config** — Use when creating, auditing, repairing, or migrating a dva.yml configuration; diagnosing `dva config validate`, `dva show`, or `dva doctor` warnings; separating DVA CLI defects from project configuration and environment issues; or applying DVA across a devbox root and active subprojects. See `skills/config/SKILL.md`.
- **dva** — This skill should be used when the user asks to "build the project", "run tests", "start services", "stop containers", "check logs", "use kubectl", or manage dev infrastructure. Enforces DVA CLI discovery and safe plan-based execution; use raw tools only for configuration validation or when DVA has no equivalent. See `skills/dva/SKILL.md`.
<!-- skills:auto:end -->
