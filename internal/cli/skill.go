package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
	"github.com/ScriptonBasestar/dva/internal/skillinstall"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install and manage DVA skills for AI runtimes",
}

var (
	skillInstallScope    string
	skillInstallRuntimes []string
	skillStatusScope     string
	skillStatusRuntimes  []string
	skillRemoveScope     string
	skillRemoveRuntimes  []string
)

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the bundled DVA skills without an AI agent",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := skillOptions(skillInstallScope, skillInstallRuntimes, dryRun)
		if err != nil {
			return err
		}
		result, err := skillinstall.Install(options)
		if err != nil {
			return err
		}
		return printSkillResult("install", dryRun, result)
	},
}

var skillStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show installed DVA skills and detect local changes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := skillOptions(skillStatusScope, skillStatusRuntimes, false)
		if err != nil {
			return err
		}
		result, err := skillinstall.Status(options)
		if err != nil {
			return err
		}
		return printSkillResult("status", false, result)
	},
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove only unmodified DVA-owned skill installations",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := skillOptions(skillRemoveScope, skillRemoveRuntimes, dryRun)
		if err != nil {
			return err
		}
		result, err := skillinstall.Uninstall(options)
		if err != nil {
			return err
		}
		return printSkillResult("uninstall", dryRun, result)
	},
}

type skillCommandResult struct {
	Operation string                           `json:"operation"`
	DryRun    bool                             `json:"dry_run"`
	Scope     skillinstall.Scope               `json:"scope"`
	Results   []skillinstall.DestinationResult `json:"results"`
}

func init() {
	const scopeUsage = "Installation scope: user or project"
	const runtimeUsage = "Target runtime(s): claude-code, codex, opencode, grok, antigravity, agent-mesh"

	skillInstallCmd.Flags().StringVar(&skillInstallScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	skillInstallCmd.Flags().StringSliceVar(&skillInstallRuntimes, "runtime", nil, runtimeUsage)
	skillStatusCmd.Flags().StringVar(&skillStatusScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	skillStatusCmd.Flags().StringSliceVar(&skillStatusRuntimes, "runtime", nil, runtimeUsage)
	skillUninstallCmd.Flags().StringVar(&skillRemoveScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	skillUninstallCmd.Flags().StringSliceVar(&skillRemoveRuntimes, "runtime", nil, runtimeUsage)
	skillCmd.AddCommand(skillInstallCmd, skillStatusCmd, skillUninstallCmd)
}

func skillOptions(scopeValue string, runtimeValues []string, isDryRun bool) (skillinstall.Options, error) {
	scope := skillinstall.Scope(strings.TrimSpace(scopeValue))
	if scope != skillinstall.ScopeUser && scope != skillinstall.ScopeProject {
		return skillinstall.Options{}, fmt.Errorf("invalid skill scope %q; supported scopes: user, project", scopeValue)
	}

	known := make(map[string]skillinstall.Runtime)
	for _, runtime := range skillinstall.DefaultRuntimes() {
		known[string(runtime)] = runtime
	}
	seen := make(map[skillinstall.Runtime]bool)
	var runtimes []skillinstall.Runtime
	for _, value := range runtimeValues {
		for token := range strings.SplitSeq(value, ",") {
			name := strings.TrimSpace(token)
			if name == "" {
				return skillinstall.Options{}, fmt.Errorf("skill runtime list contains an empty name")
			}
			runtime, ok := known[name]
			if !ok {
				return skillinstall.Options{}, fmt.Errorf("unsupported skill runtime %q; supported runtimes: %s", name, strings.Join(runtimeNames(), ", "))
			}
			if !seen[runtime] {
				seen[runtime] = true
				runtimes = append(runtimes, runtime)
			}
		}
	}
	slices.Sort(runtimes)
	return skillinstall.Options{Scope: scope, Runtimes: runtimes, DryRun: isDryRun, Version: config.Version}, nil
}

func runtimeNames() []string {
	runtimes := skillinstall.DefaultRuntimes()
	names := make([]string, len(runtimes))
	for index, runtime := range runtimes {
		names[index] = string(runtime)
	}
	slices.Sort(names)
	return names
}

func printSkillResult(operation string, isDryRun bool, result skillinstall.Result) error {
	document := skillCommandResult{
		Operation: operation,
		DryRun:    isDryRun,
		Scope:     result.Scope,
		Results:   result.Destinations,
	}
	if jsonOutput {
		return output.PrintJSON(document)
	}
	for _, entry := range document.Results {
		fmt.Printf("%-16s %-15s %s\n", entry.Status, joinSkillRuntimes(entry.Runtimes), entry.Destination)
		if entry.Status == "partial" {
			for _, runtimeStatus := range entry.RuntimeStatuses {
				fmt.Printf("  %-14s %s\n", runtimeStatus.Runtime, runtimeStatus.Status)
			}
		}
		if entry.Detail != "" {
			fmt.Printf("  %s\n", entry.Detail)
		}
	}
	return nil
}

func joinSkillRuntimes(runtimes []skillinstall.Runtime) string {
	names := make([]string, len(runtimes))
	for index, runtime := range runtimes {
		names[index] = string(runtime)
	}
	return strings.Join(names, ",")
}
