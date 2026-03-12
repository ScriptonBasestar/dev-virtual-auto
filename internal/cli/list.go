package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ScriptonBasestar/dva/internal/runner"
)

var (
	lsFormat   string
	lsDetailed bool
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List available run commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		tree := runner.NewInteractionTree(c.Interaction)
		commands := tree.List()

		// Sort keys
		keys := make([]string, 0, len(commands))
		for k := range commands {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		if jsonOutput {
			lsFormat = "json"
		}

		switch lsFormat {
		case "json":
			return printJSON(commands, keys)
		case "yaml":
			return printYAML(commands, keys)
		default:
			return printTable(commands, keys)
		}
	},
}

func init() {
	lsCmd.Flags().StringVarP(&lsFormat, "format", "f", "table", "Output format (table, json, yaml)")
	lsCmd.Flags().BoolVarP(&lsDetailed, "detailed", "d", false, "Show detailed information")
}

func printTable(commands map[string]*runner.ResolvedCommand, keys []string) error {
	// Calculate max width for alignment
	maxName := 0
	for _, k := range keys {
		if len(k) > maxName {
			maxName = len(k)
		}
	}

	for _, k := range keys {
		cmd := commands[k]
		if lsDetailed {
			runnerType := detectRunnerType(cmd)
			detail := ""
			if cmd.Service != "" {
				detail = fmt.Sprintf("service:%s", cmd.Service)
			} else if cmd.Pod != "" {
				detail = fmt.Sprintf("pod:%s", cmd.Pod)
			}
			fmt.Printf("%-*s  [%-14s]  %-20s  %s\n", maxName, k, runnerType, detail, cmd.Command)
			if cmd.Description != "" {
				fmt.Printf("%s  # %s\n", strings.Repeat(" ", maxName), cmd.Description)
			}
		} else {
			if cmd.Description != "" {
				fmt.Printf("%-*s  # %s\n", maxName, k, cmd.Description)
			} else {
				fmt.Println(k)
			}
		}
	}
	return nil
}

func printJSON(commands map[string]*runner.ResolvedCommand, keys []string) error {
	output := make(map[string]interface{})
	for _, k := range keys {
		cmd := commands[k]
		entry := map[string]interface{}{
			"command": cmd.Command,
			"runner":  detectRunnerType(cmd),
			"shell":   cmd.Shell,
		}
		if cmd.Description != "" {
			entry["description"] = cmd.Description
		}
		if cmd.Service != "" {
			entry["service"] = cmd.Service
			entry["compose_method"] = cmd.Compose.Method
		}
		if cmd.Pod != "" {
			entry["pod"] = cmd.Pod
		}
		output[k] = entry
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printYAML(commands map[string]*runner.ResolvedCommand, keys []string) error {
	output := make(map[string]interface{})
	for _, k := range keys {
		cmd := commands[k]
		entry := map[string]interface{}{
			"command": cmd.Command,
			"runner":  detectRunnerType(cmd),
			"shell":   cmd.Shell,
		}
		if cmd.Description != "" {
			entry["description"] = cmd.Description
		}
		if cmd.Service != "" {
			entry["service"] = cmd.Service
		}
		output[k] = entry
	}
	data, err := yaml.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func detectRunnerType(cmd *runner.ResolvedCommand) string {
	if cmd.RunnerName != "" {
		return cmd.RunnerName
	}
	if cmd.Service != "" {
		return "DockerCompose"
	}
	if cmd.Pod != "" {
		return "Kubectl"
	}
	return "Local"
}
