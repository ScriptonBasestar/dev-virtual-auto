package config

import "sort"

// SortedStack returns stack entries sorted by Order with Name populated.
func (c *Config) SortedStack() []LifecycleEntry {
	entries := make([]LifecycleEntry, 0, len(c.Stack))
	for name, e := range c.Stack {
		entry := *e
		entry.Name = name
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Order < entries[j].Order
	})
	return entries
}

// PrimaryComposeEntry returns the lifecycle entry with lowest order that has a compose config.
// Name is already populated from map keys during Load().
func (c *Config) PrimaryComposeEntry() *LifecycleEntry {
	var best *LifecycleEntry
	bestOrder := int(^uint(0) >> 1) // max int
	for _, e := range c.Stack {
		if e.Compose != nil && (best == nil || e.Order < bestOrder) {
			best = e
			bestOrder = e.Order
		}
	}
	return best
}

// PrimaryComposeConfig returns the ComposePluginConfig from the primary compose lifecycle entry.
func (c *Config) PrimaryComposeConfig() *ComposePluginConfig {
	if e := c.PrimaryComposeEntry(); e != nil {
		return e.Compose
	}
	return nil
}

// AllComposeFiles aggregates compose files from all lifecycle entries.
func (c *Config) AllComposeFiles() []string {
	var files []string
	for _, e := range c.Stack {
		if e.Compose != nil {
			files = append(files, e.Compose.Files...)
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
func (c *Config) PrimaryKubectlConfig() *KubectlPluginConfig {
	var best *LifecycleEntry
	bestOrder := int(^uint(0) >> 1) // max int
	for _, e := range c.Stack {
		if e.Kubectl != nil && (best == nil || e.Order < bestOrder) {
			best = e
			bestOrder = e.Order
		}
	}
	if best != nil {
		return best.Kubectl
	}
	return nil
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
