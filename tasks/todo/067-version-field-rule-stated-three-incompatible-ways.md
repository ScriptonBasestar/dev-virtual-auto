---
id: TASK-067
title: "The `version:` rule is stated across nine files and encodes three incompatible rules — the AI generator is taught the harmful one"
type: fix
priority: P2
status: todo
effort: S
created-at: 2026-07-30T00:00:00+09:00
reopened-at: 2026-08-03T12:10:00+09:00
reopened-because: "shared-checklist.md:9 — a file this task's own scope names — still states the rule this task exists to remove"
scope: "dva repo — internal/config/schema.json, agent-mesh-flows/shared/library/{shared-guardrails,dva-schema,shared-checklist,reference-examples}.md"
---

# Task 067: Make every statement of the `version:` rule agree with the code

## Problem

`internal/config/version.go:1-12` states the semantics, and states them well:

> `version:` states what a config requires of its reader, not which binary produced it —
> scaffolding the running version would make every new config refuse to load on any older
> DVA, ratcheting the floor upward on each release.

So `version:` is a **minimum required reader version**, and it is **optional**. Eight other
files restate the rule. Five of those statements say something else.

| site | what it says | agrees? |
| --- | --- | --- |
| `internal/config/version.go:1-12` | optional; a floor, deliberately not the running version | **authoritative** |
| `internal/config/config.go:1138` | `Version == ""` → `return nil`; "Empty version is allowed (no gate)" | yes |
| `internal/config/subproject.go:31,46,63` | same `checkConfigVersion` per file; **no root comparison exists** | yes |
| `internal/cli/init.go:238-241` | writes `MinScaffoldVersion` (`0.1.44`), with a comment explaining why not `Version` | yes |
| `internal/config/schema.json` top-level `required` | `["version"]` — **mandatory** | **no — rule A** |
| `agent-mesh-flows/shared/library/*` (4 files, 5 statements) | must **equal** the current CLI version; subprojects must **match root** | **no — rule B** |

## Rule A: the schema makes `validate` stricter than DVA itself

A `dva.yml` with no `version:` key loads fine everywhere except the one command whose job is
to tell you whether it is valid:

```
$ cd <config with no version: key>
$ dva ls        → rc=0
$ dva doctor    → rc=0
$ dva manifest  → rc=0
$ dva show      → rc=0
$ dva validate  → rc=1
ERROR: schema validation failed in dva.yml:
  - (root): version is required
```

That is the worst direction for a validator: it reports a defect in a config the rest of the
tool runs without complaint. `checkConfigVersion` deliberately allows the empty case; the
schema never got the memo.

**Blast radius is small but not zero.** All 31 live configs in `~/mydevbox` declare
`version:`, so no existing config is affected, and `dva init` always emits it. It bites
someone hand-writing a minimal config — exactly the person most likely to run `validate`
to find out what a minimal config needs.

## Rule B: the AI generator is taught to ratchet the compatibility floor

Four library files, five statements, tell the generator the opposite of what `version.go`
warns about:

- `shared-guardrails.md:19` — "**`version:` field** — Must match the current DVA CLI version.
  Subproject versions must also match root."
- `dva-schema.md:11` — "Use the current DVA version … Subprojects should match."
- `dva-schema.md:662` — "Version must match root (use current DVA version)"
- `shared-checklist.md:54` — "Imported subproject `version` matches root"
- `reference-examples.md:548` — "# version MUST match root"

Both halves are wrong:

1. **"must equal the current CLI version"** is precisely the behavior `version.go` exists to
   prevent. A generated config pinned to the generating binary's version refuses to load on
   any older DVA, and the floor climbs on every release. `init.go` was deliberately written
   *not* to do this; the generator is told to do it anyway.
2. **"subprojects must match root"** has **no implementation anywhere.** `subproject.go` runs
   the same per-file `checkConfigVersion` against the running binary at lines 31, 46 and 63 —
   there is no comparison to the root config's value. Grepping every `Version` mention in
   `subproject.go` and `merge.go` returns only those three call sites. The rule is invented.

Rule B is the worse of the two: rule A over-rejects a shape nobody writes, while rule B makes
the generator actively emit configs with an artificially high floor, and asserts a
constraint DVA never checks.

## Root cause

Same defect class as [TASK-057](../_archive/057-dead-self-referencing-urls.md) (a hardcoded URL), TASK-060 (a module path) and
[TASK-065](../_archive/065-gitignore-check-misses-ancestor-rules.md) (a hand-written gitignore
predicate): **a second copy of knowledge that nothing compiles.** `schema.json` is data and
the library files are prose, so neither can disagree with `checkConfigVersion` loudly enough
to fail a build. `tools/libgen` already proves the fix direction — it injects reserved
commands and canonical section order into `shared-guardrails.md` *from Go*, so those two
facts cannot drift. `version:` was left as prose.

## Fix shape

**Rule A** — drop `"version"` from `schema.json`'s top-level `required`. Loosening only; all
31 live configs and every `dva init` output keep validating. Add a test pinning that a
version-less config passes `validate`, since nothing pins either behavior today.

**Rule B** — correct all five statements to state the actual rule: `version:` is optional and
declares the **minimum** DVA version the config requires; it is not the generating binary's
version, and subprojects are not compared to root. Delete the `shared-checklist.md:54` item,
the `dva-schema.md:662` bullet and the `reference-examples.md:548` comment outright — they
assert a check that does not exist.

