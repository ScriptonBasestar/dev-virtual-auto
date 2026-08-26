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

// SAMPlugin manages AWS SAM local services as background processes.
type SAMPlugin struct{}

func (p *SAMPlugin) Name() string { return "sam" }

func (p *SAMPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.SAM
	if cfg == nil {
		return &Result{}, nil
	}

	name := pctx.Entry.Name

	// Build command
	command := "sam local start-api"
	if cfg.Template != "" {
		tmpl := pctx.Env.Interpolate(cfg.Template)
		if !filepath.IsAbs(tmpl) {
			tmpl = filepath.Join(pctx.ConfigDir, tmpl)
		}
		command += " -t " + tmpl
	}
	if cfg.Port > 0 {
		command += " --port " + strconv.Itoa(cfg.Port)
	}
	if len(cfg.Args) > 0 {
		command += " " + strings.Join(cfg.Args, " ")
	}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", command, "dir", pctx.ConfigDir)
		return &Result{}, nil
	}

	dir := pctx.ConfigDir

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
		return nil, fmt.Errorf("sam up %s: %w", name, err)
	}

	fmt.Fprintf(os.Stderr, "[+] started %s\n", name)

	return &Result{
		Services: []ServiceStatus{{
			Name:  name,
			State: "running",
		}},
	}, nil
}

func (p *SAMPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.SAM == nil {
		return nil
	}
	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "action", "stop process", "name", pctx.Entry.Name)
		return nil
	}
	return p.stopProcess(pctx)
}

func (p *SAMPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.SAM == nil {
		return nil
	}
	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "action", "stop process", "name", pctx.Entry.Name)
		return nil
	}
	return p.stopProcess(pctx)
}

func (p *SAMPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, config.PidsDirName, name+".pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return managedProcessStatus(name, pid)
}

func (p *SAMPlugin) stopProcess(pctx *PluginContext) error {
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
	if !signalableProcessGroupPID(pid) {
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
