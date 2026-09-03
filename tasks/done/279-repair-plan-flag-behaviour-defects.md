---
id: TASK-279
title: "Repair lifecycle flags that are accepted and then discarded"
type: bug
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-03T12:55:00+09:00
source: "TASK-273 audit — surfaced as evidence there, excluded from its scope as behaviour rather than guidance defects; §3 added from the TASK-273 implementer's measurement"
scope: "internal/cli/plan_lifecycle.go restart/stop/down plan routes, the build route in internal/cli/compose.go, internal/lifecycle StopOptions/DownOptions"
status: done
closed-at: 2026-09-03T13:56:31+09:00
depends-on: []
---

# Task 279: repair lifecycle flags that are accepted and then discarded

## Summary

Three lifecycle routes parse a flag and then throw the value away. `restart` overwrites
`--force` with a hardcoded `true`, so the flag does nothing and restart force-recreates whether
or not it was typed. `stop` and `down` accept `--no-wait` and pass it to option structs with no
field to receive it. `build` discards `--env`, `--tag` and `--exclude-tag` at the parse call
itself. Every one of them answers exit 0, which reads as "your flag was honoured".

Opened at effort S, when the card held only §1 and §2 and both looked like one-line repairs.
Raised to **M** once §3 landed: the three defects sit on three different routes, the direction
choice below is not the same on all of them, and the last criterion asks for a regression test
covering all four routes at once. Treat the estimate as covering the decision, not just the
edits.

## Problem

1. **`restart` discards `flags.force` and hardcodes `Force: true`.**
   `runPlanRestart` builds `lifecycle.UpOptions{DryRun: effectiveDryRun, Force: true, Wait:
   flags.wait, ...}` (`/usr/bin/grep -n 'Force:  true' internal/cli/plan_lifecycle.go`), while
   the `up` route one screen earlier passes `Force: flags.force` faithfully
   (`/usr/bin/grep -n 'Force:  flags.force' internal/cli/plan_lifecycle.go`). Two consequences,
   and the second is the worse one:

   - `dva restart <plan> --force` and `dva restart <plan>` are the same command. The flag is
     accepted and has no effect.
   - Restart force-recreates unconditionally. A user who did *not* ask for `--force-recreate`
     gets it anyway, which for the compose plugin means containers are destroyed and rebuilt
     rather than restarted.

   The manifest describes the option as `Compose only: pass --force-recreate; other plugins
   ignore it` (`optForce`, `/usr/bin/grep -n 'optForce' internal/cli/manifest.go`). On the
   restart route that description is false in both directions: passing it changes nothing, and
   not passing it does not prevent it.

2. **`stop` and `down` accept `--no-wait` into a struct that cannot hold it.**
   `parsePlanFlags` sets `flags.wait = false` for `--no-wait`
   (`/usr/bin/grep -n 'flags.wait = false' internal/cli/plan_lifecycle.go`), but
   `lifecycle.StopOptions` and `lifecycle.DownOptions` declare no `Wait` field — only
   `UpOptions` does (`/usr/bin/grep -n 'type StopOptions' -A8 internal/lifecycle/orchestrator.go`).
   The value is parsed, stored, and never read.

   Measured against the binary built at `fdc6925`, on a fixture with one plan `local-dev`:

   ```
   $ dva stop local-dev --no-wait --dry-run   → [lifecycle] stopping db / stopping web   exit 0
   $ dva down local-dev --no-wait --dry-run   → [lifecycle] stopping db / stopping web   exit 0
   ```

   Neither warns. The manifest does not advertise `--no-wait` on `stop` or `down`
   (`/usr/bin/grep -n '"down":' -A2 internal/cli/manifest_static_commands_test.go`), so the
   parser is more permissive than the advertised surface — the flag is neither documented nor
   rejected, just absorbed.

3. **`build` discards `--env`, `--tag` and `--exclude-tag` at the parse site.**
   `buildCmd`'s `RunE` calls `mode, _, _, _, remaining, err := parseDvaFlags(args)`
   (`/usr/bin/grep -n 'mode, _, _, _, remaining, err' internal/cli/compose.go`). The four
   selectors are all parsed — which is why none of them leaks through to docker — but only
   `mode` is bound to a name. The env, include-tag and exclude-tag return values go to `_`.

   This is worse than a no-op in one specific way: on the stack path `dva up --env prod` against
   a config declaring no `environments:` *fails* with `env 'prod' not found`, because the parsed
   value is looked up. On the build route the same flag is silently accepted, because the value
   never reaches a lookup. The same flag on two routes of the same tool gives an error on one and
   silence on the other.

   Measured by the TASK-273 implementer on a plan fixture: `dva build local-dev --exclude-tag app`
   still built the entry tagged `app`, and `--env prod` did not fail against a config declaring no
   `environments:`. The code reading above is the reason.

   `--mode` is not affected — it is bound and used, which is why TASK-273 could give it an
   accurate manifest qualifier while the other three could not be described honestly at all.

## Direction

Two directions, and the card does not prejudge which:

