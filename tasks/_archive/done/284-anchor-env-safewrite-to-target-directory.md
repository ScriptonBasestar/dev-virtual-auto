---
id: TASK-284
title: "Anchor the env-bridge safe write to the target's directory, not the config root"
type: bug
priority: P1
effort: M
exec-tier: strong
created-at: 2026-09-03T19:45:00+09:00
source: "Independent review of TASK-246 (`cccb310`) by a reviewer that did not write it; root cause re-read and the symlink bypass independently reproduced with a standalone os.Root program"
scope: "internal/cli/config_env_safewrite.go newTemp/newSafeWriter/Commit/syncDir, the temp naming rule, internal/cli/config_env_test.go TestConfigEnvRejectsPathSwap and its fakeGit"
status: done
closed-at: 2026-09-03T14:27:42+09:00
depends-on: []
parent: PLAN-002
completed-at: 2026-09-03T14:27:42+09:00
completion-summary: "Opened a second os.Root on the target's own directory during preflight and held it through the rename, so the temp is created beside the target, Commit renames a bare leaf with no path component left to re-resolve, and syncDir flushes the directory the rename actually touched. The temp now carries the target's leaf so glob ignore rules cover it, TestConfigEnvRejectsPathSwap hard-asserts the rejection code instead of accepting either outcome, and fakeGit records which directory the guard was asked about."
verification-status: verified
verification-evidence:
  - kind: automated
    command-or-step: "go test ./internal/cli -run PathSwap -count=1 && go test ./internal/cli ./internal/config -count=1"
    result: "passed; the in-root symlink swap subtest now requires codePathComponentSymlnk and asserts no bytes landed in either the decoy or the moved directory"
  - kind: automated
    command-or-step: "make lint && make test && make test-integration && make doc-check && make commit-check"
    result: "passed; every gate exited zero"
quality-review: pass
quality-reviewed-at: 2026-09-05T09:48:35+09:00
quality-review-evidence:
  - "independent re-review re-ran every machine binding on master 4d80158: go test ./internal/cli -run PathSwap (12 subtests including target_parent_replaced_by_an_in-root_symlink_mid-run), go test ./internal/cli and ./internal/config, and make lint / test / test-integration / doc-check / commit-check, all exit 0"
  - "criterion 3 was checked by reading the test rather than by its exit code, since a passing test proves nothing about whether it would fail: the subtest now calls requireCode(t, err, codePathComponentSymlnk) and asserts .env is absent from both the decoy and the moved directory, so the either-outcome acceptance the card indicted is gone"
  - "criterion 1/2/5 confirmed in source: Commit renames through w.anchor.dir.root with w.anchor.leaf and re-checks stillAnchored() immediately before the rename, and both syncDir calls go through the anchor rather than the config root"
  - "the three human bindings were verified directly rather than taken on trust. Criterion 9: newTemp's comment states it creates the temp in the anchor, that is the target's own directory, which matches tempName and the call site. Criterion 10: sops receives the temp fd as cmd.Stdout so plaintext never enters a DVA buffer, stderr is capped at 8 KiB by limitedWriter and never echoed, and tempName derives from the target leaf, pid and a random token with no component from file content. Criterion 6: the residue is bounded rather than eliminated, and that limit is stated in three places - a test asserting the temp matches .env*, .env.* and *.tmp, the tempName comment, and USAGE.md, which tells the user to widen an exact-name ignore rule to .env*"
  - "no blocker. One observation: the temp name shape frozen in TASK-245 sections 7-4 and 8-5 as .dva-env-<pid>-<nanos>.tmp is superseded by <leaf>.dva-env-<pid>-<token>.tmp; the change is deliberate and recorded in the tempName comment and USAGE.md, but PLAN-002 lists TASK-284 by title only and carries no supersession note for it"
archived-at: 2026-09-05T09:48:35+09:00
verified-at: 2026-09-05T09:48:35+09:00
verification-summary: "The implementation now meets the sections 5-3, 5-4 and 8-1 properties below the config root that TASK-246 delivered only at the root; the frozen rulings were not reopened, and the one contract-visible change is the temp name shape from sections 7-4 and 8-5."
---

# Task 284: anchor the env-bridge safe write to the target's directory

## Summary

