---
id: TASK-276
title: "Correct the tracked example corpus and close the markdown-YAML validation gap"
type: bug
priority: P2
effort: M
exec-tier: standard
created-at: 2026-09-03T00:17:00+09:00
source: "Docs audit of README.md, examples/*.md, examples/*.yml, and internal/config/examples_schema_test.go at HEAD 5eb1af5"
scope: "README.md destructive-teardown confirmation claim, examples/MAKEFILE.md CLI and provision errors, examples/*.md sweep, examples_schema_test.go markdown coverage, examples corpus --strict compliance"
status: todo
depends-on: []
---

# Task 276: correct example corpus and close the markdown/YAML gate gap

## Summary

The tracked example corpus contains commands that error and a config shape that fails to
parse, one of which is a safety-relevant misstatement about when DVA prompts before
destroying data; and the schema test that is supposed to catch this class of defect only
walks `*.yml` files, so none of the errors embedded in markdown were ever caught. This task
fixes the confirmed corpus defects and extends the gate so the same class of defect cannot
recur silently.

**This card is startable now.** It declared `depends-on: [TASK-266]` because both cards touch
`examples/`, and TASK-266 is still open on its Stage B release gate — which made this one read
as blocked for as long as 0.1.48 has not shipped. The dependency was only ever about the shared
files, and TASK-266 **Stage A** already cleared them in `c6aa64b`: `examples/env-file-priority.yml`
now declares `env_file:` at the root only, with no interaction-level copy, and the
"Command-specific env_file" section is gone from `examples/README.md`
(verify: `/usr/bin/grep -n env_file examples/env-file-priority.yml` and
`/usr/bin/grep -c 'Command-specific' examples/README.md`). Stage B touches the schema and the Go
types, not `examples/`. The declaration was therefore dropped on 2026-09-03; the Non-goals below
still hold and still mark those two files out of scope, so a session working this card must not
edit them even though nothing now blocks the rest.

## Problem

> Line numbers below are as observed at HEAD `5eb1af5` and shift as other sessions commit.
> The quoted strings are the authoritative anchors; locate each defect by string, not by line.

### Half A — corpus defects

1. **Safety-relevant**: `README.md:87` states, in the Lifecycle bullet, that both `` `down
   -v` `` and `` `--purge` `` destructive teardowns go through a confirmation prompt
   ("`down -v`·`--purge` 같은 파괴적 teardown은 확인 프롬프트를 거칩니다"). `confirmDestruction`
   (`internal/cli/confirm.go:50`) has exactly one call site,
   `internal/cli/plan_lifecycle.go:417`, gated on the `--purge` flag alone. `dva down <plan>
   -v` with no `--purge` deletes volumes with **no prompt at all**. The README currently tells
   a reader that `-v` alone is safe-guarded when it is not.

2. `examples/MAKEFILE.md:160` documents `dva --version`. `rootCmd` sets no `Version:` field
   (confirmed absent from `internal/cli/root.go`), so cobra never registers a `--version`
   flag; the only working invocation is the `version` subcommand
   (`internal/cli/version.go:12-13`, wired at `root.go:111`). `dva --version` errors.

3. `examples/MAKEFILE.md` (around line 41-44) documents:
   ```yaml
   provision:
     - dva compose up -d postgres redis
     - dva bundle install
     - dva rails db:setup
   ```
   `ProvisionConfig.UnmarshalYAML` (`internal/config/config.go:481-483`) requires the
   `provision:` value to be a YAML mapping (`node.Kind != yaml.MappingNode` is a hard error);
   a bare sequence does not parse.