- **(a) Honour the flags.** Pass `flags.force` through on `restart`; add `Wait` to
  `StopOptions`/`DownOptions` and thread it. This makes the accepted flags mean what their
  names say.
- **(b) Reject what is not implemented.** Make `parsePlanFlags` verb-aware so `--no-wait` on
  `stop`/`down` fails the way an unsupported plan flag already does, and decide explicitly
  whether restart's force is a flag or a property of the verb — if it is a property, say so in
  the manifest and reject `--force` there.

Direction (a) is the smaller change for `restart` and the larger one for `stop`/`down`, because
"wait" has no meaning for every plugin.

`build` (§3) is the one place where the two directions are not equally available. It cannot
simply reject the three selectors: `parseDvaFlags` is what keeps them from leaking into docker's
argv, so the parse has to stay. The choice is between binding the values and using them, and
binding them only to fail on an unsupported combination. Whichever is chosen, `--env` must stop
being silent on this route while it errors on the stack route.

An implementer should not mix directions across the defects without saying why in the commit.

## Completion Criteria

- [x] `dva restart <plan> --force` and `dva restart <plan>` are distinguishable — either the flag reaches the orchestrator, or it is rejected on this route | verify: `go test ./internal/cli -count=1`
- [x] Restart no longer force-recreates on behalf of a user who did not ask for it, or the manifest states that restart always force-recreates | verify: `go test ./internal/cli -count=1`
- [x] `--no-wait` on `stop`/`down` either reaches the orchestrator or is rejected; it is not silently absorbed | verify: `go test ./internal/cli -count=1`
- [x] `build` either honours `--env`/`--tag`/`--exclude-tag` or rejects them; in particular `--env NAME` against a config with no `environments:` does not stay silent on the build route while failing on the stack route | verify: `go test ./internal/cli -count=1`
- [x] A regression test pins the chosen behaviour for all four routes, so a later refactor cannot quietly restore the discard | verify: `go test ./internal/cli -count=1`
- [x] `optForce`'s manifest text matches what every route actually does with the flag | verify: `human — read optForce against the up and restart call sites: the description must hold on both routes, not only on up`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No change to `up`, which already passes `Force` and `Wait` faithfully.
- No change to which flags `parsePlanFlags` accepts beyond the ones named here.
  `--tag`/`--exclude-tag`/`--mode`/`--env` are path-conditional, and their *guidance* — what
  help text, the manifest and error strings say about them — was settled by
  [TASK-273](273-repair-misleading-cli-guidance.md), which closed as `206918a`. What that card
  deliberately left behind is the *behaviour* on the build route, which is §3 here. Read its
  landed diff before starting: it fixed the descriptions, so a description that now looks
  wrong is more likely to be §3's behaviour showing through than a missed string.
- No change to the `--purge` confirmation gate, which was reviewed and closed in
  [PLAN-004](../plan/004-restore-documentation-truth.md).

## Troubleshooting Log

- 2026-09-03 — 증상: `internal/cli/compose.go`에 §3 수정(`unsupportedBuildSelectors` 추가)을
  적용하자 파일 크기 검증 hook이 500 code-line 초과 경고를 띄움 / 원인: `git show
  HEAD:internal/cli/compose.go | wc -l`로 확인한 결과 편집 전에도 이미 829 code lines로
  임계값을 넘어선 상태였고, 이번 편집은 829→842로 13줄만 늘린 것 — 파일이 이미 이 카드
  범위 밖의 선행 조건으로 초과 상태였음 / 해결: 근본 원인이 이번 변경과 무관함을 확인하고
  분리 없이 그대로 진행 — `compose.go` 분할은 이 카드의 세 결함 수리와 무관한 별도의 큰
  리팩터이므로 범위에 포함하지 않음 / 걸린시간: 약 10분

- 2026-09-03 — 발견(수리 아님): 전체-스택 경로(`internal/cli/compose.go`의
  `restartCmd.RunE`, `NewOrchestrator` 사용)에도 §1과 동일한 모양의 `Force: true, Wait:
  true` 하드코딩이 남아있음. 다만 이 경로에서는 `--force`가애초에 `stackSelectorFlags`
  허용 목록에 없어 미지원 플래그로 거부되므로 "받고 버림"이 아니라 "애초에 받지 않음"이라
  이 카드의 결함 형태(수용 후 폐기)와 다르고, scope 필드도 "plan routes"로 명시했으므로
  손대지 않음 — 향후 리뷰어가 같은 코드를 다시 발견하고 놀라지 않도록 기록. 같은 이유로
  restart의 `manifest.go` Options 목록에는 `force` 키가 아예 없음(`optForce`가 나열되지
  않음) — up과 달리 restart는 이번 수정 전까지 `--force`가 있으나 없으나 결과가 같았기
  때문으로 보이며, criterion 6의 검증 방식(콜사이트를 직접 읽는 human 검증)은 통과하지만
  `--help` 상에는 여전히 문서화되지 않은 채로 남음 — manifest.go는 이 카드의 선언된
  scope 밖이라 추가하지 않음.
