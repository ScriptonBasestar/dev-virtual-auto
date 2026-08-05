package lifecycle

import (
	"context"
	"os/exec"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// dockerDaemonProbeTimeout bounds `docker info`. A daemon that is mid-start can
// accept the connection and then stall, and this probe runs on a path that has
// already failed — it must not be able to turn a failed command into a hang.
const dockerDaemonProbeTimeout = 5 * time.Second

// DockerDaemonReachable reports whether the Docker daemon answers `docker info`.
//
// One implementation, two callers: `dva doctor` reports it as a check, and the compose
// lifecycle path consults it after a failed command to say whether the daemon was the
// reason. The point of consulting it at the failure point is that the answer is the same
// one doctor would give, so a second copy — free to drift on timeout, on argv, on what
// "reachable" means — would defeat it.
//
// env supplies the same environment the failed command ran with, so a DOCKER_HOST set in
// dva.yml is probed rather than the ambient one. A nil env inherits this process's
// environment, which is what doctor wants. The timeout is owned here rather than by
// callers so both get the same bound. Output is discarded: callers need the verdict, not
// docker's inventory.
func DockerDaemonReachable(env *config.Environment) bool {
	ctx, cancel := context.WithTimeout(context.Background(), dockerDaemonProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	if env != nil {
		cmd.Env = env.EnvSlice()
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// DockerDaemonError reports that a compose command failed while the Docker daemon was
// unreachable.
//
// It exists so the lifecycle path stops relaying docker's raw stderr followed by a bare
// exit status. DVA already recognises this condition in `dva doctor`, but reaching it
// takes knowing to run doctor first and knowing to pass --strict, because a failing
// built-in check is advisory by default; nothing at the failure point said so. The
// failure is where that answer is worth the most, so it is given there.
//
// Mirrors ComposeConfigError's cause-wrapping shape (Error/Unwrap).
type DockerDaemonError struct {
	// Op is the compose subcommand that failed: up, down, stop, rm.
	Op    string
	cause error
}

func (e *DockerDaemonError) Error() string {
	op := e.Op
	if op == "" {
		op = "<command>"
	}
	msg := "Docker daemon is not reachable, so `docker compose " + op + "` could not run"
	msg += "\n       → start Docker Desktop, colima, or dockerd; if DOCKER_HOST is set, check it points at a live socket"
	msg += "\n       → re-check with: dva doctor   (`dva doctor --strict && dva up` stops before this failure)"
	return msg
}

func (e *DockerDaemonError) Unwrap() error { return e.cause }
