---
title: "Enhance package.json Scripts Mapping"
priority: P2
effort: S
created: 2026-04-02
status: todo
---

# Enhance package.json Scripts Mapping

## Description
We parse `Makefile` targets successfully and push them to the LLM for DVA interaction mapping. However, `package.json` relies on the presence of the file, not explicitly extracting its "scripts" block. 

By explicitly running `jq '.scripts' package.json` during the `scan_build` context step, we can provide the LLM with exact scripts (`npm run <task>`). This ensures that complex build, test, and dev hooks (e.g., `test:e2e:ci`) are accurately migrated to `interaction.replace` or `interaction.subcommands` without the LLM guessing.

## Acceptance Criteria
- [ ] Modify `scan_build` to conditionally extract `"scripts"` from `package.json` using `jq`.
- [ ] Modify `dva-improve.yaml` and `00-analyze.yaml` LLM instructions to migrate these explicitly provided scripts to DVA commands.
