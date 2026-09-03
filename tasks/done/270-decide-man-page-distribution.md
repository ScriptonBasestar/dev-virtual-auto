---
id: TASK-270
title: "Decide whether DVA ships man pages (cobra/doc GenManTree)"
type: chore
priority: P2
effort: S
exec-tier: strong
created-at: 2026-09-03T00:56:00+09:00
source: "CLI discoverability audit, 2026-09-03 session (docs vs help gap review)"
scope: "decision only: man page generation, packaging surface, and maintenance cost; implementation is a follow-up card if adopted"
status: done
depends-on: []
---

# Task 270: decide man page distribution

## Summary

`man dva` does not work: no `cobra/doc` usage exists in the repo (`grep -rn "GenMan"` is
empty) and release archives carry only the binary and checksums. The user-facing promise is
"learn the tool from the CLI alone"; `--help` currently carries that alone. Decide whether
man pages add enough reach (offline packagers, `man`-first users, Homebrew/Linux distro
packaging conventions) to justify a generation step and a release-asset change — or record a
reasoned rejection so the gap stops resurfacing in audits.

## Problem

1. cobra makes generation nearly free (`doc.GenManTree` over rootCmd), but the real cost is
   distribution: a `man/` tree in release archives, install targets (`make install` currently
   copies one binary), and keeping generated pages in sync per release
   (`docs/52-manual-release-runbook.md` owns the release procedure).
2. Long help quality is being raised by TASK-268/269; generated man pages inherit that text,
   so sequencing matters — generating before those land would snapshot the thin help.
3. Precedent check needed: peer Go dev-tools (kubectl, helm, gh) each answered this
   differently (gh ships man via packaging; helm generates on demand).

## Completion Criteria

- [x] A decision record states adopt/reject with rationale covering: expected consumers, generation point (build-time vs release-time), distribution channel (archive layout, `make install` behavior), and sync guarantee per release | verify: human — decision record reviewed
- [x] If adopted: a follow-up implementation task exists specifying `doc.GenManTree` wiring, Makefile target, release-runbook (docs/52) delta, and ordering after TASK-268/269 | verify: human — follow-up card exists and is linked
- [x] If rejected: the rationale names `--help` + USAGE.md + `dva manifest` as the supported discovery surfaces so future audits close this line of inquiry by reference | verify: human — rationale recorded
- [x] This card's outcome is linked from TASK-268/269 if it changes their scope | verify: human — cross-links checked

## Non-goals

- No implementation in this card — generation, Makefile, and release packaging land only via the follow-up task if adopted.
- No shell-completion changes; `dva completion` already exists and is out of scope.
- No web-docs/static-site generation decision; this card is man(1) only.

## Decision (2026-09-03, claude/mst) — REJECT

**DVA는 man page를 배포하지 않는다.** 사용자 승인으로 확정.

### 1. 결정 근거 — man page가 도달할 수 있는 설치 경로가 없다

man page는 무언가가 `MANPATH` 아래에 설치해야만 `man dva`로 읽힌다. 현재 DVA의 설치
경로는 두 가지뿐이고 **둘 다 `MANPATH`를 건드리지 않는다**:

| 경로 | 실제 동작 | 근거 |
|---|---|---|
| `make install` | 바이너리 1개를 `~/.local/bin`에 설치 | `Makefile` `install:` → `install-binary` |
| 릴리스 아카이브 | 바이너리 + `checksums.txt` | `.goreleaser.yml` `archives:` / `checksum:` |

패키지 매니저 채널은 **0개**다 — `.goreleaser.yml`에 `brews:`, `nfpms:`, `aurs:`,
`scoops:`가 하나도 없다. man page가 사용자에게 닿는 통상 경로(Homebrew formula, deb/rpm)가
존재하지 않으므로, 생성해서 아카이브에 동봉해도 사용자가 손으로 `MANPATH`에 복사하지 않는
한 아무도 읽지 않는다.

`grep -rn "GenMan" --include="*.go" .` 는 비어 있다(카드 §Summary의 진술 확인).

### 2. 비용이 작지 않다

카드 §Problem 1은 "cobra makes generation nearly free"라고 쓰지만, 이 저장소에서 실제
비용은 생성이 아니라 **동기화 보장**이다. 공개 릴리스는 CI가 아니라 승인된 태그의 clean
detached worktree에서 **수동으로** 진행된다(`docs/52-manual-release-runbook.md`;
`.github/workflows/`에는 `ci.yml`만 있고 태그 트리거 워크플로우가 없다). 채택하면 수동
런북에 "man 재생성 + 커밋본과 일치 확인" 단계가 늘어나고, 그 단계를 건너뛴 릴리스는
**낡은 man page를 조용히 배포**한다. 읽히지 않는 산출물에 수동 릴리스 단계를 얹는 것은
순손실이다.

### 3. 지원하는 discovery 표면 (이 질문을 참조로 닫기 위한 명시)

DVA의 CLI 학습 경로는 다음 셋이며, man page는 여기에 포함되지 않는다:

1. **`dva <command> --help`** — 모든 커맨드가 `Long`을 가지며(TASK-268), 라이프사이클·`run`·
   `doctor`·`init`은 `Example:` 섹션을 렌더한다(TASK-269).
2. **`USAGE.md`** — Command Quick Reference 및 plan/vars 우선순위 등 산문 레퍼런스.
3. **`dva manifest`** — 기계 판독용 커맨드/옵션 목록(JSON). LLM·툴링의 정본.

향후 감사에서 "`man dva`가 동작하지 않는다"가 다시 제기되면 이 결정 기록을 참조로 닫는다.

### 4. 재오픈 트리거 (명시)

**`.goreleaser.yml`에 패키지 매니저 채널(`brews:`, `nfpms:`, `aurs:`, `scoops:` 중 하나)이
추가되는 시점**에 이 결정을 다시 판단한다. 그 순간이 man page가 사용자에게 실제로 도달할 수
있게 되는 순간이고, 동시에 배포 인프라가 이미 존재해 §2의 수동 동기화 비용이 사라지는
순간이기 때문이다. 그 전에는 조건이 바뀌지 않았으므로 재검토할 근거가 없다.

### 5. TASK-268/269와의 관계

카드 §Problem 2의 시퀀싱 우려("generating before those land would snapshot the thin help")는
해소됐다 — 268(`2d0b8fb`)과 269(`5eb1af5`)가 모두 착지한 뒤 이 결정을 내렸다. 거부이므로
두 카드의 범위는 바뀌지 않으며 교차 링크가 필요한 변경도 없다.

### 6. 후속 카드

없다. 거부 결정이므로 구현 카드를 발행하지 않는다.
