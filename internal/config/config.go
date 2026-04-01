package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DotDirName is the directory used for transient state and modules.
const DotDirName = ".sb/dva"

// Config represents the parsed dva.yml configuration.
type Config struct {
	Version      string                         `yaml:"version"`
	Environment  map[string]string              `yaml:"environment"`
	EnvFile      any                            `yaml:"env_file"`
	Interaction  map[string]*InteractionCommand `yaml:"interaction"`
	Provision    ProvisionConfig                `yaml:"provision"`
	Infra        map[string]InfraConfig         `yaml:"infra"`
	Modules      []string                       `yaml:"modules"`
	Devcontainer map[string]any                 `yaml:"devcontainer"`
	Subprojects  map[string]SubprojectConfig    `yaml:"subprojects"`
	HealthChecks map[string]HealthCheckConfig   `yaml:"health_checks"`
	Endpoints    map[string]EndpointConfig      `yaml:"endpoints"`
	DefaultMode       string                          `yaml:"default_mode"`
	SuggestionIgnore  []string                        `yaml:"suggestion_ignore"`
	Modes             map[string]ModeConfig           `yaml:"modes"`
	Environments map[string]EnvironmentProfile  `yaml:"environments"`
	Ssh          SshConfig                      `yaml:"ssh"`
	DoctorChecks []DoctorCheck                  `yaml:"checks"`
	Stack        map[string]*LifecycleEntry    `yaml:"stack"`
	Applications map[string]*ApplicationConfig `yaml:"applications"`

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
	Provision       string            `yaml:"provision"`  // provision profile to suggest on first run
	Stack           []string          `yaml:"stack"`       // stack entry names to include (empty=all)
	Build           string            `yaml:"build"`       // build strategy: "docker" (compose build), "native" (run command), or custom shell command
	Run             string            `yaml:"run"`         // run strategy: "docker" (compose up), "native" (process via health_checks.start), or custom shell command
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
	Description string            `yaml:"description"`
	Environment map[string]string `yaml:"environment"`
	Stack       []string          `yaml:"stack"` // stack entry names to include (empty=all)
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
	Description string            `yaml:"description"`
	Tags        []string          `yaml:"tags"`
	Run         AppExecPaths      `yaml:"run"`
	Build       AppExecPaths      `yaml:"build"`
	Dev         AppExecPaths      `yaml:"dev"`
	Health      *HealthCheckConfig `yaml:"health"`
	DependsOn   []string          `yaml:"depends_on"` // compose services or other app names
	Environment map[string]string `yaml:"environment"`
	Dir         string            `yaml:"dir"` // working directory (default: config dir)
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
	Description       string                         `yaml:"description"`
	Service           string                         `yaml:"service"`
	Command           string                         `yaml:"command"`
	Workdir           string                         `yaml:"workdir"`
	User              string                         `yaml:"user"`
	DefaultArgs       string                         `yaml:"default_args"`
	Environment       map[string]string              `yaml:"environment"`
	EnvFile           any                    `yaml:"env_file"`
	Compose           *ComposeOptions                `yaml:"compose"`
	Shell             *bool                          `yaml:"shell"`
	Entrypoint        string                         `yaml:"entrypoint"`
	Runner            string                         `yaml:"runner"`
	Pod               string                         `yaml:"pod"`
	Subcommands map[string]*InteractionCommand `yaml:"subcommands"`
	Tags              []string                       `yaml:"tags"`

	// Hook fields: extend or replace hookable built-in commands (up, down, build, etc.)
	Before  []ProvisionItem `yaml:"before"`
	Replace []ProvisionItem `yaml:"replace"`
	After   []ProvisionItem `yaml:"after"`
}

