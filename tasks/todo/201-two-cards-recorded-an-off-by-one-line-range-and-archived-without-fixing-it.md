---
id: TASK-201
title: "Two cards recorded an off-by-one line range as a finding and archived without fixing it"
type: docs
priority: P3
effort: XS
created-at: 2026-08-19T17:36:34+09:00
source: "measured 2026-08-19 — 082:162 and 123:157 both cite lines 74–79 for a clause that runs 74–80; both cards flag the discrepancy in their own evidence (082:39, 123:46) and neither corrects the criterion"
scope: "The AC suffix on tasks/_archive/082 and tasks/_archive/123. The clause in workflows/dva-dogfood/40-evaluate.md is correct and is not touched."
status: todo
---

# Task 201: Two cards recorded an off-by-one line range as a finding and archived without fixing it

## Summary

`tasks/_archive/082-…:162` and `tasks/_archive/123-…:157` carry a byte-identical acceptance
criterion:

```
- [x] The cross-run-promotion note reaches stage 40 (not deleted `60-evaluate.md`) | verify: `rg -n 'Cross-run promotion|case_manifest_hash' workflows/dva-dogfood/40-evaluate.md` — prints the hash-delta-is-a-promotion clause at lines 74–79
```

Measured 2026-08-19: the clause opens at `workflows/dva-dogfood/40-evaluate.md:74`
("**Cross-run promotion.** A run whose `case_manifest_hash` differs from its") and closes at
`:80` ("prompt change but never replaces current-run gates."), with `</gate>` at `:81`. The
range is 74–80. The binding itself is sound — `rg` finds the clause and exits 0 — so the
defect is confined to the human-readable suffix.

What makes this worth a card rather than a silent edit is that **both cards already found
it**. `082:39` records "FINDING — the criterion's suffix says the clause sits at lines 74-79;
it runs 74-80", and `123:46` records the same sentence. Both then reached disposition,
`quality-review: pass`, and archived with the criterion unchanged. Two independent
observations of the same defect produced two records of it and zero corrections.

That pattern is the point. A finding written into an evidence block feels like it has been
handled; nothing in the disposition flow forces it to become either a fix or a follow-up
card. This card exists to close that particular one and to leave the general question
visible.

## Completion Criteria

- [ ] Both criteria state the range the clause actually occupies | verify: `grep -c '74–80\|74-80' tasks/_archive/082-*.md tasks/_archive/123-*.md` returns ≥ 1 for each file in the criterion line, not only in the evidence block
- [ ] No occurrence of the wrong range survives outside an explicit "first read" note | verify: `grep -rn '74–79\|74-79' tasks/_archive/*.md` — every remaining hit is inside a sentence recording what the line first said
- [ ] The clause boundary is re-derived rather than copied from this card | verify: `rg -n 'Cross-run promotion|never replaces current-run gates' workflows/dva-dogfood/40-evaluate.md` prints the opening and closing lines, and the cited range matches them
- [ ] The corrected criterion still passes its own binding | verify: `rg -n 'Cross-run promotion|case_manifest_hash' workflows/dva-dogfood/40-evaluate.md` exits 0
- [ ] `make doc-check` passes | verify: `export PATH="$HOME/.local/share/mise/shims:$PATH" && make doc-check`

## References

- `tasks/_archive/082-the-dogfood-loop-cannot-score-an-absent-section.md:39,70,162`
- `tasks/_archive/123-dogfood-loop-cannot-score-a-reserved-name-collision.md:46,72-73,157`
- `workflows/dva-dogfood/40-evaluate.md:74-80` — the clause, and `:81` — the closing `</gate>`

## Open Questions

- The general question this card only illustrates: a `FINDING` recorded in an evidence block
  has no defined disposition. It is neither a blocking criterion nor a registered follow-up,
  so it can be observed truthfully and still vanish. Whether the card schema should require
  each `FINDING` to name a resolution ("fixed here" / "card N" / "accepted, because") is a
  larger change and belongs in its own card — but this is the second time the archive has
  shown the failure mode.
- Line-number citations in criteria go stale whenever the referenced file is edited. Citing
  the anchor string instead of the range would make the suffix durable; that convention change
  is out of scope here and should not be settled by one card.

## Technical Notes

- The two cards are byte-identical on this line. Fix them in one edit and verify both, rather
  than fixing one and assuming the other followed.
- Do not renumber or touch `40-evaluate.md`; it is the correct side of this discrepancy.
