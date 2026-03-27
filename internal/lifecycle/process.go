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
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, "pids", name+".pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && isProcessRunning(pid) {
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
	return p.stopProcess(pctx)
}

func (p *ProcessPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	return p.stopProcess(pctx)
}

func (p *ProcessPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, "pids", name+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid > 0 && isProcessRunning(pid) {
		return []ServiceStatus{{Name: name, State: "running"}}, nil
	}

	return []ServiceStatus{{Name: name, State: "stopped"}}, nil
}

func (p *ProcessPlugin) stopProcess(pctx *PluginContext) error {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, "pids", name+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil // not running
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		os.Remove(pidFile)
		return nil
	}

	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		// Process already dead
	} else {
		fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", name, pid)
	}

	os.Remove(pidFile)
	return nil
}

// startLocalProcess starts a command in background, saves PID and redirects output to log.
func startLocalProcess(name, command, dir string, pctx *PluginContext) error {
	pidDir := filepath.Join(pctx.ConfigDir, config.DotDirName, "pids")
	logDir := filepath.Join(pctx.ConfigDir, config.DotDirName, "logs")

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
		logFile.Close()
		return fmt.Errorf("start: %w", err)
	}

	pidPath := filepath.Join(pidDir, name+".pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		logFile.Close()
		os.Remove(filepath.Join(logDir, name+".log"))
		return fmt.Errorf("save pid: %w", err)
	}

	logFile.Close()

	// Reap zombie in background goroutine
	go cmd.Wait()

	return nil
}

// isProcessRunning checks if a process with the given PID is still alive.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
