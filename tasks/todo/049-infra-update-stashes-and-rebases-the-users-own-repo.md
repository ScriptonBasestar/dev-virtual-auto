---
id: TASK-049
title: "dva infra update runs 'git stash' + 'git pull --rebase' on the user's OWN repo, never clones, and misattributes their files to the infra service"
type: bug
priority: P0
status: todo
effort: M
needs-human: true
created-at: 2026-07-16T23:24:00+09:00
source-run-id: 20260716T112622Z-5729d98
discovered-in: sweep-infra report; mechanism re-derived from source and empirically confirmed by the orchestrator
source-severity: CRITICAL
---

# Task 049: `dva infra update` Mutates The User's Project Repository

## Summary

For any git-based infra service, `dva infra update <name>` runs `git stash` and
`git pull --rebase` **against the user's own project directory** — the directory containing
`dva.yml` — instead of against an infra checkout. It **never clones the configured repo**, and
the configured `git:` URL is **never used at all** on this path.

The confirmation prompt does not protect the user, because it **misidentifies whose files it is
about to stash**. It attributes the user's own uncommitted work to the infra service name.

This is the only finding in this audit that can destroy work the user has not committed.

## Root cause — an unreachable clone branch

`internal/cli/infra.go:85-125`.

```go
if svc.Git == "" { return ... }                          // :85  git-based services only
location := resolveInfraPath(svc.Path, c.FileDir())      // :89
if _, err := os.Stat(location); err == nil {             // :91  always true (see below)
    ... git status --porcelain in `location`              // :93-95
    ... prompt, then: runInDir(location, "git", "stash")  // :105
    return runInDir(location, "git", "pull", "--rebase")  // :112  UNGUARDED
}
// Clone — UNREACHABLE for every git service                :115-125
```

```go
func resolveInfraPath(path, cfgDir string) string {       // :133-138
    if filepath.IsAbs(path) { return path }
    return filepath.Join(cfgDir, path)                    // path=="" -> returns cfgDir
}
```

The chain is forced by the schema, not by chance:

1. `schema.json` `.properties.infra` declares `oneOf: [{required:["git"]}, {required:["path"]}]`.
   Under `oneOf`, supplying **both** `git` and `path` matches **both** branches, so validation
   **fails**. A git-based service therefore **cannot legally declare `path`**.
2. So for any git service, `svc.Path == ""`.
3. So `resolveInfraPath("", cfgDir)` returns `filepath.Join(cfgDir, "")` == **`cfgDir`** — the
   directory holding `dva.yml`, i.e. the user's project.
4. `os.Stat(cfgDir)` **always succeeds** (the config was just loaded from it), so `:91` is always
   taken and the clone branch at `:115-125` is **dead code for git services**.
5. Every git command therefore executes in the user's own repository.

`svc.Git` appears exactly once on a live path — `:124`, inside the unreachable clone branch. **The
configured URL is never fetched.** `:112` pulls from whatever `origin` the *user's* repo has.

## Evidence — measured

Probe (`git init` throwaway repo, `dva validate` EXIT=0 liveness gate passed):

```yaml
version: "0.1.0"
infra:
  gitsvc:
    git: https://github.com/example/does-not-exist.git
    ref: main
```

Control — the schema really does forbid pairing `path` with `git`, which is what forces `svc.Path == ""`:

```
$ dva validate            # with BOTH git: and path: set
ERROR: schema validation failed in dva.yml:
  - infra.s: Must validate one and only one schema (oneOf)
EXIT=1
```

The finding:

```
$ echo y | dva infra update gitsvc
[warn] gitsvc has uncommitted changes:
?? bad.yml
?? important_work.txt

Stash changes before updating? [y/N] No local changes to save
EXIT=1
```

`bad.yml` and `important_work.txt` are **the probe repo's own files**. DVA ran
`git status --porcelain` in the project directory and reported the user's files as belonging to
`gitsvc`. `No local changes to save` is **`git stash`'s own output** — `git stash` really executed
against the probe repo. The URL `does-not-exist.git` was never contacted; nothing was cloned.

**Why nothing was lost in this probe:** the two files were *untracked* (`??`), and plain `git stash`
without `-u` ignores untracked files. That is luck, not a safeguard. A user with **tracked, modified**
files hits `:105` and has them shelved. (Not separately measured — plain `git stash` semantics.)

