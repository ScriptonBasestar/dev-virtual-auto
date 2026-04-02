package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ModulesDirExt is the extension for module files.
// Config represents the parsed dva.yml configuration.
type Config struct {
	Version          string                         `yaml:"version"`
	Environment      map[string]string              `yaml:"environment"`
	EnvFile          any                            `yaml:"env_file"`
	Interaction      map[string]*InteractionCommand `yaml:"interaction"`
	Provision        ProvisionConfig                `yaml:"provision"`
	Infra            map[string]InfraConfig         `yaml:"infra"`
	Modules          []string                       `yaml:"modules"`
	Devcontainer     map[string]any                 `yaml:"devcontainer"`
	Subprojects      map[string]SubprojectConfig    `yaml:"subprojects"`
	HealthChecks     map[string]HealthCheckConfig   `yaml:"health_checks"`
	Endpoints        map[string]EndpointConfig      `yaml:"endpoints"`
	DefaultMode      string                         `yaml:"default_mode"`
	SuggestionIgnore []string                       `yaml:"suggestion_ignore"`
	Modes            map[string]ModeConfig          `yaml:"modes"`
	Environments     map[string]EnvironmentProfile  `yaml:"environments"`
	Ssh              SshConfig                      `yaml:"ssh"`
	DoctorChecks     []DoctorCheck                  `yaml:"checks"`
	Stack            map[string]*LifecycleEntry     `yaml:"stack"`
	Applications     map[string]*ApplicationConfig  `yaml:"applications"`

	// Internal fields
	filePath string
}

// DoctorCheck defines a single environment check for `dva doctor`.
type DoctorCheck struct {
	Name    string `yaml:"name"`     // human-readable check name
	Type    string `yaml:"type"`     // file_exists, command, docker_socket
	Path    string `yaml:"path"`     // for file_exists type
	Command string `yaml:"command"`  // for command type
	FixHint string `yaml:"fix_hint"` // suggestion shown when check fails
	Fix     string `yaml:"fix"`      // shell command to auto-fix (used by dva doctor --fix)
}

// SubprojectConfig defines a sub-project reference.
type SubprojectConfig struct {
	Path        string   `yaml:"path"`
	ExcludeTags []string `yaml:"exclude_tags"`
}

// ModeConfig defines a named operational mode for dva up (--mode/-M flag).
type ModeConfig struct {
	Description     string            `yaml:"description"`
	ComposeProfiles []string          `yaml:"compose_profiles"`
	ComposeServices *[]string         `yaml:"compose_services"` // nil=all, empty=none, items=only those
	HealthChecks    []string          `yaml:"health_checks"`
	EndpointTags    []string          `yaml:"endpoint_tags"` // filter endpoints by tags (empty=show all)
	Environment     map[string]string `yaml:"environment"`
	Provision       string            `yaml:"provision"`    // provision profile to suggest on first run
	Stack           []string          `yaml:"stack"`        // stack entry names to include (empty=all)
	Build           string            `yaml:"build"`        // build strategy: "docker" (compose build), "native" (run command), or custom shell command
	Run             string            `yaml:"run"`          // run strategy: "docker" (compose up), "native" (process via health_checks.start), or custom shell command
	Applications    any               `yaml:"applications"` // app strategy: "native"/"docker" (string) or per-app map[string]string
}

// HasApplications returns true if this mode explicitly defines application strategies.
// When false, applications should not be started in this mode.
func (m *ModeConfig) HasApplications() bool {
	return m.Applications != nil
}

// AppStrategy returns the execution strategy for a named application in this mode.
// Returns "native", "docker", or "" (not specified / use default).
func (m *ModeConfig) AppStrategy(appName string) string {
	if m.Applications == nil {
		return ""
	}
	switch v := m.Applications.(type) {
	case string:
		return v
	case map[string]any:
		if s, ok := v[appName]; ok {
			if str, ok := s.(string); ok {
				return str
			}
		}
		// Check for global "_default" key
		if s, ok := v["_default"]; ok {
			if str, ok := s.(string); ok {
				return str
			}
		}
	}
	return ""
}

