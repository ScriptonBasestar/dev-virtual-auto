---
id: TASK-222
title: "compose_exec splits its command by a rule run: does not use"
type: bug
priority: P2
effort: M
created-at: 2026-08-20T21:30:00+09:00
source: "found while fixing TASK-199's bindings, from TASK-115's note that all four compose argv builders shared the same splitting bug. TASK-115 fixed the four `command:` builders and left the two provision-step fields, which no card has named"
scope: "the ten `strings.Fields((s|step).Compose{Exec,Run})` sites in internal/runner/step_keys.go, internal/cli/hooks.go and internal/cli/provision.go — six written `step.`, four written `s.` — plus the one prose claim in skills/config/references/schema-reference.md. No change to `command:` or `run:`, which do not leak quote characters into argv"
status: todo
---

# Task 222: `compose_exec` splits its command by a rule `run:` does not use

## Summary

`compose_exec:` and `compose_run:` hold a command string. So does `run:` — and
`run:` is a **sibling field of the same struct**, `config.ProvisionItem`
(`internal/config/config.go:441-459`), dispatched seven lines away in the same
step loop — `runComposeStepKeys` at `internal/runner/steps.go:69`, `execCommands`
at `:76`. Under `run:` a quote character
never reaches argv as part of a token. Under `compose_exec:` it always does,
because that field is split with `strings.Fields`, which has no notion of
quoting.

Measured against `bin/dva` at sha256 `006d346…02eda1`, with the compose backend
pointed at a stub that prints its argv one slot per line. The payload
`sh -c 'echo a b'` is authored identically in all three rows; row A additionally
carries the leading service word `web` because `compose_exec:` takes
`<service> <command…>`. Slots 0–4 are byte-identical in every row —
`<stub> <--project-name> <probe218> <exec> <web>`, 5 slots — so only the payload
is tabulated:

| Sibling key | How it is split | Payload slots, 5 onward | Payload tokens | ARGC |
|---|---|---|---|---|
| A `compose_exec: "web sh -c 'echo a b'"` | `strings.Fields` (`step_keys.go:48`) | 5 `<sh>` · 6 `<-c>` · 7 `<'echo>` · 8 `<a>` · 9 `<b'>` | 5 | 10 |
| B `run: "sh -c 'echo a b'"`, `shell` default | not split — `sh -c` wrap (`docker_compose.go:167`) | 5 `<sh>` · 6 `<-c>` · 7 `<sh -c 'echo a b'>` | 3 | 8 |
| C `run: "sh -c 'echo a b'"`, `shell: false` | `dvaexec.SplitCommand` (`docker_compose.go:169`) | 5 `<sh>` · 6 `<-c>` · 7 `<echo a b>` | 3 | 8 |

Prefix 5 + payload 5 = 10 = row A's ARGC; 5 + 3 = 8 = rows B and C. The
difference, 2, is the same in the ARGC column and the token column.

Row A's slots 7 and 9 are the defect: the apostrophes survive into argv, so
`docker compose exec web` runs `sh -c "'echo"` with `a` and `b'` as `$0` and
`$1` — a different program from the one written.

**Rows B and C are quote-safe for two different reasons, and only C is
quote-aware.** `shell` defaults to true, so the ordinary `run:` step takes the
`sh -c` branch and is never split at all; the string survives because nothing
touched it. Only `shell: false` reaches `SplitCommand`, which consumes the
quotes as delimiters and emits the one intended token. The claim this card makes
is therefore about the *outcome* — no `run:` shape leaks a quote character into
a token, and every `compose_exec:` shape does — not about `SplitCommand` being
the rule the sibling uses by default.

## Where

Ten sites, all `strings.Fields`, none `SplitCommand`:

| file:line | field | receiver |
|---|---|---|
| `internal/runner/step_keys.go:48` | `ComposeExec` | `step.` |
| `internal/runner/step_keys.go:50` | `ComposeRun` | `step.` |
| `internal/cli/hooks.go:144` | `ComposeExec` | `step.` |
| `internal/cli/hooks.go:160` | `ComposeRun` | `step.` |
| `internal/cli/provision.go:181` | `ComposeExec`, serial | `step.` |
| `internal/cli/provision.go:197` | `ComposeRun`, serial | `step.` |
| `internal/cli/provision.go:334` | `ComposeExec`, parallel dry-run | `s.` |
| `internal/cli/provision.go:336` | `ComposeRun`, parallel dry-run | `s.` |
| `internal/cli/provision.go:359` | `ComposeExec`, parallel live | `s.` |
| `internal/cli/provision.go:361` | `ComposeRun`, parallel live | `s.` |

