package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const legacyInfraOrderBase = 10000

// migrateInfraToStack folds the deprecated top-level `infra:` map into `stack:`
// as source-backed compose entries (TASK-051). Each infra service becomes a
// stack entry tagged "infra" with a `source:` (git or local path) and the
// compose runner, so `dva up`/plans manage it like any other entry.
//
// Legacy precedence is preserved: when both git and path are set, path wins
// (matching the old infraServiceLocation behavior). A name that already exists
// in `stack:` is a hard conflict. Returns the migrated names for the caller to
// surface as a deprecation notice.
func (c *Config) migrateInfraToStack() ([]string, error) {
	if len(c.Infra) == 0 {
		return nil, nil
	}
	if c.Stack == nil {
		c.Stack = make(map[string]*LifecycleEntry)
	}

	names := make([]string, 0, len(c.Infra))
	for name := range c.Infra {
		names = append(names, name)
	}
	sort.Strings(names)

	for i, name := range names {
		if _, exists := c.Stack[name]; exists {
			return nil, fmt.Errorf("infra service %q conflicts with an existing stack entry of the same name; rename one (infra: is deprecated, see TASK-051)", name)
		}
		v := c.Infra[name]

		src := &SourceConfig{}
		if strings.TrimSpace(v.Path) != "" {
			src.Path = v.Path
		} else {
			src.Git = v.Git
			src.Ref = v.Ref
		}

		c.Stack[name] = &LifecycleEntry{
			Name: name,
			// Keep migrated legacy entries in a deterministic high order range.
			Order:         legacyInfraOrderBase + i,
			Tags:          []string{"infra"},
			Plugin:        "compose",
			DefaultRunner: "compose",
			// Emit the runners shape, not the Compose field: this migration exists to
			// move users off a deprecated form, so it must not land them on another one
			// that LifecycleEntry.rejectLegacyComposeShape refuses.
			Runners: map[string]any{"compose": &ComposePluginConfig{}},
			Source:  src,
		}
	}
	return names, nil
}

// validateEntrySource enforces the v1 source contract on every config load,
// not only when `dva config validate` is invoked.
func validateEntrySource(entryName string, entry *LifecycleEntry, cfgDir string) error {
	if entry.Source == nil {
		return nil
	}
	if err := entry.Source.Validate(); err != nil {
		return fmt.Errorf("stack.%s.%w", entryName, err)
	}
	if _, err := SourceDir(entry.Source, entryName, cfgDir); err != nil {
		return fmt.Errorf("stack.%s.%w", entryName, err)
	}
	for runnerName := range entry.Runners {
		if runner := normalizeRunnerName(runnerName); runner != "compose" {
			return fmt.Errorf("stack.%s.source: v1 does not support runner %q; only compose is supported", entryName, runnerName)
		}
	}
	plugin := normalizeRunnerName(entry.DefaultRunner)
	if plugin == "" {
		plugin = normalizeRunnerName(entry.DetectPlugin())
	}
	if plugin == "" && len(entry.Runners) == 1 {
		if _, ok := entry.Runners["compose"]; ok {
			plugin = "compose"
		}
	}
	if plugin != "compose" {
		return fmt.Errorf("stack.%s.source: v1 supports only the compose runner (resolved runner: %q)", entryName, plugin)
	}
	return nil
}

// SourceDir returns the base directory a stack entry's source resolves to.
//
//   - git sources are cloned into a cache dir: <cfgDir>/.sb/dva/sources/<name>
//   - path sources resolve relative to cfgDir (absolute paths pass through)
//
// A path source must never resolve to cfgDir itself; operating docker compose
// on the project directory is refused. (TASK-051; mirrors legacy infra guard)
func SourceDir(src *SourceConfig, entryName, cfgDir string) (string, error) {
	if src == nil {
		return "", fmt.Errorf("entry %q has no source", entryName)
	}
	if src.IsGit() {
		return filepath.Join(cfgDir, DotDirName, "sources", entryName), nil
	}

	path := strings.TrimSpace(src.Path)
	if path == "" {
		return "", fmt.Errorf("source for %q has neither git nor path", entryName)
	}

	location := path
	if !filepath.IsAbs(location) {
		location = filepath.Join(cfgDir, location)
	}
	if filepath.Clean(location) == filepath.Clean(cfgDir) {
		return "", fmt.Errorf(
			"source for %q resolves to the project directory (%s); refuse to operate on the config directory",
			entryName, location,
		)
	}
	return location, nil
}
