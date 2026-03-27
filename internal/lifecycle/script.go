package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// ScriptPlugin executes arbitrary shell scripts for lifecycle events.
type ScriptPlugin struct{}

func (p *ScriptPlugin) Name() string { return "script" }

func (p *ScriptPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Script
	if cfg == nil || cfg.Up == "" {
		return &Result{}, nil
	}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "script", cfg.Up)
		return &Result{}, nil
	}

	if err := runScript(ctx, cfg.Up, pctx); err != nil {
		return nil, fmt.Errorf("script up: %w", err)
	}

	return &Result{}, nil
}

func (p *ScriptPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Script
	if cfg == nil || cfg.Down == "" {
		return nil
	}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "script", cfg.Down)
		return nil
	}

	return runScript(ctx, cfg.Down, pctx)
}

func (p *ScriptPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Script
	if cfg == nil || cfg.Stop == "" {
		return nil
	}

	if pctx.DryRun {
		pctx.Logger.Info("dry-run", "script", cfg.Stop)
		return nil
	}

	return runScript(ctx, cfg.Stop, pctx)
}

func (p *ScriptPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	// Scripts are fire-and-forget; no persistent status
	return nil, nil
}

// runScript executes a shell command with the plugin context's environment.
func runScript(ctx context.Context, script string, pctx *PluginContext) error {
	fmt.Fprintf(os.Stderr, "  $ %s\n", script)

	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Dir = pctx.ConfigDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = pctx.Env.EnvSlice()

	return cmd.Run()
}
