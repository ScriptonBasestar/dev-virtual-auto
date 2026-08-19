---
id: TASK-186
title: "flowcheck cannot see an unquoted bare word, the one flow-shell defect that only appears at runtime"
type: feature
priority: P1
effort: M
created-at: 2026-08-18T15:24:47+09:00
completed-at: 2026-08-18T18:41:00+09:00
quality-review: pass
quality-reviewed-at: 2026-08-19T13:58:00+09:00
verified-at: 2026-08-19T14:19:33+09:00
archived-at: 2026-08-19T14:19:33+09:00
quality-review-evidence: |
  - kind: automated
    command-or-step: "go test ./tools/flowcheck/... (AC1-AC3 verify binding)"
    result: exit 0 — TestBareWordArg has 15 named subtests, non-vacuous, covering fires-on `[ -f dva.yml ]`, silent-on `[ -f 'dva.yml' ]`, silent-on expansions/flags/operators/numbers, and the `printf true` vs `printf yes` split
  - kind: automated
    command-or-step: "go run ./tools/flowcheck (AC4 verify binding)"
    result: exit 0 — 10 flow files, 103 shell fields, OK, no decision-path defects
  - kind: manual
    command-or-step: "AC4 fixed-not-exempted check: read the six lines the card names"
    result: pass — 00-analyze.yaml uses `[ -f 'go.mod' ]`, `[ -f 'Cargo.toml' ]`, `[ -f 'package.json' ]`, `[ -f 'pyproject.toml' ]`; dva-improve.yaml:809 and :893 use `$([ -f 'dva.yml' ] && …)`. Operands are quoted; no exemption entry was added for them
  - kind: automated
    command-or-step: "am validate agent-mesh-flows/dva-improve.yaml (AC5 verify binding)"
    result: exit 0 — valid, no issues found
  - kind: manual
    command-or-step: "drift note (not a defect in this card)"
    result: Technical Notes lists the 9 rule ids current at completion; the corpus now carries 16. Later cards added rules; TASK-195 owns documenting the full set
source: "TASK-183 implementation — the backup step was blocked twice before the rule was found"
scope: "dva repo — tools/flowcheck/rules.go, tools/flowcheck/rules_test.go"
status: done
verification-summary: |
  quality-review pass, re-observed at disposition. `go test ./tools/flowcheck/...`, `go run
  ./tools/flowcheck` and `am validate agent-mesh-flows/dva-improve.yaml` all exit 0;
  TestBareWordArg still carries 15 passing subtests and `bare-word-arg` lives in shell.go.
  Widened past the review: every `[ -f ` operand in agent-mesh-flows/ re-read — 61 in all,
  being 40 quoted literals, 19 `"$var"` expansions (exempt by rule design) and the two
  comment lines that document the rule. No bare literal operand in the corpus. Corrected
  after archival: this line first read "26 quoted literals" against "the 19 remaining",
  a count that does not sum to the corpus.
---

# Task 186: Catch the quoting defect before the run does

## Summary

am's shell policy analyzer reads the first unquoted argument of certain commands as a
command name and blocks the step. `am validate` reports the flow valid; the block appears
only at run time, names the offending word but not the step's line, and gives no hint
which `context:` key produced it — a multi-key step has to be bisected one key at a time
in a throwaway flow to find it.

**Correction to this card as originally written.** It claimed the analyzer reads a bare
word as a command name *wherever it appears, not only in command position*. That is
wrong, and implementing it would have flagged most of the corpus. Measured against am
cb8b4ce before writing the rule:

| shell | result |
| --- | --- |
| `[ -f dva.yml ]` | blocked: `command "dva.yml" not in allowlist` |
| `test -f dva.yml` | blocked |
| `[[ -f dva.yml ]]` | blocked |
| `printf hello` | blocked |
| `eval hello` / `exec hello` | blocked |
| `[ -f 'dva.yml' ]` | runs |
| `echo dva.yml` | runs |
| `ls dva.yml` | runs |
| `grep name dva.yml` | runs |
| `cp a b` | runs |
| `mkdir -p tmp/x` | runs |
| `[ "$A" = "$B" ]` / `[ 1 -eq 1 ]` | runs |
| `printf true` | runs — only because `true` is allowlisted |

The trigger set is **command-specific**: `printf`, `test`, `[`, `[[`, `eval`, `exec`. The
rule implements the first four. `eval` and `exec` are deliberately excluded — there the
first argument really is a command name, so the allowlist is doing its intended job
rather than misreading data, and checking the rest of an `eval` line correctly would
require am's allowlist, which lives outside the repo.

