package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/ScriptonBasestar/dva/internal/config"
)

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
	dir := cfg.Dir
	if dir == "" {
		dir = pctx.ConfigDir
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(pctx.ConfigDir, dir)
	}

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
	return p.removeProcess(pctx)
}

func (p *ProcessPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	return p.haltProcess(pctx)
}

func (p *ProcessPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid > 0 && IsProcessRunning(pid) {
		return []ServiceStatus{{Name: name, State: "running"}}, nil
	}

	return []ServiceStatus{{Name: name, State: "stopped"}}, nil
}

// haltProcess sends SIGTERM but preserves the PID file so the process
// can be restarted quickly via `dva stack up` (Vagrant halt semantics).
func (p *ProcessPlugin) haltProcess(pctx *PluginContext) error {
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

	if pid > 0 && IsProcessRunning(pid) {
		if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
			fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", name, pid)
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

	if pid > 0 {
		if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
			fmt.Fprintf(os.Stderr, "[-] removed %s (pid %d)\n", name, pid)
		}
	}

	_ = os.Remove(pidFile)
	_ = os.Remove(logFile)
	return nil
}

// startLocalProcess starts a command in background, saves PID and redirects output to log.
func startLocalProcess(name, command, dir string, pctx *PluginContext) error {
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

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Pass accumulated environment variables to the process
	cmd.Env = pctx.Env.EnvSlice()

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start: %w", err)
	}

	pidPath := filepath.Join(pidDir, name+".pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
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
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