// StackEntries returns the stack entry names for mode filtering.
func (m *ModeConfig) StackEntries() []string {
	return m.Stack
}

// EnvironmentProfile defines a named environment configuration for --env flag.
type EnvironmentProfile struct {
	Description    string                     `yaml:"description"`
	Environment    map[string]string          `yaml:"environment"`
	Stack          []string                   `yaml:"stack"` // stack entry names to include (empty=all)
	StackOverrides map[string]*LifecycleEntry `yaml:"stack_overrides"`
}

// StackEntries returns the stack entry names for environment filtering.
func (ep *EnvironmentProfile) StackEntries() []string {
	return ep.Stack
}

// SshConfig holds SSH agent configuration.
type SshConfig struct {
	AgentImage string `yaml:"agent_image"`
}

// EndpointConfig defines a user-facing endpoint URL for the project.
type EndpointConfig struct {
	URL    string            `yaml:"url"`
	Label  string            `yaml:"label"`
	Tags   []string          `yaml:"tags"`
	Paths  map[string]string `yaml:"paths"`  // sub-path -> description
	Source string            `yaml:"source"` // compose "service:host_port" reference (URL auto-resolved)
}

// HealthCheckConfig defines a health check for a non-compose service.
type HealthCheckConfig struct {
	Type         string `yaml:"type"`          // http, tcp, command
	URL          string `yaml:"url"`           // for http type
	Address      string `yaml:"address"`       // for tcp type
	Command      string `yaml:"command"`       // for command type
	Start        string `yaml:"start"`         // command to auto-start (background)
	StartHint    string `yaml:"start_hint"`    // human-readable start instructions
	Timeout      int    `yaml:"timeout"`       // health check timeout in seconds (default: 2)
	ReadyTimeout int    `yaml:"ready_timeout"` // max wait after start in seconds (default: 30)
}

// ServiceTagConfig defines per-service tag configuration.
type ServiceTagConfig struct {
	Tags    []string `yaml:"tags"`
	Related []string `yaml:"related"` // related service names (shown as hints when not running)
	Hint    string   `yaml:"hint"`    // human-readable hint shown when related services are missing
}

// ApplicationConfig declares a long-running application process with
// native and docker execution paths.
type ApplicationConfig struct {
	Description string             `yaml:"description"`
	Tags        []string           `yaml:"tags"`
	Port        int                `yaml:"port"` // listening port (shown in dva app ls)
	Run         AppExecPaths       `yaml:"run"`
	Build       AppExecPaths       `yaml:"build"`
	Dev         AppExecPaths       `yaml:"dev"`
	Health      *HealthCheckConfig `yaml:"health"`
	DependsOn   []string           `yaml:"depends_on"` // compose services or other app names
	Environment map[string]string  `yaml:"environment"`
	Dir         string             `yaml:"dir"` // working directory (default: config dir)
	Variants    map[string]*AppVariant `yaml:"variants"` // sub-components with own build/run/dev
}

// AppVariant defines a sub-component of an application that overrides
// specific fields while inheriting the rest (dir, tags, depends_on) from the parent.
type AppVariant struct {
	Description string             `yaml:"description"`
	Port        int                `yaml:"port"`
	Run         AppExecPaths       `yaml:"run"`
	Build       AppExecPaths       `yaml:"build"`
	Dev         AppExecPaths       `yaml:"dev"`
	Health      *HealthCheckConfig `yaml:"health"`
	Environment map[string]string  `yaml:"environment"`
}

// AppExecPaths holds native and docker execution variants for an application.
type AppExecPaths struct {
	Native string       `yaml:"native"`
	Docker AppDockerRef `yaml:"docker"`
}

// UnmarshalYAML handles both string shorthand and object form for AppExecPaths.
// String form sets Native only: "cargo run --bin api"
// Object form sets Native and/or Docker explicitly.
func (p *AppExecPaths) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		p.Native = node.Value
		return nil
	}
	type plain AppExecPaths
	return node.Decode((*plain)(p))
}

// AppDockerRef holds docker-specific execution config for an application.
type AppDockerRef struct {
	Service string `yaml:"service"` // compose service name
	Profile string `yaml:"profile"` // compose profile to activate
	Command string `yaml:"command"` // override command (for docker exec)
}

