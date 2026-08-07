---
id: TASK-181
title: "prompt_bundle_hash has no pinned derivation"
type: bug
priority: P3
status: done
effort: S
completed-at: 2026-08-07
scope: "workflows/dva-dogfood/"
---

# Task 181

## Result

**Derivation** (tracked only, on `ref-artifacts.md`):

```bash
git ls-files -z workflows/dva-dogfood/ | sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1
```

Stages 00 and 10 point at ARTIFACTS rather than restating. Mismatch → list files, record
mid-run drift (not auto gate fail).

**skill_source_hash / installed_skill_hash:** already path-independent content digests of
named skill trees — not dirty-hash porcelain — so they do not need the same ls-files pin;
comments on ARTIFACTS say not to redefine them as `find | sha256`.
