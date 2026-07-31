package lifecycle

import (
	"fmt"
	"log/slog"
	"maps"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// NewPlanOrchestrator creates an orchestrator from resolved plan entries rather
// than falling back to the undeclared stack defaults.
func NewPlanOrchestrator(cfg *config.Config, env *config.Environment, plan *ExecutionPlan) (*Orchestrator, error) {
	if plan == nil {
		return nil, fmt.Errorf("execution plan is nil")
	}

	entries := make([]config.LifecycleEntry, 0, len(plan.Entries))
	composeServices := make(map[string][]string)
	for _, resolved := range plan.Entries {
		entry, err := materializeResolvedEntry(resolved)
		if err != nil {
			return nil, fmt.Errorf("materialize plan entry %q: %w", resolved.Name, err)
		}
		entries = append(entries, entry)
		if resolved.Runner == "compose" && resolved.Services != nil {
			composeServices[resolved.Name] = append([]string(nil), resolved.Services...)
		}
	}

	return &Orchestrator{
		entries:         entries,
		composeServices: composeServices,
		cfg:             cfg,
		env:             env,
		logger:          slog.Default(),
		hc:              &HealthChecker{},
	}, nil
}

func materializeResolvedEntry(resolved ResolvedEntry) (config.LifecycleEntry, error) {
	if resolved.StackEntry == nil {
		return config.LifecycleEntry{}, fmt.Errorf("stack entry is nil")
	}

	entry := *resolved.StackEntry
	entry.Name = resolved.Name
	entry.Order = resolved.Order
	entry.Vars = make(map[string]string, len(resolved.Vars))
	maps.Copy(entry.Vars, resolved.Vars)
	entry.Plugin = ""
	entry.Compose = nil
	entry.Process = nil
	entry.Script = nil
	entry.Docker = nil
	entry.Kubectl = nil
	entry.Helm = nil
	entry.Kustomize = nil
	entry.Tilt = nil
	entry.Skaffold = nil
	entry.PodmanCompose = nil
	entry.Vagrant = nil
	entry.SAM = nil
	entry.Serverless = nil
	entry.Multipass = nil

	switch runner := resolved.RunnerConfig.(type) {
	case *config.NativeRunnerConfig:
		entry.Plugin = "process"
		entry.Process = &config.ProcessPluginConfig{Command: runner.Run, Dir: runner.Dir}
	case *config.ComposePluginConfig:
		entry.Plugin, entry.Compose = "compose", runner
	case *config.ProcessPluginConfig:
		entry.Plugin, entry.Process = "process", runner
	case *config.ScriptPluginConfig:
		entry.Plugin, entry.Script = "script", runner
	case *config.DockerRunnerConfig:
		entry.Plugin = "docker"
		entry.Docker = &config.DockerPluginConfig{
			Image: runner.Image, Name: entry.Name, Ports: runner.Ports,
			Volumes: runner.Volumes, Env: runner.Env, Options: runner.Options,
		}
	case *config.DockerPluginConfig:
		entry.Plugin, entry.Docker = "docker", runner
	case *config.KubectlPluginConfig:
		entry.Plugin, entry.Kubectl = "kubectl", runner
	case *config.HelmPluginConfig:
		entry.Plugin, entry.Helm = "helm", runner
	case *config.KustomizePluginConfig:
		entry.Plugin, entry.Kustomize = "kustomize", runner
	case *config.TiltPluginConfig:
		entry.Plugin, entry.Tilt = "tilt", runner
	case *config.SkaffoldPluginConfig:
		entry.Plugin, entry.Skaffold = "skaffold", runner
	case *config.PodmanComposePluginConfig:
		entry.Plugin, entry.PodmanCompose = "podman-compose", runner
	case *config.VagrantPluginConfig:
		entry.Plugin, entry.Vagrant = "vagrant", runner
	case *config.SAMPluginConfig:
		entry.Plugin, entry.SAM = "sam", runner
	case *config.ServerlessPluginConfig:
		entry.Plugin, entry.Serverless = "serverless", runner
	case *config.MultipassPluginConfig:
		entry.Plugin, entry.Multipass = "multipass", runner
	default:
		return config.LifecycleEntry{}, fmt.Errorf("unsupported runner %q (%T)", resolved.Runner, resolved.RunnerConfig)
	}

	return entry, nil
}
