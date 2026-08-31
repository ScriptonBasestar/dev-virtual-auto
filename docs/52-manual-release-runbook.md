# 수동 공개 릴리스 런북

이 문서는 DVA의 현재 공개 릴리스 경계를 설명합니다. CI는 snapshot 검증만 수행하며,
GitHub tag와 Release 생성은 승인된 commit의 clean detached worktree에서 수동으로 진행합니다.
이미 공개된 tag는 이동하거나 다시 만들지 않습니다.

## 준비

1. 릴리스 commit을 source branch에 통합하고 push한 뒤 local/remote tip과 같은지 확인합니다.
2. 그 commit에 lightweight tag를 만들되 별도로 push하지 않습니다. GoReleaser가 공개 과정에서
   tag와 Release를 함께 생성합니다.
3. 해당 tag의 clean detached worktree를 만들고 그 루트로 이동합니다.
4. 검토된 release notes의 절대 경로와 SHA-256을 기록합니다.
5. `mise`가 고정한 GoReleaser를 사용합니다. 전용 fine-grained token은 대상 저장소의
   Contents read/write 권한을 가져야 하며 명령 환경에만 주입합니다.

아래 값은 해당 릴리스에 맞게 바꿉니다.

```bash
export RELEASE_TAG=v0.1.47
export RELEASE_COMMIT=<40-hex-commit>
export RELEASE_NOTES=/absolute/path/release-notes/v0.1.47.md
export RELEASE_NOTES_SHA256=$(shasum -a 256 "$RELEASE_NOTES" | awk '{print $1}')
```

## 실행 순서

전용 Keychain 항목을 사용하는 경우 token 값은 shell 변수로 영구 export하지 않고 각 명령에
직접 전달합니다. 서비스 이름은 로컬 운영 환경의 전용 릴리스 항목으로 바꿀 수 있습니다.

```bash
GITHUB_TOKEN="$(security find-generic-password \
  -s github-token-scriptonbasestar-dva-release -w)" \
  make release-preflight

GITHUB_TOKEN="$(security find-generic-password \
  -s github-token-scriptonbasestar-dva-release -w)" \
  goreleaser release --clean --release-notes "$RELEASE_NOTES" --verbose

make release-clean

GITHUB_TOKEN="$(security find-generic-password \
  -s github-token-scriptonbasestar-dva-release -w)" \
  make release-postflight
```

`release-preflight`는 detached/clean 상태, origin, local tag·HEAD·version·notes digest,
GoReleaser pin, remote tag/Release 부재, 비영속 GitHub write-capability probe를 fail-closed로
검사합니다. `release-clean`은 저장소 루트에서만 실제 `dist`, `bin`, `tmp` 디렉터리를
제거하며 symlink나 일반 파일은 거부합니다. `release-postflight`는 remote tag와 final
Release identity, 정확한 7개 asset을 검사하고, 모든 archive를 내려받아 공개
`checksums.txt`와 대조한 뒤 임시 다운로드를 제거합니다.

## 실패 처리

- preflight 실패 시 공개를 시작하지 않습니다.
- GoReleaser 실패 후 remote tag와 Release가 모두 없다면 local 출력만 정리하고 원인을
  수정한 뒤 같은 tag/commit에서 다시 시작합니다.
- remote tag 또는 Release가 하나라도 생겼다면 tag를 이동하지 않습니다. 같은 immutable
  identity에서 기존 draft/artifact 복구 정책을 사용하고 postflight가 통과할 때까지
  상태를 확인합니다.
- token 값이나 `security ... -w` 결과를 로그, task 문서, commit에 기록하지 않습니다.
