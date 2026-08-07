---
id: TASK-180
title: "`type: docker_socket` stats a hardcoded path, so it fails on every Colima, Podman, rootless or remote host"
type: bug
priority: P2
status: done
effort: S
created-at: 2026-08-05T17:46:00+09:00
completed-at: 2026-08-07
source: "dogfood run 20260805-143543-f82daf stage 10 — DVA-009, re-verified on 50a888b"
scope: "dva repo — internal/cli/doctor.go, internal/cli/doctor_docker_socket_test.go"
---

# Task 180: Ask Docker where its socket is instead of assuming

## Problem

DVA answers "is Docker reachable" two different ways, and only one of them is portable.

```go
// internal/cli/doctor.go:318 — the built-in check
r.Passed = lifecycle.DockerDaemonReachable(nil)     // runs `docker info`

// internal/cli/doctor.go:283 — the user-declared check type
case "docker_socket":
	r.Passed = isDockerSocketAccessible()
// → checkDockerSocketPermissions, doctor.go:340
	sockPath := "/var/run/docker.sock"                 // hardcoded
	_, err := os.Stat(sockPath)
```

`DOCKER_HOST` is never consulted. On a Colima host — a supported, ordinary macOS setup — the
daemon is healthy and the path does not exist.

## Acceptance criteria

- [x] `type: docker_socket` passes on a host where `docker info` succeeds and
      `/var/run/docker.sock` does not exist. Reuse `lifecycle.DockerDaemonReachable`, or resolve
      the path from `DOCKER_HOST` — decide which and say why.
      Verify: `human — the decision and its reasoning are in the Result section`
- [x] The finding text stops asserting "Docker socket is NOT accessible" when what was measured
      is a missing file at one specific path. Name the path that was checked.
- [x] Assertable without Docker installed — the path resolution is testable from `DOCKER_HOST`
      alone, rather than only through a live daemon.
      Verify: `go test ./internal/cli/ -run Docker -count=1`
- [x] A `docker_socket` check and the built-in daemon check cannot return opposite verdicts for
      the same daemon. Add the case that would have caught this.
      Verify: `go test ./internal/cli/ -run Docker -count=1`
- [x] `make test` exits 0.

## Result

### Decision: resolve `DOCKER_HOST` first, fall through to the daemon probe

Neither pure option covers the check's name and the portability requirement alone:

| Approach | Colima / explicit `unix://` | Default path missing, Desktop/context | Permissions finding |
|----------|----------------------------|----------------------------------------|---------------------|
| Path only from `DOCKER_HOST` | OK when set | Still fails if env empty | Keeps |
| `DockerDaemonReachable` only | OK | OK | Loses path/permission detail |

**Both:**

1. `resolveDockerSocketPath(DOCKER_HOST)` — empty → `/var/run/docker.sock`; `unix://` → that path;
   other schemes → no local path.
2. If a path exists, `Stat` + `Open` (permission finding names the path).
3. If an **explicit** `unix://` path is missing, fail and name it (docker would fail too).
4. If the **default** path is missing, or `DOCKER_HOST` is non-unix → **fall through to
   `DockerDaemonReachable`**, so this check cannot disagree with `checkDocker` on a healthy host.

`runSingleCheck` now keeps the detailed `Finding` from that evaluation instead of replacing a
failed boolean with the generic "Docker socket is NOT accessible" line.

### Verification

```
go test ./internal/cli/ -run 'Docker|ResolveDocker|EvaluateDocker|RunSingleCheck_Docker' -count=1  # ok
make test  # exit 0
```
