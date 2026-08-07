---
id: TASK-171
title: "a declined `clean` exits 0, so a script cannot tell it from a completed one"
type: bug
priority: P3
effort: S
created-at: 2026-08-03T17:30:00+09:00
source: "TASK-170 — measured while confirming the prompt still guards the real path"
depends-on: [TASK-170]
scope: "dva repo — internal/cli/compose.go"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: confirmDestruction EOF error; purge decline exit semantics
verification-summary: |
  quality-review pass; re-checked deliverables. confirmDestruction EOF error; purge decline exit semantics. Shared: make test OK; make doc-check OK (mise go 1.26.4).
---

# Task 171: Make a declined `clean` distinguishable from one that ran

## Problem

`cleanCmd`'s confirmation prompt returns `nil` when the answer is not `y`
(`internal/cli/compose.go`, the `Aborted.` branch). Measured after TASK-170:

```
$ dva clean --volumes </dev/null
This will remove all containers, networks, and VOLUMES (data loss!).
Continue? [y/N] Aborted.
$ echo $?
0
```

A caller that checks the exit code — which is the only thing a script can check — is told
the clean succeeded. Nothing was removed.

Two different situations are being collapsed into that one silent zero:

1. **A person typed `n`.** They meant it, and rc 0 is arguably right: the command did what
   they asked. This is the case worth being careful about changing.
2. **`fmt.Scanln` hit EOF**, because there is no terminal — a pipe, a CI runner, a Makefile
   recipe. Nobody declined anything. There was no way to answer, and the command reports
   success for work it did not do.

Case 2 is the one with no defensible reading. `--force` exists precisely for this situation,
and the fix is to say so rather than to guess an answer on the operator's behalf.

TASK-170 removed this trap from the `--dry-run` path by exempting the preview from the
prompt entirely. The real path still has it.

## Acceptance criteria

- [x] `dva clean --volumes </dev/null` (no terminal, no `--force`) fails rather than
      silently succeeding, and its message names `--force` as the way to proceed
      non-interactively.
      Verify: `d=$(mktemp -d) && printf 'version: "0.1.44"\nproject_name: t171\n' > $d/dva.yml && (cd $d && dva clean --volumes </dev/null); test $? -ne 0`
- [x] An interactive `n` keeps whatever exit code the task decides on, and that decision is
      written down with its reasoning. Changing it is a compatibility question — a wrapper
      script today may well treat `n` as "fine, carry on".
      Verify: `human — state the decision and the reason in the Result section`
- [x] EOF is distinguished from an explicit decline at the call site, not inferred. `Scanln`
      returns an error on EOF; the current code discards it (`_, _ = fmt.Scanln(&answer)`).
      Verify: `grep -n 'fmt.Scanln' internal/cli/compose.go` shows the error being read
- [x] A test covers both arms — EOF and an explicit `n` — through `cleanCmd.RunE`, asserting
      on the returned error rather than only on output. `internal/cli/clean_prompt_test.go`
      has the stdin and stream capture helpers already.
      Verify: `go test ./internal/cli/ -run Clean -count=1`
- [x] `make test` exits 0.
      Verify: `make test`

## Notes

Check the other direction before changing the interactive answer: does anything in
`examples/`, the docs, or the dogfood workflows run `dva clean` and branch on its exit code?
If so, that is the compatibility surface.

Worth deciding at the same time whether an EOF failure should be a general rule for
confirmation prompts or stay local to this one. There is still only one prompt in the
codebase, so the same argument TASK-170 made against a shared helper applies — but if a
second prompt is ever added, this is the behaviour it should inherit.

## Result

Case 2 now fails; case 1 is unchanged. The distinction is read from `fmt.Scanln`'s error
rather than guessed, and the two cases were never actually ambiguous at the call site — the
information needed to tell them apart was being discarded one character at a time by
`_, _ =`.

### The measurement the fix rests on

Whether this could be done at all depends on `Scanln` reporting EOF differently from a
person pressing Enter. Probed before writing anything, since keying the failure on the wrong
predicate would turn the prompt's own documented default into a hard error:

| stdin | n | answer | err | `errors.Is(err, io.EOF)` |
|---|---|---|---|---|
| `""` (EOF) | 0 | `""` | `EOF` | **true** |
| `"\n"` (bare Enter) | 0 | `""` | `unexpected newline` | false |
| `"   \n"` | 0 | `""` | `unexpected newline` | false |
| `"n\n"` | 1 | `"n"` | `<nil>` | false |
| `"y\n"` | 1 | `"y"` | `<nil>` | false |

So `errors.Is(err, io.EOF)` separates "nobody could answer" from "somebody declined"
exactly. `err != nil` does not — it collapses the bare-Enter default into the failure, which
is falsification F2 below.

