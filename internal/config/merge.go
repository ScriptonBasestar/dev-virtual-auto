package config

import "fmt"

// mergeStringMap merges src into dst (key-level merge).
// Existing keys in dst are overwritten by src.
func mergeStringMap(dst, src map[string]string) map[string]string {
	if src == nil {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// MergeLifecycleEntry deep-merges other into base.
// Returns an error if a restricted field (plugin) is changed.
func MergeLifecycleEntry(base, other *LifecycleEntry) (*LifecycleEntry, error) {
	if base == nil {
		return other, nil
	}
	if other == nil {
		return base, nil
	}

	// Restricted field: plugin must not change
	if other.Plugin != "" && base.Plugin != "" && other.Plugin != base.Plugin {
		return nil, fmt.Errorf("cannot override plugin type for stack entry %q: %q → %q (restricted field)", base.Name, base.Plugin, other.Plugin)
	}

	// Scalar replace (non-zero takes precedence)
	if other.Order != 0 {
		base.Order = other.Order
	}

	// List replace
	if other.Tags != nil {
		base.Tags = other.Tags
	}

	// Map merge
	base.Exports = mergeStringMap(base.Exports, other.Exports)

	// HealthChecks map merge
	if other.HealthChecks != nil {
		if base.HealthChecks == nil {
			base.HealthChecks = make(map[string]HealthCheckConfig)
		}
		for k, v := range other.HealthChecks {
			base.HealthChecks[k] = v
		}
	}

	// Plugin configs: merge if same type, set if base had none
	if other.Compose != nil {
		if base.Compose != nil {
			mergeComposeConfig(base.Compose, other.Compose)
		} else {
			base.Compose = other.Compose
		}
	}
	if other.Process != nil {
		if base.Process != nil {
			mergeProcessConfig(base.Process, other.Process)
		} else {
			base.Process = other.Process
		}
	}
	if other.Script != nil {
		if base.Script != nil {
			mergeScriptConfig(base.Script, other.Script)
		} else {
			base.Script = other.Script
		}
	}
	if other.Docker != nil {
		if base.Docker != nil {
			mergeDockerConfig(base.Docker, other.Docker)
		} else {
			base.Docker = other.Docker
		}
	}
	if other.Kubectl != nil {
		if base.Kubectl != nil {
			mergeKubectlConfig(base.Kubectl, other.Kubectl)
		} else {
			base.Kubectl = other.Kubectl
		}
	}
	if other.Helm != nil {
		if base.Helm != nil {
			mergeHelmConfig(base.Helm, other.Helm)
		} else {
			base.Helm = other.Helm
		}
	}
	if other.Kustomize != nil {
		if base.Kustomize != nil {
			mergeKustomizeConfig(base.Kustomize, other.Kustomize)
		} else {
			base.Kustomize = other.Kustomize
		}
	}
	// Tier 2/3 plugins: simple replace (less common, deep merge not needed yet)
	if other.Tilt != nil {
		base.Tilt = other.Tilt
	}
	if other.Skaffold != nil {
		base.Skaffold = other.Skaffold
	}
	if other.PodmanCompose != nil {
		base.PodmanCompose = other.PodmanCompose
	}
	if other.Vagrant != nil {
		base.Vagrant = other.Vagrant
	}
	if other.SAM != nil {
		base.SAM = other.SAM
	}
	if other.Serverless != nil {
		base.Serverless = other.Serverless
	}
	if other.Multipass != nil {
		base.Multipass = other.Multipass
	}

	if other.Description != "" {
		base.Description = other.Description
	}
	base.Vars = mergeStringMap(base.Vars, other.Vars)
	if other.DefaultRunner != "" {
		base.DefaultRunner = other.DefaultRunner
	}

	if other.Runners != nil {
		if base.Runners == nil {
			base.Runners = make(map[string]any)
		}
		for runnerName, runnerConfig := range other.Runners {
			if existing, ok := base.Runners[runnerName]; ok {
				base.Runners[runnerName] = mergeRunnerConfig(existing, runnerConfig)
			} else {
				base.Runners[runnerName] = runnerConfig
			}
		}
	}

	return base, nil
}

// --- Plugin config merge helpers ---
// Strategy: scalar replace (non-zero), list replace (non-nil), map merge

func mergeComposeConfig(base, other *ComposePluginConfig) {
	if other.ProjectName != "" {
		base.ProjectName = other.ProjectName
	}
	if other.Command != "" {
		base.Command = other.Command
	}
	if other.Method != "" {
		base.Method = other.Method
	}
	if other.Files != nil {
		base.Files = other.Files
	}
	if other.UpOptions != nil {
		base.UpOptions = other.UpOptions
	}
	if other.Tags != nil {
		base.Tags = other.Tags
	}
	if other.Services != nil {
		if base.Services == nil {
			base.Services = make(map[string]ServiceTagConfig)
		}
		for k, v := range other.Services {
			base.Services[k] = v
		}
	}
}

func mergeProcessConfig(base, other *ProcessPluginConfig) {
	if other.Command != "" {
		base.Command = other.Command
	}
	if other.Dir != "" {
		base.Dir = other.Dir
	}
	if other.ReadyTimeout != 0 {
		base.ReadyTimeout = other.ReadyTimeout
	}
}

func mergeScriptConfig(base, other *ScriptPluginConfig) {
	if other.Up != "" {
		base.Up = other.Up
	}
	if other.Down != "" {
		base.Down = other.Down
	}
	if other.Stop != "" {
		base.Stop = other.Stop
	}
}

func mergeDockerConfig(base, other *DockerPluginConfig) {
	if other.Image != "" {
		base.Image = other.Image
	}
	if other.Name != "" {
		base.Name = other.Name
	}
	if other.Ports != nil {
		base.Ports = other.Ports
	}
	if other.Volumes != nil {
		base.Volumes = other.Volumes
	}
	if other.Options != nil {
		base.Options = other.Options
	}
	base.Env = mergeStringMap(base.Env, other.Env)
}

func mergeKubectlConfig(base, other *KubectlPluginConfig) {
	if other.Namespace != "" {
		base.Namespace = other.Namespace
	}
	if other.Context != "" {
		base.Context = other.Context
	}
	if other.Kubeconfig != "" {
		base.Kubeconfig = other.Kubeconfig
	}
	if other.Manifests != nil {
		base.Manifests = other.Manifests
	}
}

func mergeHelmConfig(base, other *HelmPluginConfig) {
	if other.Chart != "" {
		base.Chart = other.Chart
	}
	if other.Release != "" {
		base.Release = other.Release
	}
	if other.Namespace != "" {
		base.Namespace = other.Namespace
	}
	if other.Context != "" {
		base.Context = other.Context
	}
	if other.Values != nil {
		base.Values = other.Values
	}
	base.Set = mergeStringMap(base.Set, other.Set)
}

func mergeKustomizeConfig(base, other *KustomizePluginConfig) {
	if other.Dir != "" {
		base.Dir = other.Dir
	}
	if other.Namespace != "" {
		base.Namespace = other.Namespace
	}
	if other.Context != "" {
		base.Context = other.Context
	}
}

// mergeHealthCheckConfig deep-merges other into base (scalar replace).
func mergeHealthCheckConfig(base, other HealthCheckConfig) HealthCheckConfig {
	if other.Type != "" {
		base.Type = other.Type
	}
	if other.URL != "" {
		base.URL = other.URL
	}
	if other.Address != "" {
		base.Address = other.Address
	}
	if other.Command != "" {
		base.Command = other.Command
	}
	if other.Start != "" {
		base.Start = other.Start
	}
	if other.StartHint != "" {
		base.StartHint = other.StartHint
	}
	if other.Timeout != 0 {
		base.Timeout = other.Timeout
	}
	if other.ReadyTimeout != 0 {
		base.ReadyTimeout = other.ReadyTimeout
	}
	return base
}

// mergeEndpointConfig deep-merges other into base.
func mergeEndpointConfig(base, other EndpointConfig) EndpointConfig {
	if other.URL != "" {
		base.URL = other.URL
	}
	if other.Label != "" {
		base.Label = other.Label
	}
	if other.Source != "" {
		base.Source = other.Source
	}
	// List replace
	if other.Tags != nil {
		base.Tags = other.Tags
	}
	// Map merge
	if other.Paths != nil {
		if base.Paths == nil {
			base.Paths = make(map[string]string)
		}
		for k, v := range other.Paths {
			base.Paths[k] = v
		}
	}
	return base
}

// mergeInteractionCommand deep-merges other into base.
// Returns an error if runner (restricted field) is changed.
func mergeInteractionCommand(base, other *InteractionCommand) (*InteractionCommand, error) {
	if base == nil {
		return other, nil
	}
	if other == nil {
		return base, nil
	}

	// Restricted field: runner must not change
	if other.Runner != "" && base.Runner != "" && other.Runner != base.Runner {
		return nil, fmt.Errorf("cannot override runner for interaction command: %q → %q (restricted field)", base.Runner, other.Runner)
	}

	// Scalar replace
	if other.Description != "" {
		base.Description = other.Description
	}
	if other.Service != "" {
		base.Service = other.Service
	}
	if other.Command != "" {
		base.Command = other.Command
		// When command is overridden as a scalar, clear multi-command list
		base.CommandLines = nil
	}
	if other.CommandLines != nil {
		base.CommandLines = other.CommandLines
		if len(other.CommandLines) > 0 {
			base.Command = other.CommandLines[0]
		}
	}
	if other.Script != "" {
		base.Script = other.Script
	}
	if other.ScriptFile != "" {
		base.ScriptFile = other.ScriptFile
	}
	if other.Steps != nil {
		base.Steps = other.Steps
	}
	if other.Workdir != "" {
		base.Workdir = other.Workdir
	}
	if other.User != "" {
		base.User = other.User
	}
	if other.DefaultArgs != "" {
		base.DefaultArgs = other.DefaultArgs
	}
	if other.Entrypoint != "" {
		base.Entrypoint = other.Entrypoint
	}
	if other.Pod != "" {
		base.Pod = other.Pod
	}
	if other.Runner != "" {
		base.Runner = other.Runner
	}
	if other.Shell != nil {
		base.Shell = other.Shell
	}
	if other.Compose != nil {
		base.Compose = other.Compose
	}
	if other.EnvFile != nil {
		base.EnvFile = other.EnvFile
	}

	// List replace
	if other.Tags != nil {
		base.Tags = other.Tags
	}

	// Map merge
	base.Environment = mergeStringMap(base.Environment, other.Environment)

	// Subcommands map merge (deep)
	if other.Subcommands != nil {
		if base.Subcommands == nil {
			base.Subcommands = make(map[string]*InteractionCommand)
		}
		for k, v := range other.Subcommands {
			if existing, ok := base.Subcommands[k]; ok {
				merged, err := mergeInteractionCommand(existing, v)
				if err != nil {
					return nil, fmt.Errorf("subcommand %q: %w", k, err)
				}
				base.Subcommands[k] = merged
			} else {
				base.Subcommands[k] = v
			}
		}
	}

	return base, nil
}

// mergeModeConfig deep-merges other into base.
func mergeModeConfig(base, other ModeConfig) ModeConfig {
	if other.Description != "" {
		base.Description = other.Description
	}
	if other.Build != "" {
		base.Build = other.Build
	}
	if other.Run != "" {
		base.Run = other.Run
	}
	if other.Provision != "" {
		base.Provision = other.Provision
	}
	if other.Applications != nil {
		base.Applications = other.Applications
	}

	// List replace
	if other.ComposeProfiles != nil {
		base.ComposeProfiles = other.ComposeProfiles
	}
	if other.ComposeServices != nil {
		base.ComposeServices = other.ComposeServices
	}
	if other.HealthChecks != nil {
		base.HealthChecks = other.HealthChecks
	}
	if other.EndpointTags != nil {
		base.EndpointTags = other.EndpointTags
	}
	if other.Stack != nil {
		base.Stack = other.Stack
	}

	// Map merge
	base.Environment = mergeStringMap(base.Environment, other.Environment)

	return base
}

// mergeApplicationConfig deep-merges other into base.
func mergeApplicationConfig(base, other *ApplicationConfig) *ApplicationConfig {
	if base == nil {
		return other
	}
	if other == nil {
		return base
	}

	if other.Description != "" {
		base.Description = other.Description
	}
	if other.Dir != "" {
		base.Dir = other.Dir
	}

	// AppExecPaths: merge each variant
	if other.Run.Native != "" {
		base.Run.Native = other.Run.Native
	}
	if other.Run.Docker.Service != "" || other.Run.Docker.Command != "" {
		base.Run.Docker = other.Run.Docker
	}
	if other.Build.Native != "" {
		base.Build.Native = other.Build.Native
	}
	if other.Build.Docker.Service != "" || other.Build.Docker.Command != "" {
		base.Build.Docker = other.Build.Docker
	}
	if other.Dev.Native != "" {
		base.Dev.Native = other.Dev.Native
	}
	if other.Dev.Docker.Service != "" || other.Dev.Docker.Command != "" {
		base.Dev.Docker = other.Dev.Docker
	}

	if other.Health != nil {
		base.Health = other.Health
	}
	if other.Port != 0 {
		base.Port = other.Port
	}

	// List replace
	if other.Tags != nil {
		base.Tags = other.Tags
	}
	if other.DependsOn != nil {
		base.DependsOn = other.DependsOn
	}

	// Map merge
	base.Environment = mergeStringMap(base.Environment, other.Environment)

	// Variants: merge per entry
	if other.Variants != nil {
		if base.Variants == nil {
			base.Variants = make(map[string]*AppVariant)
		}
		for k, v := range other.Variants {
			if existing, ok := base.Variants[k]; ok {
				base.Variants[k] = mergeAppVariant(existing, v)
			} else {
				base.Variants[k] = v
			}
		}
	}

	return base
}

// mergeAppVariant deep-merges other into base.
func mergeAppVariant(base, other *AppVariant) *AppVariant {
	if base == nil {
		return other
	}
	if other == nil {
		return base
	}

	if other.Description != "" {
		base.Description = other.Description
	}
	if other.Port != 0 {
		base.Port = other.Port
	}
	if other.Run.Native != "" {
		base.Run.Native = other.Run.Native
	}
	if other.Run.Docker.Service != "" || other.Run.Docker.Command != "" {
		base.Run.Docker = other.Run.Docker
	}
	if other.Build.Native != "" {
		base.Build.Native = other.Build.Native
	}
	if other.Build.Docker.Service != "" || other.Build.Docker.Command != "" {
		base.Build.Docker = other.Build.Docker
	}
	if other.Dev.Native != "" {
		base.Dev.Native = other.Dev.Native
	}
	if other.Dev.Docker.Service != "" || other.Dev.Docker.Command != "" {
		base.Dev.Docker = other.Dev.Docker
	}
	if other.Health != nil {
		base.Health = other.Health
	}
	base.Environment = mergeStringMap(base.Environment, other.Environment)

	return base
}

// mergeEnvironmentProfile deep-merges other into base.
func mergeEnvironmentProfile(base, other EnvironmentProfile) EnvironmentProfile {
	if other.Description != "" {
		base.Description = other.Description
	}

	// List replace
	if other.Stack != nil {
		base.Stack = other.Stack
	}

	// Map merge
	base.Environment = mergeStringMap(base.Environment, other.Environment)

	// StackOverrides map merge (deep)
	if len(other.StackOverrides) > 0 {
		if base.StackOverrides == nil {
			base.StackOverrides = make(map[string]*LifecycleEntry)
		}
		for k, v := range other.StackOverrides {
			if existing, ok := base.StackOverrides[k]; ok {
				merged, err := MergeLifecycleEntry(existing, v)
				if err != nil {
					// plugin type conflict in override is a config error;
					// caller (mergeFrom) propagates it via mergeEnvironmentProfile
					// but this function cannot return errors. Log and skip.
					fmt.Printf("[warn] stack_override %q: %v\n", k, err)
					continue
				}
				base.StackOverrides[k] = merged
			} else {
				base.StackOverrides[k] = v
			}
		}
	}

	return base
}

func mergePlanConfig(base, other *PlanConfig) *PlanConfig {
	if base == nil {
		return other
	}
	if other == nil {
		return base
	}
	if other.Description != "" {
		base.Description = other.Description
	}
	if other.Environment != "" {
		base.Environment = other.Environment
	}
	if other.Site != "" {
		base.Site = other.Site
	}
	base.Vars = mergeStringMap(base.Vars, other.Vars)
	if other.Entries != nil {
		base.Entries = other.Entries
	}
	return base
}

func mergeSiteConfig(base, other *SiteConfig) *SiteConfig {
	if base == nil {
		return other
	}
	if other == nil {
		return base
	}
	if other.Description != "" {
		base.Description = other.Description
	}
	base.Vars = mergeStringMap(base.Vars, other.Vars)
	if other.EntryOverrides != nil {
		if base.EntryOverrides == nil {
			base.EntryOverrides = make(map[string]*SiteEntryOverride)
		}
		for k, v := range other.EntryOverrides {
			if existing, ok := base.EntryOverrides[k]; ok {
				base.EntryOverrides[k] = mergeSiteEntryOverride(existing, v)
			} else {
				base.EntryOverrides[k] = v
			}
		}
	}
	return base
}

func mergeSiteEntryOverride(base, other *SiteEntryOverride) *SiteEntryOverride {
	if base == nil {
		return other
	}
	if other == nil {
		return base
	}
	if other.Runner != "" {
		base.Runner = other.Runner
	}
	base.Vars = mergeStringMap(base.Vars, other.Vars)
	return base
}

func mergeRunnerConfig(base, other any) any {
	baseMap, baseOk := base.(map[string]any)
	otherMap, otherOk := other.(map[string]any)
	if baseOk && otherOk {
		result := make(map[string]any, len(baseMap))
		for k, v := range baseMap {
			result[k] = v
		}
		for k, v := range otherMap {
			result[k] = v
		}
		return result
	}
	return other
}

func mergeSubprojectConfig(base, other SubprojectConfig) SubprojectConfig {
	if other.Path != "" {
		base.Path = other.Path
	}
	if other.ExcludeTags != nil {
		base.ExcludeTags = other.ExcludeTags
	}
	if other.Import != nil {
		base.Import = other.Import
	}
	return base
}
