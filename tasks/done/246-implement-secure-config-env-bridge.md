---
id: TASK-246
title: "Implement the decided config env bridge without exposing plaintext"
type: feature
priority: P0
effort: L
exec-tier: standard
created-at: 2026-09-01T19:22:00+09:00
source: "PLAN-002 frozen security boundary and TASK-245 decision"
scope: "env_file model, Cobra commands, sops runner, safe writer, output fixtures, integration tests, user documentation"
status: done
depends-on: [TASK-245, TASK-248]
---

# Task 246: implement the secure config env bridge

## Summary

Implement the contract selected by TASK-245 with fail-closed selection, cross-platform write safety,
and negative secret-leak tests.

## Problem

The bridge must materialize an explicitly selected dotenv target without making DVA a secret manager,
without changing legacy env-file shapes, and without leaving a partial or leaked plaintext file.

## Completion Criteria

- [x] Implement exactly TASK-245's schema and command grammar across schema, runtime normalization, merged config output, and documentation; existing string/list/object shapes preserve their observable load order, required semantics, merge result, and show round-trip | verify: `/usr/bin/grep -Eq '^func TestConfigEnvLegacyShapeRoundTrip\(' internal/config/envfile_test.go && go test ./internal/config -count=1`
- [x] Encrypted source metadata is accepted only at the decided effective top-level origins with preserved provenance and is rejected rather than ignored in interaction, nested subcommand, ambiguous module/override merge, and subproject shapes outside that contract | verify: `/usr/bin/grep -Eq '^func TestConfigEnvSourceMetadataScope\(' internal/config/envfile_test.go && go test ./internal/config -count=1`
- [x] Invoke an injectable sops runner by argv without a shell, pin dotenv input/output behavior, and return exit 1 for binary absence, decryption failure, invalid/empty output, or cancellation | verify: `go test ./internal/cli -count=1`
- [x] Resolve root/module/override/subproject paths exactly as TASK-245 decided, revalidate source and target type/containment at use time, and prevent path-component or symlink swaps between preflight and replace | verify: `/usr/bin/grep -Eq '^func TestConfigEnvRejectsPathSwap\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [x] Write through a same-directory 0600/O_EXCL temporary file, validate dotenv before replacement, sync the file and required parent directory, and implement the decided owned stale-temp recovery without claiming SIGKILL or power-loss cleanup that cannot be guaranteed | verify: `/usr/bin/grep -Eq '^func TestConfigEnvAtomicWriteFaultMatrix\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [x] Failure preserves an existing target byte-for-byte; success leaves no live temporary artifact; concurrent writers serialize or one fails explicitly without lost update; create/write/sync/close/replace/recovery failures are injected | verify: `/usr/bin/grep -Eq '^func TestConfigEnvConcurrentWriters\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [x] A secret sentinel is absent from stdout, stderr, JSON, debug logs, error strings, and temporary names for success and every injected failure | verify: `/usr/bin/grep -Eq '^func TestConfigEnvNeverEmitsSecretSentinel\(' internal/cli/config_env_test.go && go test ./internal/cli -count=1`
- [x] Fake-sops fault tests and a pinned real-sops integration test pass on every OS TASK-245 declares supported; safe-writer and command tests continuously cover that declared matrix, while every undeclared platform takes an explicitly tested unsupported fail-closed path | verify: `/usr/bin/grep -Eq '^func TestConfigEnvRealSOPS\(' internal/integration/config_env_test.go && /usr/bin/grep -q 'config-env-platform' .github/workflows/ci.yml && make test-integration`
- [x] User docs explain explicit unseal, no stdout show, no lifecycle auto-unseal, Git/path safeguards, and recovery without recommending a command that does not exist | verify: `make doc-check`
- [x] Full repository and release snapshot gates pass | verify: `make lint && make test && make test-integration && make check-generate && make release-check && make commit-check`

## Completion Summary

TASK-245가 동결한 계약을 그대로 구현했습니다. `sops_source`는 `env_file` **엔트리 레벨** 메타데이터이고
로드 경로에는 보이지 않습니다 — `LoadEnvFile`은 이 필드를 읽지 않으며 `required` 의미, 선언 순서,
override 우선순위, `config show` round-trip이 모두 그대로입니다(§2-1). 스키마는 두 갈래로 나뉩니다:
최상위는 `#/definitions/env_file`(`sops_source` 허용), `interaction_command`는
`#/definitions/env_file_plain`(거부). Provenance는 exported 필드가 아니라 `Config.envFileOrigin`에
기록되므로 `config show` 문서에 키가 하나도 늘지 않습니다(§5-2).

명령은 `dva config env unseal`과 `dva config env edit` 둘뿐입니다. `edit`은 암호화된 소스만 열고
평문 타깃을 만들지 않으며, stdout은 에디터 세션 몫이라 갱신 힌트는 stderr로 나갑니다. 복호화는 언제나
사용자가 시킬 때만 일어나고 lifecycle 명령은 자동 unseal하지 않습니다.

쓰기 경로는 preflight부터 rename까지 **하나의 `os.Root` 핸들**로 관통합니다. `os.Root`는 루트 안에
머무는 심볼릭 링크를 따라가므로 경로 컴포넌트마다 `Root.Lstat`으로 확인해야 하고(`path_component_symlink`),
`syncDir`과 stale-temp 회수도 같은 핸들을 거칩니다. 평문은 DVA 버퍼에 들어오지 않습니다 — DVA가 만든
0600 `O_EXCL` 임시 fd가 곧 sops 자식의 stdout이고, dotenv 검증기는 개수와 줄 번호만 돌려줍니다(§7-4).
검사 순서는 §4-2의 12단계를 고정했고, 모든 실패는 예외 없이 exit 1이며 자식의 종료 코드는 전파되지
않습니다.

검증: `internal/cli`의 fake-sops fault matrix(19행)와 `internal/integration`의 실제 sops 3.13.1
고정 실행. 후자가 있는 이유는 fake로는 DVA가 조립한 argv를 sops가 실제로 받아들이는지 증명할 수 없기
때문입니다. 실측한 종료 코드는 키 실패 128, 변경 없는 `sops edit` 200, 빈 dotenv 암·복호화 0이며,
전부 DVA에서 exit 1로 수렴하는 것을 테스트가 고정합니다.

### 계약과 다르게 구현했거나 도중에 바뀐 지점

1. **테스트 파일 분할** — 저장소 훅이 파일당 코드 600줄을 강제해서 `internal/cli`의 인수 스위트를
   `config_env_test.go` / `config_env_grammar_test.go` / `config_env_faultrows_test.go` /
   `config_env_fixture_test.go` 넷으로 나눴습니다. fault matrix 행 목록을 별도 파일에 둔 것은 두
   테스트(원자성·sentinel 부재)가 같은 실패 집합을 재생해야 하기 때문이고, 목록이 하나여야 두 테스트가
   서로 다른 집합을 덮는 일이 생기지 않습니다.
2. **criterion 1·2의 verify binding 대응** — 카드가 `internal/config/envfile_test.go`를 grep하는데
   구현 초기에 테스트를 `envsource_test.go`에 두었습니다. binding을 고치는 대신 테스트를
   `envfile_test.go`로 합치고 `envsource_test.go`를 삭제했습니다 — 닫힌 카드의 binding은 동결
   시점의 기록이므로 코드 쪽을 맞추는 편이 옳습니다.
3. **`tempName`의 시계 기반 유일성 결함** — 새로 쓴 동시성 테스트가 잡았습니다. 이름을
   `time.Now().UnixNano()`에서 파생했는데 darwin은 마이크로초 해상도로 보고하므로 같은 마이크로초에
   출발한 두 writer가 같은 이름을 만들고 `O_EXCL`이 동결된 코드가 하나도 없는 맨 "file exists"로
   실패했습니다. `randomToken()` 기반으로 바꾸고 10000회 무충돌 + 이름 선점 시 재시도를 고정했습니다.
4. **`fakeSops.calls` 데이터 레이스** — `-race`가 잡았습니다. 제품이 아니라 하네스의 레이스지만,
   레이스 상태의 fake는 코드가 아니라 하네스를 보고하게 되므로 뮤텍스와 `callsMade()` 복사본으로
   고쳤습니다.
5. **edit 실패 문구** — 실제 sops가 "변경 없음"에 200을 반환하는 것을 확인하고 문구를 바꿨습니다.
   동결된 코드 집합에는 "취소된 편집"에 해당하는 이름이 없고 새로 만드는 것은 계약 변경이라,
   `decrypt_failed` 하나가 키 실패·에디터 비정상 종료·변경 없음 셋을 덮습니다. 대신 메시지를 셋 모두에
   맞게 써서 취소를 복호화 실패로 부르지 않도록 했습니다 — 그렇게 부르면 사용자를 키 점검으로 보냅니다.
6. **툴체인 고정** — `.mise.toml`에 `sops 3.13.1` / `age 1.3.1`을 핀했고, CI에는 버전과 sha256을
   함께 고정한 `config-env-platform` job(ubuntu/macos 매트릭스)을 추가했습니다. 알 수 없는 아키텍처는
   하드 에러입니다 — 고정된 툴체인이 조용히 비고정 다운로드로 퇴화하면 안 됩니다.
7. **`tools/doccheck` 앵커 슬러그 발산** — `stripHeadingInline`이 `_`를 제거해서 `env_file`을 담은
   제목이 `envfile` 앵커가 되고, 실제 GitHub와 어긋납니다. 이번에는 제목에서 `_`를 빼는 것으로
   우회했습니다. 도구 쪽 결함이므로 후속 카드 대상입니다.
8. **`errors.As` → `errors.AsType`** — `make lint`의 `gopls check -severity=hint`가 지적했습니다
   (Go 1.26, 반환값 2개).

### 이번 작업 중 발견한 무관한 baseline 결함

`TestLoadEnvFileKeepsSuccessfulPrecedence`가 clean master(`1e36b15`)에서 30회 중 3회 실패합니다.
`parseEnvFileStrict`가 선언 순서를 잃는 맵을 돌려주고 `MergeVars`가 그 무순서 맵을 순회하며 값마다
보간하기 때문에, 같은 설정으로도 실행마다 값이 달라집니다. 이 카드의 변경과 무관하며 TASK-277로
열었습니다.

### 문서

`USAGE.md`에 「암호화된 소스 브리지」 절을 추가했습니다 — load-invisible 메타데이터, 두 명령,
"복호화는 사용자가 시킬 때만", 쓰기 전 검사(플랫폼·경로·Git·기존 타깃과 `--force`가 덮지 **않는**
것), 실패 시 보장(원자적 쓰기, 바이트 단위 보존, exit 1, `error.code`, 1시간 기준 소유 임시파일 회수,
그리고 **별도 복구 커맨드는 없다**는 사실), 그리고 dotenv 인용 주의(sops는 `TOK=v # c`를 통째로
보존하지만 DVA의 `parseEnvFile`은 ` #`에서 자르므로 따옴표가 필요합니다). `CHANGELOG.md`에도
기록했습니다.

## Non-goals

- No top-level `env` reservation.
- No `env show`.
- No age provider/key configuration in `dva.yml`.
- No automatic unseal from lifecycle commands.
