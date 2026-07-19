---
id: TASK-051
title: "dva infra를 stack 소싱으로 흡수하고 명령 표면을 2계층으로 정리"
type: enhancement
priority: P1
status: done
effort: L
needs-human: false
created-at: 2026-07-19T00:55:00+09:00
updated-at: 2026-07-19T00:55:00+09:00
completed-at: 2026-07-19T00:55:00+09:00
moved-at: 2026-07-19T21:32:00+09:00
source: matdosa-devbox 운영 중 발견
source-severity: HIGH
decision: direction-A (fold infra into stack sourcing, deprecate top-level infra)
completion-summary: |
  Added stack.source git/path sourcing for Compose, deterministic legacy infra
  migration, source-relative Compose execution, non-interactive cache safety,
  deprecated infra lifecycle delegation, and raw compose escape-hatch guidance.
  Hardened source validation, pinned commit checkout, cache identity checks,
  teardown no-clone behavior, and installation to GOBIN or GOPATH/bin.
verification-status: verified
verification-evidence:
  - "PASS: go test -race -cover ./... (Go 1.26.3 via mise, GOTOOLCHAIN=local)"
  - "PASS: go test -tags=integration -race ./internal/integration/..."
  - "PASS: go vet ./..."
  - "PASS: make check-generate"
  - "PASS: built, ~/.local/bin, and ~/go/bin binaries share SHA-256 c2baaf6f10dafff39d064cf6dc288405d70cd26f1e5bfbbfcdaf916e1e41d9eb"
  - "PASS: matdosa-devbox dry-run accepts zero-argument 'dva infra up' and emits deprecation guidance"
---

# Task 051: `dva infra`를 stack 소싱으로 흡수하고 명령 표면을 정리

## Summary

