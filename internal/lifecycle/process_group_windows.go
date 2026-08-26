//go:build windows

package lifecycle

import (
	"errors"
	"os/exec"
)

// Local background lifecycle entries require Unix process groups so down/stop can terminate
// the shell and every child it started. Windows cannot provide that contract with os/exec, so
// reject startup and stale-PID teardown explicitly instead of signalling only one process.
var errProcessGroupsUnsupported = errors.New("local background process groups are unsupported on Windows; run the command outside DVA or use a supported remote runner")

func configureProcessGroup(_ *exec.Cmd) error { return errProcessGroupsUnsupported }

func terminateProcessGroup(_ int) error { return errProcessGroupsUnsupported }

func processGroupsUnsupported() bool { return true }

// Windows cannot probe an arbitrary PID's liveness through os.FindProcess; startup is rejected,
// so a retained PID is never reported as a running DVA-managed process.
func isProcessRunning(_ int) bool { return false }
