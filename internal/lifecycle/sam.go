package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, "pids", name+".pid")
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
	return p.stopProcess(pctx)
}

func (p *SAMPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.SAM == nil {
		return nil
	}
	return p.stopProcess(pctx)
}

func (p *SAMPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, "pids", name+".pid")

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

func (p *SAMPlugin) stopProcess(pctx *PluginContext) error {
	name := pctx.Entry.Name
	pidFile := filepath.Join(pctx.ConfigDir, config.DotDirName, "pids", name+".pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		os.Remove(pidFile)
		return nil
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
		fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", name, pid)
	}
	os.Remove(pidFile)
	return nil
}
