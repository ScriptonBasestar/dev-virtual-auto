---
id: TASK-030
title: "dva stack --help documents none of the flags it actually accepts, and --var appears in no help surface"
type: docs
priority: P3
status: done
effort: S
completed-at: 2026-07-17T03:50:00+09:00
created-at: 2026-07-17T02:00:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: fresh Phase 1 sweep (docs-vs-binary drift)
source-severity: LOW
---

# Task 030: `--help` Understates What The Binary Really Accepts

## Summary

`dva stack up --help` lists exactly one flag (`-h`). In reality `-M`/`--mode`, `-E`/`--env`,
`-T`/`--tag`, and `--exclude-tag` all parse and take effect. Separately, `--var` works on the
plan path and is documented in `USAGE.md:175`, but appears in **no** `--help` surface at all.

The binary silently does more than it admits. A user cannot discover working functionality from
the tool itself.

## Evidence

All measured at HEAD `b557e9f` against a rebuilt `bin/dva`. Every claim carries a control, so
each result is a measurement rather than an assumption.

### `dva stack up --help` advertises nothing

```
Usage:
  dva stack up [NAME...] [OPTIONS] [flags]

Flags:
  -h, --help   help for up

Global Flags:
      --debug / --dry-run / --json
```

```
mentions --mode = 0 · mentions --env = 0 · mentions --tag = 0
```

### …yet those flags demonstrably work

Config: env `dev` sets `TIER=devtier`; entry `s2` carries tag `extra`.

```
$ dva stack up s1            ->  TIER=[]            # control: no -E, var unset
$ dva stack up s1 -E dev     ->  TIER=[devtier]     # -E is real
$ dva stack up -T extra      ->  S2RAN only         # -T filters; s1 not run
$ dva stack up s1 -E nosuch  ->  ERROR: env 'nosuch' not found. Available: dev   # -E is genuinely parsed
```

The `-E nosuch` error is the decisive control: the flag is not merely tolerated and ignored, it
is parsed and validated.

### `--var` is in no help surface, but works

```
dva up         --help : --var mentions=0
dva stack up   --help : --var mentions=0
dva app up     --help : --var mentions=0
dva down       --help : --var mentions=0
```

```
$ dva up p1                     ->  FOO=[fromplan]   # control: plan vars applied
$ dva up p1 --var FOO=fromcli   ->  FOO=[fromcli]    # --var overrides. it works.
```

Documented at `USAGE.md:175` (`| --var KEY=VAL | 실행 시점 변수 override |`) — so the docs and
the binary disagree about the tool's own interface.

## Root cause

Commands with `DisableFlagParsing: true` bypass cobra's flag registration, so cobra has nothing
to render — the help text must be written by hand in the command's `Long` field.

`internal/cli/compose.go` does this correctly: all four of its commands carry a hand-written
"DVA-specific flags" block. `internal/cli/stack.go` does not — its `up`, `down`, `stop`, and
`log` commands all set `DisableFlagParsing: true` and document none of what they accept.

```
$ grep -rn "DVA-specific flags" --include="*.go" internal/cli/
  -> 4 occurrences, all in compose.go. stack.go: none.
```

So the gap is not a missing feature; it is help text that was never written for a command family
that opted out of automatic help.

## Severity: LOW

Nothing here is a wrong action — no unintended mutation, no silent scope change. The cost is
discoverability: working, documented-elsewhere capability is invisible from the binary. Recorded
LOW on its own evidence.

Worth noting for future audits: this gap is *why* `--help` is unreliable as a source of truth in
this repo. During TASK-027 the orchestrator nearly rejected a correct agent finding by trusting
`dva stack up --help`'s empty flag list over the binary's real behavior. The help surface has
already caused one near-miss.

## Relationship to TASK-028 (partial entanglement — read before fixing `--var`)

`--var` is only honored on the **plan** path (`parsePlanFlags`); off that path it is silently
swallowed (evidence in TASK-027). So `--var` must be documented as plan-path-only, not as a
general `up` flag.

Whether bare `dva up --var` *should* work is exactly TASK-028's undecided question. Documenting
current true behavior is safe and decision-independent; do not use this task to change which
paths honor `--var`. The `-M`/`-E`/`-T` half of this task has no such entanglement and can
proceed freely.