// UnmarshalYAML handles both string shorthand and object form for AppDockerRef.
// String form: "docker compose build api-rs" → treated as raw command.
// Object form: { service: api-rs, profile: rust }
func (d *AppDockerRef) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		d.Command = node.Value
		return nil
	}
	type plain AppDockerRef
	return node.Decode((*plain)(d))
}

// HasNative reports whether a native execution path is configured.
func (p *AppExecPaths) HasNative() bool {
	return p.Native != ""
}

// HasDocker reports whether a docker execution path is configured.
func (p *AppExecPaths) HasDocker() bool {
	return p.Docker.Service != "" || p.Docker.Command != ""
}

// InteractionCommand defines a command in the interaction section.
type InteractionCommand struct {
	Description string                         `yaml:"description"`
	Service     string                         `yaml:"service"`
	Workdir     string                         `yaml:"workdir"`
	User        string                         `yaml:"user"`
	DefaultArgs string                         `yaml:"default_args"`
	Environment map[string]string              `yaml:"environment"`
	EnvFile     any                            `yaml:"env_file"`
	Compose     *ComposeOptions                `yaml:"compose"`
	Shell       *bool                          `yaml:"shell"`
	Entrypoint  string                         `yaml:"entrypoint"`
	Runner      string                         `yaml:"runner"`
	Pod         string                         `yaml:"pod"`
	Subcommands map[string]*InteractionCommand `yaml:"subcommands"`
	Tags        []string                       `yaml:"tags"`

	// Command execution: one of the following should be set.
	// command: string or []string — single command or list executed sequentially
	Command string `yaml:"-"` // set by UnmarshalYAML from a scalar
	// CommandLines holds multiple commands when command: is specified as a list.
	CommandLines []string `yaml:"-"` // set by UnmarshalYAML from a sequence
	// script: inline shell script block (multi-line heredoc / block scalar)
	Script string `yaml:"script"`
	// script_file: path to an external shell script (relative to dva.yml)
	ScriptFile string `yaml:"script_file"`
	// steps: named steps executed sequentially (reuses ProvisionItem)
	Steps []ProvisionItem `yaml:"steps"`

	// Hook fields: extend or replace hookable built-in commands (up, down, build, etc.)
	Before  []ProvisionItem `yaml:"before"`
	Replace []ProvisionItem `yaml:"replace"`
	After   []ProvisionItem `yaml:"after"`
}

// HasHooks reports whether the command defines any hook steps (before/replace/after).
func (c *InteractionCommand) HasHooks() bool {
	return len(c.Before) > 0 || len(c.Replace) > 0 || len(c.After) > 0
}

// HasSteps reports whether the command uses step-based execution.
func (c *InteractionCommand) HasSteps() bool {
	return len(c.Steps) > 0
}

// HasScript reports whether the command uses an inline script.
func (c *InteractionCommand) HasScript() bool {
	return c.Script != ""
}

// HasScriptFile reports whether the command references an external script file.
func (c *InteractionCommand) HasScriptFile() bool {
	return c.ScriptFile != ""
}

// HasMultiCommand reports whether the command was specified as a list.
func (c *InteractionCommand) HasMultiCommand() bool {
	return len(c.CommandLines) > 0
}

// EffectiveCommand returns the single command string.
// For multi-command lists, returns a joined representation for display.
func (c *InteractionCommand) EffectiveCommand() string {
	if len(c.CommandLines) > 0 {
		return strings.Join(c.CommandLines, " && ")
	}
	return c.Command
}

