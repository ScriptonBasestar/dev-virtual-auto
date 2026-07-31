package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// ComposePlugin manages services via Docker Compose.
type ComposePlugin struct{}

func (p *ComposePlugin) Name() string { return "compose" }

func (p *ComposePlugin) Up(ctx context.Context, pctx *PluginContext) (*Result, error) {
	if pctx.Entry.ComposeConfig() == nil {
		return &Result{}, nil
	}

	args := composeUpArgs(pctx)

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return &Result{}, nil
	}

	// Preflight: validate the compose file set resolves before `up`. This makes a
	// compose file whose -f or include: target is missing/invalid surface as a
	// dva-owned, actionable error instead of docker's raw stderr followed by a
	// bare exit status. `config` only parses/merges — it needs no daemon.
	if cfg := pctx.Entry.ComposeConfig(); len(cfg.Files) > 0 {
		if err := p.preflightConfig(pctx); err != nil {
			return nil, err
		}
	}

	if err := p.runSubprocess(pctx, args); err != nil {
		return nil, fmt.Errorf("compose up: %w", err)
	}

	// Query service status after up
	services, _ := p.queryServices(pctx)

	return &Result{Services: services}, nil
}

func composeUpArgs(pctx *PluginContext) []string {
	cfg := pctx.Entry.ComposeConfig()
	upOpts := cfg.UpOptions
	if len(upOpts) == 0 {
		upOpts = []string{"-d", "--wait"}
	}
	if !pctx.Wait {
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
	if pctx.Force {
		hasForce := slices.Contains(upOpts, "--force-recreate")
		if !hasForce {
			upOpts = append(upOpts, "--force-recreate")
		}
	}
	return append([]string{"up"}, upOpts...)
}

func (p *ComposePlugin) Down(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.ComposeConfig() == nil {
		return nil
	}

	args := composeDownArgs(pctx)

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	return p.runSubprocess(pctx, args)
}

func composeDownArgs(pctx *PluginContext) []string {
	args := []string{"down", "--remove-orphans"}
	if pctx.ComposeServices != nil && len(*pctx.ComposeServices) > 0 {
		args = []string{"rm", "--force", "--stop"}
		if pctx.Volumes {
			args = append(args, "--volumes")
		}
		args = append(args, *pctx.ComposeServices...)
	}
	if pctx.Volumes {
		if len(args) > 0 && args[0] == "down" {
			args = append(args, "--volumes")
		}
	}
	if pctx.RemoveImages {
		args = append(args, "--rmi", "local")
	}
	return args
}

func (p *ComposePlugin) Stop(ctx context.Context, pctx *PluginContext) error {
	if pctx.Entry.ComposeConfig() == nil {
		return nil
	}

	args := composeStopArgs(pctx)

	if pctx.DryRun {
		cmd, cmdArgs := p.buildArgs(pctx, args)
		pctx.Logger.Info("dry-run", "command", cmd, "args", cmdArgs)
		return nil
	}

	return p.runSubprocess(pctx, args)
}

func composeStopArgs(pctx *PluginContext) []string {
	args := []string{"stop"}
	if pctx.ComposeServices != nil && len(*pctx.ComposeServices) > 0 {
		args = append(args, *pctx.ComposeServices...)
	}
	return args
}

func (p *ComposePlugin) Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error) {
	return p.queryServices(pctx)
}

