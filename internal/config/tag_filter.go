package config

import "slices"

func (c *Config) HasTag(tag string) bool {
	return slices.Contains(c.ComposeTags(), tag)
}

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

func (c *Config) GetComposeServicesExcluding(excludeTags []string) []string {
	services := c.ComposeServices()
	if len(excludeTags) == 0 || len(services) == 0 {
		return nil
	}
	exclude := toSet(excludeTags)
	defaults := c.ComposeTags()

	var included []string
	for svcName, svcCfg := range services {
		tags := svcCfg.Tags
		if len(tags) == 0 {
			tags = defaults
		}
		if !hasAnyTag(tags, exclude) {
			included = append(included, svcName)
		}
	}
	return included
}

func (c *Config) GetComposeServicesIncluding(includeTags []string) []string {
	services := c.ComposeServices()
	if len(includeTags) == 0 || len(services) == 0 {
		return nil
	}
	include := toSet(includeTags)
	defaults := c.ComposeTags()

	var included []string
	for svcName, svcCfg := range services {
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

func (c *Config) GetExcludedComposeServices(excludeTags []string) []string {
	services := c.ComposeServices()
	if len(excludeTags) == 0 || len(services) == 0 {
		return nil
	}
	exclude := toSet(excludeTags)
	defaults := c.ComposeTags()

	var result []string
	for svcName, svcCfg := range services {
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

func hasAnyTag(tags []string, exclude map[string]bool) bool {
	for _, t := range tags {
		if exclude[t] {
			return true
		}
	}
	return false
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
