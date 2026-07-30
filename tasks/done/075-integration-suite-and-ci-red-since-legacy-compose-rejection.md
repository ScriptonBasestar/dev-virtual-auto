---
id: TASK-075
title: "The integration suite — and therefore CI on master — has been red since the legacy compose rejection"
type: fix
priority: P2
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/integration — legacy_compose_test.go + testdata/fixtures/legacy-compose/dva.yml"
---

# Task 075: Reconcile the legacy-compose test with the loader that rejects it

## Problem

`make test-integration` fails:

```
--- FAIL: TestLoadLegacyComposeFixture (0.00s)
    legacy_compose_test.go:16: failed to load fixture "legacy-compose": entry "compose":
    compose must be declared under runners.compose, not on the entry itself
```

The test existed to assert that `Load()` **accepts** the legacy `stack.<entry>.compose`
shape. `553c478` (2026-07-29, "fix(config): reject legacy compose shapes in the loader")
deliberately removed that support, and did not update this test.

`.github/workflows/ci.yml:29` runs `make test-integration` with no `continue-on-error`, so
CI on `master` has been failing since that commit. It went unnoticed because `make test`
— the target run locally — does not include the `-tags=integration` suite.

The test's doc comment also became false: it claimed the path "`ComposeConfig()` still
supports", and the fixture's own comment said "The loader still supports it for back-compat".

## Why it matters

`553c478` fixed a real disagreement — schema.json rejected three compose shapes that
`Load()` accepted, so 31 of 83 real configs behaved differently under `dva validate` than
under every other command. The fix was right.

But it created a second disagreement of the same kind, one layer up: a test asserting the
old contract and a loader implementing the new one, with nothing reconciling them because
the suite that would notice is not in the default target. This is the recurring shape of
[TASK-072](072-flows-read-version-from-a-flag-that-never-existed.md) and
[TASK-073](073-version-error-blames-the-config-for-a-build-defect.md) — a second statement
of a rule that nothing compiles — except here the second statement *is* a test, and it
still went unread for a day.

P2 rather than P4 because a red gating CI step suppresses signal for every subsequent
change, not just this one.

## Fix shape

Two options; the second was taken.

1. Delete the test and fixture. The path they cover no longer exists.
2. Flip the assertion: keep the fixture and assert the loader **rejects** it, with the
   actionable message intact.

(2) is worth the four extra lines. The fixture is the only place the legacy shape is written
down from the outside, so if the loader ever quietly starts accepting it again, the
disagreement with schema.json reappears in a test rather than in someone's `dva.yml`.

## Non-goals

- Do not migrate the fixture to `runners.compose`. It must keep failing to load.
- Do not revisit `553c478`'s decision to reject the legacy shape.
- Do not add the fixture to the validate tests.

## Acceptance criteria

- [x] The integration suite passes | verify: `make test-integration`
- [x] It is not passing vacuously | verify: `test 17 -eq "$(go test -tags=integration -race ./internal/integration/... -v 2>&1 | /usr/bin/grep -c '^--- PASS')"`
- [x] The new test is not vacuous | verify: `human — mutation check below; pointed at a loadable fixture it FAILS`
- [x] The unit suite still passes | verify: `make test`
- [x] No comment still claims the loader supports the legacy shape | verify: `test 0 -eq "$(/usr/bin/grep -rc 'still supports it for back-compat\|path that ComposeConfig() still supports' internal/integration/ 2>/dev/null | /usr/bin/grep -cv ':0$')"`

## Evidence

The breakage is `553c478`'s and not this session's:

```
$ git log --oneline -S 'compose must be declared under runners.compose' -- internal/config/
553c478 fix(config): reject legacy compose shapes in the loader     # 2026-07-29
$ git log --oneline -3 -- internal/integration/legacy_compose_test.go
5b98bb9 fix(integration): migrate fixtures to runners.compose ...   # 2026-07-16
$ git merge-base --is-ancestor 5b98bb9 553c478 && echo "rejection came after the test"
rejection came after the test
```

`git diff --stat` at the time of discovery showed only `internal/config/config.go` and
`config_test.go` modified (TASK-073), neither of which touches compose shapes.

## Resolution

`TestLoadLegacyComposeFixture` became `TestLegacyComposeFixtureIsRejected`, using the
existing `loadFixtureConfigErr` helper (`integration_test.go:31`, previously used only by
`TestLoadInvalidFixture`). It asserts both that loading fails and that the error still names
the cause *and* prints the replacement shape — the error's value is that it is actionable,
so the actionable half is pinned too.

The fixture comment now says the shape is rejected by both layers, cites `553c478`, and says
explicitly that the fixture must keep failing to load.

**Mutation check.** A test that asserts an error is thrown passes trivially if it never
reaches a working path, so the fixture name was temporarily swapped for `basic`:

```
--- FAIL: TestLegacyComposeFixtureIsRejected
    legacy_compose_test.go:25: Load(legacy-compose) error = nil, want a rejection of the
    legacy compose shape
```

Restored from a scratchpad copy afterwards, not re-edited.

**Count beside the verdict.** `make test-integration` printing `ok` proves little on its own
— a build-tag mistake produces the same line with zero tests run. 17 `--- PASS` lines with
`-race` is the actual result.

## Left open

`make test` and `make test-integration` are separate targets and only CI runs both, which is
how a red suite survived a day. Whether the default local target should include the
integration suite — it needs no external services, and adds about 1.5s — was not decided
here.
