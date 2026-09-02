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
	Long: `Manage the bundled DVA skills — portable how-to guides for using DVA — across AI
coding runtimes: claude-code, codex, opencode, grok, antigravity, and agent-mesh.
'install' places them at --scope (user or project), 'status' reports what is installed
and whether it was modified locally, 'uninstall' removes unmodified DVA-owned
installations, and 'backup' inspects backups retained from a prior --takeover install.`,
}

var (
	skillInstallScope                string
	skillInstallRuntimes             []string
	skillInstallTakeover             bool
	skillStatusScope                 string
	skillStatusRuntimes              []string
	skillRemoveScope                 string
	skillRemoveRuntimes              []string
	skillRemoveRestoreTakeoverBackup bool
	skillBackupListScope             string
	skillBackupListRuntimes          []string
)

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the bundled DVA skills without an AI agent",
	Long: `Install DVA's bundled skills into the target AI runtime(s) at --scope (user or
project); --takeover backs up and replaces only receipt-less DVA-name collisions instead
of leaving them alone.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := skillOptions(skillInstallScope, skillInstallRuntimes, dryRun)
		if err != nil {
			return err
		}
		options.Takeover = skillInstallTakeover
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
	Long: `Report which DVA skills are installed at --scope for the targeted --runtime(s),
and flag any whose files were modified locally since installation.`,
	Args: cobra.NoArgs,
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
	Long: `Remove DVA-owned skill installations at --scope that still match their install
receipt exactly, leaving locally modified files in place; --restore-takeover-backup
restores a verified backup from a prior 'skill install --takeover' instead — never
automatic.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := skillOptions(skillRemoveScope, skillRemoveRuntimes, dryRun)
		if err != nil {
			return err
		}
		options.RestoreTakeoverBackup = skillRemoveRestoreTakeoverBackup
		result, err := skillinstall.Uninstall(options)
		if err != nil {
			return err
		}
		return printSkillResult("uninstall", dryRun, result)
	},
}

var skillBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Inspect retained takeover backups",
	Long: `Group parent for inspecting backups retained from a prior
'skill install --takeover'; 'list' is its only subcommand.`,
}

var skillBackupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List verified retained takeover backups without changing state",
	Long: `List retained takeover backups at --scope for the targeted --runtime(s) — their
backup ID, status, runtimes, and destination — without restoring or deleting anything.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := skillOptions(skillBackupListScope, skillBackupListRuntimes, false)
		if err != nil {
			return err
		}
		result, err := skillinstall.ListTakeoverBackups(options)
		if err != nil {
			return err
		}
		return printSkillBackupList(result)
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
	skillInstallCmd.Flags().BoolVar(&skillInstallTakeover, "takeover", false, "Back up and replace only receipt-less DVA-name collisions")
	skillStatusCmd.Flags().StringVar(&skillStatusScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	skillStatusCmd.Flags().StringSliceVar(&skillStatusRuntimes, "runtime", nil, runtimeUsage)
	skillUninstallCmd.Flags().StringVar(&skillRemoveScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	skillUninstallCmd.Flags().StringSliceVar(&skillRemoveRuntimes, "runtime", nil, runtimeUsage)
	skillUninstallCmd.Flags().BoolVar(&skillRemoveRestoreTakeoverBackup, "restore-takeover-backup", false, "Restore a verified backup created by --takeover; never automatic")
	skillBackupListCmd.Flags().StringVar(&skillBackupListScope, "scope", string(skillinstall.ScopeUser), scopeUsage)
	skillBackupListCmd.Flags().StringSliceVar(&skillBackupListRuntimes, "runtime", nil, runtimeUsage)
	skillBackupCmd.AddCommand(skillBackupListCmd)
	skillCmd.AddCommand(skillInstallCmd, skillStatusCmd, skillUninstallCmd, skillBackupCmd)
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

func printSkillBackupList(result skillinstall.BackupListResult) error {
	if jsonOutput {
		return output.PrintJSON(result)
	}
	if len(result.Backups) == 0 {
		fmt.Println("no retained takeover backups")
		return nil
	}
	for _, backup := range result.Backups {
		fmt.Printf("%-32s %-10s %-24s %s\n", backup.BackupID, backup.Status, joinSkillRuntimes(backup.Runtimes), backup.Destination)
		fmt.Printf("  skills: %s\n", strings.Join(backup.Skills, ","))
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
