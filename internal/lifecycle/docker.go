package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// DockerPlugin manages standalone Docker containers via docker run.
type DockerPlugin struct{}

func (p *DockerPlugin) Name() string { return "docker" }

func (p *DockerPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Docker
	if cfg == nil {
		return &Result{}, nil
	}

	args := p.buildRunArgs(pctx)

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "docker", "args", args)
		return &Result{}, nil
	}

	pctx.Logger.Debug("docker run", "command", "docker", "args", args)
	if err := dvaexec.ExecSubprocess(pctx.Env, "docker", args, false); err != nil {
		return nil, fmt.Errorf("docker run: %w", err)
	}

	name := pctx.Env.Interpolate(cfg.Name)
	return &Result{
		Services: []ServiceStatus{{
			Name:  name,
			State: "running",
		}},
	}, nil
}

func (p *DockerPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Docker
	if cfg == nil {
		return nil
	}

	name := pctx.Env.Interpolate(cfg.Name)
	args := []string{"rm", "-f", name}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "docker", "args", args)
		return nil
	}

	pctx.Logger.Debug("docker rm", "command", "docker", "args", args)
	return dvaexec.ExecSubprocess(pctx.Env, "docker", args, false)
}

func (p *DockerPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Docker
	if cfg == nil {
		return nil
	}

	name := pctx.Env.Interpolate(cfg.Name)
	args := []string{"stop", name}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", "docker", "args", args)
		return nil
	}

	pctx.Logger.Debug("docker stop", "command", "docker", "args", args)
	return dvaexec.ExecSubprocess(pctx.Env, "docker", args, false)
}

func (p *DockerPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	cfg := pctx.Entry.Docker
	if cfg == nil {
		return nil, nil
	}

	name := pctx.Env.Interpolate(cfg.Name)
	out, err := exec.Command("docker", "inspect", "--format", "{{json .State}}", name).Output()
	if err != nil {
		// Container not found or not running
		return []ServiceStatus{{Name: name, State: "stopped"}}, nil
	}

	var state dockerInspectState
	if err := json.Unmarshal(out, &state); err != nil {
		return []ServiceStatus{{Name: name, State: "unknown"}}, nil
	}

	health := ""
	if state.Health != nil {
		health = state.Health.Status
	}

	return []ServiceStatus{{
		Name:   name,
		State:  state.Status,
		Health: health,
	}}, nil
}

// dockerInspectState mirrors the relevant fields from docker inspect .State JSON.
type dockerInspectState struct {
	Status  string `json:"Status"`
	Running bool   `json:"Running"`
	Health  *struct {
		Status string `json:"Status"`
	} `json:"Health,omitempty"`
}

// buildRunArgs constructs the docker run argument list from plugin config.
func (p *DockerPlugin) buildRunArgs(pctx *PluginContext) []string {
	cfg := pctx.Entry.Docker

	args := []string{"run", "-d"}

	if cfg.Name != "" {
		args = append(args, "--name", pctx.Env.Interpolate(cfg.Name))
	}

	for _, port := range cfg.Ports {
		args = append(args, "-p", pctx.Env.Interpolate(port))
	}

	for _, vol := range cfg.Volumes {
		args = append(args, "-v", pctx.Env.Interpolate(vol))
	}

	// Sort env keys for deterministic output
	keys := make([]string, 0, len(cfg.Env))
	for k := range cfg.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := pctx.Env.Interpolate(cfg.Env[k])
		args = append(args, "-e", k+"="+v)
	}

	for _, opt := range cfg.Options {
		args = append(args, pctx.Env.Interpolate(opt))
	}

	if cfg.Image != "" {
		args = append(args, pctx.Env.Interpolate(cfg.Image))
	}

	return args
}
