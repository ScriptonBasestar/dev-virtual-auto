package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

var manifestFormat string

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Output the structured command manifest in JSON/YAML (for LLMs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		manifest := buildManifest(c)

		switch manifestFormat {
		case "yaml":
			return output.PrintYAML(manifest)
		default:
			return output.PrintJSON(manifest)
		}
	},
}

func init() {
	manifestCmd.Flags().StringVarP(&manifestFormat, "format", "f", "json", "Output format (json, yaml)")
}

type Manifest struct {
	DvaVersion      string                         `json:"dva_version" yaml:"dva_version"`
	SchemaVersion   string                         `json:"schema_version" yaml:"schema_version"`
	GeneratedAt     string                         `json:"generated_at" yaml:"generated_at"`
	ConfigFile      string                         `json:"config_file" yaml:"config_file"`
	ProjectDir      string                         `json:"project_dir" yaml:"project_dir"`
	ComposeFiles    []string                       `json:"compose_files,omitempty" yaml:"compose_files,omitempty"`
	EnvKeys         []string                       `json:"environment_keys,omitempty" yaml:"environment_keys,omitempty"`
	StaticCommands  map[string]ManifestCmd         `json:"static_commands" yaml:"static_commands"`
	DynamicCommands map[string]ManifestDynCmd      `json:"dynamic_commands" yaml:"dynamic_commands"`
	Runners         map[string]ManifestRunner      `json:"runners" yaml:"runners"`
	Plans           map[string]ManifestPlan        `json:"plans,omitempty" yaml:"plans,omitempty"`
	Subprojects     map[string]ManifestSubproject  `json:"subprojects,omitempty" yaml:"subprojects,omitempty"`
	HealthChecks    map[string]ManifestHealthCheck `json:"health_checks,omitempty" yaml:"health_checks,omitempty"`
}

// ManifestHealthCheck describes a health check in the manifest.
type ManifestHealthCheck struct {
	Type         string `json:"type" yaml:"type"`
	URL          string `json:"url,omitempty" yaml:"url,omitempty"`
	Address      string `json:"address,omitempty" yaml:"address,omitempty"`
	Command      string `json:"command,omitempty" yaml:"command,omitempty"`
	Start        string `json:"start,omitempty" yaml:"start,omitempty"`
	StartHint    string `json:"start_hint,omitempty" yaml:"start_hint,omitempty"`
	ReadyTimeout int    `json:"ready_timeout,omitempty" yaml:"ready_timeout,omitempty"`
}

type ManifestSubproject struct {
	Path        string                    `json:"path" yaml:"path"`
	ExcludeTags []string                  `json:"exclude_tags,omitempty" yaml:"exclude_tags,omitempty"`
	Commands    map[string]ManifestDynCmd `json:"commands,omitempty" yaml:"commands,omitempty"`
}

