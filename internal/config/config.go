package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)


// Config represents the parsed dva.yml configuration.
type Config struct {
	Version      string                         `yaml:"version"`
	Compose      ComposeConfig                  `yaml:"compose"`
	Kubectl      KubectlConfig                  `yaml:"kubectl"`
	Environment  map[string]string              `yaml:"environment"`
	EnvFile      any                    `yaml:"env_file"`
	Interaction  map[string]*InteractionCommand `yaml:"interaction"`
	Provision    map[string][]ProvisionItem     `yaml:"provision"`
	Infra        map[string]InfraConfig         `yaml:"infra"`
	Modules      []string                       `yaml:"modules"`
	Devcontainer map[string]any         `yaml:"devcontainer"`
	Subprojects  map[string]SubprojectConfig    `yaml:"subprojects"`
	HealthChecks map[string]HealthCheckConfig   `yaml:"health_checks"`

	// Internal fields
	filePath string
}

// SubprojectConfig defines a sub-project reference.
type SubprojectConfig struct {
	Path        string   `yaml:"path"`
	ExcludeTags []string `yaml:"exclude_tags"`
}

// HealthCheckConfig defines a health check for a non-compose service.
type HealthCheckConfig struct {
	Type      string `yaml:"type"`       // http, tcp, command
	URL       string `yaml:"url"`        // for http type
	Address   string `yaml:"address"`    // for tcp type
	Command   string `yaml:"command"`    // for command type
	StartHint string `yaml:"start_hint"` // how to start this service
	Timeout   int    `yaml:"timeout"`    // seconds (default: 2)
}

// ServiceTagConfig defines per-service tag configuration.
type ServiceTagConfig struct {
	Tags []string `yaml:"tags"`
}

// ComposeConfig holds Docker Compose settings.
type ComposeConfig struct {
	Files       []string                    `yaml:"files"`
	ProjectName string                      `yaml:"project_name"`
	Command     string                      `yaml:"command"`
	Method      string                      `yaml:"method"`
	UpOptions   []string                    `yaml:"up_options"`
	Tags        []string                    `yaml:"tags"`
	Services    map[string]ServiceTagConfig `yaml:"services"`
}

// KubectlConfig holds Kubernetes settings.
type KubectlConfig struct {
	Namespace string `yaml:"namespace"`
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
	ComposeRunOptions []string                       `yaml:"compose_run_options"`
	Subcommands       map[string]*InteractionCommand `yaml:"subcommands"`
	Tags              []string                       `yaml:"tags"`
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

// ProvisionItem represents a single item in a provision profile.
type ProvisionItem struct {
	// Step-based format
	Step string      `yaml:"step"`
	Run  any `yaml:"run"`
	Note string      `yaml:"note"`

	// Compose-aware commands (inherit compose.files and compose.project_name)
	ComposeUp   []string `yaml:"compose_up"`   // Services to start: [postgres, minio, redis]
	ComposeExec string   `yaml:"compose_exec"` // Command in service: "pg_isready -U ndstack"
	ComposeRun  string   `yaml:"compose_run"`  // One-off command in service

	// Legacy structured format
	Echo   string      `yaml:"echo"`
	Cmd    string      `yaml:"cmd"`
	ShellC string      `yaml:"shell"`
	Sleep  any `yaml:"sleep"`
	Docker any `yaml:"docker"`

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

// Load discovers and loads the dva.yml configuration.
func Load(workDir string) (*Config, error) {
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
	if cfg.Version != "" {
		if !isVersionCompatible(cfg.Version) {
			return nil, fmt.Errorf("your dva version is `%s`, but config requires minimum version `%s`. Please upgrade dva", Version, cfg.Version)
		}
	}

	// Load modules
	if len(cfg.Modules) > 0 {
		modulesDir := filepath.Join(filepath.Dir(filePath), ".dva")
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
	if cfg.Provision == nil {
		cfg.Provision = make(map[string][]ProvisionItem)
	}

	// Warn if interaction commands shadow reserved built-in commands
	WarnReservedCommandConflicts(cfg.Interaction)

	return cfg, nil
}

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
		candidate := filepath.Join(dir, "dva.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find dva.yml config (searched from %s to /)", workDir)
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
	if other.Provision != nil {
		if c.Provision == nil {
			c.Provision = make(map[string][]ProvisionItem)
		}
		for k, v := range other.Provision {
			c.Provision[k] = v
		}
	}

	// Merge compose
	if len(other.Compose.Files) > 0 {
		c.Compose.Files = other.Compose.Files
	}
	if other.Compose.ProjectName != "" {
		c.Compose.ProjectName = other.Compose.ProjectName
	}
	if other.Compose.Command != "" {
		c.Compose.Command = other.Compose.Command
	}
	if len(other.Compose.UpOptions) > 0 {
		c.Compose.UpOptions = other.Compose.UpOptions
	}

	// Merge kubectl
	if other.Kubectl.Namespace != "" {
		c.Kubectl.Namespace = other.Kubectl.Namespace
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

	// Merge infra
	if other.Infra != nil {
		if c.Infra == nil {
			c.Infra = make(map[string]InfraConfig)
		}
		for k, v := range other.Infra {
			c.Infra[k] = v
		}
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
