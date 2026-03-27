package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// startUnreadyServices starts local services that have a start command and failed health checks.
// Returns a set of service names that were started.
func startUnreadyServices(checks map[string]config.HealthCheckConfig, results []HealthCheckResult, configDir string) map[string]bool {
	started := make(map[string]bool)

	for _, r := range results {
		if r.Ready {
			continue
		}
		hc, ok := checks[r.Name]
		if !ok || hc.Start == "" {
			continue
		}

		// Skip if already running via PID file
		pidFile := filepath.Join(configDir, config.DotDirName, "pids", r.Name+".pid")
		if data, err := os.ReadFile(pidFile); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 && isProcessRunning(pid) {
				continue
			}
		}

		if err := startLocalService(r.Name, hc.Start, configDir); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] failed to start %s: %v\n", r.Name, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[+] started %s\n", r.Name)
		started[r.Name] = true
	}

	return started
}

// startLocalService starts a command in background, saves PID and redirects output to log.
func startLocalService(name, command, configDir string) error {
	pidDir := filepath.Join(configDir, config.DotDirName, "pids")
	logDir := filepath.Join(configDir, config.DotDirName, "logs")

	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	logFile, err := os.Create(filepath.Join(logDir, name+".log"))
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = configDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start: %w", err)
	}

	// Save PID
	pidPath := filepath.Join(pidDir, name+".pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		logFile.Close()
		os.Remove(filepath.Join(logDir, name+".log"))
		return fmt.Errorf("save pid: %w", err)
	}

	// Close parent's copy of log fd (child keeps its own)
	logFile.Close()

	// Reap zombie in background goroutine
	go cmd.Wait()

	return nil
}

// stopLocalServices reads PID files and terminates all managed local services.
func stopLocalServices(configDir string) {
	pidDir := filepath.Join(configDir, config.DotDirName, "pids")
	entries, err := os.ReadDir(pidDir)
	if err != nil {
		return // no pids directory = nothing to stop
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}

		pidFile := filepath.Join(pidDir, entry.Name())
		data, err := os.ReadFile(pidFile)
		if err != nil {
			os.Remove(pidFile)
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			os.Remove(pidFile)
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".pid")

		// Kill the process group (negative PID)
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			// Process already dead
		} else {
			fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", name, pid)
		}

		os.Remove(pidFile)
	}
}

// isProcessRunning checks if a process with the given PID is still alive.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

const defaultReadyTimeout = 30 * time.Second

// maxReadyTimeout returns the maximum ready_timeout across started services.
func maxReadyTimeout(checks map[string]config.HealthCheckConfig, startedNames map[string]bool) time.Duration {
	maxT := defaultReadyTimeout
	for name := range startedNames {
		if hc, ok := checks[name]; ok && hc.ReadyTimeout > 0 {
			t := time.Duration(hc.ReadyTimeout) * time.Second
			if t > maxT {
				maxT = t
			}
		}
	}
	return maxT
}

// runHealthChecksWithAutoStart runs health checks, auto-starts services with start commands,
// and optionally polls until all started services are ready.
//
// When wait is true: polls every 2s until all started services pass health checks or timeout.
// When wait is false: starts services and returns immediately with current status.
func runHealthChecksWithAutoStart(checks map[string]config.HealthCheckConfig, configDir string, wait bool) []HealthCheckResult {
	results := runHealthChecks(checks)

	startedNames := startUnreadyServices(checks, results, configDir)
	if len(startedNames) == 0 {
		return results
	}

	if wait {
		timeout := maxReadyTimeout(checks, startedNames)

		// Collect names of services we're waiting for
		var pending []string
		for name := range startedNames {
			pending = append(pending, name)
		}
		fmt.Fprintf(os.Stderr, "  waiting for %s to be ready (timeout %ds)...\n", strings.Join(pending, ", "), int(timeout.Seconds()))

		// Poll until all started services are ready or timeout
		start := time.Now()
		deadline := start.Add(timeout)
		for time.Now().Before(deadline) {
			time.Sleep(2 * time.Second)
			results = runHealthChecks(checks)

			var notReady []string
			for _, r := range results {
				if startedNames[r.Name] && !r.Ready {
					notReady = append(notReady, r.Name)
				}
			}
			if len(notReady) == 0 {
				elapsed := time.Since(start).Truncate(time.Second)
				fmt.Fprintf(os.Stderr, "  all services ready (%s)\n", elapsed)
				break
			}
		}
	}

	// Mark started services
	for i, r := range results {
		if startedNames[r.Name] {
			results[i].Started = true
		}
	}

	return results
}
