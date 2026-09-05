---
id: TASK-316
title: "compose drift detection: include, compose-*.yaml, subdirectories, asymmetry"
type: bug
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-05T10:30:00+09:00
source: "docs/dogfood/{gizzahub,sigdock-pass,sigdock-idp,primeno1,flow-knowchain}.md"
status: todo
---

# Task 316: compose drift 감지 결함

## Findings

1. `include:`로 분산된 서비스를 따라가지 않음 — gizzahub 루트 compose.yaml에 services 블록 없음에도 drift warn 0 (false negative 의심).
2. `compose.yml` + `compose-*.yaml` 명명은 감지 목록에 없어 미등록 overlay 10개여도 경고 없음. 반대로 등록하면 "선언 파일이 감지 목록에 없음" 경고 (sigdock-pass, 편집 전/후 validate로 재현).
3. 서브디렉터리 compose(env/docker-compose/*.verify.yml)를 보지 않음 (primeno1).

ignore 선언 수단은 TASK-309 범위.

## Analysis (2026-09-05, 코드 미변경 — 다음 세션 착수 지점)

코드 확인 결과와 설계안. 대상: `internal/cli/validate.go` `detectConfigDriftWarnings`
(385행), `detectComposeFilesInDir`(651행), `internal/cli/compose_inspection.go` collector.

1. **Finding 1은 코드상 이미 해소됨.** `composeServiceCollector.collect`가 `include:`를
   재귀 추적한다(`composeIncludePaths`, 순환 방지 `seen`). 테스트
   `TestDetectConfigDriftWarnings_InteractionServiceFromIncludedComposeMatches`가 이를 증명.
   틀린 것은 `configuredComposeServices`의 doc comment("does NOT resolve compose include")
   — TASK-068 시점 서술이 남은 것. gizzahub drift 0건은 정상 동작. → 코멘트만 수정.
2. **Finding 2 (compose-*.yaml 미감지 + 비대칭)** — 실제 결함. 원인 두 가지:
   - `detectComposeFilesInDir`가 접두어 `compose.`/`docker-compose.`만 인정. `compose-`,
     `docker-compose-` 추가 필요. `init.go` `detectComposeFiles`(~440행)도 같은 접두어 목록.
   - 비교가 `sameStringSlice(configured, detected)` 양방향 동치라서 "등록했는데 감지 안 됨"이
     drift가 됨. 선언 파일이 실존하면 drift가 아니다(존재하지 않으면
     `missingConfiguredComposeFiles`가 이미 별도 경고). → 경고는 **감지됐지만 미등록**인 파일만
     열거하는 단방향으로 바꾼다.
3. **Finding 3 (서브디렉터리)** — 실제 결함. 설계: 스캔 디렉터리 = 루트 + (root 엔트리의)
   설정 compose 파일이 있는 디렉터리 + `include:`로 도달한 파일의 디렉터리. `source:` 엔트리는
   외부 코퍼스라 제외. 등록 집합 = 설정 파일 ∪ include 도달 파일(`canonicalComposePath`로
   symlink 동일성 유지 — `SymlinkAliasRepresentsOneComposeFile` 테스트 보존). primeno1은
   `env/docker-compose/`가 스캔 대상이 되어 `*.verify.yml`이 경고됨. flow-pipechain §4
   (루트 shim이 include하는 `deploy/local/`)도 같은 규칙으로 커버.

### 새 메시지 초안

`compose files <list> exist <beside dva.yml | in <dir>/> but no stack entry lists them under
runners.compose.files; add them to an entry or leave them out on purpose (suppression: TASK-309)`.
`compose.files` 부분 문자열은 유지(기존 테스트가 이 문구를 찾음).

### 갱신해야 할 기존 테스트

- `internal/cli/validate_test.go` `TestDetectConfigDriftWarnings_MissingConfiguredRootComposeFile`:
  2건 → 1건(missing만). "compose.files is X but detected root compose files are (none)" 문구 소멸.
- `internal/config/examples_schema_test.go` `composeAbsenceWarningRE`: 첫 대안 제거.
- `TestDetectConfigDriftWarnings_ComposeFilesMismatch`(override 미등록): 그대로 통과해야 함.
- `TestDetectConfigDriftWarnings_IgnoresConfiguredSubdirectoryComposeFiles`: 이름이
  `minor-guardian-e2e.yaml`라 접두어 미매치 — 그대로 통과. 서브디렉터리 미등록 감지용 픽스처
  (`env/docker-compose/docker-compose.verify.yml`), `compose-ha.yaml` 등록/미등록 양쪽 픽스처 추가.
- 새 테스트는 구현 라인을 되돌려 실패를 확인한 뒤 보고.

### 문서

USAGE.md `dva config validate --strict` 부근(720행)에 drift 범위 한 단락. docs/56(suppression)은
감지 폭이 넓어졌음을 전제로 309에서 갱신.

## Completion Criteria

- [ ] 감지 패턴 확장 + include 추적 + 등록 파일 비대칭 경고 제거, 픽스처 테스트 | verify: `make test`