The env bridge creates its temporary file at the **config root** and then renames it to the
target's declared path. When the target has a directory component — `env_file: sub/.env` — the
directory part is re-resolved by name at rename time, and every guarantee the preflight
established for that directory is discarded at exactly the moment it is needed.

Three defects follow from that one fact, and the first lets decrypted plaintext land in a
directory that passed neither the symlink gate nor the git guard.

Two doc comments assert the property that is missing. `newTemp` says it "creates a uniquely
named temporary **in the target's directory**" (`internal/cli/config_env_safewrite.go:175`) and
`newSafeWriter` says it creates the temp "**in the same directory as the target**" (line 218).
`tempName` (line 166) returns `.dva-env-<pid>-<token>.tmp` with no directory component at all,
and `newSafeWriter` opens it directly on the root handle. Both comments are true only for a
target that sits at the config root.

[TASK-282](282-implement-gated-env-bridge-commands.md) §35 states the same convention as a
requirement for the second writer — "same-directory 0600 O_EXCL temp → 검증 → rename → parent
fsync". This card makes the first writer meet the convention the second one is being held to.

## Problem

1. **The symlink-component gate and the git guard are both defeated at commit time.** (HIGH)

   `checkPath` walks every component through the handle and rejects a symlinked one with
   `codePathComponentSymlnk` (`internal/cli/config_env_safewrite.go:89-91`). Its own comment
   explains why the gate must be explicit: `os.Root` blocks escapes but **deliberately follows
   symlinks that stay inside the root**, so containment alone is not the property the contract
   wants.

   `Commit` then calls `w.root.root.Rename(w.name, target)` (line 269) with `target` still
   carrying its directory component. The handle is the root's, so `sub` is resolved by name at
   that instant — after the sops child has run, which for a KMS-backed key is an unbounded
   network wait.

   Reproduced with a standalone program using only `os` on go1.26.5 — the same sequence the
   implementation performs, with an in-root symlink swapped during the window:

   ```
   preflight Lstat(sub/.env): no such file or directory   (gate: missing leaf, allowed)
   [temp created at the root, payload written]
   [attacker: rm sub; ln -s decoy sub]
   Rename(tmp, sub/.env) after swap: err=<nil>
     absent real/.env
     LANDED decoy/.env -> "SECRET=plaintext\n"
   ```

   The rename **succeeds**. Neither `real/` — the directory the preflight approved — nor the
   contract's stated guarantee survives. The §5-4 git guard is bypassed identically: it asked
   about a path in `real/`, and the bytes landed in `decoy/`.

   `TestConfigEnvRejectsPathSwap` (`internal/cli/config_env_test.go:184-216`) exercises this
   scenario and **passes because of the hole**: it accepts either outcome, logging rather than
   failing when the write is not rejected, and then asserts only that the payload is intact in
   whichever directory it found. It documents the behaviour instead of pinning the property.
   `fakeGit` compounds this — its method discards both arguments, so no unit test can observe
   *which* directory the guard was asked about.

2. **A subdirectory target's parent is never fsynced.** (MEDIUM)

   `syncDir` opens `"."` on the root handle (line 288-294). For `sub/.env` the durability step
   flushes the config root, not `sub`. §8-1 requires the file and its parent directory to be
   fsynced; at the root that holds, one level down it does not, and a crash can lose the rename
   after the command reported success.

3. **The plaintext temp sits at the config root under a name nothing ignores.** (MEDIUM)

   §5-4 refuses to write a target that git tracks or does not ignore. The temp holds the same
   decrypted bytes and is subject to no such check: `.dva-env-*.tmp` matches no pattern in the
   repository's ignore files, and it is created at the config root where `git add -A` will find
   it. `reclaimStaleTemps` collects it on the *next* run after an hour (§8-4 is explicit that
   SIGKILL cleanup is not promised) — so between a `kill -9` and that next run, a commit picks
   up secrets.

4. **A dotenv line over 64 KiB is reported as malformed at line 0.** (LOW) The scanner's buffer
   limit surfaces as a parse error with no line number, which reads as a corrupt file rather
   than an oversized line.

5. **A post-rename `syncDir` failure exits 1 after the target was already replaced.** (LOW)
   `Commit`'s final `return w.root.syncDir()` (line 275) runs after the rename. Its error
   reaches the user as a codeless envelope, contradicting the help text's promise that "any
   failure leaves an existing target byte-for-byte unchanged" — here the target was changed and
   the command still reported failure.

