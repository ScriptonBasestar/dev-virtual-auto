---
id: TASK-073
title: "A malformed binary version tells the user to edit their config, and no test reads that branch"
type: fix
priority: P4
status: done
effort: XS
created-at: 2026-07-30T00:00:00+09:00
scope: "internal/config — config.go (isVersionCompatible, malformedVersionError) + config_test.go"
---

# Task 073: Blame the right side for an unreadable version

## Problem

`parseVersion` is called on two different values from
[`isVersionCompatible`](../../internal/config/config.go):

- `required` — the config's `version:` field, which the user can edit
- `Version` — the running binary's own version, which the user cannot

Both failures went through `malformedVersionError`, whose message ended with:

> Omit `version:` entirely for no compatibility gate

On the binary path that reads as advice about a file the user cannot fix, for a fault in the
program reading it. The full message was:

```
this dva binary reports an unreadable version: version "bogus" is not a version:
expected MAJOR.MINOR.PATCH or MAJOR.MINOR, optionally v-prefixed (e.g. "0.1.44").
Omit `version:` entirely for no compatibility gate
```

The wrapper names the binary as the faulty side and then, one sentence later, sends the
reader to their config.

Second, smaller finding: **no test reached that branch.** No test in the repo mentioned
`unreadable`, and `Version` is a `var` whose value comes from `internal/config/version.go`
(`Version = "0.1.44"`) re-injected by `Makefile:19` — so the branch is reachable only through
a build that overrides `-X`, or through an edit to the literal. Surfaced by the TASK-070
review as a dormant finding.

## Why it matters

Low severity by construction: an unreadable binary version cannot happen through
`make build`, which greps the literal out of `version.go` and injects the same value back.
It is a P4 because nothing is broken today.

It is worth fixing anyway because the branch that only fires by mistake is exactly the branch
whose message goes unread until someone is already confused — a custom `-X`, a goreleaser
config with a different version expression, or a hand-edited `version.go`. Misdirected advice
at that moment costs more than usual.

## Fix shape

Move the remedy out of `malformedVersionError` and append the correct one at each of the two
call sites. The shared function keeps only the shape description, which is genuinely shared.
Add a test that overrides `Version` and asserts the message blames the build and does **not**
carry the config advice.

## Non-goals

- Do not change the config-side message. It is correct and users see it.
- Do not make `Version` a const or remove the ldflags injection.

## Acceptance criteria

- [x] The binary-side message no longer advises a config edit | verify: `go test ./internal/config/ -run TestUnreadableBinaryVersionBlamesTheBuild`
- [x] The new test is not vacuous | verify: `human — mutation check below; reinstating the old message makes it FAIL`
- [x] The config-side message is unchanged for users | verify: `human — ./bin/dva validate on version "O.2.0" below; byte-for-byte identical`
- [x] Full suite passes under -race | verify: `make test`
- [x] Binary still builds through the project's own path | verify: `make build`

## Evidence

The branch was reached without editing any tracked file, by overriding the ldflags value on
the test binary:

```
$ go test ./internal/config/ -run TestCheckConfigVersion -v \
    -ldflags '-X github.com/ScriptonBasestar/dva/internal/config.Version=bogus'
checkConfigVersion("0.1.0") error = this dva binary reports an unreadable version: version
"bogus" is not a version: ... Omit `version:` entirely for no compatibility gate, want nil
```

Three of five `TestCheckConfigVersion` subtests fail under that override — which is correct
behaviour for a broken build, and also the proof that no existing test asserted anything
about this path.

## Resolution

`malformedVersionError` now returns only the shape description. `isVersionCompatible` appends
the config remedy on the `required` path and `Reinstall dva or rebuild it with `make build``
on the `Version` path.

`TestUnreadableBinaryVersionBlamesTheBuild` sets `Version = "bogus"` with `t.Cleanup` restore,
then asserts the message contains `this dva binary` and does **not** contain
``Omit `version:` ``. Mutating a package var is safe here because no test in
`internal/config` calls `t.Parallel`.

**Mutation check.** A test that asserts the absence of a string passes trivially if the string
was never going to be there, so the old message was reinstated and the test re-run:

```
--- FAIL: TestUnreadableBinaryVersionBlamesTheBuild
  error = "... Omit `version:` entirely for no compatibility gate. Reinstall dva or rebuild
  it with `make build`", must not advise a config edit for a build defect
```

It fails on the real defect and passes on the fix. `config.go` was restored from a scratchpad
copy afterwards, not re-edited.

**Positive control.** The config-facing message is what users actually meet, so it was checked
end-to-end rather than by reading the format string:

```
$ ./bin/dva validate            # on a config with version: "O.2.0"
ERROR: version "O.2.0" is not a version: expected MAJOR.MINOR.PATCH or MAJOR.MINOR,
optionally v-prefixed (e.g. "0.1.44"). Omit `version:` entirely for no compatibility gate
```

Identical to before the change. `make test` passes with `-race -cover` across all packages
(`internal/config` at 62.7%).

## Left open

`Makefile:19` derives `VERSION` by grepping the Go literal out of `version.go` and injecting
it back with `-X`. That round trip cannot disagree with itself, which is why this branch is
dormant — but it also means the `-X` accomplishes nothing for `make build`, and the only
builds that can reach the branch are the ones that bypass the Makefile. Whether the injection
should be dropped, or whether `version.go` should stop carrying a literal at all, was not
decided here.
