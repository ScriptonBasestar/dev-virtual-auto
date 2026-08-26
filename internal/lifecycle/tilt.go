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

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TiltPlugin manages Tilt local dev environments as background processes.
type TiltPlugin struct{}

func (p *TiltPlugin) Name() string { return "tilt" }

func (p *TiltPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Tilt
	if cfg == nil {
		return &Result{}, nil
	}

	command := "tilt up --stream"
	if len(cfg.Args) > 0 {
		command += " " + strings.Join(cfg.Args, " ")
	}

	dir := p.resolveDir(pctx)
	name := pctx.Entry.Name

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
		return nil, fmt.Errorf("tilt up %s: %w", name, err)
	}

	fmt.Fprintf(os.Stderr, "[+] started %s (tilt)\n", name)

	return &Result{
		Services: []ServiceStatus{{
			Name:  name,
			State: "running",
		}},
	}, nil
}

func (p *TiltPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Tilt
	if cfg == nil {
		return nil
	}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "tilt down", "dir", p.resolveDir(pctx))
		return nil
	}

	// Run tilt down to clean up resources
	cmd := exec.Command("tilt", "down")
	cmd.Dir = p.resolveDir(pctx)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	// Stop the background process
	return p.stopBackgroundProcess(pctx)
}

func (p *TiltPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	return p.Down(ctx, pctx)
}

func (p *TiltPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
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

// resolveDir resolves cfg.Dir relative to ConfigDir.
func (p *TiltPlugin) resolveDir(pctx *PluginContext) string {
	cfg := pctx.Entry.Tilt
	dir := cfg.Dir
	if dir == "" {
		return pctx.ConfigDir
	}
	if !filepath.IsAbs(dir) {
		return filepath.Join(pctx.ConfigDir, dir)
	}
	return dir
}

// stopBackgroundProcess reads the PID file and kills the process group.
func (p *TiltPlugin) stopBackgroundProcess(pctx *PluginContext) error {
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
	if err := terminateProcessGroup(pid); errors.Is(err, errProcessGroupsUnsupported) {
		return fmt.Errorf("stop %s: %w", name, err)
	}
	_ = os.Remove(pidFile)
	return nil
}
