//go:build !windows

package lifecycle

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

var errProcessGroupsUnsupported = errors.New("process groups are unsupported on Windows")

func configureProcessGroup(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func terminateProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

func processGroupsUnsupported() bool { return false }

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