4. `examples/DISCOURSE.md` and `examples/provision-step-syntax.yml` were swept for the same
   class of error (removed commands/sections, unparseable provision shape). Both are clean:
   `provision-step-syntax.yml`'s `provision:` block already uses the flat `<profile>:
   [ProvisionItem, ...]` shape the real type expects, and `DISCOURSE.md` contains no reference
   to a removed command or section. No changes needed in either file.

### Half B — gate gap

5. `internal/config/examples_schema_test.go:12-19` walks `examplesDir()` filtering
   `filepath.Ext(path) != ".yml"` — it validates every `examples/*.yml` file against the
   schema but has no path that ever looks inside a `.md` file. The `provision:` sequence
   defect in item 3 lives entirely inside a fenced YAML block in `examples/MAKEFILE.md` and is
   invisible to this test by construction; the same is true of any future config-shape defect
   introduced inside example markdown.

6. Measured at this HEAD: running `dva config validate --strict` against each of the 16 files
   in `examples/*.yml` (each copied alone into an empty directory, since none of the tracked
   examples ships its referenced compose file), 15 of 16 fail — all 15 with a "section order"
   semantic warning, most also with a config-drift warning for a referenced compose file that
   does not exist in the repo. Only `examples/stack-source.yml` passes `--strict` cleanly.
   `skills/dva/references/patterns.md:26` instructs agents to run `dva config validate
   --strict` as the last step of its Standard Workflow — an agent following that guidance
   against most of the tracked examples gets a non-zero exit from the corpus it was told to
   model.

## Completion Criteria

- [ ] The README Lifecycle bullet states that only `--purge` (not `-v` alone) triggers the confirmation prompt, matching `confirmDestruction`'s single `--purge`-gated call site | verify: `human — read the bullet against confirmDestruction's call site; the sentence must not leave a reader believing bare -v is guarded`
- [x] `examples/MAKEFILE.md` uses `dva version` in place of the non-existent `dva --version` flag | verify: `! /usr/bin/grep -q -- 'dva --version' examples/MAKEFILE.md`
- [ ] `examples/MAKEFILE.md`'s `provision:` example is rewritten as a mapping (`default_profile` plus a flat `<profile>: [ProvisionItem, ...]` list, or a bare `<profile-name>:` key holding the list) that unmarshals against `ProvisionConfig.UnmarshalYAML` | verify: human — reviewer copies the corrected block into a scratch `dva.yml` and confirms `dva config validate` does not reject it as unparseable
- [x] `internal/config/examples_schema_test.go` (or a sibling test) extracts fenced YAML code blocks from `examples/*.md` and validates each against the same schema/semantic path used for `examples/*.yml`, so a defect like item 3 fails a test instead of shipping silently | verify: `go test ./internal/config -count=1`
- [ ] Every file in `examples/*.yml` passes `dva config validate --strict` cleanly (reorder sections to `canonicalSectionOrder` and either ship the referenced compose files or adjust the examples so config-drift warnings do not fire) | verify: human — reviewer runs `dva config validate --strict` from each example's directory (with its referenced compose files present, or the example adjusted not to reference missing ones) and confirms zero warnings for all 16 files
- [x] Repository gates pass | verify: `make lint && make test && make doc-check`

## Non-goals

- `examples/env-file-priority.yml`'s interaction-level `env_file` declarations and the
  "Command-specific env_file" section of `examples/README.md` are owned by
  [TASK-266](266-deprecate-and-reject-interaction-env-file.md) and are out of scope here —
  do not touch either as part of this task.
- No change to `confirmDestruction`'s gating behaviour. This is not a deferral: `-v` and
  `--purge` have different reach, and the prompt asymmetry follows that difference exactly.
  `-v` removes named volumes and nothing else, which is what the flag says and what
  `docker compose down -v` means — typing it is the consent. `--purge` additionally removes
  locally built images and every provision marker in the config directory, reaching outside
  the plan the user named, which is what the prompt guards. The gating condition is correct;
  only the README sentence is wrong. See PLAN-004's 검토 종료된 항목 for the full record —
  **do not add a prompt to the `-v` path.**
- No change to `ProvisionConfig.UnmarshalYAML` or any other runtime type — corpus and docs are
  brought into agreement with the existing parser, not the other way around.
- No new example files beyond what's needed to demonstrate a corrected provision shape in
  `examples/MAKEFILE.md`.

## Troubleshooting Log

- Section-order and config-drift warning counts were measured by copying each
  `examples/*.yml` file alone into an empty scratch directory as `dva.yml` and running `dva
  config validate --strict` there, since the repository does not ship compose files alongside
  these examples. This mirrors how the tracked corpus actually ships (no sibling compose
  files exist under `examples/`), so the drift warnings are not an artifact of the
  measurement method.
- Symptom: text-preserving reorder of top-level `examples/*.yml` sections silently dropped
  blank lines between sections. Cause: first script split the file via `text.split("\n")`
  and rejoined section slices with `"\n".join(...)`, which loses the separating blank line
  that belonged to neither slice. Fix: rewrote the reorder step to operate on raw text with
  `re.finditer` + character-offset slicing instead of a line-list join, preserving whitespace
  byte-for-byte; re-verified via `diff` against the pre-edit file. ~20 min.
- Symptom: fixing `applications.yml`'s "interaction.db has subcommands but is not directly
  callable" warning by adding both `service:` and `command:` to the parent `db:` node
  introduced a new "subcommand.shell.command is identical to parent; subcommand is
  redundant" warning. Cause: the added parent `command:` exactly matched the `shell`
  subcommand's `command:`. Fix: kept only `service: postgres` on the parent (confirmed via
  `hasExecutionTarget()` in `internal/config/validate_warnings.go` that a bare `service:` is
  sufficient to count as an execution target), dropping the duplicate `command:`. ~10 min.
- Not fixed, by design: compose-file-existence drift warnings remain on 14/16
  `examples/*.yml` files (all but `kubernetes.yml`/`stack-source.yml`), because no tracked
  example ships its referenced compose file. Considered three fixes and rejected all: (1)
  flat sidecar files collide on the shared name `docker-compose.yml` across examples: (2)
  uniquely-named sidecars aren't recognized by `detectComposeFilesInDir`'s naming
  heuristic, so the mismatch warning persists anyway; (3) per-example subdirectories would
  be a structural repo change touching paths referenced by README.md, DISCOURSE.md, runner
  tests, and other concurrently-edited cards — out of this card's scope. Left unresolved
  with this reasoning recorded for the next card that wants to pick it up.
- Not fixed, by design: `service-orchestration.yml` still reports a "merge infra-compose and
  frontend into one entry" warning because both reference the same compose file. The
  suggested merge would collapse `frontend`'s `order: 40`/`depends_on: [api]` into
  `infra-compose`'s `order: 10`, changing the ordered-startup behavior the example exists to
  demonstrate. Left unresolved rather than mechanically applying a fix that changes what the
  example teaches.

## Orchestrator measurement (2026-09-03, batch run)

Criterion 5 (`examples/*.yml` strict-clean) was measured rather than accepted from the
implementer's report. Each `examples/*.yml` was copied into its own scratch directory as
`dva.yml` and validated there, since `dva config validate` takes no file flag and reads the
`dva.yml` in its working directory:

```
clean=2 dirty=14 total=16   # 14 files exit 1 with 2-4 warnings each
```

Only `kubernetes.yml` and `stack-source.yml` are clean. The criterion demands zero warnings
for all 16, so it is **not met** and this card stays in `todo`.

A first measurement attempt used `dva config validate --strict -f <file>` and reported
`clean=16`. That was a false pass: `-f` is not a registered flag, every invocation exited 1
with `unknown shorthand flag: 'f'`, and the warning-count grep found nothing in the error
text. Recorded because the same shape — a grep count over output from a command that never
ran — will pass any criterion bound this way.

Criteria 2, 4 and 6 were executed and pass. Criteria 1 and 3 are human-bound and carry the
implementer's assertion, not an orchestrator verification.