인프라를 시작하는 경로가 세 개(`dva compose`, `dva infra`, `dva up -M infra`)이고
이름·인자 규칙이 서로 달라 사용자가 어떤 명령을 골라야 할지 판단하기 어렵다.
표면적으로는 인자 규칙 문제로 보이지만, 근본 원인은 **같은 실행(compose 스택
기동)을 여러 명령 표면이 중복 소유**한다는 설계 문제다. 이는 SOUL.md 원칙 #3
("하나의 동작에는 하나의 소유자만 둔다 / 같은 서비스를 여러 계층에서 중복
관리하지 않는다")에 정면으로 어긋난다.

## 코드 실체 (설계 판단의 근거)

세 경로는 "겹치는 엔진"이 아니라 **서로 다른 config 섹션**을 읽는다.

| 명령 | 읽는 config | 실행 |
|---|---|---|
| `dva compose` | `stack:`의 compose entry (내 dva.yml 소유) | raw `docker compose` passthrough |
| `dva infra up <svc>` | 최상위 **`infra:`** 맵 (`git`/`ref`/`path`) | 외부 repo/디렉토리로 `cd` 후 `docker compose up -d` |
| `dva up -M infra` | `stack:` + `modes:` (`infra`는 mode 이름) | `lifecycle.Orchestrator` |

확인된 사실:

- `dva infra`와 `dva up -M infra`는 **이름만 같고 다른 기능**이다. 필드 혼란의
  근본 원인은 이 이름 충돌이다.
- 최상위 `infra:` 섹션을 쓰는 예시 파일이 **0건**(`examples/`), README도
  `dva infra`를 **문서화하지 않는다** → 사실상 미사용 레거시 표면.
- `infra`가 compose 위에서 유일하게 더 하는 일은 **"외부 소유 스택을 git으로
  가져와 그 자리에서 실행"** 뿐이다. 실행 자체는 고유하지 않다.
- `stack:`의 compose entry는 이미 외부 경로(절대/`../`)를 `files:`로 가리킬 수
  있다. 따라서 infra의 순수 고유 가치는 **`git clone`/`pull` 소싱 한 조각**으로
  좁혀진다.
- `infra.go`는 `docker compose`를 **하드코딩**(`infra.go:47,74`)해 DVA의
  plugin/runner 추상화를 위반한다. 외부 스택이 helm/Make 등을 들고 있으면 대응
  불가.

## Reproduction (관찰된 증상)

```bash
dva infra up
# ERROR: requires at least 1 arg(s), only received 0
```

`dva infra up`이 전체 인프라 기동을 기대하는 사용자에게 인자 부족으로 실패한다.
단, 이는 근본 문제(표면 중복)의 표면적 증상일 뿐이다.

## Decision — Direction A (흡수 + 폐기)

인자 규칙만 손보는 대신, 중복 표면을 제거한다.

1. **최상위 `infra:` 맵과 `dva infra` 명령을 deprecate**하고 stack으로 흡수한다.
2. stack entry에 **git 소싱 선언**(`source:`)을 추가해, 외부 소유 스택을 가져와
   기존 runner(compose/helm/…)에 위임한다. "무엇으로 실행하나"가 아니라
   "어디서 가져오나"를 stack이 소유한다.
3. `dva compose`는 **compose runner로의 low-level escape hatch**로 명시
   재포지셔닝한다 (raw passthrough, 디버그 용도).
4. 일반 사용자·에이전트의 권장 진입점은 `dva up` / plans / modes **하나**로
   수렴시킨다.

### 목표 명령 표면 (3계층)

```text
계층 1 (기본):  dva up / plans / modes   선언된 stack을 검증된 계획으로 실행
계층 2 (escape): dva compose ps|logs      compose runner로의 raw 통로
계층 3 (제거):   dva infra                 → stack 소싱으로 흡수, 폐기
```

### 설계 스케치 (stack 소싱)

기존 `infra:` 항목:

```yaml
infra:
  postgres:
    git: https://example.com/shared-infra.git
    ref: main
```

흡수 후 (stack entry로):

```yaml
stack:
  postgres:
    order: 10
    default_runner: compose
    source:                         # 신규: 외부 소유 스택 소싱 (git | path)
      git: https://example.com/shared-infra.git
      ref: v1.2.0                    # 재현성 위해 SHA/tag 권장 (D4)
    runners:
      compose:
        files: [docker-compose.yml] # source 디렉토리 기준 상대 경로 (D2)
```

로컬 외부 디렉토리는 `source: {path: ../shared-infra}`로 선언한다(D6). `dva up`이
`source:`를 만나면 git이면 없을 때만 clone(D4), 이후 compose runner를 fetch
디렉토리 기준(CWD·files, D2/D3)으로 위임한다. v1은 compose runner 전용(D9).
소싱과 실행이 분리되어 SOUL #1(선언/계획 분리)·#3(단일 소유자)을 동시에 만족한다.

## Migration / deprecation plan

1. stack entry에 `source: {git, ref}` 필드와 fetch 단계를 추가한다.
2. 로드 시 `infra:` 맵을 등가 stack entry로 변환하는 shim + deprecation 경고를
   제공한다 (한 마이너 버전 유지).
3. `dva infra` 서브커맨드는 deprecation 경고와 함께 stack 경로로 위임한다.
4. 다음 마이너에서 `infra:` 맵·`dva infra` 명령·`infra.go`의 compose 하드코딩을
   제거한다 (참고: TASK-044에서 legacy 제거 선례 있음).
5. README·examples에 `source:` 사용법과 3계층 표면을 문서화한다.

## Acceptance criteria (현실 정합 재작성)

- [x] stack entry가 `source: {git | path}`로 외부 스택을 확보(git=없을 때 clone,
      path=로컬 참조)한 뒤 compose runner로 실행한다.
- [x] compose runner가 sourced entry를 fetch 디렉토리 기준(CWD·`files:`)으로
      실행한다 (D2/D3).
- [x] `dva up` / plan / mode 경로가 소싱된 stack entry를 다른 entry와 동일하게
      기동·종료한다 (compose 하드코딩 없이 runner 위임).
- [x] 기존 `infra:` 맵이 등가 stack entry로 로드되며 deprecation 경고를 출력한다.
- [x] `dva infra`(존치 기간 동안)는 stack 경로로 위임하고 deprecation을 안내한다.
- [x] `dva compose --help`와 사용자 문서에 raw Docker Compose escape hatch임이
      명시된다.
- [x] `source:` 사용법을 USAGE.md·examples에 문서화한다 (README는 AI-deny·stale
      없음 → 미변경).
- [x] 전체 CLI 테스트와 config/lifecycle 테스트가 통과한다.

## Implementation (2026-07-19)

6개 phase로 구현 완료:

1. `config`: `SourceConfig` + `LifecycleEntry.Source`, 검증, schema (`source.go`, `lifecycle.go`, `validate.go`, `schema.json`)
2. `lifecycle`: sourced entry를 source dir 기준 CWD로 실행 (`compose.go`, `exec.go`)
3. `lifecycle`: git fetch clone-if-missing, 비대화형 (`source_fetch.go`, `orchestrator.go`)
4. `config`: `infra:` → stack 마이그레이션 shim + deprecation (`migrateInfraToStack`)
5. `cli`: `dva infra` orchestrator 위임, 무인자 버그 해결, 레거시 네트워크 제거 (`infra.go`)
6. `docs`: USAGE.md `stack.source` 섹션 + `examples/stack-source.yml`

미실행: 통합 테스트(`make test-integration`, docker 필요). 단위 스위트는 `race -cover` green.

## Superseded — 이전 접근(Option A: SERVICE 선택 인자화)

초안의 "`dva infra up`의 SERVICE를 선택 인자로" 방안은 폐기한다. 인자 규칙만
고치면 표면 중복(SOUL #3 위반)이 그대로 남고, 존재하지 않는 서브커맨드
(`infra stop`/`restart`/`logs`)와 통합 불가한 기준(`infra up` == `up -M infra`,
서로 다른 config)을 요구해 닫히지 않는다.

## 설계 검토 — 열린 결정 (구현 전 확정 필요)

코드 검증 결과 흡수는 "경로 번역"보다 크다. 아래 결정이 구현 범위를 좌우한다.

### 확정된 결정 (검토 반영)

- **D1 = `source`** — 소싱 필드명 확정.
- **D6 = `source: {git | path}` 통일** — git/local 모두 `source:`로 선언, base-dir/CWD 의미 일원화.
- **D9 = v1 compose 전용** — 현 infra 동작의 충실한 이관까지. helm/kubectl/script 등 runner-무관 소싱은 v2.
- D2/D3/D4/D5/D7/D8/D10 = 각 항목 권장안대로 진행.

### D1. `source:` 필드 네이밍 (config 공개 API)
- 후보: `source` / `from` / `fetch`.
- 권장: **`source`** — "이 스택이 어디서 오는가"라는 선언적 명사. `fetch`는
  동작(명령형)이라 선언 철학과 어긋나고, `from`은 Dockerfile과 혼동.

### D2. Fetch 위치 & `files:` 경로 해석 (기술 핵심)
- compose 플러그인은 `files:`를 `ConfigDir` 기준 해석하고 CWD 개념이 없다
  (`compose.go:157-162`). sourced entry는 **fetch된 디렉토리 기준**으로 해석해야
  한다. → `PluginContext`에 per-entry base-dir 개념 도입 필요.
- Fetch 위치: 기존 `.sb/dva/`(`constants.go:11` `DotDirName`) 재사용,
  예: `.sb/dva/sources/<name>/`.

### D3. CWD 실행 모델 (relative build context / `.env` autoload)
- infra.go는 CWD=외부 디렉토리로 compose를 실행(`infra.go:47`)해 상대 build
  context, 자동 `.env`, 상대 `env_file`/volumes가 동작했다. compose 플러그인은
  `-f` 절대경로 + CWD 없음이라 그대로 흡수 시 **외부 저작 compose가 깨질 수 있다.**
- 권장: `source:` 존재 시 compose 실행 CWD를 fetch 디렉토리로 설정(충실한 이관).

### D4. Fetch 정책 & 재현성 (SOUL #2)
- `dva up`에서 fetch는 **없으면 clone만**(auto-pull 금지 → 재현 가능). 갱신은
  명시적 명령/플래그로 분리.
- `ref: main`은 이동 대상이라 SOUL #2 위반. 권장: SHA/tag pinning을 문서로
  권고, branch ref에는 경고. Lockfile은 v1 out-of-scope.

### D5. 비대화형 안전성 (SOUL #5)
- infra.go `update`는 uncommitted 변경 시 대화형 stash 프롬프트가 있다
  (`infra.go:106-119`). `dva up`은 **절대 대화형 프롬프트/pull 금지**(agent/CI
  차단). fetch 디렉토리는 up 경로에서 read-only 캐시로 취급.

### D6. 로컬 외부 디렉토리(`path:`) 처리
- 현재 `InfraConfig.Path`(git 없는 로컬 dir) 대응. 선택:
  (a) `source: {path: ../foo}`로 통일해 base-dir 의미를 일원화, 또는
  (b) 로컬은 기존 상대 `files:`로 두고 `source:`는 git 전용.
- 권장: **(a)** — D2/D3의 base-dir·CWD 의미를 git/local 모두 동일하게.

### D7. 레거시 네트워크 생성 제거
- infra.go는 `<name>_default` 네트워크를 선생성(`infra.go:43`)하나 compose가
  기본 네트워크를 자동 생성한다. 권장: **흡수 시 제거**(중복). 사전에 이 네트워크
  이름에 의존하는 예시가 없는지 확인.

### D8. Deprecation shim 충돌 규칙
- `infra:` 맵 → 등가 stack entry 합성 시: 기존 `stack:` 키와 이름 충돌하면
  **에러**, order 미지정 infra는 높은 기본 order 부여, 이관 키 목록을 담은
  deprecation 경고 1회 출력.

### D9. Runner-agnostic 범위 / phasing
- 방향 A의 본질은 runner 무관 소싱(helm/make)이나, v1은 현실적으로 compose 전용
  소싱(현 infra 동작의 충실한 이관)으로 좁히고 v2에서 일반화 가능.
- 권장: **v1 = `source:` + compose runner**, v2 = 타 runner. 범위 한정.

### D10. `dva up -M infra` 잔존 혼란
- 흡수 후에도 사용자 정의 mode 이름 `infra`는 여전히 혼동 소지. 코드 범위 밖(사용자
  config). 문서에 "infra 스택 소싱 시 mode 이름 `infra` 회피" 권고만.

## References

- `SOUL.md` — 원칙 #1(선언/계획 분리), #3(단일 소유자) — 설계 판단의 최상위 근거
- `internal/cli/infra.go` — 폐기 대상 legacy infra lifecycle (compose 하드코딩)
- `internal/cli/compose.go` — raw Compose escape hatch (계층 2로 재포지셔닝)
- `internal/config/config.go` — `InfraConfig`(:577), `Infra` 맵(:21) 제거/흡수 대상
- `internal/config/lifecycle.go` — `LifecycleEntry`·`ComposePluginConfig`, `source:` 추가 지점
- `internal/cli/root.go` — 명령 등록(infraCmd 폐기)
- `README.md` — 명령 표면 문서화 대상
- `matdosa-devbox` `Makefile` — `make up` → `dva up -M infra` (권장 진입점 수렴)
