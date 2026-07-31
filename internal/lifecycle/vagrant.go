package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VagrantPlugin manages VMs via Vagrant.
type VagrantPlugin struct{}

func (p *VagrantPlugin) Name() string { return "vagrant" }

// resolveDir returns the resolved Vagrantfile directory path.
func (p *VagrantPlugin) resolveDir(pctx *PluginContext) string {
	cfg := pctx.Entry.Vagrant
	dir := pctx.Env.Interpolate(cfg.Dir)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(pctx.ConfigDir, dir)
	}
	return dir
}

// buildArgs appends the machine name to the base args if configured.
func (p *VagrantPlugin) buildArgs(pctx *PluginContext, baseArgs []string) []string {
	cfg := pctx.Entry.Vagrant
	args := make([]string, len(baseArgs))
	copy(args, baseArgs)
	if cfg.Machine != "" {
		args = append(args, pctx.Env.Interpolate(cfg.Machine))
	}
	return args
}

// runInDir executes a vagrant command in the Vagrantfile directory.
func (p *VagrantPlugin) runInDir(pctx *PluginContext, args []string) error {
	cmd := exec.Command("vagrant", args...)
	cmd.Dir = p.resolveDir(pctx)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = pctx.Env.EnvSlice()
	return cmd.Run()
}

func (p *VagrantPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Vagrant
	if cfg == nil {
		return &Result{}, nil
	}

	args := p.buildArgs(pctx, []string{"up"})

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "vagrant", "args", args, "dir", p.resolveDir(pctx))
		return &Result{}, nil
	}

	if err := p.runInDir(pctx, args); err != nil {
		return nil, fmt.Errorf("vagrant up: %w", err)
	}

	return &Result{}, nil
}

func (p *VagrantPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Vagrant
	if cfg == nil {
		return nil
	}

	args := p.buildArgs(pctx, []string{"destroy", "-f"})

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "vagrant", "args", args, "dir", p.resolveDir(pctx))
		return nil
	}

	return p.runInDir(pctx, args)
}

func (p *VagrantPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Vagrant
	if cfg == nil {
		return nil
	}

	args := p.buildArgs(pctx, []string{"halt"})

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "vagrant", "args", args, "dir", p.resolveDir(pctx))
		return nil
	}

	return p.runInDir(pctx, args)
}

func (p *VagrantPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	cfg := pctx.Entry.Vagrant
	if cfg == nil {
		return nil, nil
	}

	args := p.buildArgs(pctx, []string{"status", "--machine-readable"})

	cmd := exec.Command("vagrant", args...)
	cmd.Dir = p.resolveDir(pctx)
	cmd.Env = pctx.Env.EnvSlice()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("vagrant status: %w", err)
	}

	return parseVagrantStatus(string(out)), nil
}

// parseVagrantStatus parses vagrant --machine-readable output and extracts service states.
// Each line is CSV: timestamp,target,type,data
func parseVagrantStatus(output string) []ServiceStatus {
	var services []ServiceStatus
	seen := make(map[string]bool)

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// CSV format: timestamp,target,type,data
		parts := strings.SplitN(line, ",", 4)
		if len(parts) < 4 {
			continue
		}

		target, typ, data := parts[1], parts[2], parts[3]

		if typ == "state" && target != "" && !seen[target] {
			seen[target] = true
			services = append(services, ServiceStatus{
				Name:   target,
				State:  mapVagrantState(data),
				Health: "unknown",
			})
		}
	}

	return services
}

// mapVagrantState converts vagrant state strings to normalized states.
func mapVagrantState(state string) string {
	switch state {
	case "running":
		return "running"
	case "poweroff", "shutoff", "aborted":
		return "stopped"
	case "not_created":
		return "not_created"
	case "saved", "suspended":
		return "suspended"
	default:
		return state
	}
}
