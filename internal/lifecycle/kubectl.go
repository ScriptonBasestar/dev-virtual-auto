package lifecycle

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// KubectlPlugin manages Kubernetes resources via kubectl.
type KubectlPlugin struct{}

func (p *KubectlPlugin) Name() string { return "kubectl" }

// buildArgs constructs the kubectl command and arguments from plugin config.
func (p *KubectlPlugin) buildArgs(pctx *PluginContext, extraArgs []string) (string, []string) {
	cfg := pctx.Entry.Kubectl

	cmd := "kubectl"
	args := make([]string, 0, len(extraArgs)+4)
	args = append(args, extraArgs...)
	args = append(args, buildKubectlContextArgs(cfg.Context)...)
	args = append(args, buildK8sNamespaceArgs(cfg.Namespace)...)

	return cmd, args
}

// manifestArgs returns -f flags for each manifest in the config.
func (p *KubectlPlugin) manifestArgs(pctx *PluginContext) []string {
	cfg := pctx.Entry.Kubectl
	var args []string
	for _, m := range cfg.Manifests {
		m = pctx.Env.Interpolate(m)
		if !filepath.IsAbs(m) {
			m = filepath.Join(pctx.ConfigDir, m)
		}
		args = append(args, "-f", m)
	}
	return args
}

func (p *KubectlPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Kubectl
	if cfg == nil {
		return &Result{}, nil
	}

	applyArgs := append([]string{"apply"}, p.manifestArgs(pctx)...)

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, applyArgs)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return &Result{}, nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, applyArgs)
	if err := dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false); err != nil {
		return nil, fmt.Errorf("kubectl apply: %w", err)
	}

	return &Result{}, nil
}

func (p *KubectlPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Kubectl
	if cfg == nil {
		return nil
	}

	deleteArgs := append([]string{"delete"}, p.manifestArgs(pctx)...)
	deleteArgs = append(deleteArgs, "--ignore-not-found")

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, deleteArgs)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, deleteArgs)
	return dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false)
}

func (p *KubectlPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	// Kubernetes has no graceful stop concept; delegate to Down.
	return p.Down(ctx, pctx)
}

func (p *KubectlPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	cfg := pctx.Entry.Kubectl
	if cfg == nil {
		return nil, nil
	}

	getArgs := append([]string{"get"}, p.manifestArgs(pctx)...)
	getArgs = append(getArgs, "-o", "json")

	cmd, cmdArgs := p.buildArgs(pctx, getArgs)
	out, err := exec.Command(cmd, cmdArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get: %w", err)
	}

	return parseK8sResourceStatus(out)
}
