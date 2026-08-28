package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ModulesDirExt is the extension for module files.
// Config represents the parsed dva.yml configuration.
type Config struct {
	Version          string                         `yaml:"version"`
	Vars             map[string]string              `yaml:"vars"`
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
	Plans            map[string]*PlanConfig         `yaml:"plans"`
	DefaultPlanName  string                         `yaml:"default_plan"`
	Sites            map[string]*SiteConfig         `yaml:"sites"`
	Ssh              SshConfig                      `yaml:"ssh"`
	DoctorChecks     []DoctorCheck                  `yaml:"checks"`
	Stack            map[string]*LifecycleEntry     `yaml:"stack"`

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
	Path        string                  `yaml:"path"`
	ExcludeTags []string                `yaml:"exclude_tags"`
	Import      *SubprojectImportConfig `yaml:"import"`
}

// PlanConfig defines a named executable plan.
type PlanConfig struct {
	Description  string            `yaml:"description"`
	Environment  string            `yaml:"environment"`
	Site         string            `yaml:"site"`
	EndpointTags []string          `yaml:"endpoint_tags"`
	Vars         map[string]string `yaml:"vars"`
	Entries      []PlanEntry       `yaml:"entries"`

	SubprojectPath string `yaml:"-"`
}

// PlanEntry is a single entry in a plan, referencing a stack declaration.
type PlanEntry struct {
	Name      string            `yaml:"name"`
	Runner    string            `yaml:"runner"`
	Order     int               `yaml:"order"`
	DependsOn []string          `yaml:"depends_on"`
	Services  []string          `yaml:"services"`
	Vars      map[string]string `yaml:"vars"`
}

// SiteConfig defines host-based execution conditions.
type SiteConfig struct {
	Description    string                        `yaml:"description"`
	Vars           map[string]string             `yaml:"vars"`
	EntryOverrides map[string]*SiteEntryOverride `yaml:"entry_overrides"`
}

// SiteEntryOverride defines site-specific overrides for a stack entry.
type SiteEntryOverride struct {
	Runner string            `yaml:"runner"`
	Vars   map[string]string `yaml:"vars"`
}

// EnvironmentV2 is the simplified environment profile (vars-only).
type EnvironmentV2 struct {
	Description string            `yaml:"description"`
	Vars        map[string]string `yaml:"vars"`
}

// SubprojectImportConfig defines what to import from a subproject.
type SubprojectImportConfig struct {
	Plans        []SubprojectImportEntry `yaml:"plans"`
	Interactions []SubprojectImportEntry `yaml:"interactions"`
	Provision    []SubprojectImportEntry `yaml:"provision"`
}

// SubprojectImportEntry represents a single import item, optionally with alias.
type SubprojectImportEntry struct {
	Name string `yaml:"name"`
	As   string `yaml:"as"`
}

// UnmarshalYAML supports both string shorthand and object forms.
func (e *SubprojectImportEntry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		e.Name = strings.TrimSpace(node.Value)
		e.As = ""
		if e.Name == "" {
			return fmt.Errorf("subproject import entry cannot be empty")
		}
		return nil
	}

	type plain SubprojectImportEntry
	if err := node.Decode((*plain)(e)); err != nil {
		return err
	}
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("subproject import entry name is required")
	}
	return nil
}

// ModeConfig defines a named operational mode for dva up (--mode/-M flag).
type ModeConfig struct {
	Description     string            `yaml:"description"`
	ComposeProfiles []string          `yaml:"compose_profiles"`
	ComposeServices *[]string         `yaml:"compose_services"` // nil=all, empty=none, items=only those
	HealthChecks    []string          `yaml:"health_checks"`
	EndpointTags    []string          `yaml:"endpoint_tags"` // filter endpoints by tags (empty=show all)
	Environment     map[string]string `yaml:"environment"`
	Provision       string            `yaml:"provision"` // provision profile to suggest on first run
	Stack           []string          `yaml:"stack"`     // stack entry names to include (empty=all)
	Build           string            `yaml:"build"`     // build strategy: "docker" (compose build), "native" (run command), or custom shell command
	Run             string            `yaml:"run"`       // run strategy: "docker" (compose up), "native" (process via health_checks.start), or custom shell command
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
	// `required` (TASK-118 opt-in strict readiness) is gone with the field. Its only
	// schema home was applications.<app>.health, and its only reader was
	// AppManager.startApp; docs/43 removed both. Top-level health_checks has always
	// rejected the key (schema.json health_checks, additionalProperties:false), so
	// keeping the struct field would leave a strictness knob nothing can turn.
	// The capability itself did not move to the plan path — see docs/43.
}

