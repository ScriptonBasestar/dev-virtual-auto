package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// HelmPlugin manages Kubernetes services via Helm chart deployments.
type HelmPlugin struct{}

func (p *HelmPlugin) Name() string { return "helm" }

func (p *HelmPlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	cfg := pctx.Entry.Helm
	if cfg == nil {
		return &Result{}, nil
	}

	args := []string{"upgrade", "--install", cfg.Release, cfg.Chart}

	for _, vf := range cfg.Values {
		vf = pctx.Env.Interpolate(vf)
		if !filepath.IsAbs(vf) {
			vf = filepath.Join(pctx.ConfigDir, vf)
		}
		args = append(args, "-f", vf)
	}

	for k, v := range cfg.Set {
		args = append(args, "--set", k+"="+v)
	}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return &Result{}, nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, args)
	pctx.Logger.Debug("helm upgrade --install", "command", cmd, "args", cmdArgs)
	if err := dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false); err != nil {
		return nil, fmt.Errorf("helm upgrade --install: %w", err)
	}

	return &Result{
		Services: []ServiceStatus{{
			Name:   cfg.Release,
			State:  "deployed",
			Health: "unknown",
		}},
	}, nil
}

func (p *HelmPlugin) Down(ctx context.Context, pctx *PluginContext) error {
	cfg := pctx.Entry.Helm
	if cfg == nil {
		return nil
	}

	args := []string{"uninstall", cfg.Release}

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, args)
	pctx.Logger.Debug("helm uninstall", "command", cmd, "args", cmdArgs)
	return dvaexec.ExecSubprocess(pctx.Env, cmd, cmdArgs, false)
}

func (p *HelmPlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	// Helm has no graceful stop; delegate to Down (uninstall). But a release
	// that was never installed (first-ever restart, or a manual `helm
	// uninstall` outside dva) has nothing to tear down — mirror
	// ProcessPlugin.haltProcess's no-op-on-nothing-to-stop pattern for a
	// missing PID file, rather than surfacing helm's "release: not found" as
	// a teardown failure. DryRun keeps going through Down, which already
	// no-ops (and logs) without touching the cluster.
	if pctx.Entry.Helm != nil && !pctx.DryRun && !p.releaseInstalled(pctx) {
		return nil
	}
	return p.Down(ctx, pctx)
}

// releaseInstalled reports whether helm currently knows about the release,
// probing with `helm status` and treating any error the same way Status does:
// as "not found".
func (p *HelmPlugin) releaseInstalled(pctx *PluginContext) bool {
	cfg := pctx.Entry.Helm
	statusArgs := []string{"status", cfg.Release, "-o", "json"}
	cmd, cmdArgs := p.buildArgs(pctx, statusArgs)
	_, err := exec.Command(cmd, cmdArgs...).Output()
	return err == nil
}

func (p *HelmPlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	cfg := pctx.Entry.Helm
	if cfg == nil {
		return nil, nil
	}

	statusArgs := []string{"status", cfg.Release, "-o", "json"}
	cmd, cmdArgs := p.buildArgs(pctx, statusArgs)
	out, err := exec.Command(cmd, cmdArgs...).Output()
	if err != nil {
		// Release not found or helm error — treat as stopped.
		return []ServiceStatus{{
			Name:   cfg.Release,
			State:  "stopped",
			Health: "unknown",
		}}, nil
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return []ServiceStatus{{
			Name:   cfg.Release,
			State:  "stopped",
			Health: "unknown",
		}}, nil
	}

	var info helmStatusInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return []ServiceStatus{{
			Name:   cfg.Release,
			State:  "unknown",
			Health: "unknown",
		}}, nil
	}

	return []ServiceStatus{{
		Name:   cfg.Release,
		State:  info.Info.Status,
		Health: "unknown",
	}}, nil
}

// buildArgs constructs the helm command and arguments from plugin config.
func (p *HelmPlugin) buildArgs(pctx *PluginContext, extraArgs []string) (string, []string) {
	cmd := "helm"
	args := make([]string, len(extraArgs))
	copy(args, extraArgs)

	args = append(args, buildHelmContextArgs(pctx.Entry.Helm.Context)...)
	args = append(args, buildK8sNamespaceArgs(pctx.Entry.Helm.Namespace)...)

	return cmd, args
}

// helmStatusInfo mirrors the relevant fields from helm status -o json output.
type helmStatusInfo struct {
	Name string `json:"name"`
	Info struct {
		Status string `json:"status"`
	} `json:"info"`
}
