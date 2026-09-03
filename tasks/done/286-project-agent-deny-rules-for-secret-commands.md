---
id: TASK-286
title: "Project agent-runtime deny rules for the commands agents must not run"
type: feature
priority: P2
effort: M
exec-tier: strong
created-at: 2026-09-03T19:40:00+09:00
source: "TASK-281 §3-6 — the runtime layer is the only one that knows its caller is an LLM"
scope: "canonical deny list, per-runtime projection targets, install/status/uninstall ownership model, drift verification, init integration boundary"
status: done
depends-on: [TASK-281]
---

# Task 286: project agent deny rules for secret-exposing commands

## Summary

DVA는 자기 호출자가 LLM인지 영원히 알 수 없다 (TASK-281 §3-6). 그 사실을 아는 유일한 계층은
**에이전트 런타임 자신**이고, 그 계층은 이미 도구 호출 전에 명령을 차단하는 permission 규칙을
갖고 있다. 이 카드는 DVA가 그 규칙을 **배포**하게 한다 — 스스로 판정하는 대신, 판정할 수 있는
계층이 쓸 규칙을 제공한다.

```jsonc
// 예: Claude Code
{ "permissions": { "deny": ["Bash(dva config env show *)"] } }
```

## Why not `dva init`

`init`에 넣자는 것이 자연스러운 제안이지만 세 가지 이유로 틀린 자리다.

1. **드리프트.** `init`은 프로젝트당 한 번 실행되고 아무도 다시 실행하지 않는다. deny 규칙은
   명령 표면을 따라가야 하는데, 명령이 추가돼도 규칙은 그대로 남는다. **드리프트한 보안 규칙은
   없는 것보다 나쁘다** — 사람들이 막혀 있다고 믿기 때문이다.
2. **기존 프로젝트가 못 받는다.** `init`은 새 프로젝트만 건드린다. 정작 secret이 쌓여 있는
   것은 이미 오래 굴러간 저장소다.
3. **소유권.** `.claude/settings.json`은 DVA가 만들지 않은, 사용자·팀이 소유하는 파일이다.
   `init`은 그 파일에 대해 아는 것이 가장 적은 시점에 통째로 쓰게 된다. 남의 설정을 덮어쓰는
   lost update는 TASK-281이 `seal`에서 막으려는 것과 같은 종류의 사고다.

## What to reuse instead

이 저장소는 이 문제를 이미 두 번 풀었다. 새 기계를 만들지 않는다.

- **`skills/_targets.yaml`** — 단일 canonical source를 런타임별 형식으로 투영하는 매니페스트다.
  `shape`, `output`, `generated`, 그리고 남의 파일에 쓸 때의 `merge: section` + `marker`까지
  이미 있다. deny 규칙도 같은 모양의 target이다.
- **`internal/skillinstall`** — `install`/`status`/`uninstall`/`backup`, `--scope user|project`,
  로컬 수정 감지, **수정되지 않은 DVA 소유 설치만 제거**, `--takeover` 백업 보존. "남의 설정
  디렉터리에 안전하게 쓰고 되돌리기"의 어려운 부분이 전부 여기 있다.
- **`make generate` / `make check-generate`** — 생성물이 소스와 어긋나면 CI가 실패한다.
  deny 규칙을 생성물로 만들면 §1의 드리프트가 기계적으로 잡힌다.

`init`의 역할은 쓰는 것이 아니라 **권하는 것**이다 — capability 감지 결과에 따라 안내하고
설치를 제안한다. 그 경계는 [TASK-249](249-redesign-capability-driven-init.md)가 소유하므로
이 카드는 `init`이 파일을 쓰게 만들지 않는다.

## Honest limits

- deny 규칙 파일은 사용자도 에이전트도 쓸 수 있는 일반 파일이다. 이것은 **런타임이 자기 설정
  파일을 신뢰한다는 전제 위의 정책 계층**이지, DVA가 강제하는 경계가 아니다.
- 따라서 실제 위협 모델은 "적대적 에이전트"가 아니라 **"하지 말라는 말을 못 들은 순종적인
  에이전트"**다. 그 모델에 대해서는 매우 효과적이고, 그 사실을 문서가 정확히 말해야 한다.
- deny 형식이 없는 런타임이 있을 수 있다. 그런 런타임은 "미지원"으로 명시 기록한다 —
  조용히 빠뜨리면 목록을 본 사람이 커버됐다고 오해한다.

