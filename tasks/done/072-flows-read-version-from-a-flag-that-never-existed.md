---
id: TASK-072
title: "Flows read the DVA version from --short, a flag that never existed, and write the sentinel into real configs"
type: fix
priority: P1
status: done
effort: S
created-at: 2026-07-30T00:00:00+09:00
scope: "agent-mesh-flows/ — dva-improve.yaml, dva-diagnose.yaml, dva-improve-guided/{00-analyze,30-configure}.yaml"
---

# Task 072: Stop reading the version from a nonexistent flag

## Problem

Four flow files resolved the running DVA version with:

```
dva_version: "dva version --short 2>/dev/null || echo 'unknown'"
```

`dva version` has no `--short` flag. It exits 1 with `ERROR: unknown flag: --short`,
`2>/dev/null` swallows that message, and `|| echo 'unknown'` converts the failure into a
value that looks like data. Nothing downstream distinguished it from a real version.

`dva-improve.yaml`'s `fix_version` step then sed-wrote it into the target config:

```
version: "unknown"
```

after which every subsequent `dva` command against that config fails —
`version "unknown" is not a version`. `dva validate` exits 1, `dva show` errors, and the
flow has broken the file it was invoked to improve.

## Why it matters

This is the same defect class as [TASK-057](057-dead-self-referencing-urls.md),
[TASK-060](060-go-module-path-does-not-resolve.md),
[TASK-061](061-go-facts-hand-copied-into-flow-library.md),
[TASK-064](064-dns-bridge-endpoints-no-mode-can-start.md) and
[TASK-067](067-version-field-rule-stated-three-incompatible-ways.md): a second copy of
knowledge that nothing compiles. Here the copy is the *interface* of a command — the flow
asserted a flag exists and no build step, test, or lint could contradict it.

The failure was pre-existing but only became destructive with
[TASK-070](070-malformed-version-is-read-as-zero-and-always-compatible.md). Before that fix
a malformed version parsed to `[0,0,0]`, so the compatibility gate silently reported
"compatible" and the flow carried on. TASK-070 made a malformed version a hard failure, which
promoted this from a defeated gate to a broken target config. Fixing 070 without 072 leaves
the flow strictly worse than before.

Call sites of `dva_version` before the fix:

| file | line | use |
| --- | --- | --- |
| `dva-improve.yaml` | 348, 487, 544, 629 | LLM instructions: "set `version:` to this" |
| `dva-improve.yaml` | 713 | `EXPECTED` → sed at 734/735 (**writes the file**) |
| `dva-diagnose.yaml` | 42 | report only |
| `dva-improve-guided/00-analyze.yaml` | 33 | report only |
| `dva-improve-guided/30-configure.yaml` | 58 → 82 | LLM instruction under 반드시 (**writes the file**) |

Two write paths, not one. The sed path is the loud one; the LLM path in `30-configure.yaml`
is quieter and has no shell where a guard could sit.

## Fix shape

Parse `dva version --json`, which is the machine-readable contract and already a dependency
of these flows, instead of inventing a flag. Guard the write path on the sentinel values the
context step can actually produce — **not** on DVA's version grammar. Re-encoding that
grammar in shell would be one more claim nothing checks, which is the defect being fixed.

## Non-goals

- Do not add a real `--short` flag to the CLI as part of this task (see Left open).
- Do not validate the version format in shell.
- Do not touch `dva_version_json` in `dva-improve.yaml`.

## Acceptance criteria

