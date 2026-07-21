package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

//go:embed schema.json
var embeddedSchema embed.FS

// Validate validates the dva.yml against the JSON schema.
func (c *Config) Validate() error {
	if c.filePath == "" {
		return fmt.Errorf("config file path is not set")
	}

	// Load schema - try embedded first, then file fallback
	schemaBytes, err := embeddedSchema.ReadFile("schema.json")
	if err != nil {
		// Fallback: look for schema.json next to binary or in project root
		schemaBytes, err = os.ReadFile("schema.json")
		if err != nil {
			return fmt.Errorf("schema file not found: %w", err)
		}
	}

	// Load and convert YAML config to JSON for validation
	yamlBytes, err := os.ReadFile(c.filePath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var yamlData any
	if err := yaml.Unmarshal(yamlBytes, &yamlData); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}

	jsonData := convertYAMLToJSON(yamlData)
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return fmt.Errorf("converting config to JSON: %w", err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	docLoader := gojsonschema.NewBytesLoader(jsonBytes)

	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, fmt.Sprintf("  - %s: %s", desc.Field(), desc.Description()))
		}
		return fmt.Errorf("schema validation failed in dva.yml:\n%s", strings.Join(errs, "\n"))
	}

	// Check for reserved command conflicts in interaction section
	if conflicts := ValidateReservedCommands(c.Interaction); len(conflicts) > 0 {
		var errs []string
		for _, conflict := range conflicts {
			var hint string
			if strings.Contains(conflict.Name, ":") {
				prefix := conflict.Name[:strings.Index(conflict.Name, ":")]
				hint = fmt.Sprintf("— namespace prefix '%s' is a reserved DVA command; use a different prefix", prefix)
			} else if IsHookableCommand(conflict.Name) {
				hint = "— use before/replace/after to extend it instead"
			} else {
				hint = "and will be shadowed"
			}
			errs = append(errs, fmt.Sprintf(
				"  - interaction.%s: '%s' is a reserved DVA command %s",
				conflict.Name, conflict.Name, hint,
			))
		}
		return fmt.Errorf("reserved command conflict in dva.yml:\n%s", strings.Join(errs, "\n"))
	}

	// Validate hook fields are only used on hookable commands
	for name, cmd := range c.Interaction {
		if cmd.HasHooks() && !IsHookableCommand(name) {
			return fmt.Errorf("interaction.%s: before/replace/after hooks are only supported on hookable commands (up, down, stop, restart, build, clean, logs)", name)
		}
	}

	for entryName, entry := range c.Stack {
		for runnerName := range entry.Runners {
			if strings.TrimSpace(runnerName) != runnerName {
				return fmt.Errorf("stack.%s.runners.%q: runner names must not include leading or trailing whitespace", entryName, runnerName)
			}
		}
		if err := validateEntrySource(entryName, entry, c.FileDir()); err != nil {
			return err
		}
	}

	// Validate default_mode references an existing mode
	if c.DefaultMode != "" {
		if _, ok := c.Modes[c.DefaultMode]; !ok {
			available := make([]string, 0, len(c.Modes))
			for k := range c.Modes {
				available = append(available, k)
			}
			if len(available) == 0 {
				return fmt.Errorf("default_mode '%s' is set but no modes are defined", c.DefaultMode)
			}
			return fmt.Errorf("default_mode '%s' not found in modes. Available: %s", c.DefaultMode, strings.Join(available, ", "))
		}
	}

	// Validate default_plan references an existing plan
	if c.DefaultPlanName != "" {
		if _, ok := c.Plans[c.DefaultPlanName]; !ok {
			available := make([]string, 0, len(c.Plans))
			for k := range c.Plans {
				available = append(available, k)
			}
			if len(available) == 0 {
				return fmt.Errorf("default_plan '%s' is set but no plans are defined", c.DefaultPlanName)
			}
			return fmt.Errorf("default_plan '%s' not found in plans. Available: %s", c.DefaultPlanName, strings.Join(available, ", "))
		}
	}

	return nil
}

// ComposeNameWarning holds details about a compose file project name mismatch.
type ComposeNameWarning struct {
	File        string // compose file path
	ComposeName string // name found in compose file ("" if absent)
	DvaName     string // project_name from dva.yml
}

// ValidateComposeProjectNames checks that the primary compose file has a top-level
// `name:` matching dva.yml's project_name. Only the first compose file is checked
// because Docker Compose uses the first file's name when merging multiple files.
// Returns warnings for missing or mismatched names.
func (c *Config) ValidateComposeProjectNames() []ComposeNameWarning {
	cc := c.PrimaryComposeConfig()
	if cc == nil || cc.ProjectName == "" || len(cc.Files) == 0 {
		return nil
	}

	cfgDir := c.FileDir()
	f := cc.Files[0]
	filePath := f
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(cfgDir, f)
	}

	composeName, err := readComposeNameKey(filePath)
	if err != nil {
		// file unreadable — skip, docker compose will catch it
		return nil
	}

	var warnings []ComposeNameWarning
	if composeName == "" {
		warnings = append(warnings, ComposeNameWarning{
			File:    f,
			DvaName: cc.ProjectName,
		})
	} else if composeName != cc.ProjectName {
		warnings = append(warnings, ComposeNameWarning{
			File:        f,
			ComposeName: composeName,
			DvaName:     cc.ProjectName,
		})
	}
	return warnings
}

// FixComposeProjectName fixes the compose file's top-level `name:` to match dva.yml's project_name.
// For missing name: inserts at the top. For mismatched name: replaces the existing value.
func (c *Config) FixComposeProjectName(w ComposeNameWarning) error {
	cfgDir := c.FileDir()
	filePath := w.File
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(cfgDir, w.File)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", w.File, err)
	}

	content := string(data)
	var updated string

	if w.ComposeName == "" {
		// Insert "name: <project>" at the top
		updated = fmt.Sprintf("name: %s\n\n%s", w.DvaName, content)
	} else {
		// Replace existing top-level name line (must not be indented)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Only match top-level name: (no leading whitespace)
			if strings.HasPrefix(trimmed, "name:") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				lines[i] = fmt.Sprintf("name: %s", w.DvaName)
				break
			}
		}
		updated = strings.Join(lines, "\n")
	}

	return os.WriteFile(filePath, []byte(updated), 0644)
}

// readComposeNameKey reads just the top-level `name:` key from a compose file.
func readComposeNameKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var top struct {
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal(data, &top); err != nil {
		return "", err
	}
	return top.Name, nil
}

// convertYAMLToJSON recursively converts YAML-decoded data to JSON-compatible types.
// YAML maps decode to map[string]any but sometimes keys are non-string.
func convertYAMLToJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = convertYAMLToJSON(v)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[fmt.Sprintf("%v", k)] = convertYAMLToJSON(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = convertYAMLToJSON(v)
		}
		return result
	default:
		return v
	}
}