// UnmarshalYAML implements custom unmarshaling for InteractionCommand.
// Handles the polymorphic `command` field (string or []string).
// All other fields are decoded normally via a plain type alias.
func (c *InteractionCommand) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("interaction command: expected mapping node")
	}

	// Decode all non-command fields using the tag-based alias.
	// We use a plain alias that has no UnmarshalYAML to avoid recursion.
	type plain struct {
		Description string                         `yaml:"description"`
		Service     string                         `yaml:"service"`
		Workdir     string                         `yaml:"workdir"`
		User        string                         `yaml:"user"`
		DefaultArgs string                         `yaml:"default_args"`
		Environment map[string]string              `yaml:"environment"`
		EnvFile     any                            `yaml:"env_file"`
		Compose     *ComposeOptions                `yaml:"compose"`
		Shell       *bool                          `yaml:"shell"`
		Entrypoint  string                         `yaml:"entrypoint"`
		Runner      string                         `yaml:"runner"`
		Pod         string                         `yaml:"pod"`
		Subcommands map[string]*InteractionCommand `yaml:"subcommands"`
		Tags        []string                       `yaml:"tags"`
		Script      string                         `yaml:"script"`
		ScriptFile  string                         `yaml:"script_file"`
		Steps       []ProvisionItem                `yaml:"steps"`
		Before      []ProvisionItem                `yaml:"before"`
		Replace     []ProvisionItem                `yaml:"replace"`
		After       []ProvisionItem                `yaml:"after"`
	}
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	c.Description = p.Description
	c.Service = p.Service
	c.Workdir = p.Workdir
	c.User = p.User
	c.DefaultArgs = p.DefaultArgs
	c.Environment = p.Environment
	c.EnvFile = p.EnvFile
	c.Compose = p.Compose
	c.Shell = p.Shell
	c.Entrypoint = p.Entrypoint
	c.Runner = p.Runner
	c.Pod = p.Pod
	c.Subcommands = p.Subcommands
	c.Tags = p.Tags
	c.Script = p.Script
	c.ScriptFile = p.ScriptFile
	c.Steps = p.Steps
	c.Before = p.Before
	c.Replace = p.Replace
	c.After = p.After

	// Manually find and parse the `command` key for polymorphism.
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value != "command" {
			continue
		}
		valNode := node.Content[i+1]
		switch valNode.Kind {
		case yaml.ScalarNode:
			c.Command = strings.TrimSpace(valNode.Value)
		case yaml.SequenceNode:
			var lines []string
			if err := valNode.Decode(&lines); err != nil {
				return fmt.Errorf("command: expected string or list of strings: %w", err)
			}
			for j, l := range lines {
				lines[j] = strings.TrimSpace(l)
			}
			c.CommandLines = lines
			if len(lines) > 0 {
				c.Command = lines[0] // first line for display/backward-compat
			}
		default:
			return fmt.Errorf("command: unsupported YAML type (expected string or sequence)")
		}
		break
	}
	return nil
}

// ShellEnabled returns whether shell mode is enabled (default: true).
func (c *InteractionCommand) ShellEnabled() bool {
	if c.Shell == nil {
		return true
	}
	return *c.Shell
}

// ComposeOptions holds per-command Docker Compose options.
type ComposeOptions struct {
	Method     string   `yaml:"method"`
	Profiles   []string `yaml:"profiles"`
	RunOptions []string `yaml:"run_options"`
}

// ProvisionConfig holds provision profiles with an optional default_profile alias.
type ProvisionConfig struct {
	DefaultProfile string                     `yaml:"-"`
	Profiles       map[string][]ProvisionItem `yaml:"-"`
}

// UnmarshalYAML handles the mixed-type provision mapping:
// "default_profile" key is extracted as a string; all other keys are profiles.
func (pc *ProvisionConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("provision: expected mapping, got kind %d", node.Kind)
	}

	pc.Profiles = make(map[string][]ProvisionItem)

	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		if key == "default_profile" {
			pc.DefaultProfile = val.Value
			continue
		}

		var items []ProvisionItem
		if err := val.Decode(&items); err != nil {
			return fmt.Errorf("provision profile '%s': %w", key, err)
		}
		pc.Profiles[key] = items
	}

	return nil
}