## Three distinct defects, one root cause

| # | Defect | Severity |
|---|---|---|
| 1 | `git stash` (`:105`) runs in the user's project | shelves uncommitted **tracked** work |
| 2 | `git pull --rebase` (`:112`) runs in the user's project, **with no prompt at all** | rebases the user's branch onto their own origin; can conflict / leave a half-finished rebase |
| 3 | The infra repo is **never cloned**; `git:` is never used | the feature does not do its one job |

Defect 2 deserves emphasis: **the prompt only guards the stash.** If the working tree is clean,
`:96` is false, no prompt is shown, and control falls straight through to `git pull --rebase` on the
user's repository. There is no confirmation on the most consequential line in the function.

## The prompt cannot be counted as consent

- `:97` prints `[warn] <serviceName> has uncommitted changes:` — the files listed are the **user's**,
  labelled with the **infra service's** name.
- `:108` prints `Changes stashed. Run 'git stash pop' in the infra dir to restore.` — there is **no
  infra dir**; the stash is in the user's project.

A user answering `y` reasonably believes they are discarding scratch changes in a throwaway infra
checkout. They are stashing their own work. Answering `N` aborts safely (`:102-103`), so the guard
only holds for users who correctly disbelieve the message they are shown.

## Why P0

Every other finding in this audit is DVA doing **less** than promised. TASK-047 was the first
inversion (doing more). This one is the only finding that **mutates state the user did not offer**,
in a repository DVA was never pointed at, behind a prompt that names the wrong target. Recovery is
possible (`git stash pop`, `git rebase --abort`) but requires the user to first realise what
happened — and the tool's own message points them at a directory that does not exist.

## Needs-human — the fix requires a product decision

The mechanism is unambiguous; the correct behaviour is not. A maintainer must decide **where a
git-based infra service should live**, and the answer shapes the fix and the schema:

- **Option A** — a conventional cache dir (`.sb/dva/infra/<name>/`, sibling to the existing marker
  dir). Makes the clone branch reachable and leaves the user's repo alone. Requires deciding
  gitignore/cleanup semantics.
- **Option B** — relax the `oneOf` so `git` + `path` may be given **together** (`git` = source,
  `path` = destination), which is what the current field pair reads like it already means. Requires
  a schema change and a migration story for existing configs.
- **Option C** — make `path` required for git services. Simplest, but breaks every existing
  git-only config.

Independent of the choice, two guards should land regardless:

- `resolveInfraPath` must **never** resolve to `c.FileDir()`. Refuse loudly (`path` resolved to the
  project directory) rather than operating there.
- `:97`/`:108` must name the **actual directory** being operated on, not the service name.

## Completion Criteria

- [ ] `dva infra update <gitsvc>` never runs any git command in the directory containing `dva.yml` | verify: `human — throwaway repo with a TRACKED modified file + a git-only infra service; run 'dva infra update'; assert the file is unmodified, 'git stash list' is empty, and the branch head is unchanged`
- [ ] A git-based infra service is actually cloned from its configured `git:` URL | verify: `human — point 'git:' at a real reachable repo; assert the repo contents appear at the decided location. The current code never contacts the URL at all`
- [ ] `resolveInfraPath` refuses to resolve to the config dir | verify: `go test ./internal/cli/ -run InfraPath — assert resolveInfraPath("", cfgDir) does not return cfgDir; proven to fail against the current implementation`
- [ ] The unguarded `git pull --rebase` at infra.go:112 cannot run against the user's repo even when the tree is CLEAN (no prompt path) | verify: `human — clean throwaway repo, git-only infra service; assert no rebase occurs. This path shows no prompt today`
- [ ] Warning/stash messages name the real target directory | verify: `human — assert the '[warn] ... has uncommitted changes' and 'git stash pop in the infra dir' texts print an actual path`
- [ ] `make test` and `go vet ./...` pass | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && make test && go vet ./...`

## References

- [047-dry-run-ignored-on-disableflagparsing-commands.md](../_archive/047-dry-run-ignored-on-disableflagparsing-commands.md) — the other "does more than promised" finding; fixed
- `internal/cli/infra.go:85-138` — the whole mechanism
- `internal/config/schema.json` `.properties.infra` — the `oneOf` that forces `svc.Path == ""`
