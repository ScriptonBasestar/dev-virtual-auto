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

// SortedStack returns stack entries sorted by Order with Name populated.
func (c *Config) SortedStack() []LifecycleEntry {
	entries := make([]LifecycleEntry, 0, len(c.Stack))
	for name, e := range c.Stack {
		entry := *e
		entry.Name = name
		if entry.Plugin == "" && entry.ComposeConfig() != nil {
			entry.Plugin = "compose"
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Order < entries[j].Order
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
		if best == nil || e.Order < best.Order || (e.Order == best.Order && e.Name < best.Name) {
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
		if best == nil || e.Order < best.Order || (e.Order == best.Order && e.Name < best.Name) {
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
		if entries[i].Order != entries[j].Order {
			return entries[i].Order < entries[j].Order
		}
		return entries[i].Name < entries[j].Name
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
		if entries[i].Order != entries[j].Order {
			return entries[i].Order < entries[j].Order
		}
		return entries[i].Name < entries[j].Name
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