// ProvisionItem represents a single item in a provision profile.
type ProvisionItem struct {
	// Step-based format
	Step     string `yaml:"step"`
	Run      any    `yaml:"run"`
	Note     string `yaml:"note"`
	Parallel bool   `yaml:"parallel"` // Run concurrently with consecutive parallel steps

	// Compose-aware commands (inherit compose.files and compose.project_name)
	ComposeUp   []string `yaml:"compose_up"`   // Services to start: [postgres, minio, redis]
	ComposeExec string   `yaml:"compose_exec"` // Command in service: "pg_isready -U ndstack"
	ComposeRun  string   `yaml:"compose_run"`  // One-off command in service

	// Legacy structured format
	Echo   string `yaml:"echo"`
	Cmd    string `yaml:"cmd"`
	ShellC string `yaml:"shell"`
	Sleep  any    `yaml:"sleep"`
	Docker any    `yaml:"docker"`

	// Raw string format (set during custom unmarshal)
	Raw string `yaml:"-"`
}

// UnmarshalYAML handles both string and object provision items.
func (p *ProvisionItem) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		p.Raw = node.Value
		return nil
	}
	// unmarshal the object fields manually to avoid recursion
	type plain ProvisionItem
	return node.Decode((*plain)(p))
}

// RunCommands extracts the run commands from a ProvisionItem.
func (p *ProvisionItem) RunCommands() []string {
	if p.Raw != "" {
		return []string{p.Raw}
	}
	if p.Run == nil {
		return nil
	}
	switch v := p.Run.(type) {
	case string:
		return []string{v}
	case []any:
		cmds := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				cmds = append(cmds, s)
			}
		}
		return cmds
	}
	return nil
}

// InfraConfig holds infrastructure service configuration.
type InfraConfig struct {
	Git  string `yaml:"git"`
	Ref  string `yaml:"ref"`
	Path string `yaml:"path"`
}

// FilePath returns the path to the loaded config file.
func (c *Config) FilePath() string {
	return c.filePath
}

// FileDir returns the directory containing the config file.
func (c *Config) FileDir() string {
	return filepath.Dir(c.filePath)
}

// ResolveApp resolves an application name (possibly with variant via dot notation)
// to a fully merged ApplicationConfig. Supports "appname" and "appname.variant".
func (c *Config) ResolveApp(name string) (string, *ApplicationConfig, error) {
	// Direct lookup (no variant)
	if app, ok := c.Applications[name]; ok {
		return name, app, nil
	}

	// Try dot split for variant
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("application %q not found", name)
	}

	parent, ok := c.Applications[parts[0]]
	if !ok {
		return "", nil, fmt.Errorf("application %q not found", parts[0])
	}
	if parent.Variants == nil {
		return "", nil, fmt.Errorf("application %q has no variants", parts[0])
	}

	variant, ok := parent.Variants[parts[1]]
	if !ok {
		return "", nil, fmt.Errorf("variant %q not found in application %q", parts[1], parts[0])
	}

	resolved := ResolveVariant(parent, variant)
	return name, resolved, nil
}

// ListAppNames returns all application names including variants (as "app.variant").
func (c *Config) ListAppNames() []string {
	var names []string
	for name, app := range c.Applications {
		names = append(names, name)
		for vName := range app.Variants {
			names = append(names, name+"."+vName)
		}
	}
	return names
}

