package lifecycle

import "errors"

var errProcessGroupsUnsupported = errors.New("local background process groups are unsupported on Windows; run the command outside DVA or use a supported remote runner")

// processGroupPIDError is pure so every caller makes the same decision for a retained PID.
func processGroupPIDError(pid int, supported bool) error {
	if pid > 0 && !supported {
		return errProcessGroupsUnsupported
	}
	return nil
}

func requireProcessGroupPID(pid int) error {
	return processGroupPIDError(pid, processGroupsSupported())
}

func signalableProcessGroupPID(pid int) bool { return pid > 0 }

func managedProcessStatus(name string, pid int) ([]ServiceStatus, error) {
	if err := requireProcessGroupPID(pid); err != nil {
		return nil, errors.New("status " + name + ": " + err.Error())
	}
	if pid > 0 && IsProcessRunning(pid) {
		return []ServiceStatus{{Name: name, State: "running"}}, nil
	}
	return []ServiceStatus{{Name: name, State: "stopped"}}, nil
}
