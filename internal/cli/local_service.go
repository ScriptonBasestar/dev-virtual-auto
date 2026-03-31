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
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// startUnreadyServices starts local services that have a start command and failed health checks.
// Returns a set of service names that were started.
func startUnreadyServices(checks map[string]config.HealthCheckConfig, results []HealthCheckResult, configDir string, env *config.Environment) map[string]bool {
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
			if pid > 0 && lifecycle.IsProcessRunning(pid) {
				continue
			}
		}

		if err := startLocalService(r.Name, hc.Start, configDir, env); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] failed to start %s: %v\n", r.Name, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[+] started %s\n", r.Name)
		started[r.Name] = true
	}

	return started
}

// startLocalService starts a command in background, saves PID and redirects output to log.
func startLocalService(name, command, configDir string, env *config.Environment) error {
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
	if env != nil {
		cmd.Env = env.EnvSlice()
	}
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