// resolveVariant creates a new ApplicationConfig by inheriting from parent
// and overriding with variant-specific fields.
func ResolveVariant(parent *ApplicationConfig, variant *AppVariant) *ApplicationConfig {
	resolved := &ApplicationConfig{
		Description: parent.Description,
		Tags:        parent.Tags,
		Dir:         parent.Dir,
		DependsOn:   parent.DependsOn,
		Port:        parent.Port,
		Run:         parent.Run,
		Build:       parent.Build,
		Dev:         parent.Dev,
		Health:      parent.Health,
		Environment: copyStringMap(parent.Environment),
	}

	// Override with variant values
	if variant.Description != "" {
		resolved.Description = variant.Description
	}
	if variant.Port != 0 {
		resolved.Port = variant.Port
	}
	if variant.Run.Native != "" || variant.Run.Docker.Service != "" || variant.Run.Docker.Command != "" {
		resolved.Run = variant.Run
	}
	if variant.Build.Native != "" || variant.Build.Docker.Service != "" || variant.Build.Docker.Command != "" {
		resolved.Build = variant.Build
	}
	if variant.Dev.Native != "" || variant.Dev.Docker.Service != "" || variant.Dev.Docker.Command != "" {
		resolved.Dev = variant.Dev
	}
	if variant.Health != nil {
		resolved.Health = variant.Health
	}
	// Merge environment: parent + variant (variant overrides)
	for k, v := range variant.Environment {
		if resolved.Environment == nil {
			resolved.Environment = make(map[string]string)
		}
		resolved.Environment[k] = v
	}

	return resolved
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// LoadOption configures optional behavior for Load.
type LoadOption func(*loadOptions)

type loadOptions struct {
	skipVersionCheck bool
}

// SkipVersionCheck returns a LoadOption that disables version compatibility checking.
// Use this for commands like "config improve" that need to load outdated configs to fix them.
func SkipVersionCheck() LoadOption {
	return func(o *loadOptions) {
		o.skipVersionCheck = true
	}
}

// Load discovers and loads the dva.yml configuration.
func Load(workDir string, opts ...LoadOption) (*Config, error) {
	var o loadOptions
	for _, opt := range opts {
		opt(&o)
	}

	filePath, err := findConfig(workDir)
	if err != nil {
		return nil, err
	}

	cfg, err := loadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", filePath, err)
	}
	cfg.filePath = filePath

	// Check version compatibility
	if !o.skipVersionCheck && cfg.Version != "" {
		if !isVersionCompatible(cfg.Version) {
			return nil, fmt.Errorf("your dva version is `%s`, but config requires minimum version `%s`. Please upgrade dva", Version, cfg.Version)
		}
	}

	// Load modules
	if len(cfg.Modules) > 0 {
		modulesDir := filepath.Join(filepath.Dir(filePath), DotDirName)
		for _, mod := range cfg.Modules {
			modFile := filepath.Join(modulesDir, mod+".yml")
			modCfg, err := loadFile(modFile)
			if err != nil {
				return nil, fmt.Errorf("loading module `%s`: %w", mod, err)
			}
			if len(modCfg.Modules) > 0 {
				return nil, fmt.Errorf("nested modules are not supported")
			}
			if err := cfg.mergeFrom(modCfg); err != nil {
				return nil, fmt.Errorf("merging module %q: %w", mod, err)
			}
		}
	}

	// Load override (if exists)
	overrideFile := strings.TrimSuffix(filePath, ".yml") + OverrideExt
	if overCfg, err := loadFile(overrideFile); err == nil {
		if err := cfg.mergeFrom(overCfg); err != nil {
			return nil, fmt.Errorf("merging override: %w", err)
		}
	}

	// Apply defaults
	if cfg.Environment == nil {
		cfg.Environment = make(map[string]string)
	}
	if cfg.Interaction == nil {
		cfg.Interaction = make(map[string]*InteractionCommand)
	}
	if cfg.Provision.Profiles == nil {
		cfg.Provision.Profiles = make(map[string][]ProvisionItem)
	}
	if cfg.Stack == nil {
		cfg.Stack = make(map[string]*LifecycleEntry)
	}
	if cfg.Applications == nil {
		cfg.Applications = make(map[string]*ApplicationConfig)
	}

	// Populate Name field and resolve deferred plugins from map keys
	for name, entry := range cfg.Stack {
		entry.Name = name
		if err := entry.ResolvePluginFromName(); err != nil {
			return nil, err
		}
	}

	// Resolve endpoint URLs from source references
	cfg.ResolveEndpoints()

	// Warn if interaction commands shadow reserved built-in commands
	WarnReservedCommandConflicts(cfg.Interaction)

	return cfg, nil
}

var yamlDeprecationWarned bool

