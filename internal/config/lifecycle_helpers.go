package config

import "sort"

func (e *LifecycleEntry) ComposeConfig() *ComposePluginConfig {
	if e == nil {
		return nil
	}
	if e.Compose != nil {
		return e.Compose
	}
	for name, runnerCfg := range e.Runners {
		if normalizeRunnerName(name) != "compose" {
			continue
		}
		if cfg, ok := runnerCfg.(*ComposePluginConfig); ok {
			return cfg
		}
	}
	return nil
}

// applyRunnerConfig assigns a runner config to its typed plugin field.
func (e *LifecycleEntry) applyRunnerConfig(cfg any) bool {
	switch c := cfg.(type) {
	case *ComposePluginConfig:
		e.Compose = c
	case *ProcessPluginConfig:
		e.Process = c
	case *NativeRunnerConfig:
		// Alias runners.native → process plugin (TASK-050 Option A).
		e.Process = &ProcessPluginConfig{Command: c.Run, Dir: c.Dir}
	case *ScriptPluginConfig:
		e.Script = c
	case *DockerPluginConfig:
		e.Docker = c
	case *KubectlPluginConfig:
		e.Kubectl = c
	case *HelmPluginConfig:
		e.Helm = c
	case *KustomizePluginConfig:
		e.Kustomize = c
	case *TiltPluginConfig:
		e.Tilt = c
	case *SkaffoldPluginConfig:
		e.Skaffold = c
	case *PodmanComposePluginConfig:
		e.PodmanCompose = c
	case *VagrantPluginConfig:
		e.Vagrant = c
	case *SAMPluginConfig:
		e.SAM = c
	case *ServerlessPluginConfig:
		e.Serverless = c
	case *MultipassPluginConfig:
		e.Multipass = c
	default:
		return false
	}
	return true
}

// resolveRunnerPlugin backfills Plugin and its typed config from the runners shape.
func (e *LifecycleEntry) resolveRunnerPlugin() {
	if e == nil || e.Plugin != "" {
		return
	}
	name := e.runnerPluginName()
	if name == "" {
		if e.ComposeConfig() != nil {
			e.Plugin = "compose"
		}
		return
	}
	cfg, err := e.GetRunnerConfig(name)
	if err != nil {
		return
	}
	if !e.applyRunnerConfig(cfg) {
		return
	}
	if name == "native" {
		e.Plugin = "process"
		return
	}
	e.Plugin = name
}

// lessByOrderName is the sequence every stack listing in this file uses: by declared Order, then
// alphabetically by Name.
//
// The Name tiebreak is not cosmetic. Stack entries are collected from a map, so without it two
// entries sharing an Order — including the common case where no entry declares `order:` at all —
// come out in Go's randomized map-iteration order, and every caller walks a different sequence each
// run. Arbitrary-but-stable is what makes an ordering bug reproducible.
func lessByOrderName(a, b *LifecycleEntry) bool {
	if a.Order != b.Order {
		return a.Order < b.Order
	}
	return a.Name < b.Name
}

// SortedStack returns stack entries sorted by Order then Name, with Name populated.
func (c *Config) SortedStack() []LifecycleEntry {
	entries := make([]LifecycleEntry, 0, len(c.Stack))
	for name, e := range c.Stack {
		entry := *e
		entry.Name = name
		entry.resolveRunnerPlugin()
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return lessByOrderName(&entries[i], &entries[j])
	})
	return entries
}

// PrimaryComposeEntry returns the lifecycle entry with lowest order that has a compose config.
// Name is already populated from map keys during Load().
// Tiebreaker: alphabetically first Name when Order values are equal.
func (c *Config) PrimaryComposeEntry() *LifecycleEntry {
	var best *LifecycleEntry
	for _, e := range c.Stack {
		if e.ComposeConfig() == nil {
			continue
		}
		if best == nil || lessByOrderName(e, best) {
			best = e
		}
	}
	return best
}

// PrimaryComposeConfig returns the ComposePluginConfig from the primary compose lifecycle entry.
func (c *Config) PrimaryComposeConfig() *ComposePluginConfig {
	if e := c.PrimaryComposeEntry(); e != nil {
		return e.ComposeConfig()
	}
	return nil
}

// AllEnvFiles aggregates env file paths from the config.
func (c *Config) AllEnvFiles() []string {
	var paths []string
	for _, cfg := range normalizeEnvFileConfig(c.EnvFile) {
		paths = append(paths, cfg.Path)
	}
	return paths
}

// AllComposeFiles aggregates compose files from all lifecycle entries.
func (c *Config) AllComposeFiles() []string {
	var files []string
	for _, e := range c.Stack {
		if cc := e.ComposeConfig(); cc != nil {
			files = append(files, cc.Files...)
		}
	}
	return files
}

// ComposeProjectName returns the project_name from the primary compose lifecycle entry.
func (c *Config) ComposeProjectName() string {
	if cc := c.PrimaryComposeConfig(); cc != nil {
		return cc.ProjectName
	}
	return ""
}

// ComposeCommand returns the command from the primary compose lifecycle entry.
func (c *Config) ComposeCommand() string {
	if cc := c.PrimaryComposeConfig(); cc != nil {
		return cc.Command
	}
	return ""
}

// PrimaryKubectlConfig returns the KubectlPluginConfig from the kubectl lifecycle entry with lowest order.
// Tiebreaker: alphabetically first Name when Order values are equal.
func (c *Config) PrimaryKubectlConfig() *KubectlPluginConfig {
	var best *LifecycleEntry
	for _, e := range c.Stack {
		if e.Kubectl == nil {
			continue
		}
		if best == nil || lessByOrderName(e, best) {
			best = e
		}
	}
	if best != nil {
		return best.Kubectl
	}
	return nil
}

// ComposeEntries returns all stack entries with a compose driver, sorted by order.
func (c *Config) ComposeEntries() []*LifecycleEntry {
	var entries []*LifecycleEntry
	for _, e := range c.Stack {
		if e.ComposeConfig() != nil {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return lessByOrderName(entries[i], entries[j])
	})
	return entries
}

// KubectlEntries returns all stack entries with a kubectl driver, sorted by order.
func (c *Config) KubectlEntries() []*LifecycleEntry {
	var entries []*LifecycleEntry
	for _, e := range c.Stack {
		if e.Kubectl != nil {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return lessByOrderName(entries[i], entries[j])
	})
	return entries
}

// FindStackEntry finds a stack entry by name.
func (c *Config) FindStackEntry(name string) *LifecycleEntry {
	return c.Stack[name]
}

// ComposeServices returns the services map from the primary compose config.
func (c *Config) ComposeServices() map[string]ServiceTagConfig {
	if cc := c.PrimaryComposeConfig(); cc != nil {
		return cc.Services
	}
	return nil
}

// ComposeTags returns the default tags from the primary compose config.
func (c *Config) ComposeTags() []string {
	if cc := c.PrimaryComposeConfig(); cc != nil {
		return cc.Tags
	}
	return nil
}
