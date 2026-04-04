package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// LoadSubprojects loads dva.yml from each sub-project path.
func LoadSubprojects(parentDir string, subs map[string]SubprojectConfig) (map[string]*Config, error) {
	result := make(map[string]*Config, len(subs))
	for name, sub := range subs {
		subPath := sub.Path
		if !filepath.IsAbs(subPath) {
			subPath = filepath.Join(parentDir, subPath)
		}
		subCfgPath := filepath.Join(subPath, FileName)
		cfg, err := loadFile(subCfgPath)
		if err != nil {
			return nil, fmt.Errorf("loading subproject %q (%s): %w", name, subCfgPath, err)
		}
		cfg.filePath = subCfgPath

		// Load sub-project modules (same .sb/dva/*.yml pattern)
		if len(cfg.Modules) > 0 {
			modulesDir := filepath.Join(subPath, DotDirName)
			for _, mod := range cfg.Modules {
				modFile := filepath.Join(modulesDir, mod+".yml")
				modCfg, err := loadFile(modFile)
				if err != nil {
					return nil, fmt.Errorf("loading subproject %q module %q: %w", name, mod, err)
				}
				if err := cfg.mergeFrom(modCfg); err != nil {
					return nil, fmt.Errorf("merging subproject %q module %q: %w", name, mod, err)
				}
			}
		}

		// Load sub-project override
		overrideFile := filepath.Join(subPath, "dva.override.yml")
		if overCfg, err := loadFile(overrideFile); err == nil {
			if err := cfg.mergeFrom(overCfg); err != nil {
				return nil, fmt.Errorf("merging subproject %q override: %w", name, err)
			}
		}

		result[name] = cfg
	}
	return result, nil
}

func resolveSubprojectImports(cfg *Config) error {
	if len(cfg.Subprojects) == 0 {
		return nil
	}

	parentDir := cfg.FileDir()
	subCfgs, err := LoadSubprojects(parentDir, cfg.Subprojects)
	if err != nil {
		return err
	}

	for subprojectName, subproject := range cfg.Subprojects {
		if subproject.Import == nil {
			continue
		}

		subCfg, ok := subCfgs[subprojectName]
		if !ok {
			return fmt.Errorf("subproject %q not loaded", subprojectName)
		}

		subprojectPath := subproject.Path
		if !filepath.IsAbs(subprojectPath) {
			subprojectPath = filepath.Join(parentDir, subprojectPath)
		}

		for _, entry := range subproject.Import.Plans {
			name := strings.TrimSpace(entry.Name)
			if name == "" {
				return fmt.Errorf("subproject %q plan import name is required", subprojectName)
			}
			plan, ok := subCfg.Plans[name]
			if !ok {
				return fmt.Errorf("subproject %q plan %q not found", subprojectName, name)
			}

			canonicalName := subprojectName + "/" + name
			if _, exists := cfg.Plans[canonicalName]; exists {
				return fmt.Errorf("plan name collision: %q already exists", canonicalName)
			}

			importedPlan := cloneImportedPlan(plan, subprojectPath)
			cfg.Plans[canonicalName] = importedPlan

			alias := strings.TrimSpace(entry.As)
			if alias != "" && alias != canonicalName {
				if _, exists := cfg.Plans[alias]; exists {
					return fmt.Errorf("plan alias collision: %q already exists", alias)
				}
				cfg.Plans[alias] = importedPlan
			}
		}

		for _, entry := range subproject.Import.Interactions {
			name := strings.TrimSpace(entry.Name)
			if name == "" {
				return fmt.Errorf("subproject %q interaction import name is required", subprojectName)
			}
			interaction, ok := subCfg.Interaction[name]
			if !ok {
				return fmt.Errorf("subproject %q interaction %q not found", subprojectName, name)
			}

			canonicalName := subprojectName + "/" + name
			if _, exists := cfg.Interaction[canonicalName]; exists {
				return fmt.Errorf("interaction name collision: %q already exists", canonicalName)
			}

			importedInteraction := cloneImportedInteraction(interaction, subprojectPath)
			cfg.Interaction[canonicalName] = importedInteraction

			alias := strings.TrimSpace(entry.As)
			if alias != "" && alias != canonicalName {
				if _, exists := cfg.Interaction[alias]; exists {
					return fmt.Errorf("interaction alias collision: %q already exists", alias)
				}
				cfg.Interaction[alias] = importedInteraction
			}
		}

		for _, entry := range subproject.Import.Provision {
			name := strings.TrimSpace(entry.Name)
			if name == "" {
				return fmt.Errorf("subproject %q provision import name is required", subprojectName)
			}
			profile, ok := subCfg.Provision.Profiles[name]
			if !ok {
				return fmt.Errorf("subproject %q provision profile %q not found", subprojectName, name)
			}

			canonicalName := subprojectName + "/" + name
			if _, exists := cfg.Provision.Profiles[canonicalName]; exists {
				return fmt.Errorf("provision profile name collision: %q already exists", canonicalName)
			}

			importedProfile := append([]ProvisionItem(nil), profile...)
			cfg.Provision.Profiles[canonicalName] = importedProfile

			alias := strings.TrimSpace(entry.As)
			if alias != "" && alias != canonicalName {
				if _, exists := cfg.Provision.Profiles[alias]; exists {
					return fmt.Errorf("provision profile alias collision: %q already exists", alias)
				}
				cfg.Provision.Profiles[alias] = importedProfile
			}
		}
	}

	return nil
}