// findConfig walks up from workDir to find dva.yml.
func findConfig(workDir string) (string, error) {
	// Check DVA_FILE env var first
	if env := os.Getenv(EnvFileKey); env != "" {
		if _, err := os.Stat(env); err != nil {
			return "", fmt.Errorf("DVA_FILE=%s: %w", env, err)
		}
		return env, nil
	}

	dir, err := filepath.Abs(workDir)
	if err != nil {
		return "", err
	}

	for {
		// Prefer dva.yml (canonical name)
		candidate := filepath.Join(dir, FileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		// Fallback: accept dva.yaml with deprecation warning (once per process)
		altCandidate := filepath.Join(dir, FileNameAlt)
		if _, err := os.Stat(altCandidate); err == nil {
			if !yamlDeprecationWarned {
				yamlDeprecationWarned = true
				fmt.Fprintf(os.Stderr, "⚠  Found %s — consider renaming to dva.yml (canonical name)\n", altCandidate)
			}
			return altCandidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			absWork, _ := filepath.Abs(workDir)
			return "", fmt.Errorf("could not find dva.yml (searched from %s to /).\n  Hint: run 'dva init' or set DVA_FILE=/path/to/dva.yml", absWork)
		}
		dir = parent
	}
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	return cfg, nil
}

// mergeFrom merges another config into this one.
//
// Merge semantics (see docs/30-config-merge-semantics.md):
//   - map sections: key-level deep merge (entries are field-merged, not replaced)
//   - list fields: replace (later layer wins entirely)
//   - scalar fields: replace (non-zero later value wins)
//   - nil/absent: inherits from base; explicit empty clears
//
// Returns an error if a restricted field override is attempted.
func (c *Config) mergeFrom(other *Config) error {
	// environment: map merge (key-level)
	c.Environment = mergeStringMap(c.Environment, other.Environment)

	// interaction: deep merge per entry
	if other.Interaction != nil {
		if c.Interaction == nil {
			c.Interaction = make(map[string]*InteractionCommand)
		}
		for k, v := range other.Interaction {
			if existing, ok := c.Interaction[k]; ok {
				merged, err := mergeInteractionCommand(existing, v)
				if err != nil {
					return fmt.Errorf("interaction %q: %w", k, err)
				}
				c.Interaction[k] = merged
			} else {
				c.Interaction[k] = v
			}
		}
	}

	// provision: scalar replace + map key-level replace (profiles are step lists)
	if other.Provision.DefaultProfile != "" {
		c.Provision.DefaultProfile = other.Provision.DefaultProfile
	}
	if len(other.Provision.Profiles) > 0 {
		if c.Provision.Profiles == nil {
			c.Provision.Profiles = make(map[string][]ProvisionItem)
		}
		for k, v := range other.Provision.Profiles {
			c.Provision.Profiles[k] = v
		}
	}

	// health_checks: deep merge per entry (struct fields replace individually)
	if other.HealthChecks != nil {
		if c.HealthChecks == nil {
			c.HealthChecks = make(map[string]HealthCheckConfig)
		}
		for k, v := range other.HealthChecks {
			if existing, ok := c.HealthChecks[k]; ok {
				c.HealthChecks[k] = mergeHealthCheckConfig(existing, v)
			} else {
				c.HealthChecks[k] = v
			}
		}
	}

	// endpoints: deep merge per entry
	if other.Endpoints != nil {
		if c.Endpoints == nil {
			c.Endpoints = make(map[string]EndpointConfig)
		}
		for k, v := range other.Endpoints {
			if existing, ok := c.Endpoints[k]; ok {
				c.Endpoints[k] = mergeEndpointConfig(existing, v)
			} else {
				c.Endpoints[k] = v
			}
		}
	}

	// infra: key-level replace (simple struct)
	if other.Infra != nil {
		if c.Infra == nil {
			c.Infra = make(map[string]InfraConfig)
		}
		for k, v := range other.Infra {
			c.Infra[k] = v
		}
	}

	// default_mode: scalar replace
	if other.DefaultMode != "" {
		c.DefaultMode = other.DefaultMode
	}

	// modes: deep merge per entry
	if other.Modes != nil {
		if c.Modes == nil {
			c.Modes = make(map[string]ModeConfig)
		}
		for k, v := range other.Modes {
			if existing, ok := c.Modes[k]; ok {
				c.Modes[k] = mergeModeConfig(existing, v)
			} else {
				c.Modes[k] = v
			}
		}
	}

	// environments: deep merge per entry
	if other.Environments != nil {
		if c.Environments == nil {
			c.Environments = make(map[string]EnvironmentProfile)
		}
		for k, v := range other.Environments {
			if existing, ok := c.Environments[k]; ok {
				c.Environments[k] = mergeEnvironmentProfile(existing, v)
			} else {
				c.Environments[k] = v
			}
		}
	}

	// ssh: scalar replace
	if other.Ssh.AgentImage != "" {
		c.Ssh.AgentImage = other.Ssh.AgentImage
	}

	// suggestion_ignore: list replace
	if other.SuggestionIgnore != nil {
		c.SuggestionIgnore = other.SuggestionIgnore
	}

	// env_file: replace as a whole
	if other.EnvFile != nil {
		c.EnvFile = other.EnvFile
	}

	// subprojects: key-level replace (simple struct)
	if other.Subprojects != nil {
		if c.Subprojects == nil {
			c.Subprojects = make(map[string]SubprojectConfig)
		}
		for k, v := range other.Subprojects {
			c.Subprojects[k] = v
		}
	}

	// stack: deep merge per entry
	if len(other.Stack) > 0 {
		if c.Stack == nil {
			c.Stack = make(map[string]*LifecycleEntry)
		}
		for k, v := range other.Stack {
			// Resolve deferred plugin from entry name before merge
			v.Name = k
			if err := v.ResolvePluginFromName(); err != nil {
				return err
			}
			if existing, ok := c.Stack[k]; ok {
				merged, err := MergeLifecycleEntry(existing, v)
				if err != nil {
					return err
				}
				c.Stack[k] = merged
			} else {
				c.Stack[k] = v
			}
		}
	}

	// applications: deep merge per entry
	if other.Applications != nil {
		if c.Applications == nil {
			c.Applications = make(map[string]*ApplicationConfig)
		}
		for k, v := range other.Applications {
			if existing, ok := c.Applications[k]; ok {
				c.Applications[k] = mergeApplicationConfig(existing, v)
			} else {
				c.Applications[k] = v
			}
		}
	}

	// doctor checks: append (existing behavior preserved)
	if len(other.DoctorChecks) > 0 {
		c.DoctorChecks = append(c.DoctorChecks, other.DoctorChecks...)
	}

	// devcontainer: replace as a whole
	if other.Devcontainer != nil {
		c.Devcontainer = other.Devcontainer
	}

	return nil
}

// isVersionCompatible checks if current DVA version >= required version.
func isVersionCompatible(required string) bool {
	cur := parseVersion(Version)
	req := parseVersion(required)
	for i := 0; i < 3; i++ {
		if cur[i] < req[i] {
			return false
		}
		if cur[i] > req[i] {
			return true
		}
	}
	return true
}

func parseVersion(v string) [3]int {
	var parts [3]int
	v = strings.TrimPrefix(v, "v")
	_, _ = fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}

// nonHTTPServices are compose service name prefixes that resolve to plain host:port
// instead of http://localhost:port. Users needing other protocols should use url: directly.
var nonHTTPServices = map[string]bool{
	// Databases
	"postgres": true, "postgresql": true, "pg": true,
	"mysql": true, "mariadb": true,
	"mssql": true, "sqlserver": true,
	"mongo": true, "mongodb": true,
	"cassandra": true, "scylla": true,
	"db": true, "database": true,
	// Caches
	"redis": true, "valkey": true,
	"memcached": true,
	"cache":     true,
	// Messaging
	"kafka": true, "zookeeper": true,
	"rabbitmq": true, "nats": true,
	"mq": true, "queue": true, "broker": true,
	// Other
	"ssh": true,
}

// ResolveEndpoints auto-fills URL for endpoints that have source but no url.
// Source format: "service:host_port" → resolves to http://localhost:{port}
// or plain localhost:{port} for known non-HTTP infrastructure services.
func (c *Config) ResolveEndpoints() {
	if c.Endpoints == nil {
		return
	}

	for name, ep := range c.Endpoints {
		if ep.Source == "" || ep.URL != "" {
			continue
		}

		parts := strings.SplitN(ep.Source, ":", 2)
		if len(parts) != 2 || parts[1] == "" {
			continue
		}

		svc := parts[0]
		port := parts[1]

		if nonHTTPServices[strings.ToLower(svc)] {
			ep.URL = "localhost:" + port
		} else {
			ep.URL = "http://localhost:" + port
		}
		c.Endpoints[name] = ep
	}
}
