package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// PodmanComposePlugin manages services via podman-compose.
type PodmanComposePlugin struct{}

func (p *PodmanComposePlugin) Name() string { return "podman-compose" }

func (p *PodmanComposePlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.PodmanCompose
	if cfg == nil {
		return &Result{}, nil
	}

	args := []string{"up", "-d"}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return &Result{}, nil
	}

	if err := p.runSubprocess(pctx, args); err != nil {
		return nil, fmt.Errorf("podman-compose up: %w", err)
	}

	// Query service status after up
	services, _ := p.queryServices(pctx)

	return &Result{Services: services}, nil
}

func (p *PodmanComposePlugin) Down(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.PodmanCompose == nil {
		return nil
	}

	args := []string{"down", "--remove-orphans"}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	return p.runSubprocess(pctx, args)
}

func (p *PodmanComposePlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.PodmanCompose == nil {
		return nil
	}

	args := []string{"stop"}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	return p.runSubprocess(pctx, args)
}

func (p *PodmanComposePlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	return p.queryServices(pctx)
}

// buildArgs constructs the podman-compose command and arguments from plugin config.
func (p *PodmanComposePlugin) buildArgs(pctx *PluginContext, extraArgs []string) (string, []string) {
	cfg := pctx.Entry.PodmanCompose

	cmd := "podman-compose"
	var args []string

	for _, f := range cfg.Files {
		f = pctx.Env.Interpolate(f)
		if !filepath.IsAbs(f) {
			f = pctx.ConfigDir + "/" + f
		}
		args = append(args, "-f", f)
	}

	if cfg.ProjectName != "" {
		args = append(args, "--project-name", pctx.Env.Interpolate(cfg.ProjectName))
	}

	args = append(args, extraArgs...)
	return cmd, args
}

// runSubprocess executes a podman-compose command as a subprocess.
func (p *PodmanComposePlugin) runSubprocess(pctx *PluginContext, args []string) error {
	cmd, cmdArgs := p.buildArgs(pctx, args)
	pctx.Logger.Debug("podman-compose subprocess", "command", cmd, "args", cmdArgs)
	return dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false)
}

// queryServices runs podman-compose ps and returns parsed service statuses.
func (p *PodmanComposePlugin) queryServices(pctx *PluginContext) ([]ServiceStatus, error) {
	if pctx.Entry.PodmanCompose == nil {
		return nil, nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, []string{"ps", "--format", "json"})
	out, err := exec.Command(cmd, cmdArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("podman-compose ps: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}

	var infos []composeServiceInfo
	if err := json.Unmarshal(out, &infos); err != nil {
		// Try JSON lines format
		for _, line := range strings.Split(trimmed, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var info composeServiceInfo
			if err := json.Unmarshal([]byte(line), &info); err != nil {
				fmt.Fprintf(os.Stderr, "[warn] failed to parse podman-compose service info: %v\n", err)
				continue
			}
			infos = append(infos, info)
		}
	}

	services := make([]ServiceStatus, 0, len(infos))
	for _, info := range infos {
		ports := make(map[int]int)
		for _, pub := range info.Publishers {
			if pub.PublishedPort > 0 {
				ports[pub.PublishedPort] = pub.TargetPort
			}
		}
		services = append(services, ServiceStatus{
			Name:   info.Service,
			State:  info.State,
			Health: info.Health,
			Ports:  ports,
		})
	}

	return services, nil
}