// HasHooks reports whether the command defines any hook steps (before/replace/after).
func (c *InteractionCommand) HasHooks() bool {
	return len(c.Before) > 0 || len(c.Replace) > 0 || len(c.After) > 0
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
			cfg.mergeFrom(modCfg)
		}
	}

	// Load override (if exists)
	overrideFile := strings.TrimSuffix(filePath, ".yml") + ".override.yml"
	if overCfg, err := loadFile(overrideFile); err == nil {
		cfg.mergeFrom(overCfg)
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
	if env := os.Getenv("DVA_FILE"); env != "" {
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
		candidate := filepath.Join(dir, "dva.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		// Fallback: accept dva.yaml with deprecation warning (once per process)
		altCandidate := filepath.Join(dir, "dva.yaml")
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

// mergeFrom merges another config into this one (other values take precedence
// for top-level scalars; maps are deep-merged).
func (c *Config) mergeFrom(other *Config) {
	// Merge environment
	if other.Environment != nil {
		if c.Environment == nil {
			c.Environment = make(map[string]string)
		}
		for k, v := range other.Environment {
			c.Environment[k] = v
		}
	}

	// Merge interaction
	if other.Interaction != nil {
		if c.Interaction == nil {
			c.Interaction = make(map[string]*InteractionCommand)
		}
		for k, v := range other.Interaction {
			c.Interaction[k] = v
		}
	}

	// Merge provision
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

	// Merge health checks
	if other.HealthChecks != nil {
		if c.HealthChecks == nil {
			c.HealthChecks = make(map[string]HealthCheckConfig)
		}
		for k, v := range other.HealthChecks {
			c.HealthChecks[k] = v
		}
	}

	// Merge endpoints
	if other.Endpoints != nil {
		if c.Endpoints == nil {
			c.Endpoints = make(map[string]EndpointConfig)
		}
		for k, v := range other.Endpoints {
			c.Endpoints[k] = v
		}
	}

	// Merge infra
	if other.Infra != nil {
		if c.Infra == nil {
			c.Infra = make(map[string]InfraConfig)
		}
		for k, v := range other.Infra {
			c.Infra[k] = v
		}
	}

	// Merge default_mode
	if other.DefaultMode != "" {
		c.DefaultMode = other.DefaultMode
	}

	// Merge modes
	if other.Modes != nil {
		if c.Modes == nil {
			c.Modes = make(map[string]ModeConfig)
		}
		for k, v := range other.Modes {
			c.Modes[k] = v
		}
	}

	// Merge environments
	if other.Environments != nil {
		if c.Environments == nil {
			c.Environments = make(map[string]EnvironmentProfile)
		}
		for k, v := range other.Environments {
			c.Environments[k] = v
		}
	}

	// Merge ssh
	if other.Ssh.AgentImage != "" {
		c.Ssh.AgentImage = other.Ssh.AgentImage
	}

	// Merge env_file (override takes precedence as a whole)
	if other.EnvFile != nil {
		c.EnvFile = other.EnvFile
	}

	// Merge subprojects
	if other.Subprojects != nil {
		if c.Subprojects == nil {
			c.Subprojects = make(map[string]SubprojectConfig)
		}
		for k, v := range other.Subprojects {
			c.Subprojects[k] = v
		}
	}

	// Merge stack entries (map merge, key=name)
	if len(other.Stack) > 0 {
		if c.Stack == nil {
			c.Stack = make(map[string]*LifecycleEntry)
		}
		for k, v := range other.Stack {
			c.Stack[k] = v
		}
	}

	// Merge applications
	if other.Applications != nil {
		if c.Applications == nil {
			c.Applications = make(map[string]*ApplicationConfig)
		}
		for k, v := range other.Applications {
			c.Applications[k] = v
		}
	}

	// Merge doctor checks
	if len(other.DoctorChecks) > 0 {
		c.DoctorChecks = append(c.DoctorChecks, other.DoctorChecks...)
	}

	// Merge devcontainer (override takes precedence as a whole)
	if other.Devcontainer != nil {
		c.Devcontainer = other.Devcontainer
	}
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
	fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
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
	"cache": true,
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