### Behaviour, on `bin/dva` v0.1.44

```
$ dva clean --volumes </dev/null
This will remove all containers, networks, and VOLUMES (data loss!).
Continue? [y/N]
ERROR: cannot ask for confirmation: stdin reached EOF, so nothing answered the prompt and nothing was removed
       → pass --force to run 'dva clean' non-interactively
rc=1                                    # was: "Aborted." at rc=0

$ printf 'n\n' | dva clean --volumes
… Continue? [y/N] Aborted.
rc=0                                    # unchanged, deliberately

$ printf '\n' | dva clean --volumes     # the documented default N
… Continue? [y/N] Aborted.
rc=0                                    # unchanged

$ dva clean --volumes --dry-run </dev/null
rc=0                                    # TASK-170's exemption still holds
```

### The decision on an explicit `n`, and why

**rc 0, unchanged.** The command was asked not to proceed and did not proceed; that is the
interaction succeeding, not failing. The person who typed `n` is at a terminal reading
`Aborted.`, and is not inspecting `$?` to discover what they just chose.

EOF differs in kind rather than in degree: nobody declined, so emitting a decline invents an
answer on the operator's behalf and then reports success for it. That is the only arm with
no defensible reading, and it is the only arm that changed.

The compatibility surface was measured before deciding, per this task's own Notes. At
`e219eec`, `dva clean` occurs 40 times in the repo, 19 of them outside `tasks/`, spread over
8 files: `USAGE.md`, `skills/dva/SKILL.md`, `skills/dva/references/commands.md`, and five Go
files where it appears in comments and test names (`compose.go`, `provision.go`,
`clean_prompt_test.go`, `dry_run_halt_test.go`, `app_manager.go`).

None of them is an invocation. `Makefile`, `workflows/`, `agent-mesh-flows/`, `examples/`,
`.github/` and every `*.sh` together contain **0** matches, and nothing anywhere branches on
the exit code. So changing `n` would have broken nothing measurable in-repo — it was left
alone because rc 0 is right for it, not because changing it was risky.

### Local, not a general rule

`grep` for code that reads stdin itself, excluding tests: **1 site**, this one. The other
six `os.Stdin` references (`ssh.go`, `provision.go:566,598`, `infra.go`, `exec.go`) hand the
descriptor to a child process and never prompt. TASK-170's argument against a shared helper
therefore still holds — it would abstract over one caller. If a second prompt is added, this
is the behaviour it should inherit, and that sentence is the whole rule.

### Falsification

Each reverts one mechanism and fires a different test:

| # | Break | Fails |
|---|---|---|
| F1 | restore `_, _ = fmt.Scanln(&answer)` | `TestCleanEOFIsNotADecline` — "returned nil, so a script is told the volumes were removed when nothing was" |
| F2 | key on `err != nil` instead of `errors.Is(err, io.EOF)` | `TestCleanAnsweredDeclineExitsZero/bare_Enter` and `/whitespace_then_Enter` — the human's default N is rejected with a message claiming "stdin reached EOF", which is false |
| F3 | drop `--force` from the error text | `TestCleanEOFIsNotADecline` — "must name the way to proceed non-interactively" |

F2 is the one worth keeping: it is the cheapest plausible version of this fix, it passes
every test that only exercises the EOF path, and it silently breaks the interactive default.

### Test changed rather than deleted

`TestCleanWithoutDryRunStillPrompts` fed the prompt EOF and asserted `Aborted.`. Under this
change EOF no longer reaches that branch, so the test was moved to a typed `"n"` — it still
asserts exactly what it was written to assert (prompt shown, marker intact, `Aborted.` on
stderr not stdout), now against the branch that still produces it. Feeding it EOF would have
left it green while asserting nothing about the path it names.

### Gates

```
make test        exit 0   (internal/cli coverage 69.2%, unchanged)
gofmt -l         0 files
go vet ./...     exit 0
make doc-check   OK       (test_funcs_found 1065→1067, unmatched_run 0)
```

### Changed

- `internal/cli/compose.go` — read `Scanln`'s error; EOF returns an error naming `--force`;
  comment records why the decline branch keeps rc 0
- `internal/cli/clean_prompt_test.go` — `stdinFrom`/`useStdin` helpers;
  `TestCleanEOFIsNotADecline`; `TestCleanAnsweredDeclineExitsZero` (4 rows);
  `TestCleanWithoutDryRunStillPrompts` moved to a typed `n`
- `USAGE.md`, `skills/dva/references/commands.md` — say what happens with no terminal, since
  a command that used to exit 0 in CI now fails
