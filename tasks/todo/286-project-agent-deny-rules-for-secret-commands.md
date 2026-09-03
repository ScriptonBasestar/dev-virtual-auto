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
status: todo
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
{ "permissions": { "deny": ["Bash(dva config env show*)"] } }
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

- [ ] Define the canonical deny list as a single source and bind it to the gated command set so a new gated command cannot ship without a rule | verify: `make check-generate`
- [ ] Declare the per-runtime projection targets and record every runtime that has no permission mechanism as explicitly unsupported | verify: human — a runtime may not be silently omitted from the coverage table
- [ ] Cover the argv variants a deny pattern must match, including any route that reaches the same command by another spelling | verify: `make test`
- [ ] Reuse the skillinstall ownership model: scope selection, local-modification detection, uninstall limited to unmodified DVA-owned installs, and backup retention | verify: `make test`
- [ ] Prove the projection never clobbers a user-owned region of a shared settings file | verify: `make test`
- [ ] Keep `init` to guidance only, with the boundary agreed against TASK-249 | verify: human — init must not write an agent settings file in this card
- [ ] Document the layer as a policy control enforced by the runtime, not a boundary DVA enforces, and state the residual pty hole from TASK-281 §3-6 | verify: `make doc-check`
