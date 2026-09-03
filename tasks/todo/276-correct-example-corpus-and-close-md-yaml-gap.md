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
depends-on: [TASK-266]
---

# Task 276: correct example corpus and close the markdown/YAML gate gap

## Summary

The tracked example corpus contains commands that error and a config shape that fails to
parse, one of which is a safety-relevant misstatement about when DVA prompts before
destroying data; and the schema test that is supposed to catch this class of defect only
walks `*.yml` files, so none of the errors embedded in markdown were ever caught. This task
fixes the confirmed corpus defects and extends the gate so the same class of defect cannot
recur silently.

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
- [ ] `examples/MAKEFILE.md` uses `dva version` in place of the non-existent `dva --version` flag | verify: `! /usr/bin/grep -q -- 'dva --version' examples/MAKEFILE.md`
- [ ] `examples/MAKEFILE.md`'s `provision:` example is rewritten as a mapping (`default_profile` plus a flat `<profile>: [ProvisionItem, ...]` list, or a bare `<profile-name>:` key holding the list) that unmarshals against `ProvisionConfig.UnmarshalYAML` | verify: human — reviewer copies the corrected block into a scratch `dva.yml` and confirms `dva config validate` does not reject it as unparseable
- [ ] `internal/config/examples_schema_test.go` (or a sibling test) extracts fenced YAML code blocks from `examples/*.md` and validates each against the same schema/semantic path used for `examples/*.yml`, so a defect like item 3 fails a test instead of shipping silently | verify: `go test ./internal/config -count=1`
- [ ] Every file in `examples/*.yml` passes `dva config validate --strict` cleanly (reorder sections to `canonicalSectionOrder` and either ship the referenced compose files or adjust the examples so config-drift warnings do not fire) | verify: human — reviewer runs `dva config validate --strict` from each example's directory (with its referenced compose files present, or the example adjusted not to reference missing ones) and confirms zero warnings for all 16 files
- [ ] Repository gates pass | verify: `make lint && make test && make doc-check`

## Non-goals

- `examples/env-file-priority.yml`'s interaction-level `env_file` declarations and the
  "Command-specific env_file" section of `examples/README.md` are owned by
  [TASK-266](266-deprecate-and-reject-interaction-env-file.md) and are out of scope here —
  do not touch either as part of this task.
- No change to `confirmDestruction`'s gating behaviour itself (whether `-v` alone *should*
  prompt is a product decision outside this task); this task only corrects the documentation
  to match the current, confirmed behaviour. Note that closing this card leaves DVA
  deleting volumes without a prompt on bare `-v` — the doc fix removes the false assurance
  but does not remove the hazard. That decision is carried as an open question in
  [PLAN-004](../plan/004-restore-documentation-truth.md).
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