Prefer making `libgen` inject `MinScaffoldVersion` into the library text rather than
hand-writing `0.1.44` in prose, so this cannot drift again. If that is more than this task
should carry, hand-write it and note the drift risk — but do not introduce a fourth
hardcoded version string.

Also stale, fix while here: `reference-examples.md` carries `version: "0.1.29"` (surfaces as
`library_reference.txt:1707`).

## Non-goals

- Do not change `checkConfigVersion`, `MinScaffoldVersion`, or the minimum-version semantics.
  The code is right; the copies are wrong.
- Do not edit `internal/cli/library_reference.txt` directly — it is a `make generate`
  artifact (`Makefile:13`) built from `agent-mesh-flows/shared/library/`. Editing it is
  overwritten on the next generate.
- Do not add a root↔subproject version check to make rule B true. `version.go` explains why
  per-file floors are the right model.
- Do not touch `~/mydevbox` configs; none are affected.

## Acceptance criteria

- [x] A `dva.yml` with no `version:` key passes `dva validate` | verify: `go test ./internal/config/ -run TestValidateWithoutVersion`
- [x] `version` is absent from schema.json's top-level `required` | verify: `python3 -c "import json;assert 'version' not in json.load(open('internal/config/schema.json')).get('required',[])"`
- [x] `dva init` output still validates | verify: `human — dva init in a temp dir with a compose file, then dva validate, expect rc=0`
- [ ] No library file claims version must equal the CLI version | verify: `! /usr/bin/grep -rniE 'version.{0,20}(matches|must match|equal).{0,20}current DVA|현재 DVA (CLI )?버전' agent-mesh-flows/shared/library/ internal/cli/library_reference.txt`
- [x] No library file claims subprojects must match root | verify: `! /usr/bin/grep -rniE 'version.*match(es)? root|Subprojects? (should|must) match' agent-mesh-flows/`
- [x] Regenerated artifact agrees | verify: `make generate && git diff --exit-code internal/cli/library_reference.txt || true`
- [x] Full suite green | verify: `make test`

## Resolution

**Rule A** — `"required": ["version"]` removed from `schema.json`. The `version` property's
description now states it is optional and a floor rather than the writing binary's version.
Verified end-to-end against a version-less config: `validate`/`ls`/`doctor`/`manifest`/`show`
all rc=0, where `validate` alone was rc=1 before.

**Rule B** — done the drift-proof way rather than by hand. `tools/libgen` gained a third
injected fact, `version_rule`, sourced from `config.MinScaffoldVersion`, and rule 4 of
`shared-guardrails.md` is now an `<!-- AUTOGEN:version_rule -->` block. The two invented
claims were deleted outright: `shared-checklist.md`'s "subproject `version` matches root"
item and `reference-examples.md`'s `# version MUST match root` comment (whose stale
`version: "0.1.29"` went with it — the example now omits `version:` and says why).
`dva-schema.md`'s two statements were corrected at their new canonical path,
`skills/config/references/schema-reference.md` (moved under `skills/` by `bb47998` while
this task was open; the library keeps a symlink, which `make generate` follows via `cat`).

`MinScaffoldVersion` being a const is what makes the injection safe: no build can inject a
different floor, so the prose cannot disagree with the binary. Prose now carries **zero**
hand-written DVA version strings.

## Evidence

Measured 2026-07-30 against `bin/dva` at `config.Version = 0.1.44`.

Reproduction — a `dva.yml` declaring only a `stack:` compose entry, no `version:`:

```
ls         rc=0
validate   rc=1   ("version is required")
doctor     rc=0
manifest   rc=0
show       rc=0
```

Corpus: 31 of 31 `~/mydevbox` configs declare `version:`, so rule A affects no existing
config. Nothing pins either behavior — no test in the repo contains the string
`version is required`, and none references `checkConfigVersion`.

"No root comparison exists" was established by listing *every* `Version` mention in
`internal/config/subproject.go` and `internal/config/merge.go`, not by a targeted grep for a
guessed pattern.

## Reopened 2026-08-03 — one in-scope file was never corrected

Rule A is fixed and independently confirmed: `version` is gone from `schema.json`'s
`required`, a hand-built version-less `dva.yml` passes `ls`/`validate`/`doctor`/
`manifest`/`show`, and `dva init` scaffolds `MinScaffoldVersion`, which validates.

Rule B is fixed in three of the four library files. It is **not** fixed in the fourth:

```
agent-mesh-flows/shared/library/shared-checklist.md:9
- [ ] `version` field matches current DVA CLI version
```

That is Rule B claim #1 verbatim — the claim this task exists to delete. The fix commit
`db07203` edited the adjacent line in the same section (the "subproject `version` matches
root" line, Rule C) and left this one. The old criterion-4 binding missed it because it
matched the exact English string `Must match the current DVA CLI version`, and this line
is phrased differently.

It is not inert prose. It is read raw into the live `dva-improve` prompt, and it is baked
into the committed artifact at `internal/cli/library_reference.txt:111`, so the generated
embed carries it too.

**To close:** correct or delete `shared-checklist.md:9` the way `shared-guardrails.md` and
`reference-examples.md` were corrected, then `make generate` so
`library_reference.txt:111` follows. Criterion 4's binding has been widened to match on
meaning rather than one exact sentence, and to sweep the generated artifact as well.

Out of scope and tracked separately: `agent-mesh-flows/dva-improve.yaml` — the flow that
actually writes configs — still instructs the LLM and its own `fix_version` shell step to
write the running DVA version. See [TASK-135](../todo/135-dva-improve-writes-the-running-version-into-every-config-it-touches.md).