The coincidence in the last row is what makes this worth a rule. `printf true || printf
false` is the required form for a `when:` gate producer, and it passes for an unrelated
reason: `true` and `false` happen to be allowlisted commands. That hides the actual
constraint, so the next flag someone writes as `printf yes` blocks, and the failure looks
like a gate defect rather than a quoting one.

## Completion Criteria

- [x] A rule reports an unquoted bare word in a flow `shell`/`context` field | verify: `go test ./tools/flowcheck/...`
- [x] The rule fires on `[ -f dva.yml ]` and stays silent on `[ -f 'dva.yml' ]` | verify: `go test ./tools/flowcheck/...`
- [x] The rule does not fire on shell variables, expansions, or allowlisted command names | verify: `go test ./tools/flowcheck/...`
- [x] The corpus passes, with every defect the new rule found fixed rather than exempted | verify: `go run ./tools/flowcheck`
- [x] The flows the rule changed still validate | verify: `am validate agent-mesh-flows/dva-improve.yaml`

## Resolution

`bare-word-arg` is the ninth rule. It required a quote-aware tokenizer rather than a
regex, because the corpus contains both halves of a pair that look alike and behave
oppositely:

- `awk '{ printf "%s", substr(buf, i, j) }'` — the `printf` is awk's, inside a quoted
  argument. The analyzer does not descend into it, so neither does the rule. A regex over
  raw text reported six words here that cannot fail.
- `DVA_FILE=$([ -f dva.yml ] && echo dva.yml)` — the `$(...)` holds shell the analyzer
  *does* read. The rule recurses into it, which is the only reason the real defect at
  `dva-improve.yaml:777` was caught.

A first regex-based attempt produced 78 findings on the corpus, of which 72 were the
tokenizer failing to end an argument list at `;`, `}` or a redirection, and 6 were real.
The tokenizer brought that to 6 findings, all real.

### The corpus was not clean

The last criterion was originally written as "the existing corpus passes unchanged". It
did not, and that is the finding rather than a problem with it. Six shipped lines were
blocked at run time on every invocation:

| file:line | shell | effect |
| --- | --- | --- |
| `00-analyze.yaml:244` | `[ -f go.mod ]` | Go project name never detected |
| `00-analyze.yaml:246` | `[ -f Cargo.toml ]` | Rust ditto |
| `00-analyze.yaml:248` | `[ -f package.json ]` | Node ditto |
| `00-analyze.yaml:250` | `[ -f pyproject.toml ]` | Python ditto |
| `dva-improve.yaml:777` | `$([ -f dva.yml ] && …)` | config filename never resolved |
| `dva-improve.yaml:861` | `$([ -f dva.yml ] && …)` | same, second occurrence |

All six were fixed by quoting the operand. Two more sat in `rules_test.go` fixtures
written for the gate-producer rule (`test -f x && …`), quoted for the same reason.

### Evidence

Measured against am cb8b4ce, one-step probe flows, `am run … -y`:

| probe | before | after |
| --- | --- | --- |
| `[ -f go.mod ] && echo "go: yes"` | `blocked: command "go.mod" not in allowlist` | — |
| `DVA_FILE=$([ -f dva.yml ] && echo dva.yml \|\| echo dva.yaml)` | `blocked: command "dva.yml" not in allowlist` | — |
| both, operands quoted | — | `✅ Done`, `v=[go: yes\ndva.yml]` |

Repo checks: `go test ./tools/flowcheck/...` ok · `go vet` clean · `gofmt -l` empty ·
`go run ./tools/flowcheck` → `10 flow file(s), 101 shell field(s)` / `OK — no
decision-path defects` · `am validate` valid on both edited flows.

## Technical Notes

- Rule ids are now `dead-gate`, `gate-operand`, `gate-filter`, `gate-producer-newline`,
  `exit-if-empty`, `param-type`, `phantom-command`, `unguarded-report`, `bare-word-arg`.
- False positives were the risk, and the exemption list is the answer: quoted words,
  anything containing an expansion or escape, flags, test operators, numbers, and
  `true`/`false`. A word the scanner cannot evaluate is left alone by design — the
  analyzer sees a value the scanner cannot predict, so silence is the only sound choice.
- `true`/`false` are exempt because they *run*, not because they are safe to imitate. The
  rule cannot warn about that coincidence without firing on every correct gate producer
  in the corpus, so a comment above the exemption carries it instead.
- Allowlisting a filename is not the fix; quoting it is. The allowlist lives outside the
  repo (`~/.config/agent-mesh/sandbox_override.yaml`) and the rule never consults it.
