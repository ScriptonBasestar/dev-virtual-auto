---
id: TASK-180
title: "`type: docker_socket` stats a hardcoded path, so it fails on every Colima, Podman, rootless or remote host"
type: bug
priority: P2
status: todo
effort: S
created-at: 2026-08-05T17:46:00+09:00
source: "dogfood run 20260805-143543-f82daf stage 10 — DVA-009, re-verified on 50a888b"
scope: "dva repo — internal/cli/doctor.go:283, :340"
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
daemon is healthy and the path does not exist:

```
$ echo $DOCKER_HOST
unix:///Users/archmagece/.colima/default/docker.sock
$ ls /var/run/docker.sock
ls: /var/run/docker.sock: No such file or directory
$ docker info            # exit 0
$ docker compose version # exit 0
```

`dva doctor` on that host reports both verdicts at once and exits 1:

```
name='Docker daemon accessible'  passed=True   finding=''
name='Docker daemon accessible'  passed=False  finding='Docker socket is NOT accessible'
name='Docker Compose available'  passed=True   finding=''
```

The same is true for Podman, rootless Docker, Docker Desktop with a non-default context, and any
remote `DOCKER_HOST`. The check does not test what its name claims; it tests one deployment's
filesystem layout. `fix_hint` then sends the user to "start Docker Desktop", which is already
running.

The two rows sharing a display name is **not** part of this defect — the name comes from the
user's own `checks[].name` in `dva.yml`. The defect is that a check type DVA supplies contradicts
DVA's own daemon check on a healthy machine.

## Acceptance criteria

- [ ] `type: docker_socket` passes on a host where `docker info` succeeds and
      `/var/run/docker.sock` does not exist. Reuse `lifecycle.DockerDaemonReachable`, or resolve
      the path from `DOCKER_HOST` — decide which and say why. They differ: one tests the daemon,
      the other tests the socket file's permissions, and the check's name and finding text
      currently promise both.
      Verify: `human — the decision and its reasoning are in the Result section`
- [ ] The finding text stops asserting "Docker socket is NOT accessible" when what was measured
      is a missing file at one specific path. Name the path that was checked.
- [ ] Assertable without Docker installed — the path resolution is testable from `DOCKER_HOST`
      alone, rather than only through a live daemon.
      Verify: `go test ./internal/cli/ -run Docker -count=1`
- [ ] A `docker_socket` check and the built-in daemon check cannot return opposite verdicts for
      the same daemon. Add the case that would have caught this.
      Verify: `go test ./internal/cli/ -run Docker -count=1`
- [ ] `make test` exits 0.

## References

- `internal/cli/doctor.go:283` — the dispatch
- `internal/cli/doctor.go:318` — `checkDocker`, the portable one, for comparison
- `internal/cli/doctor.go:340` — the hardcoded path
- `internal/lifecycle` — `DockerDaemonReachable`
- [TASK-156](../done/156-doctors-compose-hint-tells-a-podman-user-to-run-docker.md) — the same
  assumption one layer up, in a hint rather than a verdict

## Notes

Re-verified on `50a888b` after the dogfood run recorded it on `488fc19`; the hardcoded path is
unchanged. Found while running the SAFETY validation ladder against `flow-pipechain-devbox`,
which declares a `docker_socket` check.

TASK-156 fixed the Podman assumption in doctor's compose *hint*. This is the same assumption in a
*verdict*, which is the more expensive place for it: a hint that is wrong wastes a minute, a
verdict that is wrong makes `dva doctor` exit 1 on a working machine and teaches people to ignore
it.