## Decisions and work

- Canonical deny 목록의 단일 소스 위치와, 그것이 게이트 대상 명령과 어긋나지 않게 하는 방법.
- 대상 런타임과 각 deny 형식. `skills/_targets.yaml`의 현재 target(claude-code, antigravity,
  opencode, cursor, codex) 중 permission 규칙을 실제로 갖는 것이 어디인지 확인한다.
- argv 변형 커버리지. `dva config env show`, `dva config env show .env`, `dva  config env show`
  (공백 변형), 그리고 `dva run` 경로로 우회되는 형태가 있는지.
- 설치 소유권 모델을 `skillinstall`과 공유할지, 별도로 둘지.
- `init` 통합 경계 — 안내만 하고 쓰지 않는다는 것을 TASK-249와 맞춘다.

## Completion Criteria

- [x] Define the canonical deny list as a single source and bind it to the gated command set so a new gated command cannot ship without a rule | verify: `make check-generate`
  - `internal/agentdeny/rules.go`'s `GatedCommands` (2 entries: `config-env-seal`, `config-env-show`, argv from `tasks/done/281-freeze-gated-env-bridge-commands.md` §3-5/§3-7) is the single source. `GatedCommand.Patterns()` projects each entry to exactly one `Bash(<argv> *)` pattern — the `Bash(...)` tool wrapper and the space before the trailing `*` are both required by Claude Code's own permission-parsing rules, not cosmetic (an earlier revision emitted a bare, unwrapped argv string that enforced nothing; see criterion 3 below for the fix). `tools/agentdenygen` renders the list into `docs/agent-deny-rules.md`; `make generate` runs it and `make check-generate` diffs the result — confirmed clean (`make check-generate` exit 0 after `make generate`, zero diff on `docs/agent-deny-rules.md`).
  - Not yet bound to the *live* cobra command tree: TASK-282 has not landed `config env seal`/`show` in this checkout (only `unseal`/`edit` exist), so there is no command tree to cross-check against. Recorded as a named follow-up in the generated doc's "Binding this list to the CLI" section, not silently assumed done.
- [x] Declare the per-runtime projection targets and record every runtime that has no permission mechanism as explicitly unsupported | verify: human — a runtime may not be silently omitted from the coverage table
  - Runtime coverage table (also in `docs/agent-deny-rules.md`):

    | Runtime | Status |
    |---|---|
    | Claude Code | **Implemented** — `.claude/settings.json` `permissions.deny` array, `dva agent-deny install/status/uninstall --scope user\|project` |
    | OpenCode | Researched, not implemented — exact deny-array key/glob semantics not independently verified against a live install |
    | Antigravity | Researched, not implemented — no independently verified settings-file/permission-key documentation found |
    | Cursor (CLI/editor) | Researched, not implemented — `.cursor/rules/*.mdc` is a context-injection projection, not a permission-enforcement one; no verified deny-rule file format confirmed |
    | Codex CLI | Researched, not implemented — an experimental execpolicy mechanism was referenced but not verified against a shipped, stable format |
    | Grok CLI (xAI) | **Unsupported** — no independently verifiable deny/permission mechanism found; a plausible-sounding tool name surfaced in research but could not be corroborated against an authoritative source |
    | agent-mesh (`am`) | **Not applicable** — DVA's own analysis/automation tooling, not a third-party agent runtime calling the `dva` CLI on a user's behalf |

    Every runtime this research considered is listed explicitly (none silently omitted); only Claude Code ships an implemented, tested projection this card.
