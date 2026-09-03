---
id: TASK-267
title: "Repair grammar-independent subproject exposure defects"
type: bug
priority: P1
effort: M
exec-tier: standard
created-at: 2026-09-03T00:20:00+09:00
completed-at: 2026-09-03T12:20:00+09:00
source: "TASK-259 section 5 items 2, 3, 6 and its Troubleshooting Log"
scope: "subproject usage_example resolution, run.go recovery hint, literal-key documentation claim, and eager-load error masking"
status: done
depends-on: [TASK-259]
---

# Task 267: repair subproject exposure defects

## Summary

Fix the four defects that [TASK-259](259-discover-qualified-project-addressing.md) confirmed against
source and that its §5 declares "separable from any addressing decision — they are correct under every
option". TASK-263 owns representation and routing; this card owns only what is provably wrong today.

## Problem

Each item below is a disagreement between what a surface prints and what the runtime does.

1. `buildManifestSubprojectCommands` (`internal/cli/manifest.go`) emits
   `usage_example: dva <project>:<key>` unconditionally and never asks whether that form reaches the
   entry it sits inside. The root path resolves the same field through `interactionUsage`
   (`internal/cli/list.go:178`) precisely because, as the comment above the root loop records, the
   unconditional form "for a shadowed key was the one form that provably ran something else — a
   different command with a different description, in the same document, silently". The subproject
   path still carries that defect.

   **This is new code, not a call to an existing helper.** `interactionUsage` and
   `config.ConflictAdvice` (`internal/config/reserved.go`) set the standard to meet — every
   invocation they name was executed against the binary — but both take a single *root* key, know
   nothing about subprojects, and never emit a `--project` form. The
   `dva run --project <project> <item>` fallback has to be written here. Scoping this item as
   "route the subproject path through the existing root helper" will not satisfy criterion 1.

   **Premise correction (measured, this card).** The card originally required the subproject entry to
   carry "the same shadow/unroutable markers the root `dynamic_commands` path already sets". Both
   halves of that are wrong:

   - There is **no unroutable state** for a subproject command. Measured against a fixture with a
     subproject literally named `run`: `dva run:go` printed `CHILD-RUN-GO`, and
     `dva run --project run go` printed the same. `UnroutableNamespacePrefix` classifies *interaction
     keys* whose prefix is reserved; a subproject reference is not one, and `dva run --project` reaches
     a declared subproject entry regardless of what shadows the colon form. Setting `Unroutable` here
     would retract a promise the binary keeps.
   - The shadowing party is **not a built-in**. It is the parent's own literal `<project>:<key>`
     interaction key, routed by `config.LiteralKeyWins`. `ShadowedByBuiltin` is documented as naming
     "the static_commands entry that runs when the bare `dva <key>` form is typed"; a parent
     interaction key is not in that table, so a consumer resolving the name against `static_commands`
     would find nothing. Reusing the field would trade one lie for another.

   The repair therefore adds a third field, `shadowed_by_literal_key`, and leaves `usage_example`
   populated with the form that works.
2. `internal/cli/run.go` tells the user to run `dva ls --project <project>` when a subproject
   command is not found. `lsCmd` registered only `--format`/`-f` and `--detailed`/`-d`;
   `--project` was registered on `runCmd` alone. The advertised recovery command exited non-zero
   with an unknown-flag error.
3. `USAGE.md` and the `LiteralKeyWins` comment claim a parent has no literal `p:item` key of its own.
   The warning that exists for exactly that case proves the claim false.
4. `cli.Execute` (`internal/cli/root.go`) discards the `loadConfig()` error on the dynamic-routing
   branch and hands control to cobra, which then reports `unknown command "hello" for "dva"` — naming
   the wrong problem and hiding the real one. `dva run hello` prints the true error, so the masking is
   specific to the bare form.

   **Premise correction (measured, this card).** The card's stated reproduction — "one `subprojects:`
   entry with a missing `path`" — does not reproduce on its own. `resolveSubprojectImports`
   (`internal/config/subproject.go:88-102`) skips every entry that declares no `import:`, so a broken
   `path:` alone never reaches `LoadSubprojects` and `config.Load` succeeds. Measured: a parent with a
   local `hello` interaction plus `subprojects: {broken: {path: ./nope}}` runs `dva hello` fine
   (`hello-from-parent`, exit 0). Adding `import: {plans: [{name: x}]}` to the same entry reproduces
   the masking. The defect is therefore **general to every `loadConfig()` failure on that branch**, not
   specific to a missing path, and the repair is scoped to the discard, not to one config shape.