The receiver column is there because a reader grepping the scope line for
`step.Compose` finds six of the ten; the criteria below use `(s|step)\.` for
that reason.

`internal/runner/step_keys.go` already imports `dvaexec` and calls into it at
`:75` (`ExecSequential`), so `SplitCommand` was one package selector away on the
line that chose `strings.Fields`.

**Only `step_keys.go:48` was measured.** `runComposeStepKeys` short-circuits on
`ComposeExec` before reaching `ComposeRun` at `:50`, and the probe built no
second interaction for it; the four `provision.go` sites and the two `hooks.go`
sites were not exercised either. That those nine behave like the measured one is
a reading of nine identical `strings.Fields` calls, not a measurement, and the
fix must not be reported as if it were.

## Why it survived

- **Both printed forms re-join the argv, so the defect is invisible in exactly
  the mode a careful user checks.** `internal/cli/provision.go:326` prints the
  dry-run line as `composeCmd + " " + strings.Join(args, " ")`, and the live
  path's echo at `:563` does the same. Measured on the probe, both lines end in
  `… exec web sh -c 'echo a b'` — the apostrophes land back exactly where the
  reader expects them, separated by the same spaces `strings.Fields` split on —
  while the stub invoked one line later reports `<sh> <-c> <'echo> <a> <b'>`.
  The printed line is *incapable* of showing this defect: it is identical
  whether the split was right or wrong.
- **No test asserts the argv.** `internal/runner/step_keys_test.go:81,86` and
  `internal/cli/hooks_test.go:140,172` set these fields, but every assertion is a
  `strings.Contains` against joined output, which cannot distinguish a wrong
  split from a right one. `internal/config/hooks_test.go:236` is a YAML
  literal whose one assertion (`:257`) is that the string survived parsing, and `internal/config/inert_step_test.go:27,28`
  sets both fields to assert `IsInert()`. That is the whole census — three files
  set these fields, none looks at argv.
- **No corpus reaches it.** No file under `examples/` (16 files) or
  `internal/integration/testdata/` uses either key at all, and the one YAML
  literal in the repo (`internal/config/hooks_test.go:236`,
  `compose_exec: "postgres pg_isready -U app"`) has no inner quote. Every value
  that exists today is single-line and space-separated, so both splitters agree
  on all of them.
- **No validation warns.** `internal/config/validate.go`,
  `validate_warnings.go`, `internal/cli/validate.go` and `schema.json:295-302`
  treat both fields as bare strings.

## The claim that is false

`skills/config/references/schema-reference.md:671` — reached by am flows as
`agent-mesh-flows/shared/library/dva-schema.md`, a symlink, and copied into
`internal/cli/library_reference.txt:847` by `make generate`:

> Always use `run:` format. While both forms work at runtime, `run:` is the standard.

Two lines above it, `:669` prints the equivalence in a table:

| Schema-Valid Field | Preferred `run:` Equivalent |
|-------------------|-----------------------------|
| `compose_exec: "svc cmd"` | `run: "docker compose exec svc cmd"` |

That row is exactly the diverging pair. The document names the two keys whose
splitting rules differ, sets them equal, and then tells the reader in prose that
the difference does not matter at runtime — which is the sentence a reader
consults when deciding whether a `compose_exec:` they already wrote is safe to
keep. `compose_run` is not documented at all.

## Fix direction

Replace `strings.Fields` with `dvaexec.SplitCommand` at all ten sites, making
the compose keys agree with `run:`'s `shell: false` branch.

It is not a behaviour change for any value that exists in the repo today. It is
**not** correct to say it changes nothing for any value without a quote
character, and this was measured rather than reasoned. `SplitCommand`
(`internal/exec/exec.go:153`) separates on `' '` and `'\t'` only, while
`strings.Fields` separates on every `unicode.IsSpace` rune. Same probe, same
binary, `shell: false` so `SplitCommand` is isolated, payload `echo a<SEP>b`:

| separator | `compose_exec:` payload | `run:` payload | ARGC exec / run | differ |
|---|---|---|---|---|
| space | `<echo> <a> <b>` | `<echo> <a> <b>` | 8 / 8 | no |
| tab `\t` | `<echo> <a> <b>` | `<echo> <a> <b>` | 8 / 8 | no |
| newline `\n` | `<echo> <a> <b>` | `<echo> <a⏎b>` | 8 / 7 | **yes** |
| carriage return `\r` | `<echo> <a> <b>` | `<echo> <a␍b>` | 8 / 7 | **yes** |
| no-break space U+00A0 | `<echo> <a> <b>` | `<echo> <a␠b>` | 8 / 7 | **yes** |

