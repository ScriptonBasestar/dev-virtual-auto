---
id: TASK-135
title: "dva-improve writes the running DVA version into every config it touches, ratcheting the floor upward"
type: bug
priority: P2
status: done
effort: M
created-at: 2026-08-03T12:10:00+09:00
source: "TASK-067 and TASK-072 finalize verification — both fixed neighbours of this line and neither fixed the policy"
depends-on: [TASK-067, TASK-072]
scope: "dva repo — agent-mesh-flows/dva-improve.yaml, agent-mesh-flows/dva-improve-guided/30-configure.yaml"
---

# Task 135: Stop dva-improve from scaffolding the running version

## Problem

`internal/config/version.go:1-11` states the rule: `version:` is the **minimum** DVA
version a config requires of its reader. It is optional, and scaffolding the *running*
version is wrong — every new config would refuse to load on any older DVA, ratcheting
the floor upward on each release.

TASK-067 removed that anti-pattern from the prose the flows read
(`shared-guardrails.md`, `reference-examples.md`, `schema-reference.md`). It did not
reach `dva-improve.yaml`, the flow that actually writes configs, because TASK-067's own
verify grep matched the English string `Must match the current DVA CLI version` and the
flow states the rule in Korean.

The flow encodes the anti-pattern in four places, verified 2026-08-03:

| location | content |
| --- | --- |
| `dva-improve.yaml:10` | flow contract `expected_results`: `버전이 현재 DVA CLI 버전과 일치한다` |
| `dva-improve.yaml:348, :487, :544` | LLM instruction: `version 필드는 반드시 현재 DVA 버전 {{check_prerequisites.dva_version}}로 설정하세요` |
| `dva-improve.yaml:707-736` | Phase 6 `fix_version`, deterministic shell, `sed -i` rewrite of `version:` to `$EXPECTED` |
| `dva-improve.yaml:713` | `EXPECTED="{{check_prerequisites.dva_version \| trim}}"` — resolves to `dva version --json`'s `.version`, i.e. the running binary |

`dva-improve-guided/30-configure.yaml` carries the same `반드시` instruction at its own
write path.

`fix_version` is deterministic and unconditional: it does not ask whether the target
declared a floor on purpose, it overwrites whatever is there. So every `dva-improve` run
raises the target's floor to whatever DVA happened to be installed on the machine that
ran it.

## Why it survived two adjacent fixes

- **TASK-067** fixed the rule statement, but swept for it in English only.
- **TASK-072** fixed `fix_version`'s *mechanism* — it was reading a `--short` flag that
  never existed, and the sentinel could corrupt the field — and added a guard that
  refuses to write `''` or `unknown`. It explicitly left the write policy alone. After
  072 the step reliably writes a real value; the value is still the wrong one.

Fixing the mechanism made the defect worse, not better: before 072 the sed could fail
loudly, now it succeeds every time.

## Acceptance criteria

- [x] `dva-improve.yaml` no longer instructs writing the running version. The contract at
      :10 and the three LLM instructions state the floor rule instead.
- [x] `fix_version` either writes `config.MinScaffoldVersion` or does not write `version:`
      at all — decide which, and record the reason in this file.
- [x] `dva-improve-guided/30-configure.yaml`'s write path agrees with the same rule.
- [x] A grep guard catches this class in **both** languages, so the next restatement is
      found: sweep for the Korean `현재 DVA 버전` alongside the English string TASK-067
      already pins. | verify: `go test ./internal/config/ -run 'TestGeneratorCorpusDoesNotScaffoldTheRunningVersion|TestVersionRuleGuardCatchesTheKnownOffenders'`
- [x] Every flow still parses: `uv run --with pyyaml python -c "import glob,yaml; [yaml.safe_load(open(f)) for f in glob.glob('agent-mesh-flows/**/*.yaml', recursive=True)]"`
- [x] `make test` exits 0.

## Decision: `fix_version` is removed, not repointed

**The flow writes nothing to `version:` — ever, in either mode.** The step is deleted and
`semantic_triage_state` now depends on `validate_final` directly.

Three reasons, in the order they decided it:

1. **`dva-improve` edits somebody else's config.** An existing `version:` is its author's
   declaration about what the config requires of a reader. The flow has no evidence about
   that floor — it can see what the config *contains*, never what the author intended to
   promise — so any rewrite replaces a claim it cannot evaluate. `fix_version` did this
   unconditionally: `sed -i` on `^version:` with no test for whether the value was
   deliberate.
2. **`MinScaffoldVersion` is unreachable from a flow.** `dva version --json` emits exactly
   `build_date`, `commit`, `version` — measured, not assumed. Writing the floor would mean
   hardcoding `0.1.44` into YAML, a second copy of a constant whose own doc comment says
   *"no build may inject a different floor"*. That copy goes stale at the next bump and
   nothing would catch it, which is this task's own failure mode one level down.