## Completion Criteria

- [x] `manifest.subprojects.*.commands.*.usage_example` is computed against the parent namespace and falls back to `dva run --project <project> <item>` when the `:` form is shadowed; the entry carries a shadow marker in the same contract the root `dynamic_commands` path uses — presence alone is the signal — using a new `shadowed_by_literal_key` field rather than `shadowed_by_builtin`/`unroutable`, for the reasons measured in Problem item 1 | verify: `go test ./internal/cli -count=1`
- [x] A regression test pins a shadowed subproject key and asserts the emitted `usage_example` invokes that entry rather than the shadowing parent key | verify: `go test ./internal/cli -count=1`
- [x] `run.go`'s recovery hint names an invocation that exists — `--project`/`-p` is registered on `lsCmd` and covered by a test rather than only documented | verify: `go test ./internal/cli -count=1`
- [x] `USAGE.md` and the `LiteralKeyWins` comment no longer claim a parent cannot own a literal `p:item` key, and cite the warning that disproves it | verify: `make doc-check`
- [x] A `loadConfig()` failure no longer masks the parent's own local interactions on the bare form; the surfaced error names the failing subproject and its path | verify: `go test ./internal/cli -count=1`
- [~] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check` — every gate exits 0 except `commit-check`, which fails on 5 pre-existing violations from other sessions' already-pushed commits (see Evidence §5)

## Non-goals

- No addressing grammar, alias, registry, or auto-import change — TASK-263 owns representation and routing.
- No owner field or canonical-vs-alias marker on `ls --json`/`manifest` (TASK-259 §5 item 1) and no completion work (item 4); both belong to TASK-263.
- No decision on the two open questions in TASK-259 §5 item 5.

## Evidence

### 1. 무엇을 했는가

| 파일 | 변경 |
|---|---|
| `internal/cli/list.go` | `subprojectUsage` 헬퍼 신설 (`manifest`와 `ls --project` 공용), `lsCmd`에 `--project`/`-p` 등록, `runLsProject` + table/json/yaml 출력 3종 |
| `internal/cli/manifest.go` | `ManifestDynCmd.ShadowedByLiteralKey` 필드 추가, `buildManifestSubprojectCommands`가 `parentCfg`를 받아 `subprojectUsage`로 usage 결정 |
| `internal/cli/run.go` | `loadSubprojectConfig` 추출 — `runSubprojectCommand`과 `ls --project`가 not-found 메시지/로드 에러 래핑에서 드리프트할 수 없게 함 |
| `internal/cli/root.go` | `shouldSurfaceDynamicRoutingConfigError` + `reportDynamicRoutingConfigError` — dynamic routing 분기의 `loadConfig()` 에러 폐기 중단 |
| `internal/config/reserved.go` | `LiteralKeyWins` 주석의 거짓 문장 교체 |
| `USAGE.md` | 같은 거짓 주장 교체(1199행대), `ls --project` 사용례 추가, `shadowed_by_literal_key` 세 번째 상태 문서화 |
| `internal/cli/subproject_usage_test.go` | 신규 — shadow/unshadowed/예약어이름 서브프로젝트, `ls --project` 등록·목록·미존재, `subprojectUsage` 단위 |
| `internal/cli/dynamic_routing_config_error_test.go` | 신규 — 설정 없음/깨진 서브프로젝트/nil 에러 3분기 |

### 2. 결함 재현과 수정 확인 (실측)

픽스처: 부모가 interaction 키 `engine:test`를 선언, 서브프로젝트 `engine`(자식에 `test`,`build`)과
`run`(자식에 `go`)을 선언.

수정 전:

```
dva engine:test          -> PARENT-LITERAL
manifest engine.test     -> usage_example: "dva engine:test"   (설명은 "child engine test")
```

즉 manifest가 같은 문서 안에서 다른 커맨드를 가리키고 있었습니다.

수정 후:

```json
"test": {
  "description": "child engine test",
  "command": "echo CHILD-ENGINE-TEST",
  "usage_example": "dva run --project engine test",
  "shadowed_by_literal_key": "engine:test"
}
```

그리고 그 문자열을 실행하면 `CHILD-ENGINE-TEST`가 나옵니다. 가려지지 않은 `engine:build`는
`dva engine:build`를 유지하고, 예약어 이름 서브프로젝트 `run`의 `go`도 `dva run:go`를 유지하며
마커가 붙지 않습니다.

### 3. `ls --project` (실측)

```
$ dva ls --project engine
build  # child engine build
test   # child engine test  (parent key 'engine:test' takes this name; run: dva run --project engine test)

