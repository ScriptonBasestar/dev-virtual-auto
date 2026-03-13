package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
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
			errs = append(errs, fmt.Sprintf(
				"  - interaction.%s: '%s' is a reserved DVA command and will be shadowed",
				conflict.Name, conflict.Name,
			))
		}
		return fmt.Errorf("reserved command conflict in dva.yml:\n%s", strings.Join(errs, "\n"))
	}

	return nil
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
