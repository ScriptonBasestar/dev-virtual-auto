package config

// PrimaryComposeEntry returns the first lifecycle entry with plugin=="compose".
func (c *Config) PrimaryComposeEntry() *LifecycleEntry {
	for i := range c.Lifecycle {
		if c.Lifecycle[i].Plugin == "compose" && c.Lifecycle[i].Compose != nil {
			return &c.Lifecycle[i]
		}
	}
	return nil
}

// PrimaryComposeConfig returns the ComposePluginConfig from the first compose lifecycle entry.
func (c *Config) PrimaryComposeConfig() *ComposePluginConfig {
	if e := c.PrimaryComposeEntry(); e != nil {
		return e.Compose
	}
	return nil
}

// AllComposeFiles aggregates compose files from all lifecycle entries.
func (c *Config) AllComposeFiles() []string {
	var files []string
	for _, e := range c.Lifecycle {
		if e.Plugin == "compose" && e.Compose != nil {
			files = append(files, e.Compose.Files...)
		}
	}
	return files
}

// ComposeProjectName returns the project_name from the first compose lifecycle entry.
func (c *Config) ComposeProjectName() string {
	if cc := c.PrimaryComposeConfig(); cc != nil {
		return cc.ProjectName
	}
	return ""
}

// ComposeCommand returns the command from the first compose lifecycle entry.
func (c *Config) ComposeCommand() string {
	if cc := c.PrimaryComposeConfig(); cc != nil {
		return cc.Command
	}
	return ""
}

// PrimaryKubectlConfig returns the KubectlPluginConfig from the first kubectl lifecycle entry.
func (c *Config) PrimaryKubectlConfig() *KubectlPluginConfig {
	for _, e := range c.Lifecycle {
		if e.Plugin == "kubectl" && e.Kubectl != nil {
			return e.Kubectl
		}
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
