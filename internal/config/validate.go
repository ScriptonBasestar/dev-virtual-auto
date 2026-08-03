package config

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

//go:embed schema.json
var embeddedSchema embed.FS

// removedSchemaKeys maps keys DVA used to accept to whatever took over their
// job. Every one of them was real schema once, emitted by DVA's own templates
// and AI flows, so configs generated against an older version still carry them
// and "Additional property X is not allowed" alone leaves those users with a
// rejection and no next step.
//
// Keyed by property name alone, not by path: the guidance has to read correctly
// wherever the key turns up, since a stale config can carry it anywhere.
//
// This is also the list TestRemovedKeysAbsentFromGeneratorCorpus holds the AI
// generator corpus to — a key is only really gone once nothing teaches it.
var removedSchemaKeys = map[string]string{
	// TASK-036: services metadata; the reader (compose_status.go) is gone.
	"hint":    "removed: 'services' is tags-only — human-readable hints live in health_checks.<name>.start_hint",
	"related": "removed: group services with 'tags', or list them explicitly in modes.<name>.compose_services",
	// ee8ac8e: port metadata moved wholesale to the endpoints: section.
	"ports": "removed: DVA reads ports from the compose files — declare user-facing ones under the top-level 'endpoints:'",
	// TASK-035: both validated green and were never read.
	"interpolate": "removed: env_file values are always interpolated, with no way to opt out",
	"priority":    "removed: precedence is fixed — environment: < env_file < OS environment",
}

// rootField is the path gojsonschema reports for an error on the document root.
const rootField = "(root)"

// removedRootKeys is the same idea as removedSchemaKeys for keys 17a74b9 folded
// out of the document root, and it is separate for one reason: their names are
// still valid elsewhere. 'compose' names a stack entry and a runner, 'kubectl' a
// runner. Keying those by name alone would append "removed" guidance to the very
// shape that replaced them, and would make the corpus test reject the correct
// examples that teach it.
//
// Guidance from here is only appended when the error is on the root itself.
var removedRootKeys = map[string]string{
	"compose":   "removed from the root: declare it as a stack entry — stack.<name>.default_runner: compose + runners.compose",
	"kubectl":   "removed from the root: declare it as a stack entry — stack.<name>.default_runner: kubectl + runners.kubectl",
	"profiles":  "removed: renamed — use 'modes:'",
	"lifecycle": "removed: renamed — use 'stack:'",
}

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
			line := fmt.Sprintf("  - %s: %s", desc.Field(), desc.Description())
			if desc.Type() == "additional_property_not_allowed" {
				if prop, ok := desc.Details()["property"].(string); ok {
					guidance, removed := removedSchemaKeys[prop]
					if !removed && desc.Field() == rootField {
						guidance, removed = removedRootKeys[prop]
					}
					if removed {
						line += "\n      " + guidance
					}
				}
			}
			errs = append(errs, line)
		}
		return fmt.Errorf("schema validation failed in dva.yml:\n%s", strings.Join(errs, "\n"))
	}

	// Check for reserved command conflicts in interaction section
	if conflicts := ValidateReservedCommands(c.Interaction); len(conflicts) > 0 {
		var errs []string
		for _, conflict := range conflicts {
			// ConflictAdvice, not a hint built here: this error and the warning logged on every
			// config load describe one condition, and the reader who sees both must not have to
			// reconcile two accounts of which invocation reaches their command.
			errs = append(errs, fmt.Sprintf("  - interaction.%s: %s", conflict.Name, ConflictAdvice(conflict.Name)))
		}
		// No filename: config is the merge of modules: and subprojects:, so the file that
		// declares the conflicting key is not knowable from the merged config.
		return fmt.Errorf("reserved command conflict in this config:\n%s", strings.Join(errs, "\n"))
	}

	if err := c.validateHookPlacement(); err != nil {
		return err
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

// validateHookPlacement rejects before/replace/after wherever they cannot execute.
//
// Hooks run through exactly one path: wrapWithHooks (cli/hooks.go:20), wired at
// cli/root.go:129 for the seven hookable built-ins, which reads `c.Interaction[cmdName]` —
// a top-level lookup. Nothing walks Subcommands looking for hooks, so a nested one has no
// path on which it could fire, whatever it or its parent is named.
//
// This used to iterate c.Interaction only, so moving the identical hook one level down
// turned a rc-1 validation failure into silence. Measured on v0.1.44, all three shapes
// validated clean with the hook dead:
//
//	interaction.db.subcommands.migrate.before      `dva db migrate` → MIGRATING, rc 0
//	interaction.up.subcommands.fast.before         parent hookable; `fast` never registers
//	interaction.db.subcommands.up.before           `dva db up` → DB-UP, rc 0
//
// The third is why the nested rule takes no account of the node's name. A check keyed off
// IsHookableCommand(leaf) waves it through — the leaf is literally called `up` — while the
// hook is exactly as dead as the other two.
//
// An error, not a warning, against the warnInertProvisionSteps precedent that a
// long-inert key should not start failing configs at upgrade. That precedent rests on the
// failure being observable: warnIgnoredParallelSteps records that a dropped `parallel:`
// "produces exactly the right output and merely takes twice as long". A skipped
// `before: [backup]` produces exactly the right output and no signal at all — and measured,
// semantic warnings do not reach the run path, so `dva db migrate` prints MIGRATING and
// nothing else. Warning here would reach nobody except someone already suspicious, which is
// not the person running a migration. The config an error breaks at upgrade is a config
// whose backup was never running; saying so is the point.
//
// Not fixed by making nested hooks execute: that is a runner change, not a validation one,
// and it would give `before:` a second meaning at depth before anyone has asked for one.
// Rejecting where it cannot run keeps the door open for that to be added deliberately.
func (c *Config) validateHookPlacement() error {
	var problems []string

	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, _ inheritedExec) {
		if !cmd.HasHooks() {
			return
		}
		if strings.Contains(path, ".subcommands.") {
			problems = append(problems, fmt.Sprintf(
				"%s: before/replace/after hooks run only on a top-level hookable command "+
					"(up, down, stop, restart, build, clean, logs); a hook nested under a "+
					"subcommand never runs, whatever the subcommand is named", path))
			return
		}
		// Top-level. The message is unchanged from when this check lived inline, and the
		// path is `interaction.<name>` there, so it renders identically.
		if name := strings.TrimPrefix(path, "interaction."); !IsHookableCommand(name) {
			problems = append(problems, fmt.Sprintf(
				"%s: before/replace/after hooks are only supported on hookable commands "+
					"(up, down, stop, restart, build, clean, logs)", path))
		}
	})

	if len(problems) == 0 {
		return nil
	}
	// c.Interaction is a map, so without this a config with two violations names a different
	// one on each run. First-only matches how the rest of Validate reports. TASK-128.
	sort.Strings(problems)
	return errors.New(problems[0])
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