// buildArgs constructs the docker compose command and arguments from plugin config.
// Mode-derived profiles are injected before the subcommand; mode-derived services
// are appended after the subcommand args (only for "up").
func (p *ComposePlugin) buildArgs(pctx *PluginContext, extraArgs []string) (string, []string) {
	cfg := pctx.Entry.ComposeConfig()

	cmd := "docker"
	args := []string{"compose"}

	if cfg.Command != "" {
		parts := dvaexec.SplitCommand(cfg.Command)
		cmd = parts[0]
		if len(parts) > 1 {
			args = parts[1:]
		}
	}

	// For sourced entries (TASK-051), relative compose files resolve against the
	// fetched/referenced source dir. The command also runs with that dir as its
	// working directory (see composeWorkdir), matching the legacy
	// `cd <dir> && docker compose` behavior so default file discovery, .env,
	// relative build contexts and volumes resolve as the external author intended.
	baseDir := pctx.ConfigDir
	if wd := composeWorkdir(pctx); wd != "" {
		baseDir = wd
	}

	for _, f := range cfg.Files {
		f = pctx.Env.Interpolate(f)
		if !filepath.IsAbs(f) {
			f = baseDir + "/" + f
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

// composeWorkdir returns the working directory a sourced compose entry runs in.
// Sourced entries (TASK-051) execute in their fetched/referenced source dir so
// relative compose files, default file discovery, .env and build contexts
// resolve as the external stack's author intended (matching the legacy
// `cd <dir> && docker compose` behavior). Returns "" for non-sourced entries,
// which then inherit the current working directory.
func composeWorkdir(pctx *PluginContext) string {
	if src := pctx.Entry.Source; src != nil {
		if d, err := config.SourceDir(src, pctx.Entry.Name, pctx.ConfigDir); err == nil {
			return d
		}
	}
	return ""
}

// runSubprocess executes a docker compose command as a subprocess.
func (p *ComposePlugin) runSubprocess(pctx *PluginContext, args []string) error {
	cmd, cmdArgs := p.buildArgs(pctx, args)
	pctx.Logger.Debug("compose subprocess", "command", cmd, "args", cmdArgs)
	return dvaexec.ExecSubprocessInDir(pctx.Env, composeWorkdir(pctx), cmd, cmdArgs, false)
}

// preflightConfig runs `docker compose ... config --quiet` to validate the
// compose file set (including that every -f and include: target resolves)
// before `up`. Output is captured so a broken reference surfaces through a
// ComposeConfigError carrying docker's own diagnostic, rather than being
// streamed raw ahead of a bare exit status.
func (p *ComposePlugin) preflightConfig(pctx *PluginContext) error {
	cmd, cmdArgs := p.buildArgs(pctx, []string{"config", "--quiet"})
	pctx.Logger.Debug("compose preflight", "command", cmd, "args", cmdArgs)
	out, err := dvaexec.ExecSubprocessCaptureInDir(pctx.Env, composeWorkdir(pctx), cmd, cmdArgs, false)
	if err != nil {
		return &ComposeConfigError{
			Files:  pctx.Entry.ComposeConfig().Files,
			Detail: strings.TrimSpace(out),
			cause:  err,
		}
	}
	return nil
}

// ComposeConfigError reports that the pre-up `docker compose config` preflight
// failed — typically a file referenced by -f or include: that does not resolve,
// or invalid/merge-conflicting YAML. It carries docker's own diagnostic (Detail)
// so the real cause surfaces instead of a bare exit status, plus a remediation
// hint. Mirrors ResolveError's cause-wrapping shape (Error/Unwrap).
type ComposeConfigError struct {
	Files  []string
	Detail string
	cause  error
}

func (e *ComposeConfigError) Error() string {
	msg := "compose config is invalid"
	if e.Detail != "" {
		msg += ": " + firstLine(e.Detail)
	}
	msg += "\n       → a compose file referenced by -f or include: does not resolve or is invalid"
	msg += "\n       → check compose.files in dva.yml and any include: paths, then run: docker compose config"
	return msg
}

func (e *ComposeConfigError) Unwrap() error { return e.cause }

// firstLine returns the first line of s (docker's diagnostics are typically a
// single line; guard against multi-line output leaking into the summary).
func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return strings.TrimSpace(before)
	}
	return s
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
	if pctx.Entry.ComposeConfig() == nil {
		return nil, nil
	}

	cmd, cmdArgs := p.buildArgs(pctx, []string{"ps", "--format", "json"})
	c := exec.Command(cmd, cmdArgs...)
	c.Dir = composeWorkdir(pctx)
	out, err := c.Output()
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
		for line := range strings.SplitSeq(trimmed, "\n") {
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
