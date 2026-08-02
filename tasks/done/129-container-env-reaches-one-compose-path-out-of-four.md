---
id: TASK-129
title: "Host env reaches a container on exactly one of four compose paths, and nothing records whether that is the design"
type: decision
priority: P2
status: done
effort: M
created-at: 2026-08-02T00:00:00+09:00
resolved-at: 2026-08-02T00:00:00+09:00
resolution: "Yes to all three, with the forwarding set bounded by measured flag support rather than by path: -e is injected on run and exec, never on up. kubectl needs no second decision — kubectl exec has no env flag."
scope: "internal/runner/docker_compose.go — runVars:189, composeArguments:147-150, buildStepArgs:105, autoDetectComposeMethod:204; internal/runner/kubectl.go for the comparison"
---

# Task 129: Decide what `-e` forwarding means on an `exec`

## The blind spot

`docker compose` is handed `-e KEY=VALUE` in exactly one place: `composeArguments` calling
`runVars`, guarded by `if method == "run"` (`docker_compose.go:147-150`). Everything else that
runs a container command builds argv without it.

That guard reads `r.Cmd.Compose.Method` *after* `Execute` has already called
`autoDetectComposeMethod` (`docker_compose.go:37`), which rewrites `run` to `exec` whenever the
service is already up (`:215`). So the effective matrix is:

| path | argv verb | `-e` injected |
|---|---|---|
| non-step, `method: run`, container **not** running | `run` | **yes** |
| non-step, `method: run`, container already running | `exec` (rewritten) | no |
| non-step, `method: exec` as configured | `exec` | no |
| `steps:` item, any config | `exec` (`buildStepArgs:105`) | no |

One row out of four. The common dev-loop state — container already up — is not that row.

`env` does reach the docker CLI child process on every path, identically: both `ExecReplace` and
`ExecSubprocess` set the child's environment from `env.EnvSlice()`
(`internal/exec/exec.go:28,52`). But the docker CLI's own environment is not the container's.
`-e` is the only per-invocation mechanism that crosses that boundary, and `docker compose exec`
does support it (verified against Docker 29.5.3 on this machine: `-e, --env stringArray`, same
flag as `run`).

## Why this is a decision and not a bug report

There is a coherent reading in which the current behaviour is correct: `run` creates a fresh
container, so `-e` is how you parameterise its creation; `exec` enters a container that already
has the environment it was built with, and injecting host values into it is a per-invocation
override the tool deliberately does not perform.

There is an equally coherent reading in which it is a defect: the same `dva` command changes what
the process inside the container can see depending on whether someone happened to leave the stack
running.

Nothing in the repo records which was intended, so no one can call the current behaviour right or
wrong — which is the actual problem.

`runVars` has a second filter that belongs to the same question: it skips any variable not
already present in the host OS environment (`os.Getenv(k) == ""`, `:196`), and says so in its own
comment. A variable declared only in `dva.yml`'s `environment:` therefore reaches
`env.EnvSlice()` and the docker CLI, and is then dropped from `-e`. So even the one row that
forwards, forwards only host-exported values — config-declared ones never cross into the
container at all. That is documented, unexplained, and surprising: see "What the history
says" below for why *deliberate* — this sentence's original word — is not supportable.

## What the history says

Both the `if method == "run"` guard and `autoDetectComposeMethod` were introduced by the same
commit — `9f23f78`, 2026-03-12, "feat: DVA (Docker Virtual Auto) — Go rewrite of Hip CLI", which
is also the only commit that creates `docker_compose.go`. So the uptime-dependence is **not an
accident that emerged when a later change was layered on**: it is as old as the file. All three
guard members entered together (`-S` on each of `r.runVars(env)`, `"--publish="+p`,
`argv = append(argv, "--rm")` → 1 commit each, the same one; control `r.Cmd.Compose.Method` → 3,
so the search form does find multi-touch strings). `-e` was never injected unconditionally and
never moved into the guard separately.

That settles the sequence and leaves the intent unstated. `9f23f78` is a port manifest that says
nothing about env forwarding, and the Ruby original it ports is not in this repo (`--diff-filter=A`
on `*.rb` → 0; control on `*.go` → 257 files). Steps have never received `-e` on any version:
before `03eb908` they routed through `composeArguments`, but set `Method = "exec"` first, so the
guard was already false.

Two records that looked like intent and are not:

