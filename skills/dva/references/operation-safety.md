# DVA Operation Safety

Read this reference before lifecycle execution, destructive diagnostics, global
installation, or any command whose preview/help behavior is uncertain for the
installed DVA version.

## Authority Boundaries

Require explicit user authority before:

- starting or changing production/staging infrastructure;
- running `dva doctor --fix`, cleanup, reset, or volume removal;
- starting services when the request is configuration analysis only;
- overwriting `.env`, decrypting or editing secret stores;
- installing or replacing a global DVA executable;
- reverting unrelated user changes.

Discover named commands and plans before execution with `dva manifest -f json`
or `dva ls`. Use the exact target and plan named by the user or resolved from
project context; do not substitute a similarly named lifecycle surface.

## Read-Only Validation Ladder

Run the lowest-risk applicable checks first:

1. Confirm referenced files and commands exist.
2. Record the installed executable path and `dva version`.
3. Run `dva config validate`; classify every warning.
4. Compare `dva config show -f yaml` and `dva manifest -f json` with the intended
   merged configuration and command surface.
5. Run `dva doctor --json` without fixes and assign failures to configuration,
   tool, project, or environment.
6. Inspect a printed plan only through a preview path already proven
   non-mutating for that installed version.
7. Validate every configured Compose combination with
   `docker compose ... config --quiet`.
8. Run target lint/tests.
9. Start services and perform live health checks only when authorized and safe.

A zero exit status does not override contradictory or materially incorrect
output.

## Prove Help and Preview Safety

- Treat lifecycle `--help` as potentially executable until the installed manual
  flag parser returns help without entering execution in a disposable fixture.
- Treat lifecycle `--dry-run`/`--explain` as potentially mutating until each
  selected runner's `up`, `down`, `stop`, `restart`, and provision path is proven
  safe in a disposable fixture.
- Snapshot fixture resources and process-backed PID/log state before and after
  the preview. A preview is safe only when those states remain unchanged.
- Do not use a live target to prove that a preview is non-mutating.

## Verify Lifecycle Symmetry

- Use the same named plan for `up`, `down`, `stop`, and `restart`.
- Confirm runner choice, service subset, order, dependencies, and teardown
  reversal independently for each lifecycle direction.
- For process runners, verify DVA tracks the actual process group and preserves
  expected PID/log state. A wrapper PID or an open port owned by another process
  is not runtime truth.
- Distinguish `stop` from `down`: stop preserves resumable resources; down tears
  down in reverse dependency order.
- An entry declares one `run` command. A hot-reload variant is a separate entry
  selected by a different plan, not a mode toggle on the same declaration — the
  `dva app up --dev` shape that used to provide this was removed with `dva app`.

## Select Raw Tools Conservatively

Use DVA for declared workflows. Use raw tools only when DVA has no equivalent or
for read-only validation, and state why. Never duplicate a named plan's lifecycle
with raw Compose, Docker, kubectl, or shell commands.