- [x] Cover the argv variants a deny pattern must match, including any route that reaches the same command by another spelling | verify: `make test`
  - Corrected after independent review found the shipped mechanism enforced nothing: `Patterns()` now emits exactly one pattern per gated command, in the literal form `Bash(<argv> *)` — the `Bash(...)` wrapper names the Bash tool (a bare, unwrapped string names no tool and matches nothing), and the space before the trailing `*` matches the bare command plus any trailing arguments/flags without over-matching a neighboring command that merely shares a text prefix (e.g. `seal` vs. `sealed`). The earlier per-token "doubled-space" variants were dropped — Claude Code parses and normalizes command text rather than doing a raw string compare, so they added file noise without covering anything the wrapped-and-spaced form doesn't.
  - `internal/agentdeny/match_test.go`'s `TestGatedCommandPatternsCoverArgvVariants` pins the exact literal string per command and proves both the must-match argv variants (bare command, trailing flag/argument) and the must-not-match neighbors (`seal`/`sealed`, `show`/`showall`); `TestMatchDenyPatternBasics` proves the wrapper requirement and the docs' own `Bash(ls *)`-vs-`lsof` example. `internal/agentdeny/deploy_test.go`'s `TestInstallWritesWrappedAndSpacedPatternsToDisk` re-proves the same thing against the literal bytes `Install` actually writes to a settings file, not just `Patterns()`'s in-memory output — this is the test the reviewer explicitly asked for, because every other test in this package compares generated output against itself and would not have caught the missing wrapper.
  - `dva run`-interaction bypass, shell-obfuscation, path-qualified invocation (`./bin/dva ...`), and environment-runner wrappers (`mise exec --`, `devbox run`, `direnv exec`, `docker exec`) are explicitly named as *not* covered — see "Honest limits" below, criterion 7, and the generated doc's `TestKnownUncoveredInvocations`-backed list.