- **The host-only filter was last touched as cleanup, not as a decision.** `becd0bb`
  (2026-03-13) produced today's form, and its only mention of this function is
  "Fix dead code logic in runVars (clarify DVA_ prefix filtering)". The birth comment
  ("Only pass through explicitly set vars") describes the *what*. No commit, comment or doc
  anywhere states *why* config-only vars are dropped. Hence the wording correction above.
- **`buildStepArgs` was not stripped of its `Environment` for env reasons.** It was created
  *with* the parameter (`03eb908`) and never read it; `4e5d4d4` (2026-08-02) removed it as item 2
  of 8 `unparam`/`govet` findings, beside "unused params on startDockerApp". TASK-128's
  "deliberately stripped" is true only as "the removal was intentional" — it carries no statement
  about what a step should see.

`git log --all -S'runVars' -- tasks/ docs/ '*.md'` → 2 commits, both from the day this was filed.
No prior task or doc has ever discussed the topic.

## What `docker compose` actually supports

Measured against Docker 29.5.3 (`--help`, control: 8 flags parsed on `exec`, so the zeroes are
real absence rather than a failed parse):

| flag | `run` | `exec` |
|---|---|---|
| `-e` / `--env` | yes | **yes** |
| `--rm` | yes | no |
| `--publish` | yes | no |

The guard bundles exactly three things, and **two of them are structurally impossible on `exec`
while `-e` is fully supported**. So the coherent reading of `if method == "run"` is "flags that
only exist on `run`" — a set `-e` does not belong to.

`autoDetectComposeMethod` strengthens this. Since `9f23f78` it has stripped `--rm` out of
`RunOptions` when it rewrites (`:217-223`), which shows the author *was* reasoning about what
breaks when `run` becomes `exec`. `--rm` was on that list. `-e` was not — it simply falls out,
because the guard reads the method after the rewrite. The accurate statement is therefore not
"nobody decided this" but **"exec-compatibility was reviewed and `-e` was not part of the review"**.

## DVA already answers this question elsewhere, the other way

`internal/lifecycle/docker.go:144-146` forwards a stack entry's declared `env:` into `-e` for
every key, with **no host-export filter**, and `internal/lifecycle/docker_test.go:152` pins it:

```go
assertContainsFlag(t, args, "-e", "POSTGRES_PASSWORD=secret")
```

`POSTGRES_PASSWORD` is a pure config value that was never exported on the host. So for
"does a value declared in `dva.yml` cross into the container", the docker plugin ships **yes**
with a test, and the compose runner ships **no**. Nothing connects them: of the commits touching
each file, the intersection is **0** (control: self-intersection returns 14), and no task or doc
mentions `lifecycle/docker.go` (control: `docker_compose.go` → 10 files).

The one caveat that keeps this a precedent rather than a proof: the docker plugin forwards
`cfg.Env`, an entry-scoped `env:` block, which is not the same source as `runVars`' merged
`env.Vars`. It establishes that the host-export gate is not a DVA-wide principle; it does not by
itself settle what the merged environment should do.

## The sharpest form of question 3 is not the top-level block

`InteractionCommand` has its own `environment:` field (`config.go:316`), and `cli/run.go:51`
merges it into the executed `Environment` (`e.MergeVars(resolved.Environment)`). Because
`MergeVars` lets OS env win, a key declared only there is present in `env.Vars` and absent from
the host — which is precisely what `runVars` drops. So:

**A variable attached to one specific interaction command reaches the container on none of the
four paths, including the fresh-`run` row.**

That statement has a shipped counter-example. `examples/DISCOURSE.md:298-302`:

```yaml
      test:fast:
        environment:
          RAILS_ENV: test
        command: bundle exec rspec plugins/gorisa-plugins --tag ~slow
```

nested under a parent declaring `service: discourse`. `RAILS_ENV: test` has exactly one possible
meaning — run that rspec in the test environment — and rspec runs inside the container. Today it
is dropped. The LLM-facing schema reference calls the field "Per-command environment variable
overrides" (`agent-mesh-flows/shared/library/dva-schema.md:696`, mirrored to
`skills/config/references/schema-reference.md:696` and the Go-embedded
`internal/cli/library_reference.txt:864`), so this is also the shape an AI writing a `dva.yml`
will copy.

