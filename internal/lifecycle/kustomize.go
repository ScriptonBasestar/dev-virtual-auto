package lifecycle

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// KustomizePlugin manages Kubernetes resources via kustomize overlays.
type KustomizePlugin struct{}

func (p *KustomizePlugin) Name() string { return "kustomize" }

// resolveDir returns the resolved kustomize directory path.
func (p *KustomizePlugin) resolveDir(pctx *PluginContext) string {
	cfg := pctx.Entry.Kustomize
	dir := pctx.Env.Interpolate(cfg.Dir)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(pctx.ConfigDir, dir)
	}
	return dir
}

// buildArgs constructs the kubectl command and arguments for kustomize operations.
func (p *KustomizePlugin) buildArgs(pctx *PluginContext, extraArgs []string) (string, []string) {
	cfg := pctx.Entry.Kustomize

	cmd := "kubectl"
	args := make([]string, 0, len(extraArgs)+4)
	args = append(args, extraArgs...)
	args = append(args, buildKubectlContextArgs(cfg.Context)...)
	args = append(args, buildK8sNamespaceArgs(cfg.Namespace)...)

	return cmd, args
}

func (p *KustomizePlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Kustomize
	if cfg == nil {
		return &Result{}, nil
	}

	dir := p.resolveDir(pctx)
	applyArgs := []string{"apply", "-k", dir}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, applyArgs)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return &Result{}, nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, applyArgs)
	if err := dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false); err != nil {
		return nil, fmt.Errorf("kustomize apply: %w", err)
	}

	return &Result{}, nil
}

func (p *KustomizePlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Kustomize
	if cfg == nil {
		return nil
	}

	dir := p.resolveDir(pctx)
	deleteArgs := []string{"delete", "-k", dir, "--ignore-not-found"}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, deleteArgs)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, deleteArgs)
	return dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false)
}

func (p *KustomizePlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	// Kubernetes has no graceful stop concept; delegate to Down.
	return p.Down(ctx, pctx)
}

func (p *KustomizePlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	cfg := pctx.Entry.Kustomize
	if cfg == nil {
		return nil, nil
	}

	dir := p.resolveDir(pctx)
	getArgs := []string{"get", "-k", dir, "-o", "json"}

	cmd, cmdArgs := p.buildArgs(pctx, getArgs)
	out, err := exec.Command(cmd, cmdArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("kustomize get: %w", err)
	}

	return parseK8sResourceStatus(out)
}