- [x] Reuse the skillinstall ownership model: scope selection, local-modification detection, uninstall limited to unmodified DVA-owned installs, and backup retention | verify: `make test`
  - Reused directly: `skillinstall.Scope`/`ScopeUser`/`ScopeProject` (import, not reimplementation), receipt-based local-modification detection, and uninstall fail-closed on any drift (`internal/agentdeny/deploy_test.go`'s `TestUninstallRefusesOnLocalModification`, `TestUninstallRemovesOnlyDVAOwnedEntries`).
  - **Not reused: backup/takeover retention.** Deliberate scope decision, not an oversight, confirmed by independent review — skillinstall's `--takeover`/backup machinery solves a foreign-directory-collision problem (a skill install owns a whole destination directory that can collide with a non-DVA-created one of the same name). A deny-rule install only ever merges known strings into one JSON array in a file the user may already own; it never replaces or deletes unrecognized content, so there is no foreign collision for a takeover/backup step to resolve. Documented in `docs/agent-deny-rules.md`'s "Ownership model" section.
  - Receipt ownership is now delta-based, fixed after review: `Install` records as DVA-owned only the patterns *this call actually added* (`internal/agentdeny/deploy.go`'s `newlyAdded`, diffed against the pre-merge deny array), not the full desired set. Before this fix, a pattern the user had already hand-written before ever running `install` would be falsely claimed by the receipt and deleted by a later `uninstall` — the exact clobber class this criterion exists to prevent. Covered by `internal/agentdeny/deploy_test.go`'s `TestInstallDoesNotClaimOwnershipOfPreExistingUserPattern`.
  - Caveat, confirmed genuine and left as-is: what's lost vs. `skillinstall` is formatting/key-order of the touched JSON containers only (re-serializing a Go map does not reproduce arbitrary source whitespace or key ordering) — cosmetic diff-noise, not content loss, and already stated in `docs/agent-deny-rules.md`'s "What 'never clobbers' means here". A single `.bak` sidecar on first modification would close even that gap cheaply; recorded as a possible future improvement, not implemented this card.
- [x] Prove the projection never clobbers a user-owned region of a shared settings file | verify: `make test`
  - `internal/agentdeny/deploy_test.go`'s `TestInstallDoesNotClobberSiblingContent`: sibling JSON keys/arrays (including pre-existing `permissions.allow`/`permissions.ask` and unrelated top-level keys) survive install byte-for-byte in *content*. Not preserved: exact source formatting/key order of the containers touched (re-serializing a Go map does not reproduce arbitrary whitespace) — a recorded, deliberate trade-off, not a content loss.
- [x] Keep `init` to guidance only, with the boundary agreed against TASK-249 | verify: human — init must not write an agent settings file in this card
  - `dva init` does not write `.claude/settings.json` or any other agent settings file in this card: `grep -rn "skillinstall\|agentdeny" internal/cli/init.go internal/cli/init_scaffold.go` returns zero matches. Deploying deny rules stays an explicit, separate `dva agent-deny install` invocation. Whether/how `init` should *recommend* running it is left to TASK-249, per this card's own "Why not `dva init`" section.
- [x] Document the layer as a policy control enforced by the runtime, not a boundary DVA enforces, and state the residual pty hole from TASK-281 §3-6 | verify: `make doc-check`
  - `docs/agent-deny-rules.md`'s "Honest limits" section states the threat model is a compliant-but-uninstructed agent, not an adversarial one; names shell obfuscation/wrapper scripts/`dva run` bypass and pty hijacking (TASK-281 §3-6's residual gap) as explicitly out of scope, and restates TASK-281 §3-6's layered strength ordering (DVA advisory detection < `/dev/tty`-only output < this runtime deny rule < `allow_show: false`, the default and strongest). `make doc-check` passes (`doc-check: OK`, `oversized_docs: 0`).
  - Two more genuine gaps added after independent review, both real per Claude Code's own docs and previously undocumented: **path-qualified invocation** (e.g. `./bin/dva config env show` — this repo's own `make build` produces `bin/dva`) is not covered, because the pattern is anchored on the literal argv `dva ...` and a path prefix is a different literal string; and **environment-runner wrappers** (`mise exec --`, `devbox run`, `direnv exec`, `docker exec`) are not covered, because Claude Code's documented stripped-wrapper list (which does cover a leading env assignment and `timeout`/`nice`/`command`/`xargs`) explicitly excludes these. Both are in `internal/agentdeny/match_test.go`'s `TestKnownUncoveredInvocations` and the generated doc.
  - Also corrected two overstated claims review flagged: `internal/agentdeny/match.go`'s doc comment no longer claims its toy verifier's "semantics are the shared subset, not a guess" (it doesn't model wrapper-stripping, env-assignment stripping, or operator splitting — corrected to say so explicitly); and `match_test.go`'s test that used to assert `FOO=bar dva config env show` was *uncovered* was replaced by `TestVerifierDoesNotModelClaudeCodeWrapperStripping`, which states the opposite and correct fact — Claude Code's real matcher strips a leading env assignment before matching, so that argv *is* blocked by the real runtime even though this package's simplified test verifier can't prove it.

### Verification run (rebased and re-verified after independent review found a blocking defect)

An independent review, performed by actually running `dva agent-deny install` against a
throwaway settings file rather than trusting the unit tests, found the originally shipped
`Patterns()` emitted bare, unwrapped argv strings with no space before the trailing `*` —
these enforce **nothing** against Claude Code, because a bare deny entry names a tool, not
a command pattern. All 12 tests that existed at that point were green because they only
compared generated output against itself. Fixed (see criteria 1, 3, 4, 7 above), the
worktree was rebased onto current `origin/master` (9 commits, including TASK-258's
overlapping `internal/cli/manifest.go` change and TASK-250's `init` rewrite — no conflicts,
both re-verified as non-interfering), and the full gate suite was re-run clean on the
rebased, fixed state:

```
$ make build            # PASS — includes make generate (agentdenygen wired in)
$ gofmt -l .            # clean after `gofmt -w internal/agentdeny/match_test.go` (alignment only)
$ go test -race -cover ./...   # PASS — every package ok, no failures, no data races
$ make check-generate   # PASS — exit 0, zero diff after regenerating
$ make doc-check        # PASS — doc-check: OK, oversized_docs: 0
```

Manual on-disk proof (not just unit tests — the same check the reviewer used to catch the
original defect), against a throwaway `$HOME` and project directory:

```
$ HOME=$TMP/home dva agent-deny install --scope project   # (run from $TMP/project)
installed  claude-code  $TMP/project/.claude/settings.json

$ cat $TMP/project/.claude/settings.json
{
  "permissions": {
    "deny": [
      "Bash(dva config env seal *)",
      "Bash(dva config env show *)"
    ]
  }
}
```

Confirms the on-disk deny entries carry both required properties: the `Bash(...)` tool
wrapper and the space before the trailing `*`. `status` reported `installed` and
`uninstall` removed exactly those two entries, leaving `"deny": null`.

Full `internal/cli` package (`go test ./internal/cli/...`, 3 static-manifest/reserved-command
consistency tests: `TestAllCommandsHaveLongHelp`, `TestStaticCommandsCoverEveryRootCommand`,
`TestStaticCommandDescriptionsMatchTheirShort`, `TestStaticCommandsAgreeWithReservedCommands`)
required adding `agent-deny`'s `Long` help text, its `StaticCommands` manifest entry, and its
entry in `internal/config/reserved.go`'s `reservedCommands` — all three are pre-existing
repo-quality gates that fire on any new root command, not TASK-286-specific work.