func cloneImportedPlan(plan *PlanConfig, subprojectPath string) *PlanConfig {
	if plan == nil {
		return nil
	}
	clone := *plan
	clone.SubprojectPath = subprojectPath
	if plan.Vars != nil {
		clone.Vars = copyStringMap(plan.Vars)
	}
	if plan.Entries != nil {
		clone.Entries = make([]PlanEntry, len(plan.Entries))
		copy(clone.Entries, plan.Entries)
		for i := range clone.Entries {
			if clone.Entries[i].Vars != nil {
				clone.Entries[i].Vars = copyStringMap(clone.Entries[i].Vars)
			}
			if clone.Entries[i].DependsOn != nil {
				clone.Entries[i].DependsOn = append([]string(nil), clone.Entries[i].DependsOn...)
			}
			if clone.Entries[i].Services != nil {
				clone.Entries[i].Services = append([]string(nil), clone.Entries[i].Services...)
			}
		}
	}
	return &clone
}

func cloneImportedInteraction(command *InteractionCommand, subprojectPath string) *InteractionCommand {
	if command == nil {
		return nil
	}

	clone := *command
	clone.SubprojectPath = subprojectPath
	if command.Environment != nil {
		clone.Environment = copyStringMap(command.Environment)
	}
	if command.CommandLines != nil {
		clone.CommandLines = append([]string(nil), command.CommandLines...)
	}
	if command.Tags != nil {
		clone.Tags = append([]string(nil), command.Tags...)
	}
	if command.Steps != nil {
		clone.Steps = append([]ProvisionItem(nil), command.Steps...)
	}
	if command.Before != nil {
		clone.Before = append([]ProvisionItem(nil), command.Before...)
	}
	if command.Replace != nil {
		clone.Replace = append([]ProvisionItem(nil), command.Replace...)
	}
	if command.After != nil {
		clone.After = append([]ProvisionItem(nil), command.After...)
	}

	if command.Subcommands != nil {
		clone.Subcommands = make(map[string]*InteractionCommand, len(command.Subcommands))
		for name, sub := range command.Subcommands {
			clone.Subcommands[name] = cloneImportedInteraction(sub, subprojectPath)
		}
	}

	return &clone
}

// HasTag checks if the compose-level default tags contain the given tag.
func (c *Config) HasTag(tag string) bool {
	for _, t := range c.ComposeTags() {
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

// GetComposeServicesIncluding returns compose service names that HAVE ANY of the included tags.
// Services without explicit tags inherit the compose-level default tags.
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

// GetExcludedComposeServices returns compose service names that HAVE any of the excluded tags.
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