type ManifestCmd struct {
	Description string            `json:"description" yaml:"description"`
	Type        string            `json:"type" yaml:"type"`
	Options     map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

type ManifestDynCmd struct {
	Description   string `json:"description,omitempty" yaml:"description,omitempty"`
	Command       string `json:"command,omitempty" yaml:"command,omitempty"`
	Runner        string `json:"runner" yaml:"runner"`
	Service       string `json:"service,omitempty" yaml:"service,omitempty"`
	Pod           string `json:"pod,omitempty" yaml:"pod,omitempty"`
	ComposeMethod string `json:"compose_method,omitempty" yaml:"compose_method,omitempty"`
	UsageExample  string `json:"usage_example" yaml:"usage_example"`
	// ShadowedByBuiltin names the static_commands entry that runs when the bare `dva <key>`
	// form is typed. Set only when the key is shadowed, so its presence is the signal; a
	// consumer must be able to detect this without reading the description or the usage string.
	ShadowedByBuiltin string `json:"shadowed_by_builtin,omitempty" yaml:"shadowed_by_builtin,omitempty"`
}

type ManifestRunner struct {
	Trigger     string `json:"trigger" yaml:"trigger"`
	Description string `json:"description" yaml:"description"`
}

func buildManifest(c *config.Config) *Manifest {
	m := &Manifest{
		DvaVersion:    config.Version,
		SchemaVersion: "1.2",
		GeneratedAt:   time.Now().Format(time.RFC3339),
		ConfigFile:    c.FilePath(),
		ProjectDir:    c.FileDir(),
		ComposeFiles:  c.AllComposeFiles(),
		// StaticCommands must name every command registered on rootCmd — this document's own doc
		// comment says the audience is an LLM, and an agent that reads a subset concludes the
		// missing commands do not exist. It was a hand-copied 13 of 27 until TASK-096.
		//
		// It stays a literal rather than a walk over rootCmd.Commands() because Type and Options
		// have no cobra equivalent (GroupID is 5 coarse groups against 8 types here), and a walk
		// would still need this table for them. TestStaticCommandsCoverEveryRootCommand is what
		// actually stops the drift; the derivation would not have.
		//
		// The 14 added by TASK-096 take their description from the command's own Short. The
		// original 13 do not — 12 of them paraphrase it and two (`up`, `down`) predate the plan
		// concept and no longer describe what the command does. Left alone here deliberately;
		// filed as TASK-105.
		StaticCommands: map[string]ManifestCmd{
			"run": {
				Description: "Run configured command (run prefix may be omitted)",
				Type:        "dynamic_router",
				Options: map[string]string{
					"publish": "Publish container ports to host",
					"explain": "Show execution plan without running",
				},
			},
			"ls":        {Description: "List available run commands", Type: "query"},
			"compose":   {Description: "Run Docker Compose commands", Type: "passthrough"},
			"up":        {Description: "Start compose + local services (--no-wait for immediate return)", Type: "compose_shortcut"},
			"down":      {Description: "Stop and remove containers", Type: "compose_shortcut"},
			"stop":      {Description: "Stop services", Type: "compose_shortcut"},
			"build":     {Description: "Build service images", Type: "compose_shortcut"},
			"clean":     {Description: "Remove all containers/networks/volumes", Type: "compose_shortcut"},
			"provision": {Description: "Execute provision scripts", Type: "lifecycle"},
			"validate":  {Description: "Validate dva.yml schema", Type: "config"},
			"manifest":  {Description: "Output command manifest", Type: "meta"},
			"ktl":       {Description: "Run kubectl commands", Type: "passthrough"},
			"version":   {Description: "Show DVA version", Type: "info"},

			// Added by TASK-096. The 13 above are the original curated set and are left byte
			// for byte as they were; these 14 take their text from the command's Short.
			"stack":   {Description: "Manage infrastructure lifecycle (compose, helm, kubectl, ...)", Type: "lifecycle"},
			"app":     {Description: "Manage application lifecycle (ls, up, build, down, restart, log)", Type: "lifecycle"},
			"ssh":     {Description: "Manage the workspace SSH agent container", Type: "lifecycle"},
			"infra":   {Description: "Manage infrastructure services (deprecated — folded into stack, use 'dva up')", Type: "lifecycle"},
			"logs":    {Description: "View output from containers", Type: "compose_shortcut"},
			"restart": {Description: "Restart services (stop + start)", Type: "compose_shortcut"},
			"console": {Description: "Launch or inject into a DVA-integrated shell", Type: "passthrough"},
			"status":  {Description: "Display workspace status (config, lifecycle entries, services)", Type: "query"},
			"show":    {Description: "Show registered configuration summary (stack entries, plans, commands)", Type: "query"},
			"doctor":  {Description: "Check environment prerequisites and diagnose common setup issues", Type: "query"},
			"config":  {Description: "View or manage DVA configuration settings", Type: "config"},
			"init":    {Description: "Scaffold a new 'dva.yml' configuration in the current directory", Type: "config"},
			"help":    {Description: "Help about any command", Type: "meta"},
			// completion and help are registered by cobra inside Execute(), not by an AddCommand
			// call, so a reader grepping for AddCommand finds 25 and this table lists 27.
			"completion": {Description: "Generate the autocompletion script for the specified shell", Type: "meta"},
		},
		Runners: map[string]ManifestRunner{
			runner.RunnerDockerCompose: {
				Trigger:     "service key present in command config",
				Description: "Executes commands in Docker Compose services",
			},
			runner.RunnerKubectl: {
				Trigger:     "pod key present in command config",
				Description: "Executes commands in Kubernetes pods via kubectl exec",
			},
			runner.RunnerLocal: {
				Trigger:     "no service or pod key defined",
				Description: "Executes commands directly on the host",
			},
		},
	}
	m.Plans = buildManifestPlans(c)

	// Collect environment keys
	envKeys := make([]string, 0, len(c.Environment))
	for k := range c.Environment {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	m.EnvKeys = envKeys

	// Build dynamic commands from interaction tree
	tree := runner.NewInteractionTree(c.Interaction)
	commands := tree.List()
	m.DynamicCommands = make(map[string]ManifestDynCmd, len(commands))

	keys := make([]string, 0, len(commands))
	for k := range commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		cmd := commands[k]
		// usage_example carries an implicit promise that running it invokes the entry it sits
		// inside. It used to be `dva <k>` unconditionally, which for a shadowed key was the one
		// form that provably ran something else — a different command with a different
		// description, in the same document, silently.
		usage, shadowedBy := interactionUsage(c, cmd)
		dynCmd := ManifestDynCmd{
			Description:       cmd.Description,
			Command:           cmd.Command,
			Runner:            runner.DetectRunnerType(cmd),
			UsageExample:      usage,
			ShadowedByBuiltin: shadowedBy,
		}
		if cmd.Service != "" {
			dynCmd.Service = cmd.Service
			dynCmd.ComposeMethod = cmd.Compose.Method
		}
		if cmd.Pod != "" {
			dynCmd.Pod = cmd.Pod
		}
		m.DynamicCommands[k] = dynCmd
	}

	if len(c.Subprojects) > 0 {
		m.Subprojects = make(map[string]ManifestSubproject, len(c.Subprojects))
		for name, subproject := range c.Subprojects {
			subManifest := ManifestSubproject{
				Path:        subproject.Path,
				ExcludeTags: subproject.ExcludeTags,
			}

			subs, err := config.LoadSubprojects(c.FileDir(), map[string]config.SubprojectConfig{name: subproject})
			if err != nil {
				m.Subprojects[name] = subManifest
				continue
			}
			subManifest.Commands = buildManifestSubprojectCommands(name, subs[name])
			m.Subprojects[name] = subManifest
		}
	}

	// Build health checks section
	if len(c.HealthChecks) > 0 {
		m.HealthChecks = make(map[string]ManifestHealthCheck, len(c.HealthChecks))
		for name, hc := range c.HealthChecks {
			m.HealthChecks[name] = ManifestHealthCheck{
				Type:         hc.Type,
				URL:          hc.URL,
				Address:      hc.Address,
				Command:      hc.Command,
				Start:        hc.Start,
				StartHint:    hc.StartHint,
				ReadyTimeout: hc.ReadyTimeout,
			}
		}
	}

	return m
}

func buildManifestSubprojectCommands(name string, subCfg *config.Config) map[string]ManifestDynCmd {
	subTree := runner.NewInteractionTree(subCfg.Interaction)
	subCommands := subTree.List()
	commands := make(map[string]ManifestDynCmd, len(subCommands))
	for k, cmd := range subCommands {
		dynCmd := ManifestDynCmd{
			Description:  cmd.Description,
			Command:      cmd.Command,
			Runner:       runner.DetectRunnerType(cmd),
			UsageExample: fmt.Sprintf("dva %s:%s", name, k),
		}
		if cmd.Service != "" {
			dynCmd.Service = cmd.Service
			dynCmd.ComposeMethod = cmd.Compose.Method
		}
		commands[k] = dynCmd
	}
	return commands
}