// ServiceTagConfig defines per-service tag configuration.
type ServiceTagConfig struct {
	Tags []string `yaml:"tags"`
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

	SubprojectPath string `yaml:"-"`
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

// EffectiveCommand was here, joining CommandLines with " && " "for display". It is deleted
// rather than wired up, and TASK-178 is where the reasoning lives: no runner gives a list those
// semantics. Local runs one subprocess per line, and compose and kubectl now run one exec per
// line, so `cd build` and `make` as two lines do not compose the way `cd build && make` does —
// a helper rendering them as if they did describes an execution dva does not perform. It had
// zero non-test callers for its whole life, which is much of how the gap it was written for
// stayed invisible: the handling existed on paper and nothing reached it.
//
// polymorphicCommand holds the polymorphic `command:` field — a scalar string or a sequence of
// strings — and is decoded by yaml.Decode rather than a hand-written node scan. Riding on Decode
// is what makes `command` honour merge keys (`<<:`) like every other InteractionCommand field
// (TASK-162).
type polymorphicCommand struct {
	scalar string
	lines  []string
}

// UnmarshalYAML accepts a scalar string or a sequence of strings, exposing both the single
// display form (scalar) and the list form (sequence).
func (p *polymorphicCommand) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		p.scalar = strings.TrimSpace(value.Value)
	case yaml.SequenceNode:
		var lines []string
		if err := value.Decode(&lines); err != nil {
			return fmt.Errorf("command: expected string or list of strings: %w", err)
		}
		for i, l := range lines {
			lines[i] = strings.TrimSpace(l)
		}
		p.lines = lines
		if len(lines) > 0 {
			// The first line also lands in the scalar, and the comment here used to call that
			// "display/backward-compat". Display is where it ended up: for every runner but
			// local the scalar *was* the execution, so a two-line list ran one line and the
			// plan printed one line (TASK-178). Both of those now read CommandLines. What is
			// left for the scalar is the reachability check in validate_warnings.go, which
			// asks whether an interaction declares any work at all.
			p.scalar = lines[0]
		}
	default:
		return fmt.Errorf("command: unsupported YAML type (expected string or sequence)")
	}
	return nil
}

// UnmarshalYAML implements custom unmarshaling for InteractionCommand. Every field is decoded
// normally via a plain type alias; the polymorphic `command` field goes through polymorphicCommand
// so it stays on the Decode path and honours merge keys like its neighbours (TASK-162).
func (c *InteractionCommand) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("interaction command: expected mapping node")
	}

	// Decode all fields using the tag-based alias. `command` is polymorphic (scalar or sequence),
	// so it goes through polymorphicCommand rather than a bare string — that keeps it on the Decode
	// path, which is what makes it honour merge keys like its neighbours (TASK-162). The alias has
	// no UnmarshalYAML of its own to avoid recursion.
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
		Command     polymorphicCommand             `yaml:"command"`
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
	c.Command = p.Command.scalar
	c.CommandLines = p.Command.lines
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

