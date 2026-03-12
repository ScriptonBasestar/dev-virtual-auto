package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

var manifestFormat string

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Output complete command manifest (LLM-optimized)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		manifest := buildManifest(c)

		switch manifestFormat {
		case "yaml":
			data, err := yaml.Marshal(manifest)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
		default:
			data, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		}
		return nil
	},
}

func init() {
	manifestCmd.Flags().StringVarP(&manifestFormat, "format", "f", "json", "Output format (json, yaml)")
}

type Manifest struct {
	DvaVersion      string                    `json:"dva_version" yaml:"dva_version"`
	SchemaVersion   string                    `json:"schema_version" yaml:"schema_version"`
	GeneratedAt     string                    `json:"generated_at" yaml:"generated_at"`
	ConfigFile      string                    `json:"config_file" yaml:"config_file"`
	StaticCommands  map[string]ManifestCmd    `json:"static_commands" yaml:"static_commands"`
	DynamicCommands map[string]ManifestDynCmd `json:"dynamic_commands" yaml:"dynamic_commands"`
	Runners         map[string]ManifestRunner `json:"runners" yaml:"runners"`
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
}

type ManifestRunner struct {
	Trigger     string `json:"trigger" yaml:"trigger"`
	Description string `json:"description" yaml:"description"`
}

func buildManifest(c *config.Config) *Manifest {
	m := &Manifest{
		DvaVersion:    config.Version,
		SchemaVersion: "1.0",
		GeneratedAt:   time.Now().Format(time.RFC3339),
		ConfigFile:    c.FilePath(),
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
			"docker_compose": {
				Trigger:     "service key present in command config",
				Description: "Executes commands in Docker Compose services",
			},
			"kubectl": {
				Trigger:     "pod key present in command config",
				Description: "Executes commands in Kubernetes pods via kubectl exec",
			},
			"local": {
				Trigger:     "no service or pod key defined",
				Description: "Executes commands directly on the host",
			},
		},
	}

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
			Description: cmd.Description,
			Command:     cmd.Command,
			Runner:      detectRunnerType(cmd),
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

	return m
}
