package config

import (
	"fmt"
	"path/filepath"
)

// LoadSubprojects loads dva.yml from each sub-project path.
func LoadSubprojects(parentDir string, subs map[string]SubprojectConfig) (map[string]*Config, error) {
	result := make(map[string]*Config, len(subs))
	for name, sub := range subs {
		subPath := sub.Path
		if !filepath.IsAbs(subPath) {
			subPath = filepath.Join(parentDir, subPath)
		}
		subCfgPath := filepath.Join(subPath, "dva.yml")
		cfg, err := loadFile(subCfgPath)
		if err != nil {
			return nil, fmt.Errorf("loading subproject %q (%s): %w", name, subCfgPath, err)
		}
		cfg.filePath = subCfgPath

		// Load sub-project modules (same .dva/*.yml pattern)
		if len(cfg.Modules) > 0 {
			modulesDir := filepath.Join(subPath, ".dva")
			for _, mod := range cfg.Modules {
				modFile := filepath.Join(modulesDir, mod+".yml")
				modCfg, err := loadFile(modFile)
				if err != nil {
					return nil, fmt.Errorf("loading subproject %q module %q: %w", name, mod, err)
				}
				cfg.mergeFrom(modCfg)
			}
		}

		// Load sub-project override
		overrideFile := filepath.Join(subPath, "dva.override.yml")
		if overCfg, err := loadFile(overrideFile); err == nil {
			cfg.mergeFrom(overCfg)
		}

		result[name] = cfg
	}
	return result, nil
}

// HasTag checks if the compose-level default tags contain the given tag.
func (c *Config) HasTag(tag string) bool {
	for _, t := range c.Compose.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// FilterInteractions returns interaction commands excluding those matching any of the given tags.
func (c *Config) FilterInteractions(excludeTags []string) map[string]*InteractionCommand {
	if len(excludeTags) == 0 {
		return c.Interaction
	}
	exclude := toSet(excludeTags)
	result := make(map[string]*InteractionCommand, len(c.Interaction))
	for name, cmd := range c.Interaction {
		if !hasAnyTag(cmd.Tags, exclude) {
			result[name] = cmd
		}
	}
	return result
}

// GetComposeServicesExcluding returns compose service names that do NOT have any of the excluded tags.
// Services without explicit tags inherit the compose-level default tags.
func (c *Config) GetComposeServicesExcluding(excludeTags []string) []string {
	if len(excludeTags) == 0 || len(c.Compose.Services) == 0 {
		return nil
	}
	exclude := toSet(excludeTags)
	defaults := c.Compose.Tags

	var included []string
	var excluded []string
	for svcName, svcCfg := range c.Compose.Services {
		tags := svcCfg.Tags
		if len(tags) == 0 {
			tags = defaults
		}
		if hasAnyTag(tags, exclude) {
			excluded = append(excluded, svcName)
		} else {
			included = append(included, svcName)
		}
	}
	_ = excluded // excluded services are simply not returned
	return included
}

// GetComposeServicesIncluding returns compose service names that HAVE ANY of the included tags.
// Services without explicit tags inherit the compose-level default tags.
func (c *Config) GetComposeServicesIncluding(includeTags []string) []string {
	if len(includeTags) == 0 || len(c.Compose.Services) == 0 {
		return nil
	}
	include := toSet(includeTags)
	defaults := c.Compose.Tags

	var included []string
	for svcName, svcCfg := range c.Compose.Services {
		tags := svcCfg.Tags
		if len(tags) == 0 {
			tags = defaults
		}
		if hasAnyTag(tags, include) {
			included = append(included, svcName)
		}
	}
	return included
}

// GetExcludedComposeServices returns compose service names that HAVE any of the excluded tags.
func (c *Config) GetExcludedComposeServices(excludeTags []string) []string {
	if len(excludeTags) == 0 || len(c.Compose.Services) == 0 {
		return nil
	}
	exclude := toSet(excludeTags)
	defaults := c.Compose.Tags

	var result []string
	for svcName, svcCfg := range c.Compose.Services {
		tags := svcCfg.Tags
		if len(tags) == 0 {
			tags = defaults
		}
		if hasAnyTag(tags, exclude) {
			result = append(result, svcName)
		}
	}
	return result
}

// hasAnyTag checks if any of the tags exist in the exclusion set.
func hasAnyTag(tags []string, exclude map[string]bool) bool {
	for _, t := range tags {
		if exclude[t] {
			return true
		}
	}
	return false
}

// toSet converts a string slice to a set (map).
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