What the docs do **not** contain: any statement about `-e`, the `run`/`exec` split, or the
host-only filter (0 hits across root docs, `docs/`, `examples/`, `schema.json`,
`dva_guide_template.txt`, `agent-mesh-flows/`, `skills/`; the 7 raw matches are all `-e` as the
`--explain` alias). The stack-entry `env:` key that the docker plugin forwards has **no
`description` at all** in `schema.json:794`, and appears in 0 shipped docs (control:
`^\s*environment:` → 102 hits, `^\s*env:` → 0, same command form). Every top-level `environment:`
mention describes only precedence among config layers, never a destination.

## The three separable questions

1. **Should `exec` forward at all?** If yes, `buildStepArgs` and the rewritten-`run` case both
   need it, and `buildStepArgs` regains an `*config.Environment` parameter it was deliberately
   stripped of.
2. **Should `autoDetectComposeMethod`'s rewrite change what the command sees?** Today a
   background `docker compose up` silently changes the environment of a later `dva run`. Whatever
   is decided for (1), the answer should not depend on container uptime.
3. **Should `environment:` from `dva.yml` reach the container?** This is independent of (1) and
   (2) and arguably the sharpest of the three: the user wrote the variable in dva's own config
   file, and dva does not pass it in.

## The cost that needs deciding with it

Forwarding is a user-visible behaviour change to every `steps:` and `exec` invocation, not a pure
bugfix — commands that today see only the container's baked-in environment would start seeing
host values. It wants a changelog entry and probably a note in `docs/`.

Nothing in the test suite constrains the answer: **0** of the **11** test files in
`internal/runner/` reference `runVars`, `composeArguments`, or `buildStepArgs` (control: **8** of
those 11 mention `DockerComposeRunner`, so the zero is real absence, not a failed search).
`compose_steps_test.go` and `kubectl_steps_test.go` assert execution *order* and marker
substrings, never argv content. So whichever way this goes, it needs new tests — there are none
to break and none to lean on.

## Recommendation: yes to (1) and (2), yes to (3) scoped to declared vars

**(1) `exec` should forward.** `-e` sits in the `run` guard beside two flags that cannot exist on
`exec`, and `docker compose exec` supports it. The author who wrote the rewrite reviewed
exec-compatibility and handled `--rm`; `-e` was not on that list. Nothing in 526 commits states a
reason to withhold it.

**(2) The rewrite must stop changing what the command sees.** This follows from (1) rather than
being decided separately: once `-e` is outside the guard, uptime cannot change the container's
environment. No option that leaves the two rows different is defensible, because the difference
is invisible to the user at the point of use.

**(3) Declared vars should cross.** The strongest evidence is not a prose sentence — it is
`examples/DISCOURSE.md:298-302`, which is inert under today's behaviour, and the docker plugin
already forwarding config-declared `env:` with a test. The host-only filter has no recorded
rationale, and its last edit was labelled dead-code cleanup.

