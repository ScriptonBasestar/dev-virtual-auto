---
id: TASK-135
title: "dva-improve writes the running DVA version into every config it touches, ratcheting the floor upward"
type: bug
priority: P2
status: todo
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

- [ ] `dva-improve.yaml` no longer instructs writing the running version. The contract at
      :10 and the three LLM instructions state the floor rule instead.
- [ ] `fix_version` either writes `config.MinScaffoldVersion` or does not write `version:`
      at all — decide which, and record the reason in this file.
- [ ] `dva-improve-guided/30-configure.yaml`'s write path agrees with the same rule.
- [ ] A grep guard catches this class in **both** languages, so the next restatement is
      found: sweep for the Korean `현재 DVA 버전` alongside the English string TASK-067
      already pins.
- [ ] Every flow still parses: `uv run --with pyyaml python -c "import glob,yaml; [yaml.safe_load(open(f)) for f in glob.glob('agent-mesh-flows/**/*.yaml', recursive=True)]"`
- [ ] `make test` exits 0.

## Notes

Decide the policy before editing: writing `MinScaffoldVersion` keeps a floor that is
honest about what the config needs; omitting `version:` entirely is what
`internal/config/version.go` calls the no-gate default, and TASK-067 already made the
field optional in `schema.json`. Both are defensible; picking one silently is not.
