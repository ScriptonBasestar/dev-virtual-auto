package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// MultipassPlugin manages lightweight Ubuntu VMs via Multipass.
type MultipassPlugin struct{}

func (p *MultipassPlugin) Name() string { return "multipass" }

func (p *MultipassPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Multipass
	if cfg == nil {
		return &Result{}, nil
	}

	name := p.vmName(pctx)
	launchArgs := p.buildLaunchArgs(pctx)

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "start", []string{"multipass", "start", name}, "launch", append([]string{"multipass"}, launchArgs...))
		return &Result{}, nil
	}

	// Try start first (idempotent for existing VMs)
	startArgs := []string{"start", name}
	if err := exec.Command("multipass", startArgs...).Run(); err == nil {
		// VM exists and was started/already running
		return &Result{
			Services: []ServiceStatus{{
				Name:  name,
				State: "running",
			}},
		}, nil
	}

	// VM doesn't exist, launch it
	if err := dvaexec.ExecSubprocess(pctx.Env, "multipass", launchArgs, false); err != nil {
		return nil, fmt.Errorf("multipass launch %s: %w", name, err)
	}

	return &Result{
		Services: []ServiceStatus{{
			Name:  name,
			State: "running",
		}},
	}, nil
}

func (p *MultipassPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.Multipass == nil {
		return nil
	}

	name := p.vmName(pctx)

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "multipass", "args", []string{"delete", "--purge", name})
		return nil
	}

	return dvaexec.ExecSubprocess(pctx.Env, "multipass", []string{"delete", "--purge", name}, false)
}

func (p *MultipassPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.Multipass == nil {
		return nil
	}

	name := p.vmName(pctx)

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "multipass", "args", []string{"stop", name})
		return nil
	}

	return dvaexec.ExecSubprocess(pctx.Env, "multipass", []string{"stop", name}, false)
}

// multipassInfo mirrors the multipass info --format json output structure.
type multipassInfo struct {
	Info map[string]struct {
		State string `json:"state"`
	} `json:"info"`
}

func (p *MultipassPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	if pctx.Entry.Multipass == nil {
		return []ServiceStatus{{Name: pctx.Entry.Name, State: "stopped"}}, nil
	}

	name := p.vmName(pctx)

	out, err := exec.Command("multipass", "info", name, "--format", "json").Output()
	if err != nil {
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	var info multipassInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return []ServiceStatus{{Name: name, State: "unknown"}}, nil
	}

	vmInfo, ok := info.Info[name]
	if !ok {
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	return []ServiceStatus{{Name: name, State: vmInfo.State}}, nil
}

// vmName returns the VM name from config or falls back to the entry name.
func (p *MultipassPlugin) vmName(pctx *PluginContext) string {
	if pctx.Entry.Multipass != nil && pctx.Entry.Multipass.Name != "" {
		return pctx.Entry.Multipass.Name
	}
	return pctx.Entry.Name
}

// buildLaunchArgs constructs the multipass launch argument list.
func (p *MultipassPlugin) buildLaunchArgs(pctx *PluginContext) []string {
	cfg := pctx.Entry.Multipass
	name := p.vmName(pctx)

	args := []string{"launch", "--name", name}

	if cfg.CPUs > 0 {
		args = append(args, "--cpus", strconv.Itoa(cfg.CPUs))
	}
	if cfg.Memory != "" {
		args = append(args, "--memory", cfg.Memory)
	}
	if cfg.Disk != "" {
		args = append(args, "--disk", cfg.Disk)
	}
	if cfg.CloudInit != "" {
		ciPath := cfg.CloudInit
		if !filepath.IsAbs(ciPath) {
			ciPath = filepath.Join(pctx.ConfigDir, ciPath)
		}
		args = append(args, "--cloud-init", ciPath)
	}
	if cfg.Image != "" {
		args = append(args, cfg.Image)
	}

	return args
}