- [x] `dva version --short` is confirmed nonexistent, not merely assumed | verify: `! dva version --short >/dev/null 2>&1`
- [x] No flow file references `--short` | verify: `test 0 -eq "$(/usr/bin/find agent-mesh-flows -name '*.yaml' | xargs /usr/bin/grep -c -- 'version --short' 2>/dev/null | /usr/bin/grep -cv ':0$')"` — 0 of 10 swept
- [x] The replacement resolves a real version | verify: `V=$(dva version --json 2>/dev/null | jq -r '.version // "unknown"'); [ -n "$V" ] && [ "$V" != "unknown" ]`
- [x] The sentinel guard precedes the sed writes | verify: `G=$(/usr/bin/grep -n 'refusing to rewrite version' agent-mesh-flows/dva-improve.yaml | cut -d: -f1); S=$(/usr/bin/grep -n 'sed -i "s/\^version' agent-mesh-flows/dva-improve.yaml | head -1 | cut -d: -f1); [ "$G" -lt "$S" ]`
- [x] Every flow still parses | verify: `uv run --with pyyaml python -c "import glob,yaml; [yaml.safe_load(open(f)) for f in glob.glob('agent-mesh-flows/**/*.yaml', recursive=True)]"`

## Evidence

Reproduced end-to-end in a throwaway config before changing anything, rather than reasoning
from the flow text:

```
$ dva version --short 2>&1 >/dev/null
ERROR: unknown flag: --short          # rc=1
$ printf 'version: "%s"\n' "$(dva version --short 2>/dev/null || echo 'unknown')"
version: "unknown"                    # what fix_version's sed writes
$ dva validate                        # rc=1
ERROR: version "unknown" is not a version: ...
$ dva show                            # same error
```

Simulating a missing `jq` with a nonexistent command in the same pipeline position:

```
... | nosuchjq -r '.version' 2>/dev/null                      -> [] len=0
... | nosuchjq -r '.version' 2>/dev/null || echo 'unknown'    -> [unknown] len=7
```

## Resolution

Fixed in `e6dd809` (the two writing flows) and `fe781a2` (the two guided stages).

All four call sites are now byte-identical:

```
dva_version: "dva version --json 2>/dev/null | jq -r '.version // \"unknown\"' 2>/dev/null || echo 'unknown'"
```

`fix_version` refuses to run when the value is `''` or `unknown`, exits 1, and names the two
things to check (`dva version --json`, `jq`). `30-configure.yaml`'s instruction carries the
same rule in prose — omit the field, keep any existing value, and report the failure — because
an `llm` step has no shell to guard.

**The trailing `|| echo 'unknown'` is load-bearing.** Without it a missing `jq` resolves the
value to *empty*, not to the sentinel. `fix_version` guards both, so `dva-improve.yaml` was
safe either way; the guided stages have no guard, and an empty value would leave the LLM
improvising a version instead of recognizing a failure.

**The first grep was incomplete and the count is what caught it.** The initial sweep found the
two obvious flows and I nearly closed there. Printing "files swept: 10" beside the verdict
surfaced two more copies in `dva-improve-guided/`, one of them a live corruption path. This is
the [[dva-measure-with-dva-not-grep]] rule paying off in the direction it was written for: the
verdict was right, the population was wrong.

## Left open

Three questions deliberately not decided here:

1. **Add a real `--short` flag to `dva version`?** It would fix any external copy of these
   flows, of which there is at least one class (dogfood workflows imported into other repos).
   Against: it is a second rendering of a value `--json` already exposes.
2. **Should sentinel substitution be banned across flows?** `|| echo '<plausible value>'` is
   the mechanism that turned a hard failure into corrupt data. It appears elsewhere in these
   flows (e.g. `|| echo 'adopt'` in `30-configure.yaml:57`, where the default is genuinely
   benign). A blanket ban would be wrong; a rule distinguishing "benign default" from
   "sentinel that gets written to disk" is what is actually needed.
3. **Four identical copies of one command is still four copies.** `imports: profiles:
   [dva-common]` exists; whether a profile can carry a shared context entry — and so make this
   one definition — was not investigated.

Also noted while sweeping: `dva-improve.yaml:62` defines `dva_version_json` and nothing
interpolates it. Left in place under `<modify>최소한만</modify>`; it now runs the same
`dva version --json` twice per flow.