$ dva ls -p engine            # -p는 lsCmd에서 비어 있음 (runCmd의 -p는 --publish)
(동일 출력)

$ dva ls --project nope
ERROR: subproject `nope` not found. Available: engine, run     (exit 1)
```

`-f json`도 같은 `shadowed_by_literal_key` 값을 노출합니다 — `ls`와 `manifest`가 같은 키를
다르게 설명하지 못하도록 `subprojectUsage` 하나를 공유합니다(`interactionUsage`가 하나인 것과
같은 이유, `TestLsAndManifestStillAgree`).

### 4. 마스킹 해제와 회귀 없음 (실측)

```
깨진 import 있는 부모에서:
  dva hello           -> ERROR: resolving subprojects: loading subproject "broken"
                         (<abs>/nope/dva.yml): open ...: no such file or directory   (exit 1)
  dva hello --json    -> {"error":{"exit_code":1,"message":"resolving subprojects: ..."}}  (exit 1)

회귀 없음 확인:
  path만 깨지고 import 없는 부모에서  dva hello  -> hello-from-parent      (exit 0)
  깨진 설정 디렉토리에서  dva --help              -> exit 0
  깨진 설정 디렉토리에서  dva (인자 없음)          -> exit 0
  깨진 설정 디렉토리에서  dva init --help          -> exit 0
  dva.yml이 아예 없는 디렉토리에서  dva somethingunknown
                                    -> ERROR: unknown command "somethingunknown" for "dva"  (수정 전과 동일)
```

마지막 항목이 `shouldSurfaceDynamicRoutingConfigError`가 존재하는 이유입니다. "could not find
dva.yml"은 이름 붙일 서브프로젝트도 경로도 없는 평범한 no-config 경우이고, cobra의 unknown
command + 제안 목록이 이미 옳은 답이라 그대로 둡니다. 문자열 매칭은 `config.go:882`가 sentinel
error 타입 없이 메시지만 반환하기 때문이며, `root.go`의 기존 `dva init` 힌트가 같은 문자열을
같은 이유로 매칭하고 있어 일관됩니다.

### 5. 게이트

| 게이트 | 결과 |
|---|---|
| `make build` | exit 0 |
| `make lint` | exit 0 (`go vet` 통과, gofmt 356 files / 0 unformatted, 0 issues) |
| `make test` | exit 0 — `internal/cli` 77.6%, `internal/config` 72.8%, FAIL 없음 |
| `make test-integration` | exit 0 (`internal/integration` 56.7s) |
| `make doc-check` | exit 0 (flowcheck 10 flow / 0 defects, cilabels OK) |
| `make generate && make check-generate` | exit 0, 생성물 변화 없음 |
| `make commit-check` | **exit 1 — 이 카드와 무관한 기존 실패** |

`commit-check`의 5건은 전부 다른 세션이 이미 푸시해 되돌릴 수 없는 커밋입니다:
`7ce7c146`(79자), `6e3f5814`(74자), `a6666c1a`·`6ab9c643`·`095f525b`(scope 누락). 이 카드는
한 건도 추가하지 않았습니다. `commitcheck`의 `baseline` 주석이 "moving this forward would retire
findings rather than fix them"이라고 못박고 있어 baseline 이동은 선택지가 아니며, 처리 방식
(5건 전부 grandfather / 3건만 / commitcheck에 브랜치 한정 모드 추가)은 사용자 결정 대기 중입니다.
이 5건은 TASK-267을 포함해 `make commit-check`를 완료 게이트로 묶은 여러 카드를 동시에 막고
있습니다.

### 6. 카드 전제 정정

Problem item 1과 item 4의 전제 두 개가 실측으로 반증되어 본문을 교체했습니다. 카드를 원문 그대로
아카이브하면 다음에 읽는 사람이 (a) 서브프로젝트 항목에 `unroutable`을 달려 하고, (b) `path:`만
깨뜨린 픽스처로 item 4를 재현하려다 실패합니다. 반증 근거는 각 항목 안의 "Premise correction"
문단에 있습니다.
