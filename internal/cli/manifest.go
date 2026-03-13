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
	DvaVersion      string                        `json:"dva_version" yaml:"dva_version"`
	SchemaVersion   string                        `json:"schema_version" yaml:"schema_version"`
	GeneratedAt     string                        `json:"generated_at" yaml:"generated_at"`
	ConfigFile      string                        `json:"config_file" yaml:"config_file"`
	ProjectDir      string                        `json:"project_dir" yaml:"project_dir"`
	ComposeFiles    []string                      `json:"compose_files,omitempty" yaml:"compose_files,omitempty"`
	EnvKeys         []string                      `json:"environment_keys,omitempty" yaml:"environment_keys,omitempty"`
	StaticCommands  map[string]ManifestCmd        `json:"static_commands" yaml:"static_commands"`
	DynamicCommands map[string]ManifestDynCmd     `json:"dynamic_commands" yaml:"dynamic_commands"`
	Runners         map[string]ManifestRunner     `json:"runners" yaml:"runners"`
	Subprojects     map[string]ManifestSubproject `json:"subprojects,omitempty" yaml:"subprojects,omitempty"`
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
}

type ManifestRunner struct {
	Trigger     string `json:"trigger" yaml:"trigger"`
	Description string `json:"description" yaml:"description"`
}

func buildManifest(c *config.Config) *Manifest {
	m := &Manifest{
		DvaVersion:    config.Version,
		SchemaVersion: "1.1",
		GeneratedAt:   time.Now().Format(time.RFC3339),
		ConfigFile:    c.FilePath(),
		ProjectDir:    c.FileDir(),
		ComposeFiles:  c.Compose.Files,
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
			"up":        {Description: "Start services (docker compose up -d --wait)", Type: "compose_shortcut"},
			"down":      {Description: "Stop and remove containers", Type: "compose_shortcut"},
			"stop":      {Description: "Stop services", Type: "compose_shortcut"},
			"build":     {Description: "Build service images", Type: "compose_shortcut"},
			"clean":     {Description: "Remove all containers/networks/volumes", Type: "compose_shortcut"},
			"provision": {Description: "Execute provision scripts", Type: "lifecycle"},
			"validate":  {Description: "Validate dva.yml schema", Type: "config"},
			"manifest":  {Description: "Output command manifest", Type: "meta"},
			"ktl":       {Description: "Run kubectl commands", Type: "passthrough"},
			"version":   {Description: "Show DVA version", Type: "info"},
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
		dynCmd := ManifestDynCmd{
			Description:  cmd.Description,
			Command:      cmd.Command,
			Runner:       runner.DetectRunnerType(cmd),
			UsageExample: fmt.Sprintf("dva %s", k),
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

	// Build subprojects section
	if len(c.Subprojects) > 0 {
		subs, err := config.LoadSubprojects(c.FileDir(), c.Subprojects)
		if err == nil {
			m.Subprojects = make(map[string]ManifestSubproject, len(subs))
			for name, subCfg := range subs {
				subManifest := ManifestSubproject{
					Path:        c.Subprojects[name].Path,
					ExcludeTags: c.Subprojects[name].ExcludeTags,
				}

				subTree := runner.NewInteractionTree(subCfg.Interaction)
				subCommands := subTree.List()
				subManifest.Commands = make(map[string]ManifestDynCmd, len(subCommands))
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
					subManifest.Commands[k] = dynCmd
				}

				m.Subprojects[name] = subManifest
			}
		}
	}

	return m
}