// MarshalYAML restores the schema shape consumed by UnmarshalYAML.
func (pc ProvisionConfig) MarshalYAML() (any, error) {
	provision := make(map[string]any, len(pc.Profiles)+1)
	if pc.DefaultProfile != "" {
		provision["default_profile"] = pc.DefaultProfile
	}
	for name, items := range pc.Profiles {
		provision[name] = items
	}
	return provision, nil
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
	Echo string `yaml:"echo"`
	Cmd  string `yaml:"cmd"`

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

// InertStepMessage is what every step runner prints in place of the work an inert item
// implied. It lives beside IsInert because the loops that print it are seven call sites
// across three packages; while the wording lived nowhere, they could not even agree on
// whether to print anything.
const InertStepMessage = "nothing ran — this item is a label with no 'run:'. Add 'run:' to execute a command, or 'note:' if it is a message."

// IgnoredParallelMessage is what the interaction step loop prints when a step asks for
// concurrency it will not get. `parallel:` is honoured on the provision path only; the
// interaction path has no scheduler, so the key parses, validates, and changes nothing.
//
// It is worded as a timing claim because that is the only symptom. An inert step announces
// itself by producing no output, which is why InertStepMessage can afford to be a
// description; a step that runs sequentially instead of concurrently produces byte-identical
// output and is simply slower, so nothing but this line tells the author the key was read
// and dropped. TASK-140.
const IgnoredParallelMessage = "'parallel:' is ignored here — interaction steps always run sequentially. It is honoured under 'provision:'."

// StepsIgnoreParallel reports whether a step list asks for concurrency the executor will not
// give it, so both executors decide to warn from one place.
//
// Two of them exist. `steps:` runs through runner.runStepLoop; `before:`/`replace:`/`after:`
// run through cli.runHookSteps, a separate loop in a package internal/runner cannot import.
// The first cut of TASK-140 put the check inline in runStepLoop only, and `dva up` with a
// parallel-marked before-hook stayed silent while `validate` warned — the exact split this
// change exists to close, since validate is the surface an author may never visit. A
// predicate rather than a bool field because the answer is derived, and rather than a copied
// three-line loop because runStepLoop's own header records what copied loops cost here.
func StepsIgnoreParallel(steps []ProvisionItem) bool {
	for _, s := range steps {
		if s.Parallel {
			return true
		}
	}
	return false
}

// IsInert reports whether this item carries no payload at all: nothing to run, nothing to
// print.
//
// `step:` is a label. Every examples/*.yml uses it that way and the runners synthesise
// "step N" when it is missing, so an item holding a label and nothing else announces work
// and performs none. Measured on 0.1.44, `- step: "make build"` in a directory with no
// Makefile printed `[hook:replace:build] [1/1] make build` and exited 0 having run nothing —
// the only signal being the absence of the `$` line that the executing form prints.
//
// Every payload field counts, not just Run. An item with compose_up, echo or cmd does
// something, and reporting it as inert would be a false positive on a working config. Raw
// needs no test of its own: RunCommands returns it.
func (p *ProvisionItem) IsInert() bool {
	return len(p.RunCommands()) == 0 &&
		p.Note == "" &&
		len(p.ComposeUp) == 0 &&
		p.ComposeExec == "" &&
		p.ComposeRun == "" &&
		p.Echo == "" &&
		p.Cmd == ""
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

// HasPlans reports whether any plans are configured.
func (c *Config) HasPlans() bool {
	return len(c.Plans) > 0
}

// DefaultPlan returns the only plan name when exactly one plan exists.
func (c *Config) DefaultPlan() string {
	// Explicit default_plan wins when it references a defined plan. A missing
	// reference falls through to "" (Validate reports it as a hard error).
	if c.DefaultPlanName != "" {
		if _, ok := c.Plans[c.DefaultPlanName]; ok {
			return c.DefaultPlanName
		}
		return ""
	}
	// Otherwise a lone plan is the implicit default.
	if len(c.Plans) != 1 {
		return ""
	}
	for name := range c.Plans {
		return name
	}
	return ""
}

// DefaultPlanSource reports why DefaultPlan selected its effective value.
//
// The value is intentionally about the resolved lifecycle behavior, not merely
// whether default_plan was declared. An invalid explicit name therefore reports
// "none": validation rejects the declaration and bare lifecycle commands have
// no effective default to select.
func (c *Config) DefaultPlanSource() string {
	if c.DefaultPlan() == "" {
		return "none"
	}
	if c.DefaultPlanName != "" {
		return "explicit"
	}
	return "implicit-single"
}

func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	maps.Copy(result, m)
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

	if !o.skipVersionCheck {
		if err := checkConfigVersion(cfg); err != nil {
			return nil, err
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
			if !o.skipVersionCheck {
				if err := checkConfigVersion(modCfg); err != nil {
					return nil, fmt.Errorf("loading module `%s`: %w", mod, err)
				}
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
		if !o.skipVersionCheck {
			if err := checkConfigVersion(overCfg); err != nil {
				return nil, fmt.Errorf("loading override: %w", err)
			}
		}
		if err := cfg.mergeFrom(overCfg); err != nil {
			return nil, fmt.Errorf("merging override: %w", err)
		}
	}

	// Apply defaults
	if cfg.Environment == nil {
		cfg.Environment = make(map[string]string)
	}
	if cfg.Vars == nil {
		cfg.Vars = make(map[string]string)
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
	if cfg.Plans == nil {
		cfg.Plans = make(map[string]*PlanConfig)
	}
	if cfg.Sites == nil {
		cfg.Sites = make(map[string]*SiteConfig)
	}

	if len(cfg.Subprojects) > 0 {
		if err := resolveSubprojectImports(cfg, opts...); err != nil {
			return nil, fmt.Errorf("resolving subprojects: %w", err)
		}
	}

	// Fold deprecated top-level infra: into stack: as source-backed entries.
	if migrated, err := cfg.migrateInfraToStack(); err != nil {
		return nil, err
	} else if len(migrated) > 0 {
		fmt.Fprintf(os.Stderr, "⚠  'infra:' is deprecated (TASK-051): migrated %s into stack: as source-backed compose entries (tag: infra).\n   Declare them under stack.<name>.source instead; 'infra:' will be removed in a future release.\n", strings.Join(migrated, ", "))
	}

	// Populate Name field and resolve deferred plugins from map keys
	for name, entry := range cfg.Stack {
		entry.Name = name
		if err := entry.ResolvePluginFromName(); err != nil {
			return nil, err
		}
		if err := validateEntrySource(name, entry, cfg.FileDir()); err != nil {
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

// decodeConfig turns raw dva.yml bytes into a Config.
//
// Every path that decodes user-supplied bytes into config types goes through here,
// because the anchor cycle scan it performs is not optional: the alternative to
// rejecting a cyclic document is a runtime stack overflow that ends the process
// (see checkAnchorCycles).
func decodeConfig(data []byte) (*Config, error) {
	// Parse to a node first: the scan must see the document before any config type
	// does, and decoding the node tree costs no second parse of the text.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	if err := checkAnchorCycles(&doc); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	cfg := &Config{}
	if doc.IsZero() {
		// Empty or comment-only input: nothing was parsed, so there is nothing to decode.
		return cfg, nil
	}
	if err := doc.Decode(cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	return cfg, nil
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return decodeConfig(data)
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
	c.Vars = mergeStringMap(c.Vars, other.Vars)

	// environment: map merge (key-level)
	c.Environment = mergeStringMap(c.Environment, other.Environment)

	if other.Plans != nil {
		if c.Plans == nil {
			c.Plans = make(map[string]*PlanConfig)
		}
		for k, v := range other.Plans {
			if existing, ok := c.Plans[k]; ok {
				c.Plans[k] = mergePlanConfig(existing, v)
			} else {
				c.Plans[k] = v
			}
		}
	}

	if other.Sites != nil {
		if c.Sites == nil {
			c.Sites = make(map[string]*SiteConfig)
		}
		for k, v := range other.Sites {
			if existing, ok := c.Sites[k]; ok {
				c.Sites[k] = mergeSiteConfig(existing, v)
			} else {
				c.Sites[k] = v
			}
		}
	}

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
		maps.Copy(c.Provision.Profiles, other.Provision.Profiles)
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
		maps.Copy(c.Infra, other.Infra)
	}

	// default_mode: scalar replace
	if other.DefaultMode != "" {
		c.DefaultMode = other.DefaultMode
	}

	// default_plan: scalar replace
	if other.DefaultPlanName != "" {
		c.DefaultPlanName = other.DefaultPlanName
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

	if other.Subprojects != nil {
		if c.Subprojects == nil {
			c.Subprojects = make(map[string]SubprojectConfig)
		}
		for k, v := range other.Subprojects {
			if existing, ok := c.Subprojects[k]; ok {
				c.Subprojects[k] = mergeSubprojectConfig(existing, v)
			} else {
				c.Subprojects[k] = v
			}
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

// checkConfigVersion refuses configs that declare a minimum version newer than
// the running DVA binary. Empty version is allowed (no gate) — see version.go.
func checkConfigVersion(cfg *Config) error {
	if cfg == nil || cfg.Version == "" {
		return nil
	}
	ok, err := isVersionCompatible(cfg.Version)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("your dva version is `%s`, but config requires minimum version `%s`. Please upgrade dva", Version, cfg.Version)
	}
	return nil
}

// isVersionCompatible reports whether the running DVA satisfies required. It returns
// an error when either version is unreadable rather than treating it as 0.0.0.
func isVersionCompatible(required string) (bool, error) {
	req, err := parseVersion(required)
	if err != nil {
		return false, fmt.Errorf("%w. Omit `version:` entirely for no compatibility gate", err)
	}
	cur, err := parseVersion(Version)
	if err != nil {
		// Version is a var set by ldflags, so an unreadable one is a build defect
		// rather than a config defect. Say which of the two is at fault, and do not
		// suggest editing `version:` — no edit to the config can fix this one.
		return false, fmt.Errorf("this dva binary reports an unreadable version: %w. Reinstall dva or rebuild it with `make build`", err)
	}
	for i := range 3 {
		if cur[i] < req[i] {
			return false, nil
		}
		if cur[i] > req[i] {
			return true, nil
		}
	}
	return true, nil
}

// versionPattern is the accepted shape of a `version:` value. The optional patch
// segment is required for backward compatibility: an unquoted `version: 0.1` is a
// YAML number that yaml.v3 coerces into the Go string "0.1", so rejecting two
// segments would break configs that load today.
//
// schema.json's `version` property carries the same rule as a `pattern`, because the
// schema runs only under `dva validate` (Config.Validate has one call site) while this
// gate runs on every Load. TestVersionPatternMatchesSchema fails if the two diverge.
var versionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)(?:\.(\d+))?$`)

// parseVersion splits a version into its three segments.
//
// It reports an error instead of returning a zero value, because the previous
// implementation discarded fmt.Sscanf's error: anything unparseable stayed [0,0,0],
// isVersionCompatible then asked whether the running DVA was at least 0.0.0, and the
// answer was always yes. That did not merely tolerate junk — it defeated the gate with
// a typo. `O.2.0` (letter O) was written to require 0.2.0 and instead required nothing,
// silently, on every command.
func parseVersion(v string) ([3]int, error) {
	var parts [3]int
	m := versionPattern.FindStringSubmatch(v)
	if m == nil {
		return parts, malformedVersionError(v)
	}
	for i, segment := range m[1:] {
		if segment == "" {
			continue // the patch group is optional and defaults to 0
		}
		n, err := strconv.Atoi(segment)
		if err != nil {
			// The pattern guarantees digits, so the only way here is overflow.
			return [3]int{}, malformedVersionError(v)
		}
		parts[i] = n
	}
	return parts, nil
}

// malformedVersionError names the offending value and the shape expected, since the
// whole point of the check is that a misspelled version is otherwise invisible.
//
// It carries no remedy on purpose. parseVersion runs on two different values — the
// config's `version:` and the binary's own Version — and the remedy differs: one is
// fixed by editing the file, the other only by rebuilding. isVersionCompatible appends
// the right one at each call site.
func malformedVersionError(v string) error {
	return fmt.Errorf("version %q is not a version: expected MAJOR.MINOR.PATCH or MAJOR.MINOR, optionally v-prefixed (e.g. %q)", v, MinScaffoldVersion)
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
