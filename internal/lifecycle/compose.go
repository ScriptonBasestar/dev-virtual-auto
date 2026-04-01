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

// ComposePlugin manages services via Docker Compose.
type ComposePlugin struct{}

func (p *ComposePlugin) Name() string { return "compose" }

func (p *ComposePlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Compose
	if cfg == nil {
		return &Result{}, nil
	}

	upOpts := cfg.UpOptions
	if len(upOpts) == 0 {
		upOpts = []string{"-d", "--wait"}
	}
	if !pctx.Wait {
		// Remove --wait for immediate return
		var filtered []string
		for _, o := range upOpts {
			if o != "--wait" {
				filtered = append(filtered, o)
			}
		}
		upOpts = filtered
		if len(upOpts) == 0 {
			upOpts = []string{"-d"}
		}
	}

	args := append([]string{"up"}, upOpts...)

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return &Result{}, nil
	}

	if err := p.runSubprocess(pctx, args); err != nil {
		return nil, fmt.Errorf("compose up: %w", err)
	}

	// Query service status after up
	services, _ := p.queryServices(pctx)

	return &Result{Services: services}, nil
}

func (p *ComposePlugin) Down(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.Compose == nil {
		return nil
	}

	args := []string{"down", "--remove-orphans"}
	if pctx.Volumes {
		args = append(args, "--volumes")
	}
	if pctx.RemoveImages {
		args = append(args, "--rmi", "local")
	}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	return p.runSubprocess(pctx, args)
}

func (p *ComposePlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.Compose == nil {
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

func (p *ComposePlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	return p.queryServices(pctx)
}

// buildArgs constructs the docker compose command and arguments from plugin config.
// Mode-derived profiles are injected before the subcommand; mode-derived services
// are appended after the subcommand args (only for "up").
func (p *ComposePlugin) buildArgs(pctx *PluginContext, extraArgs []string) (string, []string) {
	cfg := pctx.Entry.Compose

	cmd := "docker"
	args := []string{"compose"}

	if cfg.Command != "" {
		parts := dvaexec.SplitCommand(cfg.Command)
		cmd = parts[0]
		if len(parts) > 1 {
			args = parts[1:]
		}
	}

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

	// Inject mode-derived --profile flags (before subcommand args)
	for _, profile := range pctx.ComposeProfiles {
		args = append(args, "--profile", profile)
	}

	args = append(args, extraArgs...)

	// Append mode-derived service names (only for "up" subcommand)
	if pctx.ComposeServices != nil && len(*pctx.ComposeServices) > 0 {
		if len(extraArgs) > 0 && extraArgs[0] == "up" {
			args = append(args, *pctx.ComposeServices...)
		}
	}

	return cmd, args
}

// runSubprocess executes a docker compose command as a subprocess.
func (p *ComposePlugin) runSubprocess(pctx *PluginContext, args []string) error {
	cmd, cmdArgs := p.buildArgs(pctx, args)
	pctx.Logger.Debug("compose subprocess", "command", cmd, "args", cmdArgs)
	return dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false)
}

// composeServiceInfo mirrors docker compose ps JSON output for parsing.
type composeServiceInfo struct {
	Name       string             `json:"Name"`
	Service    string             `json:"Service"`
	State      string             `json:"State"`
	Health     string             `json:"Health"`
	Publishers []composePublisher `json:"Publishers"`
}

type composePublisher struct {
	TargetPort    int `json:"TargetPort"`
	PublishedPort int `json:"PublishedPort"`
}

// queryServices runs docker compose ps and returns parsed service statuses.
func (p *ComposePlugin) queryServices(pctx *PluginContext) ([]ServiceStatus, error) {
	if pctx.Entry.Compose == nil {
		return nil, nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, []string{"ps", "--format", "json"})
	out, err := exec.Command(cmd, cmdArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("compose ps: %w", err)
	}

	return parseComposeServicesJSON(out)
}

// parseComposeServicesJSON parses docker compose ps JSON output (array or JSON-lines)
// into ServiceStatus slice. Shared by ComposePlugin and PodmanComposePlugin.
func parseComposeServicesJSON(out []byte) ([]ServiceStatus, error) {
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
				fmt.Fprintf(os.Stderr, "[warn] failed to parse compose service info: %v\n", err)
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