Scope this one deliberately: forward `env.Vars` minus the `DVA_` prefix, which is the set the
user declared in `dva.yml` (plus DVA's own runtime vars, excluded) — **not** `os.Environ()`.
`MergeVars` only ever populates `env.Vars` from config keys, so this is bounded by what was
written down, and it makes the compose runner agree with `lifecycle/docker.go`.

The residual risk that argues for care rather than against the change: a declared variable now
overrides one baked into the image. `PATH` is the obvious example. That is a real behaviour change
for existing users and is why this wants a CHANGELOG entry and a docs line, not a silent fix.



## Comparison: kubectl is consistent and consistently silent

`KubectlRunner` has no env-forwarding on either path — `kubectl.go` contains zero `-e`/`--env`
argv logic, step or non-step. It has the broader limitation without the asymmetry. A decision to
forward on compose raises the question for kubectl; a decision not to forward makes kubectl the
model. Either way it should not be answered twice.

## Resolution

Answered yes to all three questions. Implemented in `internal/runner/docker_compose.go`:
`-e` moved out of the `run` guard, the host-export filter deleted, `buildStepArgs` given back
an `*config.Environment` it now reads.

Three things implementation established that the analysis above did not have.

**The approved shape was "every method", and that would have been a regression.** `docker
compose up` does not parse `-e` — 34 flags, none of them env, against `run`'s 25 and `exec`'s 8
which both carry `-e, --env stringArray`. `composeProfiles` sets `Method = "up"` whenever
`profiles:` is configured, so injecting unconditionally would abort every profile-based
invocation on an unknown flag: strictly worse than the bug. The guard is therefore not removed
but **split by measured flag support** — `composeMethodAcceptsEnv` covers `run` and `exec`,
while `--publish`/`--rm` stay under `run`, which is the only method that has them. That the old
`if method == "run"` bundled three flags with three different support ranges is the structural
cause of the defect, not an incidental grouping.

**The kubectl question closes on measurement, not on a second decision.** The Comparison
section below expected a decision to forward on compose to raise the same question for kubectl.
It does not: `kubectl exec` has six flags — container, filename, pod-running-timeout, quiet,
stdin, tty — and no occurrence of "env" in its help at all, while `kubectl run` does have
`--env=[]`, which is what makes the absence meaningful rather than a failed search. Both
`KubectlRunner` paths build `exec` (`kubectl.go:31` and `:79`); there is no kubectl path that
could carry the flag. That is a platform limitation, not an inconsistency with compose, and it
is now recorded in `KubectlRunner.buildStepArgs` so it is not refiled.

**The forwarded set is wider than this task described.** The recommendation below says
"`environment:`, `env_file`, mode/plan vars". `internal/lifecycle/resolver.go` also merges
global `vars` (`:94`), environment profiles (`:108`), **site vars** (`:125`), plan vars
(`:131`) and **CLI `--var`** (`:134`) into `Plan.EnvVars`, which `plan_lifecycle.go` merges
into the Environment. So site vars and `--var` cross too. The docs and the `envVars` comment
state the merged set rather than enumerating layers, because an enumeration goes stale the next
time a layer is added.

What made deleting the host filter safe is that `DVA_` covers all five runtime variables DVA
injects (`DVA_OS`, `DVA_WORK_DIR_REL_PATH`, `DVA_CURRENT_USER`, `DVA_CURRENT_UID`,
`DVA_HOOK_DEPTH`). The filter was doing two jobs — blocking non-host values and, incidentally,
blocking those — and only the first was intended. `config.EnvPrefix` now names the invariant
the second job depends on.

Keys are sorted on the way out. `env.Vars` is a map and Go randomises iteration, so without it
two identical `dva run` invocations produce different argv.

## Acceptance criteria

| # | criterion | result |
|---|---|---|
| 1 | A variable declared only in `dva.yml` reaches the container | e2e: `showenv-override` prints `test`; pre-fix binary prints empty |
| 2 | The `steps:` path forwards it too | e2e: `showenv-steps` prints `test`; pre-fix binary fails the step, `printenv` exiting 1 |
| 3 | The run→exec rewrite no longer changes what the command sees | e2e against an already-running container: no `Creating` line, value still arrives |
| 4 | `up` is not given `-e` | `TestComposeArgumentsWithholdsEnvFromUp`, both the declared-`up` and profiles-forced routes |
| 5 | `DVA_*` does not cross | `TestEnvVarsExcludesDVAPrefix`, seeded from `NewEnvironment`+`WithHookDepth` so a future unprefixed runtime var fails here |
| 6 | An undeclared host variable does not cross | `TestEnvVarsExcludesUndeclaredHostVars` — the claim that bounds "declared values only" |
| 7 | argv is deterministic | `TestEnvVarsIsSortedAndStable`, 21 calls compared |
| 8 | Every criterion above fails when its fix is reverted | 5 probes run; each broke exactly its own test, file restored byte-identical each time |
| 9 | Four gates green | `make test`, `make lint`, `make doc-check` (471 links, 0 broken), `make check-generate` — all exit 0 |

Criterion 6 is the one that carries the risk. Dropping the host-export requirement is easy to
misread as "dva now forwards your shell"; `MergeVars` overwrites keys it was given and never
adds one, so the set stays bounded by what was written down. Without that test the docs'
"declared values only" would be an unverified claim about a container boundary.

The residual behaviour change is documented rather than mitigated: a declared key now overrides
the image's own value, `PATH` being the obvious case. CHANGELOG, `USAGE.md` and the schema
reference all state it.

Not fixed here, found while measuring: `--project-name` is passed twice on paths where both
`dvaexec.ComposeArgv` (`compose_argv.go:56`) and the runner's `detectedProject`
(`docker_compose.go:46`, `:95`) supply it. Docker takes the last, so it is cosmetic today, but
it surfaces in error output. Filed as [TASK-132](../todo/132-project-name-is-passed-twice-on-the-detected-project-paths.md).

## Related

- [TASK-128](128-the-recursion-was-right-the-nodes-it-walked-were-not.md) — found this
  while correcting `buildStepArgs`' doc comment, and its first correction of that comment
  overstated the shape: it framed the split as step vs non-step when the measured split is
  fresh-container `run` vs everything else. Corrected there; the accurate framing is the table
  above.
