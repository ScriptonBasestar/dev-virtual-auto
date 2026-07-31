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

// fillStaticCommandDescriptions copies each command's cobra Short into its manifest entry.
//
// Description used to be a second, hand-written string per command, and the two drifted exactly
// as two hand-maintained copies of one fact do. Of the original 13 entries, `version` was the
// only one whose text still matched its Short; the other 12 paraphrased it, and two of those had
// gone stale rather than merely reworded — `up` and `down` still described compose containers
// after plans became the primary object, so the manifest did not contain the word "plan" while
// `dva up --help` led with it.
//
// Deriving removes the class instead of correcting the 12 instances: there is now one string to
// edit per command, and `dva <cmd> --help` and the manifest cannot disagree.
//
// The two Init calls are what cobra's Execute() does before dispatching. `help` and `completion`
// are registered there rather than by an AddCommand call, so without them those two entries would
// derive an empty description. Both are idempotent, so calling them here costs nothing on the
// production path, where Execute() has already run them.
//
// An entry naming a command that does not exist keeps an empty description rather than being
// dropped, so it stays visible: TestEveryStaticCommandCarriesAType fails on the empty string and
// TestStaticCommandsCoverEveryRootCommand names it as a phantom.
func fillStaticCommandDescriptions(static map[string]ManifestCmd) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	for _, c := range rootCmd.Commands() {
		entry, ok := static[c.Name()]
		if !ok {
			continue
		}
		entry.Description = c.Short
		static[c.Name()] = entry
	}
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
		// Description is not here: fillStaticCommandDescriptions below copies it from each
		// command's own cobra Short. See that function for why (TASK-105).
		StaticCommands: map[string]ManifestCmd{
			"run": {
				Type: "dynamic_router",
				Options: map[string]string{
					"publish": "Publish container ports to host",
					"explain": "Show execution plan without running",
				},
			},
			"ls":        {Type: "query"},
			"compose":   {Type: "passthrough"},
			"up":        {Type: "compose_shortcut"},
			"down":      {Type: "compose_shortcut"},
			"stop":      {Type: "compose_shortcut"},
			"build":     {Type: "compose_shortcut"},
			"clean":     {Type: "compose_shortcut"},
			"provision": {Type: "lifecycle"},
			"validate":  {Type: "config"},
			"manifest":  {Type: "meta"},
			"ktl":       {Type: "passthrough"},
			"version":   {Type: "info"},

			"stack":   {Type: "lifecycle"},
			"app":     {Type: "lifecycle"},
			"ssh":     {Type: "lifecycle"},
			"infra":   {Type: "lifecycle"},
			"logs":    {Type: "compose_shortcut"},
			"restart": {Type: "compose_shortcut"},
			"console": {Type: "passthrough"},
			"status":  {Type: "query"},
			"show":    {Type: "query"},
			"doctor":  {Type: "query"},
			"config":  {Type: "config"},
			"init":    {Type: "config"},
			"help":    {Type: "meta"},
			// completion and help are registered by cobra inside Execute(), not by an AddCommand
			// call, so a reader grepping for AddCommand finds 25 and this table lists 27.
			"completion": {Type: "meta"},
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
	fillStaticCommandDescriptions(m.StaticCommands)
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
