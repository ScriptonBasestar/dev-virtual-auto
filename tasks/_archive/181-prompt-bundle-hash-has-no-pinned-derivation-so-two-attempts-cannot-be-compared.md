---
id: TASK-181
title: "prompt_bundle_hash has no pinned derivation"
type: bug
priority: P3
effort: S
completed-at: 2026-08-07
scope: "workflows/dva-dogfood/"
status: done
quality-review: pass
quality-reviewed-at: 2026-08-07T18:05:08+09:00
verified-at: 2026-08-07T18:05:08+09:00
archived-at: 2026-08-07T18:05:08+09:00
quality-review-evidence: |
  - kind: test
    command-or-step: make test && make doc-check (mise go 1.26.4)
    result: exit 0; shared suite green
  - kind: recheck
    command-or-step: acceptance criteria re-observed
    result: ref-artifacts pin ls-files sha256 derivation
verification-summary: |
  quality-review pass; re-checked deliverables. ref-artifacts pin ls-files sha256 derivation. Shared: make test OK; make doc-check OK (mise go 1.26.4).
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