## Direction

§1, §2 and §3 have one repair between them: **open a second `os.Root` on the target's parent
during preflight and hold it through the rename.** The temp is then created in that directory,
the rename is a same-directory operation with no path component left to re-resolve, and
`syncDir` flushes the directory the rename actually touched. §3 shrinks to the target's own
directory, which is where the user's ignore rules for the env file already point.

This is not a contract change. §5-3, §5-4 and §8-1 already state the properties; the
implementation does not deliver them below the root. Do not reopen the frozen rulings for this.

For §3, decide separately whether the temp should additionally be named so that a stray one is
covered by the same ignore rule as the target — that is a new rule and should be stated, not
assumed.

Whatever the mechanism, `TestConfigEnvRejectsPathSwap` must be rewritten to fail when the write
is not rejected, and `fakeGit` must record the path it was asked about, or the test suite will
keep certifying this class of defect as passing.

## Completion Criteria

- [x] The temp for a target with a directory component is created in that target's directory, not at the config root | verify: `go test ./internal/cli -count=1`
- [x] A rename whose directory component was swapped for an in-root symlink after preflight is refused, not committed into the swapped directory | verify: `go test ./internal/cli -run PathSwap -count=1`
- [x] `TestConfigEnvRejectsPathSwap` fails when the write is not rejected — it no longer accepts either outcome | verify: `go test ./internal/cli -run PathSwap -count=1`
- [x] `fakeGit` records the path it was asked about, and a test asserts the guard was asked about the directory the bytes actually land in | verify: `go test ./internal/cli -count=1`
- [x] `syncDir` flushes the directory the rename landed in for a subdirectory target | verify: `go test ./internal/cli -count=1`
- [x] A `kill -9` between temp creation and rename leaves no plaintext file that `git status` reports as untracked-and-not-ignored | verify: `human — create the temp, kill the process, run git status --porcelain in the fixture repo and confirm the temp is absent or ignored`
- [x] A dotenv line over the scanner limit is reported as an oversized line with its line number, not as a malformed file at line 0 | verify: `go test ./internal/config -count=1`
- [x] A post-rename failure is either impossible or reported in a way that does not claim the target is unchanged | verify: `go test ./internal/cli -count=1`
- [x] The `newTemp` and `newSafeWriter` doc comments describe where the temp is actually created | verify: `human — read both comments against tempName and the newSafeWriter call site`
- [x] The eight §7-4 secrecy constraints still hold after the change — plaintext never enters a DVA buffer, temp names carry nothing derived from content, sops stderr stays bounded and unechoed | verify: `human — re-read the §7-4 checklist against the new temp and rename path`
- [x] Repository gates pass | verify: `make lint && make test && make test-integration && make doc-check && make commit-check`

## Non-goals

- No change to the frozen §5-3/§5-4/§8-1 rulings. This card makes the implementation meet them.
- No change to the `env_bridge` gate, `seal`, or `show`, which are
  [TASK-281](281-freeze-gated-env-bridge-commands.md) and
  [TASK-282](282-implement-gated-env-bridge-commands.md).
- No change to `reclaimStaleTemps`' one-hour rule (§8-5), which exists so a concurrent run's
  temp is never a candidate.

## Troubleshooting Log

- 증상: 길이 정확히 `bufio.MaxScanTokenSize`인 줄이 통과할 것으로 기대한 테스트가 실패 / 원인: `bufio.Scanner`가 반환하는 최대 토큰은 버퍼 크기가 아니라 그보다 1 작음(65535 OK, 65536 거부) / 해결: `MaxDotenvLineBytes = bufio.MaxScanTokenSize - 1`로 "실제 허용 길이"를 이름 붙이고 메시지도 그 값으로 고침 / 25분
- 증상: post-rename fsync 실패가 `permission_denied`("아무것도 쓰이지 않았다")로 뒤바뀌어 보고될 수 있음 / 원인: `postRenameError.Unwrap()` 때문에 `errors.Is(err, fs.ErrPermission)`가 참이 되어 먼저 매칭됨 / 해결: `runEnvUnseal`에서 `errors.AsType[*postRenameError]` 검사를 permission 매핑보다 앞에 두고, 그 전제를 테스트로 고정 / 20분
