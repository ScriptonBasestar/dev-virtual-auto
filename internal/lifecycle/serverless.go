package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// ServerlessPlugin manages serverless-offline services as background processes.
type ServerlessPlugin struct{}

func (p *ServerlessPlugin) Name() string { return "serverless" }

func (p *ServerlessPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Serverless
	if cfg == nil {
		return &Result{}, nil
	}

	name := pctx.Entry.Name

	// Build command
	command := "serverless offline start"
	if cfg.Port > 0 {
		command += " --httpPort " + strconv.Itoa(cfg.Port)
	}
	if len(cfg.Args) > 0 {
		command += " " + strings.Join(cfg.Args, " ")
	}

	// Resolve working directory
	dir := cfg.Dir
	if dir == "" {
		dir = pctx.ConfigDir
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(pctx.ConfigDir, dir)
	}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", command, "dir", dir)
		return &Result{}, nil
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

	if err := startLocalProcess(name, command, dir, pctx); err != nil {
		return nil, fmt.Errorf("serverless up %s: %w", name, err)
	}

	fmt.Fprintf(os.Stderr, "[+] started %s\n", name)

	return &Result{
		Services: []ServiceStatus{{
			Name:  name,
			State: "running",
		}},
	}, nil
}

func (p *ServerlessPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.Serverless == nil {
		return nil
	}
	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "action", "stop process", "name", pctx.Entry.Name)
		return nil
	}
	return p.stopProcess(pctx)
}

func (p *ServerlessPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.Serverless == nil {
		return nil
	}
	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "action", "stop process", "name", pctx.Entry.Name)
		return nil
	}
	return p.stopProcess(pctx)
}

func (p *ServerlessPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return managedProcessStatus(name, pid)
}

func (p *ServerlessPlugin) stopProcess(pctx *PluginContext) error {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		_ = os.Remove(pidFile)
		return nil
	}
	if err := requireProcessGroupPID(pid); err != nil {
		return fmt.Errorf("stop %s: %w", name, err)
	}
	if err := terminateProcessGroup(pid); err == nil {
		fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", name, pid)
	} else if errors.Is(err, errProcessGroupsUnsupported) {
		return fmt.Errorf("stop %s: %w", name, err)
	}
	_ = os.Remove(pidFile)
	return nil
}
