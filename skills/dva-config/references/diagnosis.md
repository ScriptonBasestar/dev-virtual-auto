# DVA Configuration Diagnosis

Read this reference when configuration symptoms may cross DVA, project, and
environment boundaries. Keep routine authoring in `SKILL.md`; use this guide for
contradictory diagnostics, lifecycle migrations, root/subproject ownership, or
process-health disputes.

## Establish Provenance

- Record the installed `dva` path, version, and build commit separately from the
  source checkout HEAD and dirty state. Equality must be proven, not inferred.
- Run every comparison with the same recorded executable. If a DVA source fix
  produces a candidate binary, keep installed, source, and candidate provenance
  distinct and do not replace a global executable without explicit authority.
- Inventory canonical `dva.yml`, legacy `dva.yaml`, modules, overrides, Compose
  files, Makefiles, scripts, and user-facing command documentation. Legacy
  presence is a migration finding, not rewrite authority.

## Decide Configuration Scope

Workspace inventory, child connection, command migration, infra vs app plans, and
Compose scaffold are defined in [`devbox-apply.md`](devbox-apply.md). This section
only states diagnosis-time checks against that policy.

- At a devbox root, read `.gz-git.yaml` before deciding scope. Do not clone a
  missing child without authority.
- Inspect each child in its own repository context. Preserve `.gz-git.yaml`
  ownership of clone, sync, and aggregate status; a parent DVA change must not
  absorb those.
- Keep shared lifecycle at the root and module-native commands in the owning
  child. Exclude archived, generated, vendor, and guidance-prohibited modules.
- When Compose changes, carry forward renamed, added, and removed files; service
  names; profiles; port variables; and environment prerequisites before editing
  DVA references.

Use these scope outcomes as a regression check:

| Evidence | Required scope decision |
|---|---|
| Local gz-git child with a dev/app surface | Child `dva.yml` plus root `subprojects`; import only names the root listing should show. |
| Local gz-git child with no executable surface | Record the assessment; do not invent a ceremonial child config. |
| Inventory child missing locally | Report unavailable; do not clone or sync without authority. |
| Explicit prohibition on child-repository edits | Change root only; report partial coverage and deferred children. |

## Attribute Each Finding

Assign one primary owner to every finding:

| Owner | Evidence | Response |
|---|---|---|
| DVA config | Stale file, service, command, endpoint, or project-owned value | Patch `dva.yml` minimally. |
| DVA tool | Parser, discovery, doctor, plan rendering, or runtime state is wrong | Reproduce against the recorded binary; do not mask it in config. |
| Environment | Required executable, daemon, socket, credential, or agent runtime is unavailable | Report the prerequisite; do not rewrite config. |
| Project | Compose, Makefile, application flags, docs, or actual process behavior disagree | Fix only when project files are in scope. |

A project defect and DVA-tool defect may coexist. Fixing a wrong run path does
not close a status bug if DVA still reports an unrelated process as healthy.

## Verify Runtime Truth

- Keep Compose operational subsets in one Compose stack declaration and select
  service subsets from plans. Create separate stack entries only for genuinely
  distinct executable declarations, such as native hot-reload and built-preview
  commands that cannot share one runner configuration.
- Treat `default_plan` as a project choice among named plans. Bare `up`, `down`,
  `stop`, `restart`, `build`, and `logs` select the effective default (the
  explicit `default_plan`, or the sole plan); multiple plans without a default
  are refused as ambiguous. Their whole-stack fallback applies only when no
  plans exist. Bare `status` also selects an effective default, but reports the
  whole workspace when none is available. Named plans explicitly select their
  entries and services.
  Keep always-on core dependencies profile-less and gate optional Compose tiers
  with Compose-native profiles when that matches project intent.
- Treat schema acceptance and plan resolution as necessary but insufficient.
  Verify that printed or safely observed `up`, `down`, `stop`, and `restart`
  behavior preserves every entry's selected runner, services, order, and
  dependencies.
- Judge status and health against the process group DVA controls. A live pidfile,
  wrapper process, or responding port is not enough when the tracked child is
  dead or another process owns the port.
- Confirm that a native command binds the port targeted by its health check.
  Trace values across `env_file` names, Makefile variable precedence, command
  flags, and application defaults; report the seam that actually owns a mismatch.
- Distinguish `run`/preview and `dev`/hot-reload commands. An idempotent `up` may
  leave an already-running process in its previous mode; use the documented
  restart transition when a mode change is intended. A built frontend may bake
  its mode at build time, so restarting the preview command does not recreate a
  development bundle.
- Validate lifecycle previews only after the installed DVA version is proven
  non-mutating in a disposable fixture. See the sibling `dva` skill's
  `skills/dva/references/operation-safety.md` for the execution protocol.

## Preserve Observable Behavior During Migration

- Default to preserve for useful existing configuration. Require explicit scope
  for rewrite.
- Use named plans for new or rewritten configuration; retain modes and
  applications only as migration inputs until equivalent behavior is proven.
- Keep one lifecycle owner for every Compose service, native process, check, and
  provision action. Compose declares its services; stack runners expose reusable
  declarations; plans select executable combinations; interactions remain
  one-shot commands; provision remains setup-only.
- A `docker` runner means standalone `docker run`; it is not another name for a
  service already owned by Compose.
- Reject a migration when any lifecycle direction loses runner selection,
  service subsets, order, dependencies, PID state, or log state.