## Completion Criteria

- [ ] `dva stack up --help` lists the flags it accepts (`-M`/`--mode`, `-E`/`--env`, `-T`/`--tag`, `--exclude-tag`) | verify: `/Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva stack up --help 2>&1 | grep -q -- "--mode" && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva stack up --help 2>&1 | grep -q -- "--env" && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva stack up --help 2>&1 | grep -q -- "--tag"`
- [ ] `dva stack down` and `dva stack stop` help likewise document their accepted flags | verify: `/Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva stack down --help 2>&1 | grep -q -- "--mode" && /Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva stack stop --help 2>&1 | grep -q -- "--mode"`
- [ ] `--var` is documented on the plan-capable help surface, stated as plan-path-only | verify: `/Users/archmagece/mywork/scripton/dev-virtual-auto/bin/dva up --help 2>&1 | grep -q -- "--var"`
- [ ] Every flag newly claimed in help text is verified to actually work, not just written down | verify: `human — re-run this task's Evidence probes; help must not overstate the binary any more than it understated it`
- [ ] `make test` and `go vet ./...` pass | verify: `make test && go vet ./...`

## Outcome

Done. `stack.go`'s `up`/`down`/`stop` gained hand-written "DVA-specific flags" blocks matching
compose.go's layout; `compose.go`'s `up`/`down`/`stop` gained a separate "Plan-path flags" section
documenting `--var … Ignored off the plan path.` Docs only — the diff touches no behavior line, and
`restart` is untouched (0 mentions), so TASK-033 merges cleanly in either order.

Independently re-verified in a scratch worktree containing **only** this change (the main worktree
held unrelated in-flight edits to the same package, which would have contaminated the result):
`make test` exit 0, `go vet ./...` exit 0, all three criterion `verify:` commands exit 0.

`--var` was placed under its own heading rather than merged into the DVA-specific list, so the help
does not imply it behaves like the others. TASK-028's question is untouched.

### Two corrections this task produced

**`stack log` was correctly left alone.** This file's root-cause section groups `log` with `up`/
`down`/`stop` as commands that "document none of what they accept". True as to `DisableFlagParsing`,
but misleading: `stackLogCmd` never calls `parseDvaFlags` and accepts none of these flags. Proven —
`dva stack log s1 -E nosuch` fails with docker compose's `unknown shorthand flag: 'E' in -E`, not
DVA's `env 'nosuch' not found`. Documenting them there would have swapped an understating help
surface for a lying one. Negative control confirms the fix did not overreach:
`stack log --help` → 0 mentions of `--mode`, `stack status --help` → 0.

**Two flags deliberately left undocumented, on evidence.** `--force`/`--no-wait` (`stack up`) and
`-v`/`--volumes` (`stack down`) are parsed by the command bodies, but `internal/lifecycle/script.go`
never references `Force`, `Wait`, or `Volumes`, and a `--dry-run` probe produced byte-identical
output with and without them. Unproven, so unwritten — which is this task's own standard. `stack
down -v` is destructive and worth documenting once someone can prove what it does.

## Follow-ups this task surfaced (not filed — no evidence of user harm yet)

- `dva stack log` accepts no DVA flags while its three siblings do. Either wire in `parseDvaFlags`
  or accept the asymmetry deliberately.
- `--force` / `--no-wait` / `--volumes` may be parsed-but-inert on the script runner — the same
  silent-no-op shape as TASK-034/035/036. Needs a probe against a compose-backed entry before
  filing; `stack down -v` being destructive makes it the one to check first.

## References

- [027-up-silently-ignores-unknown-args.md](../_archive/027-up-silently-ignores-unknown-args.md) — records `--var`'s absence from help as out of scope; this task picks it up
- [028-flag-suppresses-default-plan-route.md](./028-flag-suppresses-default-plan-route.md) — owns whether `--var` should work off the plan path
- [033-restart-discards-service-names.md](../_archive/033-restart-discards-service-names.md) — adjacent hunk in compose.go; this task's +9 line shift is why 033 cites its code by content rather than line number
