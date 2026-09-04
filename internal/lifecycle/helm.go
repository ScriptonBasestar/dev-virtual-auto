package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
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
	//
	// Only helm's own not-found signal counts as nothing-to-stop. Every other probe
	// failure propagates, because a probe that could not be completed is not evidence
	// the release is absent: returning nil there would report a teardown that never
	// happened as a success, and Orchestrator.Stop can only aggregate what this returns
	// (TASK-295).
	if pctx.Entry.Helm != nil && !pctx.DryRun {
		installed, err := p.releaseInstalled(ctx, pctx)
		if err != nil {
			return err
		}
		if !installed {
			return nil
		}
	}
	return p.Down(ctx, pctx)
}

// releaseInstalled reports whether helm currently knows about the release, probing with
// `helm status`. A clean exit means installed and helm's not-found message means not
// installed; anything else is returned as an error rather than guessed at.
//
// The probe runs under ctx and with pctx.Env — the same environment
// dvaexec.ExecSubprocess gives the `helm uninstall` this gates. A KUBECONFIG supplied
// through dva.yml reaches that uninstall but not the ambient process environment, so a
// probe reading only os.Environ() would fail to find an installed release and skip a real
// teardown.
func (p *HelmPlugin) releaseInstalled(ctx context.Context, pctx *PluginContext) (bool, error) {
	cfg := pctx.Entry.Helm
	statusArgs := []string{"status", cfg.Release, "-o", "json"}
	cmd, cmdArgs := p.buildArgs(pctx, statusArgs)

	c := exec.CommandContext(ctx, cmd, cmdArgs...)
	c.Env = pctx.Env.EnvSlice()
	if _, err := c.Output(); err != nil {
		// A cancelled or timed-out probe reports nothing about the release; helm's
		// stderr on a kill is empty anyway, so classify it explicitly.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, fmt.Errorf("helm status %s: %w", cfg.Release, ctxErr)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && helmReportsReleaseNotFound(exitErr.Stderr) {
			return false, nil
		}
		return false, fmt.Errorf("helm status %s: %w", cfg.Release, err)
	}
	return true, nil
}

// helmReportsReleaseNotFound recognizes helm's own "this release does not exist" report.
// Helm exits 1 for a missing release and for an unreachable cluster alike, so the exit
// code cannot separate them — the message is the only signal.
func helmReportsReleaseNotFound(stderr []byte) bool {
	msg := strings.ToLower(string(stderr))
	return strings.Contains(msg, "release: not found") || strings.Contains(msg, "no release found")
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