3. **Omitting is a legal, honest answer.** `version:` is optional (TASK-067 made
   `schema.json` agree), and absence is what `internal/config/version.go` calls the no-gate
   default: *this config requires nothing of its reader*. For a config the flow just wrote,
   that is true. `dva init` writes `MinScaffoldVersion` and keeps doing so — init knows it
   emits the multi-runner model; improve does not know what it produced.

Rejected alternative: keep the step but make it report-only. It would have nothing to
report — a floor above the running binary already fails `check_prerequisites` and
`validate_final` upstream, so every case reaching the step is one where silence is correct.

The guided flow had already written the right rule at `30-configure.yaml:87-90` — *"leave
an existing value alone; if absent, omit the field"* — but filed it as **error handling**,
reached only when the version lookup returned `unknown`. This promotes that branch to the
policy and deletes the `dva_version` context key it fed, since nothing else in that stage
used it.

## Result

**Sites changed (4 + 1, all of the ones this task measured):**

| location | before | after |
|---|---|---|
| `dva-improve.yaml:10` | contract: `버전이 현재 DVA CLI 버전과 일치한다` | contract: `version 필드를 건드리지 않는다 — reader floor는 설정 작성자의 선언이다` |
| `dva-improve.yaml:348` | `반드시 현재 DVA 버전 {{dva_version}}로 설정` | reader-floor rule + leave/omit instruction |
| `dva-improve.yaml:487` | `MANDATORY: version 업데이트` | `MANDATORY: version 필드는 그대로 둔다` |
| `dva-improve.yaml:544` | checklist: `version이 {{dva_version}}인가?` | checklist: `version을 손대지 않았는가?` |
| `dva-improve.yaml:707-736` | `fix_version` shell step, `sed -i` rewrite | removed; replaced by a comment recording why no such phase exists |
| `30-configure.yaml:61,85` | `dva_version` key + `반드시 …로 설정` | key deleted, write path states the floor rule |

`dva-improve.yaml:634` (`DVA schema {{dva_version}} 호환 필수`) is deliberately unchanged:
it constrains what the LLM may *generate* against the running schema, which is a different
claim from what goes in the `version:` field. The guard is written to let it through, and a
test case pins that (see below), so the next person does not "fix" it.

**Guard — `internal/config/version_rule_test.go`.** Two tests:

- `TestGeneratorCorpusDoesNotScaffoldTheRunningVersion` walks `agent-mesh-flows/`,
  `skills/` and `library_reference.txt` (**34 files / 8351 lines** — printed, and a zero
  count is a hard failure, since a guard that scans nothing is as green as a clean corpus).
  Three rules: a `version`-field reference sharing a line with a `{{…dva_version…}}`
  interpolation; any `sed` rewriting `version:`; and the prose restatement in Korean
  (`현재 DVA … 버전`) or English. YAML comment lines are skipped — a YAML comment never
  reaches the LLM's prompt — but Markdown `#` lines are headings and are not skipped.
- `TestVersionRuleGuardCatchesTheKnownOffenders` feeds the guard the seven pre-change lines
  verbatim, plus five legitimate lines it must not flag. Without it the corpus scan is
  unfalsifiable: it passes on a clean tree whether or not its patterns match anything.

That second test earned itself on first run. The guided offender is `version 필드는 반드시
{{detect_track.dva_version}}로 설정` — **no backticks**, so the initial `` `version` ``
pattern slid straight past it. The corpus scan was green at that moment and would have
stayed green; only replaying the historical line exposed the hole. The pattern now matches
`version 필드` / `version field` unbackticked too.

**Measured before → after**, same three patterns run against the `HEAD` blobs and the
working tree:

| file | write-instruction | `sed` rewrite | ko prose |
|---|---|---|---|
| `dva-improve.yaml` @HEAD | 3 | 3 | 2 |
| `30-configure.yaml` @HEAD | 1 | 0 | 0 |
| both, after | **0** | **0** | **0** |

Nine hits to zero. TASK-067's grep was already bilingual in its final form; what let this
survive was its **scope** — `agent-mesh-flows/shared/library/` and `library_reference.txt`,
which excludes the flows that write the configs. The new guard's corpus is the union.

**Gates.** `uv run --with pyyaml …` → `parsed 10 flow yaml files OK`. `make test` exits 0.

## Notes

Decide the policy before editing: writing `MinScaffoldVersion` keeps a floor that is
honest about what the config needs; omitting `version:` entirely is what
`internal/config/version.go` calls the no-gate default, and TASK-067 already made the
field optional in `schema.json`. Both are defensible; picking one silently is not.