`\v`, `\f` and U+0085 are in `unicode.IsSpace` too and by the same mechanism
should behave the same way, but they were not probed and no measured claim about
them belongs in the commit.

So the honest scope of the "no behaviour change" clause is: **payloads whose
whitespace is entirely spaces and tabs and which contain no quote character.**
Two shapes fall outside it and must be named in the commit message, because
`SplitCommand` is the more conservative rule for the first and the more
destructive for the second: a YAML double-quoted scalar, block scalar or folded
value carrying `\n`/`\r`/U+00A0 stops splitting there, and a literal `''` is
consumed entirely by the quote branch and yields no argument at all, where
`strings.Fields` passes it through as a two-character token.

The alternative worth stating and rejecting: routing these through `sh -c` like
`run:`'s default branch. That would change the meaning of every existing
`compose_exec:` — globs, redirections and `&&` would start working — which is a
feature request, not this bug.

## Completion Criteria

- [ ] Both fields split by the same rule as their sibling `run:` | verify: `f=$(/usr/bin/grep -rn --include='*.go' -E 'strings\.Fields\((s|step)\.Compose(Exec|Run)\)' . | /usr/bin/grep -vc _test.go || true); tot=$(/usr/bin/grep -rn --include='*.go' -E '(strings\.Fields|SplitCommand)\((s|step)\.Compose(Exec|Run)\)' . | /usr/bin/grep -vc _test.go || true); echo "Fields=$f of $tot split sites"; [ "$tot" -ge 10 ] || { echo "fewer than the 10 recorded split sites — nothing was measured"; exit 2; }; [ "$f" -eq 0 ]` — prints `Fields=10 of 10 split sites` and exits 1 today. Proven in three states against a copy of `internal/` outside the module: untouched `Fields=10 of 10` exit 1, four sites converted `Fields=6 of 10` exit 1, all ten converted `Fields=0 of 10` exit 0. The denominator is the same 10 in all three, so a rewrite that deleted the call sites rather than fixing them trips the `exit 2` guard instead of passing
- [ ] A test asserts the argv for a quoted value, not the joined output | verify: `n=$(/usr/bin/grep -rho 'func TestComposeStepQuoting[A-Za-z]*' internal/ | sort -u | wc -l | tr -d ' '); echo "test funcs=$n"; [ "$n" -ge 1 ]` — prints `test funcs=0` and exits 1 today. Bound on the test source rather than a `go test -run` invocation, which exits 0 when it selects nothing and which `doccheck`'s TASK-136 guard rejects for that reason
- [ ] The test pins `compose_exec:` against its sibling `run:`, not against a remembered constant | verify: human — the assertion compares the argv produced for `compose_exec: "web sh -c 'echo a b'"` against the argv produced for the same payload under `run:` with `shell: false`, and fails if they differ. `shell: false` is required: the default `run:` path never splits at all, so comparing against it would pin `compose_exec:` to a three-token `sh -c` wrap and pass for the wrong reason. A test that hard-codes the expected token list instead passes for ten converted sites and one converted site alike; a test that compares the two paths cannot pass while any of the ten is unconverted, and stays correct if `SplitCommand`'s own rule ever changes
- [ ] The test covers `compose_run` as well as `compose_exec` | verify: `n=$(/usr/bin/grep -rl "ComposeRun:.*'" internal/runner/*_test.go internal/cli/*_test.go 2>/dev/null | wc -l | tr -d ' '); tot=$(ls internal/runner/*_test.go internal/cli/*_test.go | wc -l | tr -d ' '); echo "tests setting ComposeRun to a quoted value=$n over $tot test files"; [ "$n" -ge 1 ]` — prints `0 over 87 test files` and exits 1 today. Bound on a *quoted* value, not on `ComposeRun` alone: 13 of those 87 files already mention the field (16 repo-wide), so the obvious binding passes before any work is done. This criterion also closes the measurement gap named in **Where** — `ComposeRun` is unreachable through `runComposeStepKeys` while `ComposeExec` is set, so only a test can exercise it
- [ ] The false runtime claim is gone from its single source and from the generated copy | verify: `src=skills/config/references/schema-reference.md; [ -f "$src" ] || { echo "$src missing — nothing was measured"; exit 2; }; n=$(/usr/bin/grep -ic 'both forms work at runtime' "$src" || true); g=$(/usr/bin/grep -ic 'both forms work at runtime' internal/cli/library_reference.txt || true); echo "source=$n generated=$g over $(wc -l < "$src" | tr -d ' ') source lines"; [ "$n" -eq 0 ] && [ "$g" -eq 0 ]` — prints `source=1 generated=1 over 765 source lines` and exits 1 today
- [ ] The generated artifact is not left stale | verify: **(writes tracked files)** `export PATH="$HOME/.local/share/mise/shims:$PATH" && make check-generate` — `Makefile:177-182` runs `make generate` at `:180`, so this binding is not read-only and a reviewer re-running the card's criteria mutates the tree. Recorded here rather than silently, because every other binding on this card can be re-run safely and a reader has no way to tell which is which
- [ ] The corpus can reach the defect | verify: `tot=$(ls examples/*.yml | wc -l | tr -d ' '); [ "$tot" -gt 0 ] || { echo "no examples — nothing was measured"; exit 2; }; q=$(/usr/bin/grep -lE "compose_(exec|run): *(\"[^\"]*('|\\\\\")|'[^\"]*\")" examples/*.yml 2>/dev/null | wc -l | tr -d ' '); echo "examples=$tot carrying a compose step with an inner quote=$q"; [ "$q" -ge 1 ]` — prints `examples=16 carrying a compose step with an inner quote=0` and exits 1 today. The regex requires a quote **inside** a quoted value. An earlier draft bound on `['\"][^'\"]* `, which matches any double-quoted value containing a space — including the benign `compose_exec: "web pg_isready -U app"` — so it would have passed the day anyone added an ordinary example, with the defect still unreachable. Proven over six constructed lines: the three defect shapes (`"… 'x y'"`, `"… \"x y\""`, `'… "x y"'`) match, the three benign ones (`"web pg_isready -U app"`, unquoted, `'web pg_isready -U app'`) do not
- [ ] The fix is re-measured against the real binary the same way the bug was | verify: human — rebuild `dva`, point `runners.compose.command:` at a script that prints its argv one entry per line, and re-run the two probes recorded in this card: the quoted payload `sh -c 'echo a b'`, and the newline payload. Confirm row A's payload becomes the three tokens `<sh> <-c> <echo a b>`, matching row C. Record the full argv with slot indices *and* ARGC — the Summary table was first drafted with a five-token payload and a three-token payload both annotated `ARGC=9`, which cannot be true of two argv sharing a five-slot prefix, and the arithmetic error was invisible while only the payload column was read. Record the binary's sha256 beside the output; a concurrent `make build` replaces it without warning
- [ ] Full gate suite green | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make test && make lint && make doc-check`

## Open Questions

- Ten call sites repeating `append([]string{"exec"}, split(...)...)` is the
  TASK-115 shape again one layer up. Whether this task also collapses them into
  one helper, or only corrects the splitter in place, is a scope call. Collapsing
  makes the next splitter change a one-line change; not collapsing keeps the diff
  reviewable against the ten lines named above.
- A quote inside these fields is currently silently wrong. Whether `dva validate`
  should warn on it *after* the fix is a separate question — once the splitter is
  correct, a quote is legitimate, and a warning would be noise.
- The same `append([]string{"exec"}, strings.Fields(...)...)` expression is
  written three times over: at `provision.go:181/197` on the serial path, at
  `334/336` in the parallel dry-run branch, and at `359/361` in the parallel live
  branch. The last two are the two arms of one `if`, so they are duplicated
  *source* rather than a value split twice per run — but they are the pair most
  likely to drift, because a fix applied to the live arm and not the dry-run arm
  produces a dry-run that lies in the opposite direction from today's.

## References

- [TASK-115](../_archive/115-four-compose-argv-builders-share-two-bugs.md) — the
  same splitting bug in the four `command:` builders; its fix is the precedent
  this task copies, and `internal/exec/compose_argv.go:15-17` records that both
  copies of the bug were in all four
- `internal/exec/exec.go:135-164` — `SplitCommand`, and the `' '`/`'\t'` test at
  `:153` that makes it disagree with `strings.Fields` on other whitespace
- `internal/runner/docker_compose.go:166-170` — the `shell` branch: default true
  wraps in `sh -c` without splitting, false calls `SplitCommand`
- `internal/runner/steps.go:18-20,69-76` — the contract that puts `run:` and
  `compose_exec:` seven lines apart in one loop
- `internal/exec/exec_test.go:41-60` — the quote behaviour that makes the two
  splitters disagree
- `skills/config/references/schema-reference.md:664-672` — the equivalence table
  and the prose to correct
