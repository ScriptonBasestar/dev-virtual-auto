package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// haltExitTimeout bounds how long haltProcess waits for a SIGTERM'd process to actually
// exit before returning. Without this wait, Up's already-running check (same PID file, same
// IsProcessRunning probe) can observe the process mid-shutdown and skip starting a
// replacement — the entry is left stopped while restart/up report it as running. The wait
// makes Stop's return synchronous with the process actually being gone, matching the
// semantics Down/removeProcess already get for free by deleting the PID file.
const (
	haltExitTimeout      = 5 * time.Second
	haltExitPollInterval = 50 * time.Millisecond
)

// waitForProcessExit polls until pid is no longer running, ctx is cancelled, or timeout
// elapses. It returns false (not true) on cancellation or timeout so the caller can decide
// whether to warn rather than assume the process is gone.
func waitForProcessExit(ctx context.Context, pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !IsProcessRunning(pid) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(haltExitPollInterval):
		}
	}
}

// EntryDir resolves an entry's declared directory against the config directory.
//
// Empty means the config directory itself, and a relative path is relative to it — not to the
// shell's cwd, which is what a bare exec.Cmd would use and what the caller's terminal happens
// to be sitting in.
//
// Exported because `dva build` resolves runners.native.build's directory and this plugin
// resolves runners.native.run's, and they describe the same entry. Two copies of the rule
// would let `build` compile in one tree while `up` runs from another, which produces no error
// at all — just a stale binary and a build whose output nothing reads.
func EntryDir(configDir, dir string) string {
	switch {
	case dir == "":
		return configDir
	case filepath.IsAbs(dir):
		return dir
	default:
		return filepath.Join(configDir, dir)
	}
}

// ProcessPlugin manages local processes as services.
type ProcessPlugin struct{}

func (p *ProcessPlugin) Name() string { return "process" }

func (p *ProcessPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Process
	if cfg == nil {
		return &Result{}, nil
	}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", cfg.Command, "dir", cfg.Dir)
		return &Result{}, nil
	}

	name := pctx.Entry.Name
	dir := EntryDir(pctx.ConfigDir, cfg.Dir)

	// Check if already running
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && IsProcessRunning(pid) {
			pctx.Logger.Info("already running", "name", name, "pid", pid)
			return &Result{
				Services: []ServiceStatus{{
					Name:  name,
					State: "running",
				}},
			}, nil
		}
	}

	if err := startLocalProcess(name, cfg.Command, dir, pctx); err != nil {
		return nil, fmt.Errorf("process up %s: %w", name, err)
	}

	fmt.Fprintf(os.Stderr, "[+] started %s\n", name)

	return &Result{
		Services: []ServiceStatus{{
			Name:  name,
			State: "running",
		}},
	}, nil
}

func (p *ProcessPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "action", "remove process", "name", pctx.Entry.Name)
		return nil
	}
	return p.removeProcess(pctx)
}

func (p *ProcessPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "action", "stop process", "name", pctx.Entry.Name)
		return nil
	}
	return p.haltProcess(ctx, pctx)
}

func (p *ProcessPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return managedProcessStatus(name, pid)
}

// haltProcess sends SIGTERM but preserves the PID file so the process
// can be restarted quickly via `dva stack up` (Vagrant halt semantics).
//
// It waits (bounded by haltExitTimeout) for the process to actually exit before returning,
// so a caller that immediately re-checks or re-starts this same entry — e.g. Orchestrator's
// Restart, which is Stop followed by Up — never observes the still-shutting-down process
// through Up's "already running" PID-file check and mistakenly skips starting a replacement.
func (p *ProcessPlugin) haltProcess(ctx context.Context, pctx *PluginContext) error {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil // not running
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil
	}

	if !signalableProcessGroupPID(pid) {
		_ = os.Remove(pidFile)
		return nil
	}
	if err := requireProcessGroupPID(pid); err != nil {
		return fmt.Errorf("stop %s: %w", name, err)
	}
	if pid > 0 && IsProcessRunning(pid) {
		if err := terminateProcessGroup(pid); err == nil {
			fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", name, pid)
			if !waitForProcessExit(ctx, pid, haltExitTimeout) {
				fmt.Fprintf(os.Stderr, "[warn] %s (pid %d) did not exit within %s of stop\n", name, pid, haltExitTimeout)
				// The process ignored SIGTERM past haltExitTimeout and is still running.
				// Report this as a real failure — not nil/success — so a caller that only
				// checks the exit code (CI, scripts, or Restart's Stop-then-Up sequencing)
				// doesn't believe stop/restart worked when the original process is still
				// alive and no replacement was started.
				return fmt.Errorf("stop %s: pid %d did not exit within %s", name, pid, haltExitTimeout)
			}
		} else if errors.Is(err, errProcessGroupsUnsupported) {
			return fmt.Errorf("stop %s: %w", name, err)
		}
	}
	// PID file preserved — process can be restarted by up
	return nil
}

// removeProcess sends SIGTERM and removes PID/log files (Vagrant destroy semantics).
func (p *ProcessPlugin) removeProcess(pctx *PluginContext) error {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")
	logFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.LogsDirName, name+".log")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil // not running
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		_ = os.Remove(pidFile)
		return nil
	}

	if signalableProcessGroupPID(pid) {
		if err := requireProcessGroupPID(pid); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
		if err := terminateProcessGroup(pid); err == nil {
			fmt.Fprintf(os.Stderr, "[-] removed %s (pid %d)\n", name, pid)
		} else if errors.Is(err, errProcessGroupsUnsupported) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}

	_ = os.Remove(pidFile)
	_ = os.Remove(logFile)
	return nil
}

// startLocalProcess starts a command in background, saves PID and redirects output to log.
func startLocalProcess(name, command, dir string, pctx *PluginContext) error {
	cmd := exec.Command("sh", "-c", command)
	if err := configureProcessGroup(cmd); err != nil {
		return fmt.Errorf("local background process %q: %w", name, err)
	}

	pidDir := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName)
	logDir := filepath.Join(pctx.ConfigDir, config.DotDirName, config.LogsDirName)

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

	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Pass accumulated environment variables to the process
	cmd.Env = pctx.Env.EnvSlice()

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start: %w", err)
	}

	pidPath := filepath.Join(pidDir, name+".pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		if signalableProcessGroupPID(cmd.Process.Pid) {
			_ = terminateProcessGroup(cmd.Process.Pid)
		}
		_ = logFile.Close()
		_ = os.Remove(filepath.Join(logDir, name+".log"))
		return fmt.Errorf("save pid: %w", err)
	}

	_ = logFile.Close()

	// Reap zombie in background goroutine
	go func() { _ = cmd.Wait() }()

	return nil
}

// IsProcessRunning checks if a process with the given PID is still alive.
func IsProcessRunning(pid int) bool {
	return isProcessRunning(pid)
}
