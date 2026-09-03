package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/agentdeny"
	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
	"github.com/ScriptonBasestar/dva/internal/skillinstall"
)

// agentDenyCmd deploys agent-runtime deny rules for the commands DVA gates behind
// env_bridge (TASK-281 §3-6, TASK-286) — a policy layer the runtime enforces on trust,
// not a boundary DVA itself enforces. See docs/agent-deny-rules.md.
var agentDenyCmd = &cobra.Command{
	Use:   "agent-deny",
	Short: "Deploy agent-runtime deny rules for gated secret commands",
	Long: `Deploy the deny rules that stop a compliant AI coding agent from directly
invoking DVA's secret-exposing commands ('dva config env seal', 'dva config env show') —
the runtime-layer mitigation TASK-281 §3-6 hands off to, because the CLI itself can never
reliably learn that its caller is an agent. 'install' adds DVA's deny patterns to the
target runtime's settings file without disturbing anything else in it, 'status' reports
whether they are present and unmodified, and 'uninstall' removes only the unmodified
DVA-owned patterns. This is advisory policy the runtime chooses to enforce, not a DVA
security boundary — see docs/agent-deny-rules.md for the full coverage table and honest
limits.`,
}

var (
	agentDenyInstallScope   string
	agentDenyStatusScope    string
	agentDenyUninstallScope string
)

var agentDenyInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Add DVA's deny patterns to the target runtime's settings file",
	Long: `Add every gated-command deny pattern (internal/agentdeny.GatedCommands) to the
Claude Code settings file at --scope (user or project) that is not already present,
leaving every other key and every pre-existing array entry untouched.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := agentDenyOptions(agentDenyInstallScope, dryRun)
		if err != nil {
			return err
		}
		result, err := agentdeny.Install(options)
		if err != nil {
			return err
		}
		return printAgentDenyResult("install", dryRun, result)
	},
}

var agentDenyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report whether DVA's deny patterns are installed and unmodified",
	Long: `Report whether DVA's deny patterns are present and unmodified at --scope
(user or project) without writing anything: "installed" (all present), "drifted" (a
DVA-owned pattern was locally removed or edited), or "absent" (no DVA receipt).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := agentDenyOptions(agentDenyStatusScope, false)
		if err != nil {
			return err
		}
		result, err := agentdeny.Status(options)
		if err != nil {
			return err
		}
		return printAgentDenyResult("status", false, result)
	},
}

var agentDenyUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove only unmodified DVA-owned deny patterns",
	Long: `Remove DVA-owned deny patterns at --scope that still match their install
receipt exactly, refusing the whole operation if any of them were locally modified or
removed rather than guessing which are still safe to remove.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := agentDenyOptions(agentDenyUninstallScope, dryRun)
		if err != nil {
			return err
		}
		result, err := agentdeny.Uninstall(options)
		if err != nil {
			return err
		}
		return printAgentDenyResult("uninstall", dryRun, result)
	},
}

type agentDenyCommandResult struct {
	Operation string                        `json:"operation"`
	DryRun    bool                          `json:"dry_run"`
	Scope     skillinstall.Scope            `json:"scope"`
	Results   []agentdeny.DestinationResult `json:"results"`
}

func init() {
	const scopeUsage = "Installation scope: user or project"
	agentDenyInstallCmd.Flags().StringVar(&agentDenyInstallScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	agentDenyStatusCmd.Flags().StringVar(&agentDenyStatusScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	agentDenyUninstallCmd.Flags().StringVar(&agentDenyUninstallScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	agentDenyCmd.AddCommand(agentDenyInstallCmd, agentDenyStatusCmd, agentDenyUninstallCmd)
}

func agentDenyOptions(scopeValue string, isDryRun bool) (agentdeny.Options, error) {
	scope := skillinstall.Scope(strings.TrimSpace(scopeValue))
	if scope != skillinstall.ScopeUser && scope != skillinstall.ScopeProject {
		return agentdeny.Options{}, fmt.Errorf("invalid agent-deny scope %q; supported scopes: user, project", scopeValue)
	}
	return agentdeny.Options{Scope: scope, DryRun: isDryRun, Version: config.Version}, nil
}

func printAgentDenyResult(operation string, isDryRun bool, result agentdeny.Result) error {
	document := agentDenyCommandResult{
		Operation: operation,
		DryRun:    isDryRun,
		Scope:     result.Scope,
		Results:   result.Destinations,
	}
	if jsonOutput {
		return output.PrintJSON(document)
	}
	for _, entry := range document.Results {
		fmt.Printf("%-18s %-14s %s\n", entry.Status, entry.Runtime, entry.Destination)
		if entry.Detail != "" {
			fmt.Printf("  %s\n", entry.Detail)
		}
	}
	return nil
}
