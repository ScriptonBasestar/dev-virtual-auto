---
id: TASK-228
title: "The hermetic dogfood fixture has never proven preservation of a dirty tracked file or the complete shared receipt after unlink"
type: test
priority: P1
effort: S
created-at: 2026-08-26T16:20:00+09:00
source: "post-TASK-225 review"
scope: "tools/skilldogfood, its black-box test, and receipt-contract wording"
status: doing
---

# Task 228: make dogfood boundary coverage prove the real contract

## Summary

The hermetic black-box fixture passes an untracked file through the real-target
dry-run. That does not prove preservation of a modified tracked file, whose
porcelain line remains ` M` even if its content changes. After Codex is unlinked
from the shared `.agents/skills` destination, the helper also checks only the
remaining runtime membership even though the installer retains the whole Schema 1
receipt and leaves the installed skills unchanged.

## Decision

Use a committed fixture file and modify it before invoking `run()`. The test must
compare both its exact bytes and the porcelain status before and after the complete
black-box execution. Verify the post-unlink receipt with the same independent
Schema 1 contract used at first install: schema, scope, absolute destination,
version, exact runtime membership, complete installed file hashes, and derived
bundle SHA-256. Compare the pre- and post-unlink shared receipts and installed-file
hashes, allowing only the exact runtime membership change. The verifier remains
independent from `internal/skillinstall`'s private receipt reader.

## Completion Criteria

- [x] The hermetic full `run()` test starts with a committed file modified in the working tree and proves that its bytes and Git porcelain status are unchanged after dogfood | verify: `make test-skill-dogfood`
- [x] After Codex unlink, the shared receipt is checked for every Schema 1 field and exact `[antigravity]` membership, and all non-membership fields plus installed file bytes are preserved from the pre-unlink snapshot | verify: `make test-skill-dogfood`
- [x] Sabotage cases prove that post-unlink version, receipt-file list, bundle SHA-256, or installed-file hash mutation is rejected | verify: `go test ./tools/skilldogfood -run 'TestSharedUnlinkPreservationRejectsReceiptOrFileMutation'`
- [x] The maintainer contract describes the complete retained Schema 1 receipt after shared-runtime unlink | verify: `make doc-check`
- [x] Focused and documentation gates pass | verify: `go test ./tools/skilldogfood && make test-skill-dogfood && make doc-check`
