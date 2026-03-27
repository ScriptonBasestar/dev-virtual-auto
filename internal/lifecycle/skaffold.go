package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// SkaffoldPlugin manages Skaffold build-push-deploy pipelines.
type SkaffoldPlugin struct{}

func (p *SkaffoldPlugin) Name() string { return "skaffold" }

func (p *SkaffoldPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Skaffold
	if cfg == nil {
		return &Result{}, nil
	}

	cmd, args := p.buildArgs(pctx, []string{"run"})

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", cmd, "args", args)
		return &Result{}, nil
	}

	pctx.Logger.Debug("skaffold run", "command", cmd, "args", args)
	if err := dvaexec.ExecSubprocess(pctx.Env, cmd, args, false); err != nil {
		return nil, fmt.Errorf("skaffold run: %w", err)
	}

	return &Result{
		Services: []ServiceStatus{{
			Name:  pctx.Entry.Name,
			State: "running",
		}},
	}, nil
}

func (p *SkaffoldPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Skaffold
	if cfg == nil {
		return nil
	}

	cmd, args := p.buildArgs(pctx, []string{"delete"})

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "command", cmd, "args", args)
		return nil
	}

	pctx.Logger.Debug("skaffold delete", "command", cmd, "args", args)
	return dvaexec.ExecSubprocess(pctx.Env, cmd, args, false)
}

func (p *SkaffoldPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	return p.Down(ctx, pctx)
}

func (p *SkaffoldPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	// Skaffold deploys to k8s; actual status would need kubectl which is out of scope.
	return []ServiceStatus{{
		Name:   pctx.Entry.Name,
		State:  "unknown",
		Health: "unknown",
	}}, nil
}

// buildArgs constructs the skaffold command and arguments from plugin config.
func (p *SkaffoldPlugin) buildArgs(pctx *PluginContext, extraArgs []string) (string, []string) {
	cfg := pctx.Entry.Skaffold
	cmd := "skaffold"
	args := make([]string, len(extraArgs))
	copy(args, extraArgs)

	if cfg.Config != "" {
		cfgPath := pctx.Env.Interpolate(cfg.Config)
		if !filepath.IsAbs(cfgPath) {
			cfgPath = filepath.Join(pctx.ConfigDir, cfgPath)
		}
		args = append(args, "-f", cfgPath)
	}

	if cfg.Profile != "" {
		args = append(args, "-p", cfg.Profile)
	}

	args = append(args, cfg.Args...)

	return cmd, args
}
